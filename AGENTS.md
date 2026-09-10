# AGENTS.md — Guía para agentes/desarrolladores de Karina

Este archivo guarda el **contexto operativo** del proyecto para que cualquier
persona o agente pueda continuar sin re-descubrir el entorno. Léelo antes de
tocar código.

## Qué es Karina

App de escritorio **local-first** para monitorear el consumo de IA de varios
proveedores en tiempo casi real. Go + Wails v2 (WebView2) + React/TS/Vite/Tailwind.
Repositorio: `https://github.com/Kevin25DC/karina-token` (usuario GitHub **Kevin25DC**).

## Entorno de esta máquina (Windows)

- **Go 1.24.4** en `C:\Users\khernandez\sdk\go` (GOROOT de usuario apunta ahí).
- **CLI de Wails v2.10.2** en `C:\Users\khernandez\go\bin\wails.exe`.
- **Node 20 / npm 11** (frontend en `frontend/`).
- `GOROOT` y `PATH` pueden no estar en una shell nueva: exportar antes de compilar:
  ```powershell
  $env:GOROOT="C:\Users\khernandez\sdk\go"
  $env:Path="C:\Users\khernandez\go\bin;C:\Users\khernandez\sdk\go\bin;"+$env:Path
  $env:GOTOOLCHAIN="local"
  ```

### ⚠️ Reglas de toolchain (importante)

- **Usar Go 1.24.x.** Con Go ≥ 1.25 el generador de bindings de Wails v2 falla
  (`internal error: package ... without types`) por su `x/tools` antiguo.
- Compilar SIEMPRE con **`wails build -skipbindings`**.
- Los bindings TS del frontend son **manuales**:
  `frontend/wailsjs/go/main/App.js` y `App.d.ts`. Al añadir/cambiar un método
  exportado en `app.go`, actualizar esos dos archivos a mano.
- `App.js` debe acceder a `window.go` **de forma diferida** (dentro de cada
  llamada). Si se evalúa al cargar el módulo, el minificador lo "hoistea" y
  aparece `Cannot read properties of undefined (reading 'main')`.

## Comandos

```powershell
# Backend
go test ./...
go vet ./...
gofmt -w ./internal ./app.go ./main.go

# Frontend
cd frontend
npm install
npm run typecheck
npm run build        # genera frontend/dist (embebido por go:embed)

# App de escritorio (desde la raíz)
wails build -skipbindings
# -> build/bin/Karina.exe

# Ejecutar la compilada
.\build\bin\Karina.exe
```

## Arquitectura (resumen)

```
main.go / app.go        Shell Wails + objeto App enlazado (bindings)
internal/domain         Modelos puros (ProviderState, UsageWindow, ...)
internal/config         config.toml (sin secretos)
internal/credentials    OS keyring (go-keyring). ClaudeOAuthAccount = "claude_subscription_oauth"
internal/storage        Historial local JSONL por día (31 días)
internal/scheduler      Polling con jitter y backoff
internal/core           Orquestación, estados, eventos, lecturas manuales/experimentales
internal/providers/api  Interfaz Provider + HTTP + errores + rate-limit headers
internal/providers/{anthropic,openai,gemini,deepseek,mock}
internal/claudesub      EXPERIMENTAL: uso de la suscripción Claude (OAuth + endpoint no oficial)
internal/updater        Chequeo de versiones contra GitHub Releases
internal/platform       Autostart (Win/mac/Linux) + bandeja del sistema (Windows)
frontend/               React/TS/Vite/Tailwind (dark, español)
docs/                   Estudio de la suscripción + estado del proyecto
```

## Proveedores y datos reales

| Proveedor | Validación | Uso global tokens | Rate limits | Billing |
| --- | --- | --- | --- | --- |
| Anthropic | `GET /v1/models` | Solo Admin key: `/v1/organizations/usage_report/messages` | headers `anthropic-ratelimit-*` | No |
| OpenAI | `GET /v1/models` | Solo Admin key: `/v1/organization/usage/completions` | headers `x-ratelimit-*` | No |
| Gemini | `GET /v1beta/models` | **No** por API | No legible | No |
| DeepSeek | `GET /models` | **No** | No documentado | `GET /user/balance` |
| Claude Suscripción | OAuth Claude Code | **Experimental** (5h + semanal) | — | — |
| Demo | — | Simulado (solo preview) | — | — |

Regla: **no inventar métricas**. Si el proveedor no expone un dato, la UI lo dice.

## Claude Suscripción (experimental) — detalles clave

- No existe API oficial para el uso de la suscripción claude.ai (Pro/Max).
- `internal/claudesub` reutiliza el **token OAuth de Claude Code** (archivo
  `~/.claude/.credentials.json` → `claudeAiOauth.accessToken`, o el keyring, o
  `KARINA_CLAUDE_TOKEN`) y llama al endpoint **no oficial**
  `GET https://api.anthropic.com/api/oauth/usage`.
- Headers: `Authorization: Bearer`, `anthropic-beta: oauth-2025-04-20`,
  `anthropic-version: 2023-06-01`, `User-Agent: claude-cli/...`.
- Respuesta: ventanas con `utilization` (0–100) en `five_hour`/`seven_day` y/o
  array `limits[]` (`percent`, `kind`), y `extra_usage`. Se **deduplican** y se
  prioriza la ventana **de sesión (5 h)**; la tarjeta muestra **ambas**.
- El **login OAuth** replica el flujo de Claude Code (`internal/claudesub/oauth.go`):
  PKCE S256, `client_id 9d1c250a-e61b-44d9-88ed-5944d1962f5e`,
  authorize `https://claude.com/cai/oauth/authorize`,
  token `https://platform.claude.com/v1/oauth/token`,
  redirect `https://platform.claude.com/oauth/code/callback` (pegar código).
- El token obtenido se guarda en el keyring (`credentials.ClaudeOAuthAccount`).
- **Throttle 5 min** y **serialización** (mutex) para evitar HTTP **409**.
- ⚠️ **Riesgo**: usar el OAuth de suscripción fuera de apps nativas puede violar
  los términos de Anthropic y conllevar suspensión. Es opt-in y avisado en la UI.
  Ver `docs/claude-subscription-study.md`.

## Trampas conocidas (ya resueltas, no reintroducir)

- **Deadlocks**: no llamar métodos que toman `s.mu` mientras ya está tomado
  (`setStatus`, `saveManual`). `saveManual` asume el lock tomado; `loadManual` lo toma.
- **409**: no lanzar lecturas concurrentes al endpoint de uso; usar `manualReadMu`.
- **`reading 'main'`**: bindings deben resolver `window.go` de forma diferida.
- **Bandeja/tray**: solo Windows por ahora (`internal/platform/tray_windows.go`).
- **Cierre de ventana** oculta a bandeja (`HideWindowOnClose`), no cierra.

## Flujo de trabajo git / releases

- Rama estable: `main`. Trabajo en ramas `feature/*` + PR.
- Versionar en `main.go` (`appVersion`) y `wails.json` (`productVersion`).
- Empaquetar: compilar, copiar `build/bin/Karina.exe` a `dist/Karina/`,
  actualizar versión en `dist/Karina/LEEME.txt` y `Manual-Usuario-Karina.html`,
  y `Compress-Archive` a `dist/Karina-Windows-vX.Y.Z.zip`.
- Publicar Release en GitHub (tag `vX.Y.Z`, label **Latest**, adjuntar el ZIP).
  El instalador remoto (`scripts/install.ps1`) y el buscador de actualizaciones
  de la app usan la última Release.
- No commitear binarios ni `dist/` (ya está en `.gitignore`).

## Reglas de seguridad

- Nunca registrar ni enviar API keys/tokens (logs, frontend, terceros).
- Las claves viven en el keyring del SO; la UI solo recibe un *preview* enmascarado.
- Las llamadas van directas de Karina al proveedor (local-first).

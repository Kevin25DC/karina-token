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
| OpenRouter | `GET /api/v1/key` | **No** (cuenta en dólares) | No | Gasto y límite de la clave; saldo de cuenta solo con Management key (`/credits`) |
| xAI | `GET /v1/api-key` (400 = clave inválida) | **No** | No | No |
| Mistral | `GET /v1/models` | **No** | Solo en inferencia | No |
| Groq | `GET /openai/v1/models` | **No** | Solo en inferencia | No |
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
- **Throttle 5 min** y **serialización** (mutex). ⚠️ Aun así puede aparecer
  **409**; ver *“409 de la suscripción”* abajo.
- ⚠️ **Riesgo**: usar el OAuth de suscripción fuera de apps nativas puede violar
  los términos de Anthropic y conllevar suspensión. Es opt-in y avisado en la UI.
  Ver `docs/claude-subscription-study.md`.

### 409 de la suscripción — causa y alternativas (resuelto en v0.7.0)

- **Causa real**: no es solo frecuencia. El token OAuth es el **mismo de Claude
  Code** y `/api/oauth/usage` está atado a esa sesión. Anthropic responde:
  - **409** = conflicto de sesión/refresh concurrente (Claude Code y Karina
    compiten por el mismo token).
  - **401** = token caducado (`claudeAiOauth.expiresAt`). Karina solo usa
    `accessToken`; **no** refrescarlo por su cuenta: rotar el refresh token
    puede **desloguear el CLI**.
  Subir el intervalo reduce el 409, pero **no lo elimina**.
- **Alternativa local sin red (recomendada; 0 llamadas, 0 409)**: agregar el
  consumo real desde los transcripts de Claude Code en
  `~/.claude/projects/**/*.jsonl` → campo `message.usage`
  (`input_tokens`, `output_tokens`, `cache_creation_input_tokens`,
  `cache_read_input_tokens`). Dato real y local; **no** da el % oficial 5h/7d.
- Otras: backoff específico de 409 (30–60 min + jitter) con caché marcada como
  desactualizada; delegar en el propio `claude`/`/usage`; medidor **manual**
  (ya existe, riesgo cero). La Admin API oficial **no** aplica a Pro/Max.
- **Decisión (2026-10-08, v0.7.0)**: resuelto así:
  - Karina usa **primero su propio token** (el del «Iniciar sesión con Claude»,
    guardado en el keyring) y solo si no existe cae al de Claude Code.
  - El token **propio** se renueva con su refresh token al caducar o ante un
    401 (`claudesub.RefreshAccessToken`). El de Claude Code **nunca** se
    refresca (sigue vigente la regla de arriba).
  - Ante **429/409** hay backoff exponencial (15 min → 2 h, o `Retry-After`) en
    `Service.manualBackoff`. No se renueva el token para saltarse un 429.

## Claude Code: consumo, costo y clientes (v0.7.0)

- `internal/transcripts` cuenta **cada respuesta una sola vez** (clave
  `message.id` + `requestId`): Claude Code escribe una línea por bloque de
  contenido repitiendo el mismo `usage`, y sin deduplicar el total sale ~×2.
- Caché en memoria por archivo (tamaño + mtime), lectura en paralelo y se
  saltan los archivos no modificados dentro del periodo.
- `internal/pricing`: tabla de precios de lista de la API por modelo (fecha en
  `pricing.AsOf`). El costo es una **estimación**; un modelo sin precio se
  marca como «sin precio», nunca se inventa. **Actualizar la tabla** cuando
  cambien los precios o salgan modelos.
- Clientes: `config.ProjectClients` (proyecto exacto) y `config.FolderClients`
  (carpeta padre). Gana el proyecto exacto y, entre carpetas, la más profunda.
  Reporte CSV en `Service.ExportClientReportCSV`.
- Asesor de plan: compara `config.SubscriptionMonthlyUSD` con el costo
  equivalente de 30 días. Solo ve Claude Code de este equipo.

## Horas, reporte y presupuestos por cliente (v0.8.0)

- Código en `internal/core/clients.go`.
- **Horas activas** (`transcripts.activeSeconds`): tiempo entre respuestas
  consecutivas de un proyecto con pausas de hasta `config.IdleGapMinutes`
  (10 por defecto). Las sesiones paralelas del mismo proyecto se fusionan; el
  total general es la suma por proyecto.
- **Reporte**: `Service.ClientReport(period)` con periodos `this_month`,
  `last_month`, `today`, `7d`, `30d` (`transcripts.ScanRange` admite `Until`).
  El PDF lo imprime el frontend (`PrintableReport`, portal oculto +
  `window.print()`); el CSV sale de `ExportClientReportCSV`.
- **Presupuestos**: `config.ClientBudgets` (USD/mes). `Service.checkBudgets`
  corre al final de cada ciclo y avisa una vez al 80 % y otra al 100 %; lo ya
  avisado se guarda en `config.BudgetAlerts` (`AAAA-MM|cliente`).
- **Ritmo de Claude Suscripción**: `config.SubscriptionIntervalSeconds`
  (300 por defecto, mínimo 120), independiente del intervalo general. Medido
  el 2026-10-08: ~20 lecturas en 5 minutos disparan un 429. No bajarlo.

## Otros agentes de código y proveedores nuevos (v0.9.0)

- La sección se llama **«Agentes de código»** (antes «Claude Code»). Los
  métodos y tipos conservan el nombre `ClaudeCode*` por compatibilidad.
- `internal/transcripts/agents.go`: lectores de **Codex CLI**
  (`$CODEX_HOME/sessions` y `archived_sessions`, uso acumulado en eventos
  `token_count`; `input_tokens` incluye los cacheados) y **Gemini CLI**
  (`~/.gemini/tmp/<proyecto>/chats/`, ruta real en `.project_root`). Hechos
  desde el código fuente de cada herramienta: **sin validar con sesiones
  reales**; sus tests usan ejemplos construidos.
- `internal/transcripts/opencode.go`: **OpenCode** guarda en SQLite
  (`~/.local/share/opencode/opencode.db`, tabla `message`). Se abre en solo
  lectura con `modernc.org/sqlite` (Go puro, sin cgo) y nunca se lee la tabla
  `part` (contenido). Verificado contra una base real de opencode 1.18. Usa
  el `cost` que calcula OpenCode; si es 0, cae a `internal/pricing`.
- `internal/pricing` solo tiene modelos de Claude: el resto sale «sin
  precio». Pendiente añadir OpenAI y Google.
- Proveedores: `internal/providers/openrouter` (dinero, no tokens) y
  `internal/providers/keyonly` (xAI, Mistral, Groq: solo validan la clave).
- Rentabilidad: `config.ClientRates` (USD/hora) y `Summary.ApplyRates`.
- ⚠️ **Tests en local**: en el equipo de la empresa el EDR marca los binarios
  de `go test` como ransomware. Verificar con el CI de GitHub, no en local.

## Widget «isla» y mascota Kari (v0.7.0)

- El modo widget es una isla que cuelga del centro superior: pastilla plegada
  que se despliega al pasar el cursor (`App.SetWidgetExpanded`,
  `App.placeIsland`). Se puede arrastrar; la posición (punto superior central)
  se guarda en `config.Widget*` y se detecta el arrastre comparando con la
  última posición que puso Karina.
- `frontend/src/components/Mascot.tsx`: **Kari**, personaje propio en SVG.
  El ánimo sale de `useMascotStatus` (mayor % de uso entre proveedores).
  No usar personajes de terceros (Clawd es de Anthropic, Mochi de Coucou).

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
- **Publicar**: subir la etiqueta `vX.Y.Z` (debe coincidir con `appVersion` y
  `productVersion`). El título y las notas de la Release salen de
  `CHANGELOG.md`: el título es lo que sigue a ` · ` en el encabezado
  `## [X.Y.Z] - fecha · Título`, y las notas son esa sección. El workflow `.github/workflows/release.yml` compila macOS
  y Windows y publica la Release con `Karina-macOS-vX.Y.Z.zip` y
  `Karina-Windows-vX.Y.Z.zip`. El instalador remoto (`scripts/install.ps1`) y
  el buscador de actualizaciones eligen el zip de su plataforma.
- **Compilar en local (Mac)**: `scripts/build.sh [mac|windows|all]` deja
  `dist/macos/Karina.app`, `dist/windows/Karina.exe` y los dos zips. Fija
  Go 1.24.4 vía `GOTOOLCHAIN`. La app de Mac lleva firma ad-hoc (sin
  Developer ID ni notarización).
- **CI** (`.github/workflows/ci.yml`, en cada push/PR): typecheck y build del
  frontend, `go vet`/`go test` de `internal/` y smoke tests de la UI con
  Playwright (`cd frontend && npm run e2e`; simulan el puente de Wails en
  `frontend/e2e/bridge.ts`, hay que ampliarlo al añadir métodos que se llamen
  al arrancar).
- No commitear binarios ni `dist/` (ya está en `.gitignore`).

## Reglas de seguridad

- Nunca registrar ni enviar API keys/tokens (logs, frontend, terceros).
- Las claves viven en el keyring del SO; la UI solo recibe un *preview* enmascarado.
- Las llamadas van directas de Karina al proveedor (local-first).

# Karina

Monitor en tiempo casi real del consumo de tokens de tus proveedores de IA.

Aplicación de escritorio **local-first** y multiplataforma (Windows / Linux / macOS)
construida con **Go + Wails v2** y un frontend **React + TypeScript + Vite + TailwindCSS**.

> Regla de oro: **datos reales, no inventados.** Karina solo muestra lo que cada
> proveedor expone oficialmente por API. Si una métrica no está disponible, la
> interfaz lo indica con claridad en lugar de fabricarla.

---

## Qué hace

- Detecta y conecta **Anthropic Claude, OpenAI, Google Gemini y DeepSeek** (más un proveedor *Demo* de desarrollo).
- Consulta cada proveedor con **polling configurable** (por defecto 30 s) con *backoff* ante errores.
- Muestra en el dashboard: estado de conexión, % y barras de **token usage** cuando el proveedor lo expone,
  **rate limits** en vivo leídos de headers, y **billing/balance** (DeepSeek).
- Guarda **historial local** con *snapshots* reales tomados en cada ciclo → gráficos por día/7/30 días.
- Almacena las API keys en el **almacén seguro del sistema** (Windows Credential Manager, macOS Keychain,
  Linux Secret Service). Nunca en archivos de configuración, logs ni el frontend.
- Arranca con el sistema si lo activas (registro `HKCU\...\Run` en Windows, LaunchAgent en macOS, XDG autostart en Linux).
- Onboarding en primera ejecución, toasts, skeletons, estados de error, atajos de teclado (`1/2/3`, `R`).

## Stack

| Capa | Tecnología |
| --- | --- |
| Backend / core | Go 1.24+, `net/http`, `context`, goroutines, `log/slog` |
| Desktop shell | [Wails v2](https://wails.io) (WebView2 / WebKit, sin Electron) |
| Frontend | React 18 · TypeScript · Vite · TailwindCSS · Zustand · lucide-react |
| Credenciales | `github.com/zalando/go-keyring` |
| Config local | TOML (sin secretos) |

## Arquitectura

```
/                       → módulo Go "karina"
├── main.go             → entrada de Wails (embebe frontend/dist)
├── app.go              → objeto App enlazado (bindings) → puente tipado al core
├── wails.json
├── cmd/                → (reservado para futura CLI headless)
├── internal/
│   ├── domain/         → modelos de dominio puros (Usage, estado, capabilities…)
│   ├── config/         → config.toml local (nunca API keys)
│   ├── credentials/    → OS keyring (preview enmascarada para la UI)
│   ├── storage/        → historial local JSONL por día (snapshots reales)
│   ├── logging/        → slog estructurado (nunca credenciales)
│   ├── scheduler/      → loop de polling con jitter y backoff exponencial
│   ├── core/           → orquestación: config+adapters+historial+eventos
│   ├── platform/       → autostart por SO (windows/darwin/linux)
│   └── providers/
│       ├── api/        → interfaz Provider, helpers HTTP, taxonomía de errores, parsing de headers
│       ├── anthropic/ openai/ gemini/ deepseek/ mock/   → adaptadores independientes
├── frontend/           → React/TS/Vite/Tailwind
│   ├── src/{components,pages,lib,store.ts}
│   └── wailsjs/        → wrappers de binding (manuales, véase nota en BUILDING)
└── build/              → icono y binarios de salida
```

El dashboard trabaja solo contra la interfaz `Provider`:

```go
type Provider interface {
    ID() string
    DisplayName() string
    Capabilities() []Capability
    Refresh(ctx context.Context, cfg Config) (domain.ProviderState, error)
}
```

## Lo que las APIs exponen de verdad (matriz de capacidades)

Investigado contra la documentación oficial actual. Karina **no inventa** métricas:

| Proveedor | Validación | Token usage global | Rate limits | Billing |
| --- | --- | --- | --- | --- |
| **Anthropic Claude** | `GET /v1/models` | Solo con **Admin API key**: `GET /v1/organizations/usage_report/messages` | Sí — headers `anthropic-ratelimit-*` | No existe endpoint de balance público |
| **OpenAI** | `GET /v1/models` | Solo con **Admin API key**: `GET /v1/organization/usage/completions` (últimos 30 días) | Sí — headers `x-ratelimit-*` | No para keys estándar |
| **Google Gemini** | `GET /v1beta/models` (clave `AIza…`) | **No disponible** por API con API key (solo consola AI Studio / Vertex AI + Cloud Monitoring) | No legible (429 server-side) | No |
| **DeepSeek** | `GET /models` | **No disponible** | No documentado | Sí — `GET /user/balance` (saldo monetario CNY/USD) |
| **Claude Suscripción (Pro/Max)** | — | **Experimental**: reutiliza el OAuth de Claude Code (endpoint no oficial) para leer la ventana de 5 h y la semanal | — | — |
| **Demo** | — | Simulado (solo desarrollo/preview) | — | — |

Consecuencia honesta de esto:

- Una key **normal** de OpenAI/Anthropic permite *validar* y ver **rate limits en vivo** (headers reales),
  pero no su consumo global: Karina lo indica con *“Usage data unavailable through API”* y explica qué
  credencial haría falta.
- DeepSeek muestra **balance monetario** (billing) —no hay barra de tokens porque no existe la API—.
- Gemini muestra estado de conexión y una explicación clara.
- El **historial** lo construye Karina localmente a partir de los snapshots que realmente puede leer;
  cuando el proveedor no expone métricas no hay filas inventadas.

Estos matices se ven en la propia UI (tarjetas, settings) y se explican por proveedor en
`frontend/src/components/ConnectSheet.tsx`.

## Requisitos

- Go **1.24.x** (recomendado para el tooling de Wails; ver BUILDING.md)
- Node.js 18+ y npm
- CLI de Wails v2: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.10.2`
- Windows: WebView2 Runtime (incluido en Windows 11 / actualizaciones recientes)
- Linux: paquetes webkit2gtk-4.1 + build-essential (véase docs de Wails)
- macOS: Xcode Command Line Tools

## Desarrollo local

```bash
# 1) dependencias de Go y del frontend
go mod tidy
(cd frontend && npm install)

# 2) ejecutar con recarga en vivo (genera/usa bindings)
wails dev

# 3) o bien: frontend suelto + `go build` de la shell
(cd frontend && npm run dev)     # http://localhost:5173
go build ./...
```

> **Nota sobre bindings:** en `frontend/wailsjs/` hay wrappers de binding **escritos a mano**
> (idénticos a los que Wails genera). Motivo: el generador de bindings de Wails v2 usa una
> versión de `golang.org/x/tools` que no puede cargar el export-data de Go ≥ 1.25.
> Compila con `wails build -skipbindings`. Si algún día cambias la API de `app.go`,
> actualiza también `frontend/wailsjs/go/main/App.js` y `App.d.ts`.

### Compilar el binario

```bash
wails build -skipbindings
# → build/bin/Karina.exe
```

### Tests

```bash
go test ./...      # debe pasar sin red ni keys (mocks + httptest)
go vet ./...
(cd frontend && npm run typecheck)
```

## Construcción por plataforma e instalador

- **Windows**: `wails build -skipbindings` produce `Karina.exe`.
  Instalador NSIS: `wails build -skipbindings -nsis` (Wails prepara el instalador y acceso directo;
  el autostart y el tray se gestionan desde la propia app). *Cross-compile* de Wails a Linux/macOS
  **no** es posible directamente desde Windows (WebView2/WebKit y empaquetado requieren el SO destino).
- **Linux**: en la máquina Linux con webkit2gtk: `wails build -skipbindings` → binario ELF;
  empaquétalo con tu gestor (`.deb`/`.rpm`/AppImage).
- **macOS**: en un Mac: `wails build -skipbindings` → `.app` (o `wails build -platform darwin` desde un Mac).

## Configuración y datos

Todo vive en el directorio de datos de usuario (Windows: `%APPDATA%\Karina`):

- `config.toml` — intervalo de refresh, providers habilitados, autostart, onboarding. **Sin API keys.**
- `history/<provider>/<aaaa-mm-dd>.jsonl` — observaciones locales (retienen 31 días).

## Seguridad

- Keys en el **keyring del SO**; la UI solo recibe una *preview* enmascarada (`sk-…abc`).
- Las llamadas van **directas** de Karina al proveedor (local-first, sin backend propio).
- Logs estructurados con `slog`; jamás se registran keys ni headers de autorización.
- Config con permisos restringidos; sin credenciales en Git (ver `.gitignore`).

## Cómo añadir un proveedor nuevo

1. Crea `internal/providers/<nuevo>/` implementando `api.Provider`
   (usa `api.GET`, `api.ErrorFromResponse` y la taxonomía de `api.ProviderError`).
2. Registra el adaptador en `internal/providers/registry.go` (catálogo + `New`).
3. Añade tema/brand y textos en el frontend (`theme.ts`, `ProviderMark.tsx`, `ConnectSheet.tsx`).
4. Añade tests con `httptest` que cubran 200/401/429/JSON inválido.
5. Documenta en la matriz del README qué expone **realmente** su API.

No declares `Capability` que el adaptador no pueda servir de verdad.

## Limitaciones reales (resumen)

- Ningún proveedor expone **consumo global de tokens** a una API key normal; solo Admin keys
  (Anthropic/OpenAI) o la consola. Karina lo refleja tal cual.
- DeepSeek y Gemini no tienen endpoint de token usage.
- El historial mide lo que Karina observa desde que se conecta el proveedor (snapshots locales),
  no consumo anterior a la instalación.
- La lectura de la **suscripción de Claude** (Pro/Max) usa un **endpoint no oficial** y el OAuth de Claude Code: es **experimental**, puede romperse y contravenir los términos de Anthropic. Es opt-in y está avisado en la app.

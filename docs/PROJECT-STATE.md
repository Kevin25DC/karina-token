# Estado del proyecto Karina

Documento de memoria: qué se construyó, decisiones, versiones y pendientes.
Última actualización: 2026-09-09 · Versión actual: **v0.3.0**.

## Resumen

Karina es una app de escritorio local-first (Go + Wails v2 + React/TS/Tailwind)
para monitorear el consumo de IA de Anthropic, OpenAI, Gemini y DeepSeek, más
un canal experimental para la **suscripción de Claude (Pro/Max)**.

Repositorio: https://github.com/Kevin25DC/karina-token

## Decisiones de arquitectura

- **Wails v2** (WebView2) en vez de Electron: menor RAM, shell Go nativo.
- Backend desacoplado del UI (sin imports de Wails en `internal/*`), testeable
  con `go test ./...` sin red ni claves.
- Interfaz `Provider` en `internal/providers/api`; adaptadores independientes.
- Credenciales en el **keyring del SO** (`zalando/go-keyring`); nunca en archivos.
- Historial local por *snapshots* reales (JSONL por día, 31 días de retención).
- Eventos core → Wails → frontend (`cycle:start`, `provider:update`, `cycle:end`,
  `mode:widget`).

## Historial de versiones

- **0.1.0** — Arquitectura, dominio, dashboard, onboarding, adaptadores reales
  (Anthropic/OpenAI/Gemini/DeepSeek), scheduler con backoff, historial y gráficos,
  autostart, README, tests.
- **0.2.0** — Renombrado a Karina, UI en español, icono propio, ventana
  frameless/translúcida, barra de título propia, modo **widget** fijo en la esquina,
  **bandeja del sistema** (Windows), paquete portable + instalador por usuario.
- **0.2.1** — Comprobador de actualizaciones contra GitHub Releases,
  `scripts/install.ps1` (instalador remoto), fix de bindings.
- **0.3.0** — Proveedor **Claude Suscripción (Pro/Max)** (experimental):
  lectura automática vía OAuth de Claude Code + endpoint no oficial, login OAuth
  por navegador, múltiples ventanas (5 h + semanal), auto-refresh con throttle,
  botón Desconectar, fixes de arranque (deadlocks) y de `window.go` diferido.

## Limitaciones reales (documentadas en la UI)

- Ningún proveedor expone consumo global de tokens a una API key normal.
- Gemini y DeepSeek no tienen endpoint de uso de tokens.
- El historial refleja lo observado desde que se conectó el proveedor.
- La suscripción de Claude usa un **endpoint no oficial**: puede romperse y su uso
  puede contravenir los términos de Anthropic (riesgo de suspensión). Opt-in.

## Pendientes / roadmap

- Bandeja del sistema en macOS/Linux (hoy solo Windows; `tray_stub.go`).
- Recordar posición/tamaño del widget y elegir proveedores mostrados.
- Instalador **NSIS** (`wails build -nsis`, requiere NSIS) además del `.cmd` por usuario.
- Auto-actualización descargando e instalando (hoy solo avisa y abre el navegador).
- Refresco automático de tokens OAuth de Claude (hoy si expira, hay que re-loguear).
- Publicación en winget/scoop (opcional).

## Estructura de release

```
dist/Karina/
  Karina.exe
  Instalar-Karina.cmd        # instalación por usuario (sin admin)
  Desinstalar-Karina.cmd
  Manual-Usuario-Karina.html # manual (imprimible a PDF)
  LEEME.txt
  logo.png
dist/Karina-Windows-vX.Y.Z.zip
```

Publicar: tag `vX.Y.Z` + Release en GitHub con label **Latest** y el ZIP adjunto.

## Enlaces

- Estudio de la suscripción: `docs/claude-subscription-study.md`
- Guía para agentes/entorno: `AGENTS.md`
- Manual de usuario: `dist/Karina/Manual-Usuario-Karina.html`

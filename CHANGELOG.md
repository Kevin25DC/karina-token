# Changelog

Todas las versiones relevantes de Karina.

## [0.4.0] - 2026-09-11

### Añadido
- **Alertas de consumo por umbral**: avisa (toast dentro de la app + intento de
  notificación nativa de Windows) cuando una ventana de uso cruza un porcentaje
  configurable (85% por defecto). Se rearma al bajar del umbral (p. ej. al
  resetearse la ventana). Activable/desactivable y ajustable desde Ajustes.

## [0.3.0] - 2026-09-09

### Añadido
- Proveedor **Claude Suscripción (Pro/Max)** (experimental): lee el uso real de la
  suscripción claude.ai (ventana de 5 h y semanal) reutilizando el OAuth de Claude Code
  y el endpoint no oficial `GET https://api.anthropic.com/api/oauth/usage`.
- **Login OAuth experimental** por navegador (PKCE + pegar código); el token se guarda
  en el almacén de credenciales del sistema.
- Tarjeta con **múltiples ventanas** (sesión 5 h + semanal), con reinicio y barra animada.
- **Actualización automática** del proveedor Claude Suscripción (con throttle de 5 min).
- Botón **Desconectar** (elimina el token OAuth guardado además del proveedor).

### Corregido
- Error `Cannot read properties of undefined (reading 'main')` (bindings `window.go`
  ahora se resuelven de forma diferida).
- Deadlocks al arrancar y al guardar lecturas.
- Manejo claro del **HTTP 409** (serialización + throttle + mensaje del servidor).
- Parser del endpoint de uso: `utilization` 0–100, `limits[]`, `extra_usage`,
  deduplicación de ventanas y descarte de claves desconocidas.

## [0.2.1] - 2026-09-09

### Añadido
- Comprobador de actualizaciones contra GitHub Releases (Ajustes → Actualizaciones).
- Instalador remoto `scripts/install.ps1` (`irm … | iex`).

### Corregido
- Bindings del frontend resueltos de forma diferida.

## [0.2.0] - 2026-09-09

### Añadido
- Renombrado de TokenPulse a **Karina**, interfaz completa en **español** e icono propio.
- Ventana **sin marco y translúcida** con barra de título propia.
- **Modo widget** (mini) fijo en una esquina, siempre visible, con las barras de consumo.
- **Bandeja del sistema** (Windows) con menú (abrir, ocultar, actualizar, widget, salir).
- Paquete portable + instalador por usuario y manual de usuario (HTML/PDF).

## [0.1.0] - 2026-09-09

### Añadido
- Arquitectura Go modular, dominio e interfaz `Provider`.
- Adaptadores reales: Anthropic, OpenAI, Gemini, DeepSeek (+ proveedor Demo).
- Dashboard con barras animadas, onboarding, historial con gráficos.
- Scheduler con jitter y backoff; almacenamiento seguro de claves en el keyring.
- Autostart, README, tests (`go test ./...`).

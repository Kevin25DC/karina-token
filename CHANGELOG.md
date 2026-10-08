# Changelog

Todas las versiones relevantes de Karina.

## [0.7.0] - 2026-10-08 · Mascota Kari, widget flotante y costo por cliente

Karina estrena mascota y un widget nuevo, y ahora dice cuánto cuesta tu uso
de Claude Code por proyecto y por cliente.

### Añadido
- **Kari, la mascota de Karina**: reacciona a tu consumo (contenta, tranquila,
  preocupada al acercarte al límite, agotada al llegar al 100 %), sigue el
  cursor con los ojos y salta si le haces clic.
- **Widget tipo «isla»**: cuelga del centro superior de la pantalla, se
  despliega al pasar el cursor y se puede arrastrar a cualquier sitio; recuerda
  dónde lo dejaste.
- **Costo estimado en dólares** del uso de Claude Code, a precios de lista de
  la API: total, por modelo, por proyecto y por día.
- **Costo por cliente**: asigna proyectos a un cliente uno a uno o por carpeta
  completa, y exporta un reporte CSV por cliente y proyecto.
- **¿Te conviene tu plan?**: compara lo que pagas por tu suscripción con lo
  que tu uso de 30 días habría costado en la API.
- **Desglose por modelo** del consumo de Claude Code.
- **Renovación automática de la sesión** de Claude Suscripción iniciada desde
  Karina, sin tocar la sesión de Claude Code.

### Cambiado
- La opción **Claude Code** del menú solo aparece si usas Claude Code en el
  equipo o añadiste un proveedor de Claude.
- La lista **Por proyecto** está paginada.
- Al conectar Claude Pro/Max queda un único campo para pegar el código.
- Ante un **429/409** de Anthropic, Karina espera cada vez más antes de
  reintentar en lugar de insistir cada 5 minutos.

### Corregido
- **Los tokens de Claude Code salían al doble**: cada respuesta se contaba una
  vez por bloque de contenido. Ahora se cuenta una sola vez.
- La pantalla de Claude Code tardaba varios segundos en cargar; ahora es casi
  instantánea.

### Notas
- El costo es una estimación; una suscripción no se cobra por token.
- El widget nuevo está probado en Windows. En macOS aún no se ha verificado.

## [0.6.1] - 2026-10-07

Primera versión publicada también para **macOS** y compilada de forma
automática (la etiqueta `v0.6.0` se creó sobre el código de la 0.5.1 y sin
binarios; esta versión la sustituye).

### Añadido
- **macOS**: app universal (Intel + Apple Silicon), con bandeja de menú,
  notificaciones nativas e iconos correctos para Dock y bandeja.
- **Claude Code**: nuevo menú con el consumo real leído de los transcripts
  locales.
- **Alertas por webhook**: las alertas de umbral se pueden enviar a Slack/Discord.
- **Releases automáticas**: GitHub Actions compila macOS y Windows al subir una
  etiqueta y publica un zip por plataforma. `scripts/build.sh` hace lo mismo en
  local.
- Smoke tests de la UI con Playwright y CI en cada push/PR.

### Corregido
- El buscador de actualizaciones y el instalador remoto eligen el zip de su
  plataforma en lugar del primero de la Release.

## [0.5.1] - 2026-09-11

### Corregido
- **Ícono de la bandeja del sistema y de la ventana/taskbar**: seguía siendo el
  logo antiguo de "TokenPulse" (previo al rebranding a Karina). Se regeneró
  `build/windows/icon.ico` a partir del logo actual y se corrigió el icono
  incrustado que usa la bandeja (antes intentaba usar un PNG plano, que la API
  de Windows rechaza silenciosamente).

## [0.5.0] - 2026-09-11

### Añadido
- **Exportar historial a CSV**: en la pestaña Historial, vuelca las observaciones
  crudas del proveedor/periodo seleccionado a un archivo CSV (diálogo nativo de
  guardar).
- **Exportar historial a PDF**: usa el diálogo de impresión de Windows con una
  vista de reporte (resumen + gráfico) para guardar como PDF vía "Microsoft
  Print to PDF".

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

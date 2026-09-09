# Estudio: lectura del uso de la SUSCRIPCIÓN de Claude (claude.ai Pro/Max)

Rama: `feature/claude-subscription` · Fecha: septiembre 2026 · Estado: estudio + prototipo manual

## Contexto y canales

Anthropic tiene dos productos distintos:

1. **API** (pago por tokens, consola/platform) → se usa con una **API key** (`sk-ant-…`).
   Karina ya lo soporta: validación, rate limits en vivo y, con clave Admin
   (`sk-ant-admin…`), el informe de uso de la organización.
2. **Suscripción claude.ai (Pro/Max)** → el usuario la usa en la web/claude.ai
   logueado. La UI muestra ventanas de uso (“5h” / “7 días”, a veces por modelo),
   pero ese dato **no se expone por ninguna API oficial** para cuentas individuales.

## Hallazgos de la investigación (con fuentes)

- **No existe endpoint oficial** de uso de suscripción para individuos.
  La API oficial de uso/costos es la **Usage & Cost Admin API** de plataforma y cubre
  el consumo **de la API de una organización**, no las suscripciones de claude.ai
  (https://platform.claude.com/docs/en/manage-claude/usage-cost-api — *“Admin API is
  unavailable for individual accounts”*). Para orgs Enterprise de claude.ai existe la
  Analytics API; no aplica a Pro/Max.
- **Claude Code** muestra el uso del plan con `/usage`, pero lo hace con credenciales
  OAuth de la suscripción contra endpoints **no documentados** (la comunidad identificó
  `GET /api/oauth/usage` en `api.anthropic.com`). Sin contrato de soporte y modificable
  en cualquier momento.
- **Política legal de Claude Code / OAuth**: Anthropic restringe que terceros ofrezcan
  login de claude.ai o recojan/almacenen/intermedien credenciales o session tokens
  (https://code.claude.com/docs/en/legal-and-compliance.md). El token OAuth está descrito
  como limitado a peticiones de modelo.
- **Consumer ToS** (https://www.anthropic.com/legal/consumer-terms):
  - §3.4: prohibido *“crawl, scrape, or otherwise harvest data”* de los Servicios.
  - §3.7: prohibido acceder por medios automatizados sin API key o permiso explícito.
- **Precedente**: durante 2026 Anthropic endureció el uso del OAuth de suscripción en
  productos de terceros y hubo reportes de baneos/enforcement (fuentes secundarias).
- **Proyectos comunitarios** que sí lo hacen (zona gris, frágiles):
  - `linuxlewis/claude-usage` — cookies de claude.ai (`sk-ant-sid`) + endpoint interno.
  - `data-wrights/claude-meter`, `klivak/claude-meter`, `jomoglobal/claude-usage-monitor`
    — reutilizan el OAuth/token que ya guarda Claude Code localmente.
  - `clauding-lab/clauge` — extensión de navegador que lee tu sesión autenticada.
  Todos se declaran *no oficiales*, *pueden romperse* y rozan los términos.

## Decisión de diseño para Karina

Principio del proyecto: **datos reales y oficiales, nunca inventados, y sin arriesgar
la cuenta del usuario.** Por eso:

| Vía | ¿La adoptamos? | Motivo |
| --- | --- | --- |
| A) **Medidor manual** del % que muestra claude.ai | ✅ Sí (prototipo en esta rama) | Cero riesgo ToS, robusto, dato real *ingresado por el usuario* y etiquetado como manual. |
| B) OAuth de Claude Code + endpoint interno | ❌ No | No documentado, frágil, en zona de enforcement 2026. |
| C) Scraping claude.ai con cookies | ❌ No | Viola Consumer ToS §3.4/§3.7 y hay anti-bot. |
| D) API oficial futura | 🗂 Roadmap | Hoy no existe; si llega (quizá estilo Analytics API), se conecta el mismo adaptador. |

## Lo construido en esta rama (prototipo)

- Proveedor especial **“Claude Suscripción (Pro/Max)”** (sin API key).
- El usuario introduce el **% de uso** que ve en claude.ai (y la ventana: p.ej.
  “ventana de 5 horas” / “semanal 7 días”) y Karina:
  - muestra la barra de consumo y “restantes” (quedan claramente marcados como **lectura manual**),
  - guarda las lecturas en su **historial local** (con gráficos como cualquier proveedor).
- No llama a ningún endpoint interno ni toca credenciales del navegador.

## Riesgos / pendientes
- La lectura es **manual** (el usuario debe actualizarla; el dato vive en claude.ai).
- Si en el futuro Anthropic publica una API oficial de uso para suscripciones,
  se sustituye la fuente manual por el adaptador real manteniendo el mismo contrato
  de datos (el resto de Karina no cambiaría).

## Referencias
- https://platform.claude.com/docs/en/manage-claude/usage-cost-api
- https://code.claude.com/docs/en/authentication
- https://code.claude.com/docs/en/legal-and-compliance.md
- https://www.anthropic.com/legal/consumer-terms
- https://github.com/linuxlewis/claude-usage
- https://github.com/data-wrights/claude-meter
- https://github.com/clauding-lab/clauge

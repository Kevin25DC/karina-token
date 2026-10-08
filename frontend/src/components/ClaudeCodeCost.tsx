import { useState } from 'react';
import { BadgeDollarSign, Download, FolderTree, Scale, Tag, Users, X } from 'lucide-react';
import { api } from '@/lib/api';
import { useStore } from '@/store';
import { cn } from '@/lib/hooks';
import { formatMoney, formatTokens } from '@/lib/format';
import { Card } from '@/components/primitives';
import type { ClaudeCodeUsageSummary, HistorySpan, UsageTokens } from '@/lib/types';

function totalOf(t: UsageTokens): number {
  return t.input_tokens + t.output_tokens + t.cache_creation_tokens + t.cache_read_tokens;
}

/** Planes habituales; el precio se puede cambiar a mano. */
const PLANS: Array<{ label: string; usd: number }> = [
  { label: 'Pro', usd: 20 },
  { label: 'Max 5x', usd: 100 },
  { label: 'Max 20x', usd: 200 },
];

/**
 * Compara lo que el usuario paga por su plan con lo que el mismo consumo de
 * Claude Code habría costado en la API durante los últimos 30 días.
 */
export function PlanAdvisor({ monthly }: { monthly: ClaudeCodeUsageSummary | null }) {
  const price = useStore((s) => s.config?.subscription_monthly_usd ?? 0);
  const reload = useStore((s) => s.reload);
  const notify = useStore((s) => s.notify);
  const [custom, setCustom] = useState('');

  async function save(usd: number) {
    try {
      await api.setSubscriptionPrice(usd);
      await reload();
      setCustom('');
    } catch (e) {
      notify('error', (e as Error).message);
    }
  }

  const equivalent = monthly?.cost_usd ?? 0;
  const activeDays = monthly?.days?.length ?? 0;
  const ratio = price > 0 ? equivalent / price : 0;
  const worthIt = price > 0 && equivalent >= price;

  return (
    <Card className="p-6">
      <h2 className="flex items-center gap-2 text-sm font-semibold text-zinc-100">
        <Scale className="h-4 w-4 text-violet-300" />
        ¿Te conviene tu plan?
      </h2>
      <p className="mt-0.5 text-xs text-zinc-500">
        Lo que pagas al mes frente a lo que tu uso de Claude Code de los últimos 30 días
        habría costado en la API.
      </p>

      <div className="mt-4 flex flex-wrap items-center gap-2">
        {PLANS.map((p) => (
          <button
            key={p.label}
            onClick={() => void save(p.usd)}
            className={cn(
              'rounded-lg border px-3 py-1.5 text-xs font-medium transition-all',
              price === p.usd
                ? 'border-violet-400/40 bg-violet-500/15 text-violet-100'
                : 'border-white/[0.07] bg-white/[0.03] text-zinc-400 hover:text-zinc-100',
            )}
          >
            {p.label} · ${p.usd}
          </button>
        ))}
        <form
          className="flex items-center gap-1.5"
          onSubmit={(e) => {
            e.preventDefault();
            const usd = Number(custom.replace(',', '.'));
            if (Number.isFinite(usd) && usd >= 0) void save(usd);
          }}
        >
          <input
            value={custom}
            onChange={(e) => setCustom(e.target.value)}
            inputMode="decimal"
            placeholder="Otro precio (USD/mes)"
            aria-label="Precio mensual del plan en USD"
            className="w-44 rounded-lg border border-white/[0.08] bg-white/[0.03] px-3 py-1.5 text-xs text-zinc-200 placeholder:text-zinc-600 outline-none focus:border-white/20"
          />
        </form>
      </div>

      {!monthly ? null : price <= 0 ? (
        <p className="mt-4 text-xs text-zinc-500">
          Elige tu plan para ver la comparación. Tu uso de 30 días equivale a{' '}
          <span className="font-mono text-zinc-300">{formatMoney(equivalent)}</span> en la API.
        </p>
      ) : (
        <div className="mt-5 grid gap-4 sm:grid-cols-3">
          <Figure label="Pagas al mes" value={formatMoney(price)} />
          <Figure label="Equivalente en API (30 días)" value={formatMoney(equivalent)} />
          <Figure
            label="Relación"
            value={`${ratio >= 10 ? Math.round(ratio) : ratio.toFixed(1)}×`}
            tone={worthIt ? 'good' : 'warn'}
          />
          <p
            className={cn(
              'sm:col-span-3 rounded-xl border p-3.5 text-xs leading-relaxed',
              worthIt
                ? 'border-emerald-400/15 bg-emerald-400/[0.06] text-emerald-100/90'
                : 'border-amber-400/15 bg-amber-400/[0.06] text-amber-100/90',
            )}
          >
            {worthIt
              ? `Tu plan te sale a cuenta: el mismo uso por API habría costado ${formatMoney(equivalent)}, unas ${ratio.toFixed(1)} veces lo que pagas.`
              : `Con este uso no amortizas el plan: por API habrías pagado ${formatMoney(equivalent)}, menos que los ${formatMoney(price)} de la suscripción. Un plan menor o pagar por API te saldría más barato.`}
          </p>
        </div>
      )}

      <p className="mt-4 text-[11px] leading-relaxed text-zinc-600">
        Estimación{monthly ? ` con ${activeDays} ${activeDays === 1 ? 'día' : 'días'} de actividad` : ''}
        : solo cuenta Claude Code en este equipo (no el chat de claude.ai ni otros equipos), a
        precios de lista de la API{monthly?.prices_as_of ? ` al ${monthly.prices_as_of}` : ''}.
        Tu uso real del plan puede ser mayor.
      </p>
    </Card>
  );
}

function Figure({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone?: 'good' | 'warn';
}) {
  return (
    <div className="rounded-xl border border-white/[0.06] bg-white/[0.02] p-4">
      <p className="text-[11px] uppercase tracking-wide text-zinc-500">{label}</p>
      <p
        className={cn(
          'mt-1 font-mono text-lg font-semibold',
          tone === 'good' ? 'text-emerald-300' : tone === 'warn' ? 'text-amber-300' : 'text-zinc-100',
        )}
      >
        {value}
      </p>
    </div>
  );
}

/** Totales por cliente del periodo seleccionado, con exportación a CSV. */
export function ClientsCard({
  summary,
  span,
  onChanged,
}: {
  summary: ClaudeCodeUsageSummary;
  span: HistorySpan;
  onChanged: () => void;
}) {
  const notify = useStore((s) => s.notify);
  const [exporting, setExporting] = useState(false);
  const clients = summary.clients ?? [];
  const hasNamed = clients.some((c) => c.name !== '');
  const maxCost = Math.max(0.0001, ...clients.map((c) => c.cost_usd));

  async function exportReport() {
    setExporting(true);
    try {
      const path = await api.exportClientReport(span);
      if (path) notify('success', `Reporte exportado a ${path}`);
    } catch (e) {
      notify('error', (e as Error).message);
    } finally {
      setExporting(false);
    }
  }

  return (
    <Card className="p-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h2 className="flex items-center gap-2 text-sm font-semibold text-zinc-100">
            <Users className="h-4 w-4 text-violet-300" />
            Por cliente
          </h2>
          <p className="mt-0.5 text-xs text-zinc-500">
            Agrupa tus proyectos por cliente o etiqueta para saber cuánto consume cada uno.
          </p>
        </div>
        <button
          onClick={() => void exportReport()}
          disabled={exporting}
          className="flex shrink-0 items-center gap-2 rounded-xl border border-white/[0.07] bg-white/[0.03] px-3 py-2 text-xs font-medium text-zinc-300 transition-all hover:bg-white/[0.07] disabled:opacity-60"
        >
          <Download className="h-3.5 w-3.5" /> Exportar reporte (CSV)
        </button>
      </div>

      {!hasNamed ? (
        <p className="mt-4 text-xs leading-relaxed text-zinc-500">
          Aún no has asignado ningún proyecto. Asigna una carpeta entera aquí abajo, o usa el
          botón <span className="text-zinc-300">Cliente</span> de cada proyecto.
        </p>
      ) : (
        <div className="mt-5 space-y-3">
          {clients.map((c) => (
            <div key={c.name || '—'}>
              <div className="flex items-center justify-between gap-3 text-sm">
                <span
                  className={cn(
                    'truncate font-medium',
                    c.name ? 'text-zinc-200' : 'text-zinc-500',
                  )}
                >
                  {c.name || 'Sin cliente'}
                </span>
                <span className="shrink-0 font-mono text-xs text-zinc-400">
                  <span className="text-zinc-200">{formatMoney(c.cost_usd)}</span> ·{' '}
                  {formatTokens(totalOf(c.tokens))} tokens · {c.projects}{' '}
                  {c.projects === 1 ? 'proyecto' : 'proyectos'}
                </span>
              </div>
              <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-white/[0.06]">
                <div
                  className="h-full rounded-full bg-gradient-to-r from-violet-400 to-sky-400"
                  style={{ width: `${Math.max(2, (c.cost_usd / maxCost) * 100)}%` }}
                />
              </div>
            </div>
          ))}
        </div>
      )}

      <FolderRules summary={summary} onChanged={onChanged} />
    </Card>
  );
}

/** Carpeta que contiene `path` (acepta separadores de Windows y de Unix). */
function parentOf(path: string): string {
  const i = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'));
  return i > 0 ? path.slice(0, i) : '';
}

/**
 * Carpetas que contienen dos o más proyectos del periodo, las más pobladas
 * primero: son las candidatas naturales a una regla.
 */
function suggestFolders(paths: string[]): Array<{ path: string; projects: number }> {
  const counts = new Map<string, { path: string; projects: number }>();
  for (const p of paths) {
    const seen = new Set<string>();
    // Padre y abuelo: cubre «cliente/proyecto» y «cliente/grupo/proyecto».
    for (let dir = parentOf(p), depth = 0; dir && depth < 2; dir = parentOf(dir), depth++) {
      const key = dir.replace(/\\/g, '/').toLowerCase();
      if (seen.has(key)) continue;
      seen.add(key);
      const entry = counts.get(key) ?? { path: dir, projects: 0 };
      entry.projects++;
      counts.set(key, entry);
    }
  }
  return [...counts.values()]
    .filter((c) => c.projects >= 2)
    .sort((a, b) => b.projects - a.projects || a.path.localeCompare(b.path))
    .slice(0, 30);
}

/** Reglas «todo lo que cuelgue de esta carpeta es de este cliente». */
function FolderRules({
  summary,
  onChanged,
}: {
  summary: ClaudeCodeUsageSummary;
  onChanged: () => void;
}) {
  const notify = useStore((s) => s.notify);
  const [folder, setFolder] = useState('');
  const [client, setClient] = useState('');
  const [saving, setSaving] = useState(false);
  const rules = summary.client_folders ?? [];
  const suggestions = suggestFolders((summary.projects ?? []).map((p) => p.path));

  async function save(path: string, name: string) {
    setSaving(true);
    try {
      await api.setFolderClient(path, name);
      if (name) {
        setFolder('');
        setClient('');
      }
      onChanged();
    } catch (e) {
      notify('error', (e as Error).message);
    } finally {
      setSaving(false);
    }
  }

  const inputClass =
    'rounded-lg border border-white/[0.08] bg-white/[0.03] px-3 py-1.5 text-xs text-zinc-200 placeholder:text-zinc-600 outline-none focus:border-white/20';

  return (
    <div className="mt-6 border-t border-white/[0.05] pt-5">
      <h3 className="flex items-center gap-2 text-xs font-semibold text-zinc-200">
        <FolderTree className="h-3.5 w-3.5 text-violet-300" />
        Asignar por carpeta
      </h3>
      <p className="mt-0.5 text-[11px] leading-relaxed text-zinc-500">
        Todos los proyectos que cuelguen de la carpeta pasan a ese cliente, también los que
        crees después. El cliente puesto a mano en un proyecto tiene prioridad.
      </p>

      <form
        className="mt-3 flex flex-wrap items-center gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          if (folder.trim() && client.trim()) void save(folder.trim(), client.trim());
        }}
      >
        <input
          value={folder}
          onChange={(e) => setFolder(e.target.value)}
          list="client-folder-suggestions"
          placeholder="Carpeta (elige una sugerida o escribe la ruta)"
          aria-label="Carpeta"
          spellCheck={false}
          className={cn(inputClass, 'min-w-0 flex-1 font-mono')}
        />
        <datalist id="client-folder-suggestions">
          {suggestions.map((s) => (
            <option key={s.path} value={s.path}>
              {s.projects} proyectos
            </option>
          ))}
        </datalist>
        <input
          value={client}
          onChange={(e) => setClient(e.target.value)}
          list="client-folder-names"
          maxLength={60}
          placeholder="Cliente"
          aria-label="Cliente de la carpeta"
          className={cn(inputClass, 'w-44')}
        />
        <datalist id="client-folder-names">
          {(summary.client_names ?? []).map((n) => (
            <option key={n} value={n} />
          ))}
        </datalist>
        <button
          type="submit"
          disabled={saving || !folder.trim() || !client.trim()}
          className="rounded-lg bg-zinc-100 px-3 py-1.5 text-xs font-semibold text-zinc-900 transition-all hover:bg-white disabled:opacity-40"
        >
          Asignar
        </button>
      </form>

      {rules.length > 0 && (
        <ul className="mt-3 space-y-1.5">
          {rules.map((r) => (
            <li
              key={r.path}
              className="flex items-center justify-between gap-3 rounded-lg border border-white/[0.05] bg-white/[0.02] px-3 py-2 text-xs"
            >
              <span className="min-w-0 truncate font-mono text-zinc-400" title={r.path}>
                {r.path}
              </span>
              <span className="flex shrink-0 items-center gap-3">
                <span className="text-violet-200">{r.client}</span>
                <span className="font-mono text-[11px] text-zinc-500">
                  {r.projects} {r.projects === 1 ? 'proyecto' : 'proyectos'}
                </span>
                <button
                  onClick={() => void save(r.path, '')}
                  disabled={saving}
                  aria-label={`Quitar la regla de ${r.path}`}
                  title="Quitar regla"
                  className="rounded-md p-1 text-zinc-500 transition-all hover:bg-white/[0.07] hover:text-zinc-200 disabled:opacity-40"
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              </span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

/** Etiqueta de cliente de un proyecto; al pulsarla se puede cambiar. */
export function ClientPicker({
  path,
  client,
  inherited,
  names,
  onChanged,
}: {
  path: string;
  client: string;
  /** El cliente viene de una regla por carpeta, no de este proyecto. */
  inherited: boolean;
  names: string[];
  onChanged: () => void;
}) {
  const notify = useStore((s) => s.notify);
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState(client);
  const listId = `clients-${path}`;

  async function commit() {
    setEditing(false);
    const next = value.trim();
    if (next === client) return;
    try {
      await api.setProjectClient(path, next);
      onChanged();
    } catch (e) {
      notify('error', (e as Error).message);
    }
  }

  if (!editing) {
    return (
      <button
        onClick={() => {
          setValue(client);
          setEditing(true);
        }}
        title={
          inherited
            ? 'Heredado de la carpeta. Pulsa para poner otro cliente solo a este proyecto.'
            : client
              ? 'Cambiar cliente'
              : 'Asignar a un cliente'
        }
        className={cn(
          'inline-flex shrink-0 items-center gap-1 rounded-md border px-1.5 py-0.5 text-[11px] transition-all',
          inherited
            ? 'border-dashed border-violet-400/30 text-violet-200/80 hover:bg-violet-500/10'
            : client
              ? 'border-violet-400/25 bg-violet-500/10 text-violet-200 hover:bg-violet-500/20'
              : 'border-white/[0.07] text-zinc-500 hover:text-zinc-200',
        )}
      >
        <Tag className="h-3 w-3" />
        {client || 'Cliente'}
      </button>
    );
  }

  return (
    <>
      <input
        autoFocus
        value={value}
        list={listId}
        maxLength={60}
        onChange={(e) => setValue(e.target.value)}
        onBlur={() => void commit()}
        onKeyDown={(e) => {
          if (e.key === 'Enter') void commit();
          if (e.key === 'Escape') setEditing(false);
        }}
        placeholder="Nombre del cliente"
        aria-label="Cliente del proyecto"
        className="w-40 shrink-0 rounded-md border border-white/[0.12] bg-white/[0.04] px-2 py-0.5 text-[11px] text-zinc-200 placeholder:text-zinc-600 outline-none focus:border-violet-400/40"
      />
      <datalist id={listId}>
        {names.map((n) => (
          <option key={n} value={n} />
        ))}
      </datalist>
    </>
  );
}

/** Tarjeta de resumen con el costo estimado del periodo. */
export function CostNote({ summary }: { summary: ClaudeCodeUsageSummary }) {
  return (
    <p className="flex items-start gap-2 text-[11px] leading-relaxed text-zinc-600">
      <BadgeDollarSign className="mt-0.5 h-3.5 w-3.5 shrink-0" />
      <span>
        Los costos son una estimación a precios de lista de la API de Anthropic al{' '}
        {summary.prices_as_of}; una suscripción no se cobra por token. Los tokens de caché
        dominan el total pero cuestan mucho menos que los de entrada y salida.
        {summary.unpriced_tokens > 0 &&
          ` ${formatTokens(summary.unpriced_tokens)} tokens son de modelos sin precio conocido y no entran en el costo.`}
      </span>
    </p>
  );
}

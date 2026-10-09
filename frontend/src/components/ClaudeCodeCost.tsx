import { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';
import {
  BadgeDollarSign,
  Clock,
  Download,
  FileText,
  FolderTree,
  HandCoins,
  Scale,
  Tag,
  Users,
  Wallet,
  X,
} from 'lucide-react';
import { api } from '@/lib/api';
import { useStore } from '@/store';
import { cn } from '@/lib/hooks';
import { formatHours, formatMoney, formatTokens } from '@/lib/format';
import { Card } from '@/components/primitives';
import type {
  ClaudeCodeClientUsage,
  ClaudeCodeUsageSummary,
  ClientBudget,
  ClientReport,
  ReportPeriod,
  UsageTokens,
} from '@/lib/types';

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

  // El plan de Claude solo se compara con el uso de Claude Code, no con el de
  // otros agentes (que tienen su propia suscripción o facturación).
  const claude = monthly?.agents?.find((a) => a.agent === 'claude');
  const equivalent = monthly?.agents ? (claude?.cost_usd ?? 0) : (monthly?.cost_usd ?? 0);
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

const REPORT_PERIODS: Array<{ id: ReportPeriod; label: string }> = [
  { id: 'this_month', label: 'Este mes' },
  { id: 'last_month', label: 'Mes pasado' },
  { id: '7d', label: 'Últimos 7 días' },
  { id: '30d', label: 'Últimos 30 días' },
  { id: 'today', label: 'Hoy' },
];

const IDLE_GAPS = [5, 10, 15, 30];

const fieldClass =
  'rounded-lg border border-white/[0.08] bg-white/[0.03] px-2.5 py-1.5 text-xs text-zinc-200 placeholder:text-zinc-600 outline-none focus:border-white/20';

/**
 * Clientes del periodo seleccionado: horas, costo y presupuesto mensual de
 * cada uno, más el reporte (PDF/CSV) y las reglas por carpeta.
 */
export function ClientsCard({
  summary,
  onChanged,
}: {
  summary: ClaudeCodeUsageSummary;
  onChanged: () => void;
}) {
  const budgets = useStore((s) => s.budgets);
  const clients = summary.clients ?? [];
  const hasNamed = clients.some((c) => c.name !== '');
  const maxCost = Math.max(0.0001, ...clients.map((c) => c.cost_usd));

  return (
    <Card className="p-6">
      <h2 className="flex items-center gap-2 text-sm font-semibold text-zinc-100">
        <Users className="h-4 w-4 text-violet-300" />
        Por cliente
      </h2>
      <p className="mt-0.5 text-xs text-zinc-500">
        Horas trabajadas y costo de IA de cada cliente en el periodo, con su presupuesto del
        mes.
      </p>

      {!hasNamed ? (
        <p className="mt-4 text-xs leading-relaxed text-zinc-500">
          Aún no has asignado ningún proyecto. Asigna una carpeta entera aquí abajo, o usa el
          botón <span className="text-zinc-300">Cliente</span> de cada proyecto.
        </p>
      ) : (
        <div className="mt-5 space-y-4">
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
                  <span className="text-zinc-200">{formatHours(c.active_seconds)}</span> ·{' '}
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
              {c.name && (
                <>
                  <RateRow client={c} onChanged={onChanged} />
                  <BudgetRow
                    client={c.name}
                    budget={budgets.find((b) => b.name === c.name)}
                  />
                </>
              )}
            </div>
          ))}
          {summary.billable_usd > 0 && (
            <p className="rounded-xl border border-white/[0.06] bg-white/[0.02] px-3 py-2.5 text-xs text-zinc-400">
              Rentabilidad del periodo (clientes con tarifa): facturable{' '}
              <span className="font-mono text-zinc-100">{formatMoney(summary.billable_usd)}</span>{' '}
              · margen tras el costo de IA{' '}
              <span
                className={cn(
                  'font-mono',
                  summary.margin_usd >= 0 ? 'text-emerald-300' : 'text-rose-300',
                )}
              >
                {formatMoney(summary.margin_usd)}
              </span>
            </p>
          )}
        </div>
      )}

      <ReportControls summary={summary} onChanged={onChanged} />
      <FolderRules summary={summary} onChanged={onChanged} />
    </Card>
  );
}

/**
 * Tarifa por hora de un cliente: con ella las horas se convierten en importe
 * facturable y se ve el margen que deja el costo de IA.
 */
function RateRow({
  client,
  onChanged,
}: {
  client: ClaudeCodeClientUsage;
  onChanged: () => void;
}) {
  const notify = useStore((s) => s.notify);
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState('');

  async function commit() {
    setEditing(false);
    const usd = value.trim() === '' ? 0 : Number(value.replace(',', '.'));
    if (!Number.isFinite(usd) || usd < 0) {
      notify('error', 'Escribe una tarifa válida en dólares');
      return;
    }
    if (usd === client.hourly_rate_usd) return;
    try {
      await api.setClientRate(client.name, usd);
      onChanged();
    } catch (e) {
      notify('error', (e as Error).message);
    }
  }

  if (editing) {
    return (
      <div className="mt-2 flex items-center gap-2 text-[11px] text-zinc-500">
        <span>Tarifa por hora (USD):</span>
        <input
          autoFocus
          value={value}
          inputMode="decimal"
          onChange={(e) => setValue(e.target.value)}
          onBlur={() => void commit()}
          onKeyDown={(e) => {
            if (e.key === 'Enter') void commit();
            if (e.key === 'Escape') setEditing(false);
          }}
          placeholder="vacío = sin tarifa"
          aria-label={`Tarifa por hora de ${client.name}`}
          className={cn(fieldClass, 'w-40 py-1')}
        />
      </div>
    );
  }

  const open = () => {
    setValue(client.hourly_rate_usd ? String(client.hourly_rate_usd) : '');
    setEditing(true);
  };

  if (!client.hourly_rate_usd) {
    return (
      <button
        onClick={open}
        className="mr-4 mt-1.5 inline-flex items-center gap-1 text-[11px] text-zinc-600 transition-colors hover:text-zinc-300"
      >
        <HandCoins className="h-3 w-3" /> Poner tarifa por hora
      </button>
    );
  }

  const marginPct =
    client.billable_usd > 0 ? Math.round((client.margin_usd / client.billable_usd) * 100) : 0;
  return (
    <button
      onClick={open}
      title="Cambiar tarifa por hora"
      className="mt-2 flex w-full items-center justify-between gap-3 rounded-lg border border-white/[0.05] bg-white/[0.02] px-2.5 py-1.5 text-left text-[11px] transition-colors hover:bg-white/[0.04]"
    >
      <span className="inline-flex items-center gap-1 text-zinc-500">
        <HandCoins className="h-3 w-3" /> Tarifa {formatMoney(client.hourly_rate_usd)}/h
      </span>
      <span className="font-mono text-zinc-400">
        facturable <span className="text-zinc-200">{formatMoney(client.billable_usd)}</span> ·
        margen{' '}
        <span className={client.margin_usd >= 0 ? 'text-emerald-300' : 'text-rose-300'}>
          {formatMoney(client.margin_usd)} ({marginPct}%)
        </span>
      </span>
    </button>
  );
}

/** Presupuesto mensual de un cliente: barra de consumo del mes y edición. */
function BudgetRow({ client, budget }: { client: string; budget?: ClientBudget }) {
  const notify = useStore((s) => s.notify);
  const reloadBudgets = useStore((s) => s.reloadBudgets);
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState('');

  async function commit() {
    setEditing(false);
    const usd = value.trim() === '' ? 0 : Number(value.replace(',', '.'));
    if (!Number.isFinite(usd) || usd < 0) {
      notify('error', 'Escribe un importe válido en dólares');
      return;
    }
    if (usd === (budget?.budget_usd ?? 0)) return;
    try {
      await api.setClientBudget(client, usd);
      await reloadBudgets();
    } catch (e) {
      notify('error', (e as Error).message);
    }
  }

  if (editing) {
    return (
      <div className="mt-2 flex items-center gap-2 text-[11px] text-zinc-500">
        <span>Presupuesto mensual (USD):</span>
        <input
          autoFocus
          value={value}
          inputMode="decimal"
          onChange={(e) => setValue(e.target.value)}
          onBlur={() => void commit()}
          onKeyDown={(e) => {
            if (e.key === 'Enter') void commit();
            if (e.key === 'Escape') setEditing(false);
          }}
          placeholder="vacío = sin presupuesto"
          aria-label={`Presupuesto mensual de ${client}`}
          className={cn(fieldClass, 'w-44 py-1')}
        />
      </div>
    );
  }

  const open = () => {
    setValue(budget ? String(budget.budget_usd) : '');
    setEditing(true);
  };

  if (!budget) {
    return (
      <button
        onClick={open}
        className="mt-1.5 inline-flex items-center gap-1 text-[11px] text-zinc-600 transition-colors hover:text-zinc-300"
      >
        <Wallet className="h-3 w-3" /> Poner presupuesto mensual
      </button>
    );
  }

  const pct = Math.round(budget.percent);
  const tone =
    budget.percent >= 100 ? 'bg-rose-400' : budget.percent >= 80 ? 'bg-amber-400' : 'bg-emerald-400';
  return (
    <button
      onClick={open}
      title="Cambiar presupuesto"
      className="mt-2 block w-full rounded-lg border border-white/[0.05] bg-white/[0.02] px-2.5 py-1.5 text-left transition-colors hover:bg-white/[0.04]"
    >
      <span className="flex items-center justify-between gap-3 text-[11px]">
        <span className="inline-flex items-center gap-1 text-zinc-500">
          <Wallet className="h-3 w-3" /> Presupuesto de este mes
        </span>
        <span
          className={cn(
            'font-mono',
            budget.percent >= 100
              ? 'text-rose-300'
              : budget.percent >= 80
                ? 'text-amber-300'
                : 'text-zinc-400',
          )}
        >
          {formatMoney(budget.spent_usd)} de {formatMoney(budget.budget_usd)} · {pct}%
        </span>
      </span>
      <span className="mt-1 block h-1 overflow-hidden rounded-full bg-white/[0.06]">
        <span
          className={cn('block h-full rounded-full', tone)}
          style={{ width: `${Math.min(100, Math.max(2, budget.percent))}%` }}
        />
      </span>
    </button>
  );
}

/** Reporte por cliente: periodo, cliente, nombre del negocio y exportación. */
function ReportControls({
  summary,
  onChanged,
}: {
  summary: ClaudeCodeUsageSummary;
  onChanged: () => void;
}) {
  const notify = useStore((s) => s.notify);
  const reload = useStore((s) => s.reload);
  const savedName = useStore((s) => s.config?.report_business_name ?? '');
  const idleGap = useStore((s) => s.config?.idle_gap_minutes ?? 10);

  const [period, setPeriod] = useState<ReportPeriod>('this_month');
  const [client, setClient] = useState('');
  const [name, setName] = useState(savedName);
  const [busy, setBusy] = useState(false);
  const [report, setReport] = useState<ClientReport | null>(null);

  useEffect(() => setName(savedName), [savedName]);

  // El reporte se pinta en un portal oculto y entonces se abre la impresión
  // (de ahí «Guardar como PDF»).
  useEffect(() => {
    if (!report) return;
    const done = () => setReport(null);
    window.addEventListener('afterprint', done, { once: true });
    const timer = window.setTimeout(() => window.print(), 80);
    return () => {
      window.clearTimeout(timer);
      window.removeEventListener('afterprint', done);
    };
  }, [report]);

  async function saveName() {
    if (name.trim() === savedName) return;
    try {
      await api.setReportBusinessName(name);
      await reload();
    } catch (e) {
      notify('error', (e as Error).message);
    }
  }

  async function exportPdf() {
    setBusy(true);
    try {
      await saveName();
      const data = await api.clientReport(period);
      const projects = (data.summary.projects ?? []).filter(
        (p) => client === '' || p.client === client,
      );
      if (projects.length === 0) {
        notify('info', 'No hay actividad de ese cliente en el periodo elegido');
        return;
      }
      setReport(data);
    } catch (e) {
      notify('error', (e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function exportCsv() {
    setBusy(true);
    try {
      const path = await api.exportClientReport(period);
      if (path) notify('success', `Reporte exportado a ${path}`);
    } catch (e) {
      notify('error', (e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function changeGap(minutes: number) {
    try {
      await api.setIdleGapMinutes(minutes);
      await reload();
      onChanged();
    } catch (e) {
      notify('error', (e as Error).message);
    }
  }

  const buttonClass =
    'flex items-center gap-2 rounded-lg border border-white/[0.07] bg-white/[0.03] px-3 py-1.5 text-xs font-medium text-zinc-300 transition-all hover:bg-white/[0.07] disabled:opacity-50';

  return (
    <div className="mt-6 border-t border-white/[0.05] pt-5">
      <h3 className="flex items-center gap-2 text-xs font-semibold text-zinc-200">
        <FileText className="h-3.5 w-3.5 text-violet-300" />
        Reporte para el cliente
      </h3>
      <p className="mt-0.5 text-[11px] leading-relaxed text-zinc-500">
        Horas trabajadas, proyectos y costo de IA del periodo. El PDF abre el diálogo de
        impresión: elige «Guardar como PDF».
      </p>

      <div className="mt-3 flex flex-wrap items-center gap-2">
        <select
          value={period}
          onChange={(e) => setPeriod(e.target.value as ReportPeriod)}
          aria-label="Periodo del reporte"
          className={fieldClass}
        >
          {REPORT_PERIODS.map((p) => (
            <option key={p.id} value={p.id} className="bg-zinc-900">
              {p.label}
            </option>
          ))}
        </select>
        <select
          value={client}
          onChange={(e) => setClient(e.target.value)}
          aria-label="Cliente del reporte"
          className={fieldClass}
        >
          <option value="" className="bg-zinc-900">
            Todos los clientes
          </option>
          {(summary.client_names ?? []).map((n) => (
            <option key={n} value={n} className="bg-zinc-900">
              {n}
            </option>
          ))}
        </select>
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          onBlur={() => void saveName()}
          maxLength={80}
          placeholder="Tu nombre o empresa (encabezado)"
          aria-label="Nombre que encabeza el reporte"
          className={cn(fieldClass, 'min-w-0 flex-1')}
        />
        <button onClick={() => void exportPdf()} disabled={busy} className={buttonClass}>
          <FileText className="h-3.5 w-3.5" /> PDF
        </button>
        <button onClick={() => void exportCsv()} disabled={busy} className={buttonClass}>
          <Download className="h-3.5 w-3.5" /> CSV
        </button>
      </div>

      <p className="mt-3 flex flex-wrap items-center gap-2 text-[11px] text-zinc-500">
        <Clock className="h-3 w-3" />
        Las horas cuentan el tiempo entre respuestas de tus agentes de código; una pausa mayor de
        <select
          value={idleGap}
          onChange={(e) => void changeGap(Number(e.target.value))}
          aria-label="Pausa que corta un tramo de trabajo"
          className={cn(fieldClass, 'px-1.5 py-0.5')}
        >
          {[...new Set([...IDLE_GAPS, idleGap])]
            .sort((a, b) => a - b)
            .map((m) => (
              <option key={m} value={m} className="bg-zinc-900">
                {m} min
              </option>
            ))}
        </select>
        corta el tramo.
      </p>

      {report && createPortal(<PrintableReport report={report} client={client} />, document.body)}
    </div>
  );
}

/** Documento imprimible (solo visible al imprimir). */
function PrintableReport({ report, client }: { report: ClientReport; client: string }) {
  const s = report.summary;
  const projects = (s.projects ?? []).filter((p) => client === '' || p.client === client);
  const groups = new Map<string, typeof projects>();
  for (const p of projects) {
    const key = p.client || 'Sin cliente';
    groups.set(key, [...(groups.get(key) ?? []), p]);
  }
  const names = [...groups.keys()].sort((a, b) =>
    a === 'Sin cliente' ? 1 : b === 'Sin cliente' ? -1 : a.localeCompare(b),
  );
  const sum = (list: typeof projects) => ({
    seconds: list.reduce((n, p) => n + p.active_seconds, 0),
    cost: list.reduce((n, p) => n + p.cost_usd, 0),
    tokens: list.reduce((n, p) => n + totalOf(p.tokens), 0),
    sessions: list.reduce((n, p) => n + p.sessions, 0),
    billable: list.reduce((n, p) => n + p.billable_usd, 0),
  });
  const total = sum(projects);
  const rateOf = (name: string) =>
    (s.clients ?? []).find((c) => c.name === name)?.hourly_rate_usd ?? 0;
  const cell = 'border-b border-zinc-200 px-2 py-1.5';

  return (
    <div className="hidden bg-white p-10 text-zinc-900 print:block">
      <div className="flex items-start justify-between gap-6 border-b-2 border-zinc-900 pb-4">
        <div>
          {report.business_name && (
            <p className="text-sm font-semibold uppercase tracking-wide text-zinc-600">
              {report.business_name}
            </p>
          )}
          <h1 className="mt-1 text-2xl font-semibold">Reporte de trabajo asistido por IA</h1>
          <p className="mt-1 text-sm text-zinc-600">
            {client ? `Cliente: ${client}` : 'Todos los clientes'} · {report.period_label}
          </p>
        </div>
        <p className="shrink-0 text-right text-xs text-zinc-500">
          Generado el
          <br />
          {new Date(report.generated_at).toLocaleDateString('es-ES', {
            day: 'numeric',
            month: 'long',
            year: 'numeric',
          })}
        </p>
      </div>

      <div className="mt-6 grid grid-cols-4 gap-4">
        {[
          ['Horas trabajadas', formatHours(total.seconds)],
          total.billable > 0
            ? ['Importe por horas', formatMoney(total.billable)]
            : ['Sesiones', String(total.sessions)],
          ['Costo de IA (estimado)', formatMoney(total.cost)],
          ['Proyectos', String(projects.length)],
        ].map(([label, value]) => (
          <div key={label} className="rounded-lg border border-zinc-300 p-3">
            <p className="text-[10px] uppercase tracking-wide text-zinc-500">{label}</p>
            <p className="mt-1 text-lg font-semibold">{value}</p>
          </div>
        ))}
      </div>

      {names.map((name) => {
        const list = groups.get(name) ?? [];
        const sub = sum(list);
        return (
          <div key={name} className="mt-8" style={{ breakInside: 'avoid' }}>
            {client === '' && <h2 className="text-base font-semibold">{name}</h2>}
            {sub.billable > 0 && (
              <p className="mt-0.5 text-xs text-zinc-600">
                Tarifa: {formatMoney(rateOf(name))} por hora
              </p>
            )}
            <table className="mt-2 w-full table-fixed border-collapse text-left text-xs">
              <thead>
                <tr className="text-[10px] uppercase tracking-wide text-zinc-500">
                  <th className={cn(cell, 'w-[34%]')}>Proyecto</th>
                  <th className={cn(cell, 'text-right')}>Sesiones</th>
                  <th className={cn(cell, 'text-right')}>Horas</th>
                  {sub.billable > 0 && <th className={cn(cell, 'text-right')}>Importe horas</th>}
                  <th className={cn(cell, 'text-right')}>Tokens</th>
                  <th className={cn(cell, 'text-right')}>Costo IA est.</th>
                </tr>
              </thead>
              <tbody>
                {list.map((p) => (
                  <tr key={p.path}>
                    <td className={cell}>{p.label}</td>
                    <td className={cn(cell, 'text-right')}>{p.sessions}</td>
                    <td className={cn(cell, 'text-right')}>{formatHours(p.active_seconds)}</td>
                    {sub.billable > 0 && (
                      <td className={cn(cell, 'text-right')}>{formatMoney(p.billable_usd)}</td>
                    )}
                    <td className={cn(cell, 'text-right')}>{formatTokens(totalOf(p.tokens))}</td>
                    <td className={cn(cell, 'text-right')}>{formatMoney(p.cost_usd)}</td>
                  </tr>
                ))}
                <tr className="font-semibold">
                  <td className={cell}>Total{client === '' ? ` ${name}` : ''}</td>
                  <td className={cn(cell, 'text-right')}>{sub.sessions}</td>
                  <td className={cn(cell, 'text-right')}>{formatHours(sub.seconds)}</td>
                  {sub.billable > 0 && (
                    <td className={cn(cell, 'text-right')}>{formatMoney(sub.billable)}</td>
                  )}
                  <td className={cn(cell, 'text-right')}>{formatTokens(sub.tokens)}</td>
                  <td className={cn(cell, 'text-right')}>{formatMoney(sub.cost)}</td>
                </tr>
              </tbody>
            </table>
          </div>
        );
      })}

      <p className="mt-8 text-[10px] leading-relaxed text-zinc-500">
        Las horas se calculan a partir de la actividad registrada por los agentes de código en el
        equipo (Claude Code, Codex CLI, Gemini CLI):
        cuentan el tiempo entre respuestas consecutivas con pausas de hasta{' '}
        {s.idle_gap_minutes} minutos. El costo es una estimación a precios de lista de la API de
        Anthropic al {s.prices_as_of} y no constituye una factura. Generado con Karina.
      </p>
    </div>
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

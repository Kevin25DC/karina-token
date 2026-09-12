import { useCallback, useEffect, useMemo, useState } from 'react';
import { createPortal } from 'react-dom';
import { BarChart3, Download, Info, Printer, TrendingUp } from 'lucide-react';
import { api } from '@/lib/api';
import { useStore } from '@/store';
import { cn } from '@/lib/hooks';
import { formatMoney, formatTokens, formatTokensFull } from '@/lib/format';
import { Button, Card, Skeleton } from '@/components/primitives';
import { ProviderMark } from '@/components/ProviderMark';
import { AreaChart, MiniLegend, formatChartAxis } from '@/components/Charts';
import type { HistoryResult, HistorySpan } from '@/lib/types';

const SPANS: Array<{ id: HistorySpan; label: string }> = [
  { id: 'today', label: 'Hoy' },
  { id: '7d', label: '7 días' },
  { id: '30d', label: '30 días' },
];

const HEX: Record<string, string> = {
  amber: '#fbbf24',
  emerald: '#34d399',
  blue: '#38bdf8',
  sky: '#22d3ee',
  violet: '#a78bfa',
};

export function History() {
  const meta = useStore((s) => s.meta);
  const cycleCount = useStore((s) => s.cycleCount);
  const setAdding = useStore((s) => s.setAdding);
  const notify = useStore((s) => s.notify);

  const usable = useMemo(() => meta.filter((m) => m.enabled), [meta]);
  const [providerId, setProviderId] = useState<string>(usable[0]?.id ?? '');
  const [span, setSpan] = useState<HistorySpan>('7d');
  const [res, setRes] = useState<HistoryResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [exporting, setExporting] = useState(false);

  useEffect(() => {
    if (usable.length && !usable.some((p) => p.id === providerId)) {
      setProviderId(usable[0].id);
    }
  }, [usable, providerId]);

  const providerMeta = meta.find((m) => m.id === providerId);

  const fetchHistory = useCallback(async () => {
    if (!providerId) return;
    setLoading(true);
    setError(null);
    try {
      setRes(await api.history(providerId, span));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, [providerId, span]);

  useEffect(() => {
    void fetchHistory();
  }, [fetchHistory, cycleCount]);

  const color = HEX[providerMeta?.brand ?? ''] ?? '#a1a1aa';
  const chartPoints = res?.points ?? [];

  async function exportCsv() {
    if (!providerId) return;
    setExporting(true);
    try {
      const path = await api.exportHistory(providerId, span);
      if (path) {
        notify('success', `Historial exportado a ${path}`);
      }
    } catch (e) {
      notify('error', (e as Error).message);
    } finally {
      setExporting(false);
    }
  }

  function exportPdf() {
    if (!chartPoints.length) return;
    window.print();
  }

  return (
    <>
    <div className="mx-auto w-full max-w-6xl px-8 pb-14">
      <header className="py-8">
        <div className="flex items-center gap-2">
          <BarChart3 className="h-5 w-5 text-zinc-400" />
          <h1 className="text-2xl font-semibold tracking-tight text-zinc-50">
            Historial de uso
          </h1>
        </div>
        <p className="mt-1 text-sm text-zinc-500">
          Observaciones locales que Karina registra de cada proveedor en cada
          consulta — sin historial inventado.
        </p>
      </header>

      {usable.length === 0 ? (
        <Card className="flex flex-col items-center px-8 py-20 text-center">
          <p className="text-sm text-zinc-400">Aún no hay proveedores conectados.</p>
          <p className="mt-1 max-w-sm text-xs leading-relaxed text-zinc-600">
            El historial se construye con las observaciones que Karina captura al
            consultar a un proveedor. Conecta uno que exponga uso o facturación
            para ver gráficos aquí.
          </p>
          <Button className="mt-6" onClick={() => setAdding(true)}>
            Añadir proveedor
          </Button>
        </Card>
      ) : (
        <div className="space-y-6">
          {/* Selección de proveedor + periodo */}
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div className="flex flex-wrap items-center gap-2">
              {usable.map((m) => (
                <button
                  key={m.id}
                  onClick={() => setProviderId(m.id)}
                  className={cn(
                    'inline-flex items-center gap-2.5 rounded-xl border px-3 py-2 text-sm font-medium transition-all',
                    m.id === providerId
                      ? 'border-white/[0.16] bg-white/[0.08] text-zinc-50'
                      : 'border-white/[0.06] bg-white/[0.02] text-zinc-400 hover:bg-white/[0.05] hover:text-zinc-200',
                  )}
                >
                  <span className="h-5 w-5 overflow-hidden rounded-md">
                    <ProviderMark id={m.id} brand={m.brand} size="sm" className="h-5 w-5" />
                  </span>
                  {m.name}
                </button>
              ))}
            </div>
            <div className="flex items-center gap-3">
              <div className="flex items-center gap-1 rounded-xl border border-white/[0.06] bg-white/[0.02] p-1">
                {SPANS.map((s) => (
                  <button
                    key={s.id}
                    onClick={() => setSpan(s.id)}
                    className={cn(
                      'rounded-lg px-3 py-1.5 text-xs font-medium transition-all',
                      s.id === span
                        ? 'bg-white/[0.1] text-zinc-100'
                        : 'text-zinc-500 hover:text-zinc-200',
                    )}
                  >
                    {s.label}
                  </button>
                ))}
              </div>
              <Button
                variant="secondary"
                className="h-9"
                loading={exporting}
                disabled={!chartPoints.length}
                onClick={() => void exportCsv()}
              >
                <Download className="h-3.5 w-3.5" /> Exportar CSV
              </Button>
              <Button
                variant="secondary"
                className="h-9"
                disabled={!chartPoints.length}
                onClick={exportPdf}
              >
                <Printer className="h-3.5 w-3.5" /> Exportar PDF
              </Button>
            </div>
          </div>

          {loading && !res ? (
            <Card className="space-y-4 p-6">
              <Skeleton className="h-5 w-40" />
              <Skeleton className="h-56 w-full" />
              <Skeleton className="h-4 w-64" />
            </Card>
          ) : error ? (
            <Card className="p-6 text-sm text-rose-300">{error}</Card>
          ) : !res || chartPoints.length === 0 ? (
            <Card className="flex flex-col items-center px-8 py-16 text-center">
              <Info className="h-6 w-6 text-zinc-600" />
              <p className="mt-3 text-sm font-medium text-zinc-300">
                Sin datos de uso registrados para este proveedor todavía
              </p>
              <p className="mt-1.5 max-w-md text-xs leading-relaxed text-zinc-500">
                {noDataHint(providerId)}
              </p>
            </Card>
          ) : (
            <>
              <SummaryBar res={res} />
              <Card className="p-6">
                <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <h2 className="flex items-center gap-2 text-sm font-semibold text-zinc-100">
                      <TrendingUp className="h-4 w-4" style={{ color }} />
                      {res.has_billing && !res.has_usage
                        ? 'Saldo prepagado'
                        : 'Tokens usados'}
                    </h2>
                    <p className="mt-0.5 text-xs text-zinc-500">
                      {res.has_billing && !res.has_usage
                        ? 'Saldo monetario reportado por el proveedor'
                        : 'Reportado por la API del proveedor — guardado localmente por Karina'}
                    </p>
                  </div>
                  <MiniLegend
                    items={[
                      {
                        label: 'Último valor por intervalo',
                        color,
                      },
                    ]}
                  />
                </div>
                <AreaChart
                  points={chartPoints}
                  value={(p) => pickMetric(p, res)}
                  color={color}
                  height={260}
                />
                <AxisLabels points={chartPoints} span={span} />
              </Card>
            </>
          )}
        </div>
      )}
    </div>

    {res &&
      chartPoints.length > 0 &&
      createPortal(
        <div className="hidden bg-white p-10 text-zinc-900 print:block">
          <h1 className="text-xl font-semibold">Karina — Historial de uso</h1>
          <p className="mt-1 text-sm text-zinc-600">
            {providerMeta?.name} · {SPANS.find((s) => s.id === span)?.label} ·
            generado el {new Date().toLocaleString('es-ES')}
          </p>
          <PrintSummary res={res} />
          <div className="mt-8">
            <AreaChart
              points={chartPoints}
              value={(p) => pickMetric(p, res)}
              color={color}
              height={260}
            />
            <AxisLabels points={chartPoints} span={span} />
          </div>
        </div>,
        document.body,
      )}
    </>
  );

  function noDataHint(id: string): string {
    if (id === 'deepseek') {
      return 'DeepSeek solo reporta saldo monetario prepagado. Los registros aparecerán cuando el saldo se lea correctamente durante las consultas.';
    }
    if (id === 'demo') {
      return 'Conecta el proveedor demo desde el panel; en unos segundos empezará a registrar uso simulado para que explores los gráficos.';
    }
    return 'Este proveedor no expone un endpoint de uso de tokens para esta clave, por lo que no hay historial que capturar. Si conectas una clave Admin que reporte uso, los gráficos se poblarán automáticamente.';
  }
}

function pickMetric(
  p: { used_tokens?: number; limit_tokens?: number; balance_total?: number },
  res: HistoryResult,
): number | undefined {
  if (res.has_billing && !res.has_usage) {
    return p.balance_total !== undefined ? p.balance_total : undefined;
  }
  return p.used_tokens !== undefined ? p.used_tokens : undefined;
}

function summaryStats(res: HistoryResult): Array<{ label: string; value: string }> {
  const pts = res.points;
  const values = pts
    .map((p) => pickMetric(p, res))
    .filter((v): v is number => v !== undefined);
  const last = values.length ? values[values.length - 1] : 0;
  const peak = values.length ? Math.max(...values) : 0;
  const isMoney = res.has_billing && !res.has_usage;
  const lastPt = pts[pts.length - 1];

  return [
    {
      label: isMoney ? 'Saldo (último)' : 'Tokens usados',
      value: isMoney
        ? formatMoney(last, lastPt?.balance_currency)
        : `${formatTokensFull(last)} tokens`,
    },
    {
      label: isMoney ? 'Saldo máximo' : 'Pico',
      value: isMoney
        ? formatMoney(peak, lastPt?.balance_currency)
        : `${formatTokens(peak)} tokens`,
    },
    { label: 'Observaciones', value: String(values.length) },
  ];
}

function SummaryBar({ res }: { res: HistoryResult }) {
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
      {summaryStats(res).map((s) => (
        <Card key={s.label} className="p-4">
          <p className="text-[11px] uppercase tracking-wide text-zinc-500">{s.label}</p>
          <p className="mt-1 truncate font-mono text-lg font-semibold text-zinc-100">
            {s.value}
          </p>
        </Card>
      ))}
    </div>
  );
}

function PrintSummary({ res }: { res: HistoryResult }) {
  return (
    <div className="mt-6 grid grid-cols-3 gap-6">
      {summaryStats(res).map((s) => (
        <div key={s.label}>
          <p className="text-[11px] uppercase tracking-wide text-zinc-500">{s.label}</p>
          <p className="mt-1 font-mono text-lg font-semibold text-zinc-900">{s.value}</p>
        </div>
      ))}
    </div>
  );
}

function AxisLabels({ points, span }: { points: { at: string }[]; span: string }) {
  if (points.length < 2) return null;
  const step = Math.max(1, Math.floor(points.length / 6));
  const chosen = points.filter((_, i) => i % step === 0);
  return (
    <div className="mt-2 flex justify-between font-mono text-[10px] text-zinc-600">
      {chosen.map((p, i) => (
        <span key={i}>{formatChartAxis(p.at, span)}</span>
      ))}
    </div>
  );
}

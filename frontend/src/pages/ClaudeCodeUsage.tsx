import { useCallback, useEffect, useState } from 'react';
import { FolderGit2, Info, Terminal } from 'lucide-react';
import { api } from '@/lib/api';
import { useStore } from '@/store';
import { cn } from '@/lib/hooks';
import { formatTokens, formatTokensFull } from '@/lib/format';
import { Card, Skeleton } from '@/components/primitives';
import type { ClaudeCodeUsageSummary, HistorySpan } from '@/lib/types';

const SPANS: Array<{ id: HistorySpan; label: string }> = [
  { id: 'today', label: 'Hoy' },
  { id: '7d', label: '7 días' },
  { id: '30d', label: '30 días' },
];

export function ClaudeCodeUsage() {
  const cycleCount = useStore((s) => s.cycleCount);
  const [span, setSpan] = useState<HistorySpan>('7d');
  const [summary, setSummary] = useState<ClaudeCodeUsageSummary | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchUsage = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setSummary(await api.claudeCodeUsage(span));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, [span]);

  useEffect(() => {
    void fetchUsage();
    // cycleCount ties this to Karina's own refresh rhythm, but a manual
    // reload on span change is what actually matters here — the polling
    // loop never touches these files.
  }, [fetchUsage, cycleCount]);

  const projects = summary?.projects ?? [];
  const days = summary?.days ?? [];
  const maxProjectTotal = Math.max(1, ...projects.map((p) => totalOf(p.tokens)));
  const maxDayTotal = Math.max(1, ...days.map((d) => totalOf(d.tokens)));

  return (
    <div className="mx-auto w-full max-w-6xl px-8 pb-14">
      <header className="py-8">
        <div className="flex items-center gap-2">
          <Terminal className="h-5 w-5 text-zinc-400" />
          <h1 className="text-2xl font-semibold tracking-tight text-zinc-50">Claude Code</h1>
        </div>
        <p className="mt-1 text-sm text-zinc-500">
          Consumo real leído directo de tus transcripts locales (
          <code className="rounded bg-white/[0.06] px-1 py-0.5 text-[11px]">
            ~/.claude/projects
          </code>
          ) — sin depender de ninguna API ni endpoint Admin.
        </p>
      </header>

      <div className="mb-6 flex items-center justify-end">
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
      </div>

      {loading && !summary ? (
        <Card className="space-y-4 p-6">
          <Skeleton className="h-5 w-40" />
          <Skeleton className="h-40 w-full" />
        </Card>
      ) : error ? (
        <Card className="p-6 text-sm text-rose-300">{error}</Card>
      ) : !summary?.available ? (
        <Card className="flex flex-col items-center px-8 py-20 text-center">
          <Info className="h-6 w-6 text-zinc-600" />
          <p className="mt-3 text-sm font-medium text-zinc-300">
            No encontramos transcripts de Claude Code en este equipo
          </p>
          <p className="mt-1.5 max-w-md text-xs leading-relaxed text-zinc-500">
            Karina busca en <code className="text-zinc-400">~/.claude/projects</code>, la
            carpeta donde Claude Code guarda el historial de tus sesiones. Si usas Claude
            Code en otra máquina, esta sección solo mostrará datos de esta.
          </p>
        </Card>
      ) : projects.length === 0 ? (
        <Card className="flex flex-col items-center px-8 py-16 text-center">
          <Info className="h-6 w-6 text-zinc-600" />
          <p className="mt-3 text-sm font-medium text-zinc-300">
            Sin actividad de Claude Code en este periodo
          </p>
          <p className="mt-1.5 max-w-md text-xs leading-relaxed text-zinc-500">
            Prueba con un rango mayor, o usa Claude Code en este equipo y vuelve a revisar.
          </p>
        </Card>
      ) : (
        <div className="space-y-6">
          <SummaryCards summary={summary} />

          <Card className="p-6">
            <h2 className="flex items-center gap-2 text-sm font-semibold text-zinc-100">
              <FolderGit2 className="h-4 w-4 text-violet-300" />
              Por proyecto
            </h2>
            <p className="mt-0.5 text-xs text-zinc-500">
              Carpeta desde la que corriste Claude Code — el directorio de trabajo real
              (cwd), no un nombre inventado.
            </p>
            <div className="mt-5 space-y-3">
              {projects.map((p) => {
                const total = totalOf(p.tokens);
                const pct = Math.max(2, (total / maxProjectTotal) * 100);
                return (
                  <div key={p.path}>
                    <div className="flex items-center justify-between gap-3 text-sm">
                      <span className="truncate font-medium text-zinc-200" title={p.path}>
                        {p.label}
                      </span>
                      <span className="shrink-0 font-mono text-xs text-zinc-400">
                        {formatTokens(total)} tokens · {p.sessions}{' '}
                        {p.sessions === 1 ? 'sesión' : 'sesiones'}
                      </span>
                    </div>
                    <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-white/[0.06]">
                      <div
                        className="h-full rounded-full bg-gradient-to-r from-violet-400 to-sky-400"
                        style={{ width: `${pct}%` }}
                      />
                    </div>
                  </div>
                );
              })}
            </div>
          </Card>

          {days.length > 1 && (
            <Card className="p-6">
              <h2 className="text-sm font-semibold text-zinc-100">Por día</h2>
              <p className="mt-0.5 text-xs text-zinc-500">
                Total de tokens (entrada + salida + caché) por día, todos los proyectos.
              </p>
              <div className="mt-6 flex h-32 items-end gap-1.5">
                {days.map((d) => {
                  const total = totalOf(d.tokens);
                  const h = Math.max(4, (total / maxDayTotal) * 100);
                  return (
                    <div
                      key={d.date}
                      className="group flex h-full flex-1 flex-col justify-end"
                      title={`${d.date}: ${formatTokensFull(total)} tokens`}
                    >
                      <div
                        className="w-full rounded-t-sm bg-gradient-to-t from-violet-500/70 to-sky-400/70 transition-all group-hover:from-violet-400 group-hover:to-sky-300"
                        style={{ height: `${h}%` }}
                      />
                    </div>
                  );
                })}
              </div>
              <div className="mt-2 flex justify-between font-mono text-[10px] text-zinc-600">
                <span>{formatAxisDate(days[0].date)}</span>
                <span>{formatAxisDate(days[days.length - 1].date)}</span>
              </div>
            </Card>
          )}

          <p className="text-[11px] leading-relaxed text-zinc-600">
            Los tokens de caché (creación/lectura) tienen precios muy distintos al token
            normal, y por eso pueden dominar el total — estas cifras son actividad real,
            no un costo en dólares.
          </p>
        </div>
      )}
    </div>
  );
}

function totalOf(t: { input_tokens: number; output_tokens: number; cache_creation_tokens: number; cache_read_tokens: number }): number {
  return t.input_tokens + t.output_tokens + t.cache_creation_tokens + t.cache_read_tokens;
}

function formatAxisDate(date: string): string {
  const d = new Date(`${date}T00:00:00`);
  if (!Number.isFinite(d.getTime())) return date;
  return d.toLocaleDateString('es-ES', { day: 'numeric', month: 'short' });
}

function SummaryCards({ summary }: { summary: ClaudeCodeUsageSummary }) {
  const t = summary.total;
  const total = totalOf(t);
  const cache = t.cache_creation_tokens + t.cache_read_tokens;
  const cards = [
    { label: 'Total', value: total },
    { label: 'Entrada', value: t.input_tokens },
    { label: 'Salida', value: t.output_tokens },
    { label: 'Caché (creación + lectura)', value: cache },
  ];
  return (
    <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
      {cards.map((c) => (
        <Card key={c.label} className="p-4">
          <p className="text-[11px] uppercase tracking-wide text-zinc-500">{c.label}</p>
          <p className="mt-1 truncate font-mono text-lg font-semibold text-zinc-100">
            {formatTokens(c.value)}
          </p>
        </Card>
      ))}
    </div>
  );
}

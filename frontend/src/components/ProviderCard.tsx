import {
  AlertTriangle,
  Clock,
  KeyRound,
  PlugZap,
  Settings2,
  ShieldAlert,
  WifiOff,
} from 'lucide-react';
import type { ProviderMeta, ProviderState } from '@/lib/types';
import { STATUS_COLORS, brandTheme } from '@/lib/theme';
import { useNow, cn } from '@/lib/hooks';
import {
  countdown,
  formatMoney,
  formatPercent,
  formatTime,
  formatTokens,
  formatTokensFull,
  timeAgo,
} from '@/lib/format';
import { ProviderMark } from './ProviderMark';
import { UsageBar } from './UsageBar';
import { Badge, Button, Card, Skeleton } from './primitives';
import { useStore } from '@/store';

export function ProviderCard({
  meta,
  state,
}: {
  meta: ProviderMeta;
  state?: ProviderState;
}) {
  const now = useNow();
  const openConnect = useStore((s) => s.openConnect);
  const manualRefresh = useStore((s) => s.manualRefresh);
  const st = state;

  const theme = brandTheme(meta.brand);
  const sc = STATUS_COLORS[st?.status ?? 'disconnected'] ?? STATUS_COLORS.disconnected;
  const showShimmer = !st || st.status === 'updating' || st.status === 'disconnected';

  const usagePct =
    st && st.usage_available && st.limit_tokens > 0
      ? formatPercent(st.used_tokens, st.limit_tokens)
      : -1;

  return (
    <Card className="group flex flex-col p-5 animate-rise-in">
      {/* Cabecera */}
      <div className="flex items-start gap-3.5">
        <ProviderMark id={meta.id} brand={meta.brand} />
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h3 className="truncate text-[15px] font-semibold text-zinc-100">
              {meta.name}
            </h3>
            {meta.demo && <Badge className="text-violet-300">Demo</Badge>}
            {meta.manual && <Badge className="text-amber-300">Manual</Badge>}
          </div>
          <div className="mt-1 flex items-center gap-2 text-xs">
            <span className={cn('inline-flex items-center gap-1.5', sc.text)}>
              <span className={cn('h-1.5 w-1.5 rounded-full', sc.dot)} />
              {statusText(st, meta)}
            </span>
          </div>
        </div>
        <button
          onClick={() => openConnect(meta.id)}
          title="Administrar proveedor"
          className="rounded-lg p-1.5 text-zinc-500 opacity-0 transition-all hover:bg-white/[0.06] hover:text-zinc-200 focus:opacity-100 group-hover:opacity-100"
        >
          <Settings2 className="h-4 w-4" />
        </button>
      </div>

      <div className="mt-5 flex-1">{renderBody()}</div>

      {/* Pie */}
      <div className="mt-5 flex items-center justify-between border-t border-white/[0.05] pt-3.5 text-[11px] text-zinc-500">
        <span className="inline-flex items-center gap-1.5">
          <Clock className="h-3 w-3" />
          Actualizado {timeAgo(st?.updated_at, now)}
        </span>
        {meta.enabled &&
          st &&
          (st.status === 'error' || st.status === 'invalid_credentials') && (
            <Button
              variant="ghost"
              className="h-6 px-2 text-[11px] text-zinc-400"
              onClick={() => manualRefresh()}
            >
              <PlugZap className="h-3 w-3" /> Actualizar
            </Button>
          )}
      </div>
    </Card>
  );

  function renderBody() {
    if (!st) {
      return (
        <div className="space-y-3">
          <Skeleton className="h-8 w-24" />
          <Skeleton className="h-2 w-full" />
          <Skeleton className="h-3 w-2/3" />
        </div>
      );
    }

    if (showShimmer) {
      return (
        <div className="space-y-3">
          <Skeleton className="h-8 w-20" />
          <Skeleton className="h-2.5 w-full" />
          <div className="flex justify-between">
            <Skeleton className="h-3 w-28" />
            <Skeleton className="h-3 w-16" />
          </div>
        </div>
      );
    }

    if (
      st.status === 'invalid_credentials' ||
      st.status === 'error' ||
      st.status === 'rate_limited'
    ) {
      const Icon =
        st.status === 'rate_limited'
          ? ShieldAlert
          : st.status === 'invalid_credentials'
            ? KeyRound
            : AlertTriangle;
      return (
        <div className="rounded-xl border border-rose-400/15 bg-rose-500/[0.06] p-3.5">
          <div className="flex items-center gap-2 text-[13px] font-medium text-rose-200">
            <Icon className="h-4 w-4" />
            {st.status === 'rate_limited'
              ? 'Límite de peticiones alcanzado'
              : st.status === 'invalid_credentials'
                ? 'Credenciales no válidas'
                : 'No se pudo obtener el uso'}
          </div>
          {(st.error || st.status_msg) && (
            <p className="mt-1.5 text-xs leading-relaxed text-zinc-400">
              {st.error || st.status_msg}
            </p>
          )}
          <div className="mt-3 flex gap-2">
            <Button className="h-7 px-2.5 text-xs" onClick={() => manualRefresh()}>
              Reintentar
            </Button>
            <Button
              variant="ghost"
              className="h-7 px-2.5 text-xs"
              onClick={() => openConnect(meta.id)}
            >
              Actualizar clave
            </Button>
          </div>
        </div>
      );
    }

    return (
      <div className="space-y-4">
        {st.balance && st.balance.total !== 0 && (
          <div className="flex items-end justify-between rounded-xl border border-white/[0.05] bg-white/[0.02] px-3.5 py-2.5">
            <div>
              <p className="text-[11px] uppercase tracking-wide text-zinc-500">
                Saldo prepagado
              </p>
              <p className={cn('mt-0.5 font-mono text-xl font-semibold', theme.text)}>
                {formatMoney(st.balance.total, st.balance.currency)}
              </p>
            </div>
            <Badge className={st.balance.is_available ? 'text-emerald-300' : 'text-amber-300'}>
              {st.balance.is_available ? 'Disponible' : 'Bajo / vacío'}
            </Badge>
          </div>
        )}

        {usagePct >= 0 ? (
          <div>
            <div className="flex items-end justify-between">
              <p className="font-mono text-3xl font-semibold tracking-tight text-zinc-50">
                {usagePct}
                <span className="text-xl text-zinc-500">%</span>
              </p>
              <p className="text-right font-mono text-[11px] text-zinc-500">
                {formatTokensFull(st.used_tokens)} / {formatTokensFull(st.limit_tokens)}{' '}
                tokens
              </p>
            </div>
            <UsageBar percent={usagePct} brand={meta.brand} className="mt-3" height="h-2.5" />
            <div className="mt-2.5 flex items-center justify-between text-xs text-zinc-400">
              <span>
                <span className="font-medium text-zinc-300">
                  {formatTokens(st.remaining_tokens)}
                </span>{' '}
                restantes
              </span>
              <span className="text-zinc-500">
                {st.usage_window || 'Uso'} · se reinicia {resetLabel(st)}
              </span>
            </div>
          </div>
        ) : st.usage_available ? (
          <div className="rounded-xl border border-white/[0.05] bg-white/[0.02] px-3.5 py-2.5">
            <p className="text-[11px] uppercase tracking-wide text-zinc-500">
              {st.usage_window || 'Tokens usados'}
            </p>
            <p className="mt-0.5 font-mono text-xl font-semibold text-zinc-100">
              {formatTokensFull(st.used_tokens)}{' '}
              <span className="text-xs font-normal text-zinc-500">tokens</span>
            </p>
            <p className="mt-1 text-xs text-zinc-500">
              Este proveedor no expone un límite de uso — solo el uso absoluto.
            </p>
          </div>
        ) : (
          <div className="rounded-xl border border-white/[0.05] bg-white/[0.02] px-3.5 py-3">
            <div className="flex items-center gap-2 text-[13px] font-medium text-zinc-200">
              <WifiOff className="h-4 w-4 text-zinc-400" />
              Datos de uso no disponibles por API
            </div>
            <p className="mt-1.5 text-xs leading-relaxed text-zinc-500">{st.note}</p>
          </div>
        )}

        {st.rate_limit && rateLimitPresent(st) && <RateStrip state={st} />}
      </div>
    );
  }

  function resetLabel(s: ProviderState): string {
    if (s.reset_at) {
      const cd = countdown(s.reset_at, now);
      if (cd) return `en ${cd}`;
      return formatTime(s.reset_at);
    }
    return '—';
  }
}

function rateLimitPresent(s: ProviderState): boolean {
  return Boolean(
    s.rate_limit &&
      (s.rate_limit.requests ||
        s.rate_limit.tokens ||
        s.rate_limit.input_tokens ||
        s.rate_limit.output_tokens),
  );
}

function RateStrip({ state }: { state: ProviderState }) {
  const rows = [
    state.rate_limit.requests && { label: 'Peticiones/min', b: state.rate_limit.requests },
    state.rate_limit.tokens && { label: 'Tokens/min', b: state.rate_limit.tokens },
    state.rate_limit.input_tokens && { label: 'Tokens entrada/min', b: state.rate_limit.input_tokens },
    state.rate_limit.output_tokens && { label: 'Tokens salida/min', b: state.rate_limit.output_tokens },
  ].filter(Boolean) as Array<{
    label: string;
    b: { limit: number; remaining: number };
  }>;

  if (!rows.length) return null;
  return (
    <div className="space-y-2 rounded-xl border border-white/[0.05] bg-white/[0.02] px-3.5 py-2.5">
      <p className="text-[11px] uppercase tracking-wide text-zinc-500">
        Estado del límite de peticiones{' '}
        <span className="normal-case text-zinc-600">(en vivo, por minuto)</span>
      </p>
      {rows.slice(0, 2).map(({ label, b }) => {
        const pct =
          b.limit > 0
            ? Math.min(100, Math.max(0, ((b.limit - b.remaining) / b.limit) * 100))
            : 0;
        return (
          <div key={label}>
            <div className="flex items-center justify-between text-xs">
              <span className="text-zinc-400">{label}</span>
              <span className="font-mono text-zinc-300">
                {formatTokens(b.remaining)}{' '}
                <span className="text-zinc-600">/ {formatTokens(b.limit)}</span>
              </span>
            </div>
            <div className="mt-1 h-1 w-full overflow-hidden rounded-full bg-white/[0.05]">
              <div
                className="h-full rounded-full bg-white/25 transition-all duration-500"
                style={{ width: `${pct}%` }}
              />
            </div>
          </div>
        );
      })}
    </div>
  );
}

function statusText(st: ProviderState | undefined, meta: ProviderMeta): string {
  if (!st) return meta.enabled ? 'Iniciando…' : 'Sin conectar';
  switch (st.status) {
    case 'connected':
      if (!st.usage_available && st.capabilities.length === 0) {
        return 'Conectado · la API no expone uso';
      }
      return 'Conectado';
    case 'updating':
      return 'Actualizando…';
    case 'disconnected':
      return st.status_msg || 'Desconectado';
    case 'invalid_credentials':
      return 'Credenciales no válidas';
    case 'rate_limited':
      return 'Límite de peticiones alcanzado';
    case 'usage_unavailable':
      return 'Uso no disponible';
    case 'error':
      return st.status_msg || 'Error';
    default:
      return st.status;
  }
}

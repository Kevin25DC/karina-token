import { Expand, Minus, RefreshCw } from 'lucide-react';
import { api } from '@/lib/api';
import { useStore } from '@/store';
import { WindowHide } from '../../wailsjs/runtime/runtime';
import { Logo } from '@/components/Logo';
import { ProviderMark } from '@/components/ProviderMark';
import { UsageBar } from '@/components/UsageBar';
import { Spinner } from '@/components/primitives';
import { cn } from '@/lib/hooks';
import { useNow } from '@/lib/hooks';
import {
  formatMoney,
  formatPercent,
  formatTokens,
  formatTokensFull,
} from '@/lib/format';
import type { ProviderMeta, ProviderState } from '@/lib/types';

export function WidgetMode() {
  const meta = useStore((s) => s.meta);
  const states = useStore((s) => s.states);
  const updatingAll = useStore((s) => s.updatingAll);
  const manualRefresh = useStore((s) => s.manualRefresh);

  const enabled = meta.filter((m) => m.enabled);

  return (
    <div className="flex h-full flex-col overflow-hidden bg-ink-950">
      {/* Cabecera compacta (arrastrable) */}
      <div className="titlebar flex items-center gap-2 border-b border-white/[0.06] bg-white/[0.03] px-3 py-2.5">
        <Logo className="h-6 w-6 rounded-md" />
        <div className="min-w-0 flex-1 leading-tight">
          <p className="truncate text-[12px] font-semibold text-zinc-100">Karina</p>
          <p className="truncate text-[9px] uppercase tracking-widest text-zinc-600">
            Consumo de IA en vivo
          </p>
        </div>
        <div className="titlebar-no-drag flex items-center gap-0.5">
          <button
            onClick={() => void manualRefresh()}
            title="Actualizar ahora"
            className="rounded-md p-1.5 text-zinc-500 transition-colors hover:bg-white/[0.06] hover:text-zinc-200"
          >
            {updatingAll ? (
              <Spinner className="h-3.5 w-3.5" />
            ) : (
              <RefreshCw className="h-3.5 w-3.5" />
            )}
          </button>
          <button
            onClick={() => {
              useStore.getState().setWidgetMode(false);
              void api.exitWidgetMode().catch(() => undefined);
            }}
            title="Ampliar al panel completo"
            className="rounded-md p-1.5 text-zinc-500 transition-colors hover:bg-white/[0.06] hover:text-zinc-200"
          >
            <Expand className="h-3.5 w-3.5" />
          </button>
          <button
            onClick={() => WindowHide()}
            title="Ocultar a la bandeja (doble clic en el icono para volver)"
            className="rounded-md p-1.5 text-zinc-500 transition-colors hover:bg-white/[0.06] hover:text-zinc-200"
          >
            <Minus className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>

      {/* Barras */}
      <div className="flex-1 space-y-2 overflow-y-auto p-2.5">
        {enabled.length === 0 && (
          <p className="px-2 py-6 text-center text-xs text-zinc-600">
            Sin proveedores conectados
          </p>
        )}
        {enabled.map((m) => (
          <WidgetRow key={m.id} meta={m} state={states[m.id]} />
        ))}
      </div>
    </div>
  );
}

function WidgetRow({ meta, state }: { meta: ProviderMeta; state?: ProviderState }) {
  const now = useNow();
  const st = state;

  const usagePct =
    st && st.usage_available && st.limit_tokens > 0
      ? formatPercent(st.used_tokens, st.limit_tokens)
      : -1;

  const valueLine = !st
    ? '…'
    : st.balance && st.balance.total !== 0
      ? formatMoney(st.balance.total, st.balance.currency)
      : st.usage_available
        ? `${formatTokensFull(st.used_tokens)} tokens`
        : null;

  return (
    <div className="rounded-xl border border-white/[0.06] bg-white/[0.025] px-2.5 py-2">
      <div className="flex items-center gap-2">
        <ProviderMark id={meta.id} brand={meta.brand} size="sm" className="h-6 w-6 rounded-md" />
        <span className="min-w-0 flex-1 truncate text-[12px] font-medium text-zinc-200">
          {meta.name}
        </span>
        {usagePct >= 0 && (
          <span className="font-mono text-[13px] font-semibold text-zinc-100">
            {usagePct}
            <span className="text-[10px] text-zinc-500">%</span>
          </span>
        )}
        {st?.status === 'updating' && <Spinner className="h-3 w-3 text-sky-300" />}
        <span
          className={cn(
            'h-1.5 w-1.5 rounded-full',
            st?.status === 'connected'
              ? 'bg-emerald-400'
              : st?.status === 'updating'
                ? 'bg-sky-400 animate-pulse-dot'
                : 'bg-zinc-500',
          )}
        />
      </div>

      {usagePct >= 0 && st ? (
        <>
          <UsageBar percent={usagePct} brand={meta.brand} className="mt-1.5" height="h-1.5" />
          <div className="mt-1 flex items-center justify-between text-[10px] text-zinc-500">
            <span>
              {formatTokens(st.remaining_tokens)} restantes
            </span>
            <span>actualizado {ago(st.updated_at, now)}</span>
          </div>
        </>
      ) : valueLine ? (
        <div className="mt-1 flex items-center justify-between text-[10px] text-zinc-500">
          <span className="font-mono text-[12px] text-zinc-200">{valueLine}</span>
          <span>actualizado {ago(st?.updated_at, now)}</span>
        </div>
      ) : (
        <div className="mt-1 flex items-center justify-between text-[10px] text-zinc-600">
          <span>uso no disponible vía API</span>
          <span>{ago(st?.updated_at, now)}</span>
        </div>
      )}
    </div>
  );
}

function ago(iso: string | undefined, now: number): string {
  if (!iso) return '—';
  const t = new Date(iso).getTime();
  if (!Number.isFinite(t)) return '—';
  const s = Math.max(0, Math.floor((now - t) / 1000));
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  return m < 60 ? `${m}m` : `${Math.floor(m / 60)}h`;
}

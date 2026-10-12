import { useEffect, useRef, useState } from 'react';
import { ArrowUpToLine, Expand, GripHorizontal, Minus, RefreshCw } from 'lucide-react';
import { api, onWidgetDocked } from '@/lib/api';
import { useStore } from '@/store';
import { WindowHide } from '../../wailsjs/runtime/runtime';
import { Mascot, moodTextClass, useMascotStatus, type MascotMood } from '@/components/Mascot';
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

/** Espera antes de plegar la isla cuando el cursor sale (evita parpadeos). */
const COLLAPSE_DELAY_MS = 380;

/**
 * Modo widget: una «isla» colgada del borde superior de la pantalla. Plegada
 * es una pastilla con la mascota y el porcentaje más alto; al pasar el cursor
 * se despliega con el detalle de cada proveedor.
 */
export function WidgetMode() {
  const meta = useStore((s) => s.meta);
  const states = useStore((s) => s.states);
  const updatingAll = useStore((s) => s.updatingAll);
  const manualRefresh = useStore((s) => s.manualRefresh);
  const status = useMascotStatus();

  const [open, setOpen] = useState(false);
  // Pegada al borde superior conserva las esquinas de arriba rectas; movida
  // a otro sitio es una tarjeta flotante con las cuatro redondeadas.
  const [docked, setDocked] = useState(true);
  const leaveTimer = useRef<number | undefined>(undefined);

  useEffect(() => {
    onWidgetDocked(setDocked);
    return () => window.clearTimeout(leaveTimer.current);
  }, []);

  function expand() {
    window.clearTimeout(leaveTimer.current);
    if (open) return;
    setOpen(true);
    void api.setWidgetExpanded(true).catch(() => undefined);
  }

  function collapseSoon() {
    window.clearTimeout(leaveTimer.current);
    leaveTimer.current = window.setTimeout(() => {
      setOpen(false);
      void api.setWidgetExpanded(false).catch(() => undefined);
    }, COLLAPSE_DELAY_MS);
  }

  const enabled = meta.filter((m) => m.enabled);
  // El proveedor que refleja la mascota va primero y marcado en la lista.
  const ordered = [...enabled].sort(
    (a, b) => Number(b.id === status.providerId) - Number(a.id === status.providerId),
  );
  const followedName = enabled.find((m) => m.id === status.providerId)?.name;
  const pct = status.percent >= 0 ? Math.round(status.percent) : null;
  const now = useNow();
  const clock = new Date(now).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });

  return (
    <div
      onMouseEnter={expand}
      onMouseLeave={collapseSoon}
      style={ringStyle(status.mood)}
      className={cn(
        // El contorno es un degradado que gira con los colores de la app:
        // indica que Karina está viva y cambia de tono según el estado.
        'island-ring absolute inset-0',
        status.mood === 'sleeping' && 'island-ring-idle',
        docked ? 'rounded-b-[22px] px-[2px] pb-[2px]' : 'rounded-[22px] p-[2px]',
      )}
    >
      <div
        className={cn(
          'flex h-full flex-col overflow-hidden bg-black',
          docked ? 'rounded-b-[20px]' : 'rounded-[20px]',
        )}
      >
      {!open ? (
        // Pastilla plegada
        <div className="titlebar flex h-full items-center justify-center gap-2.5 px-4">
          <Mascot mood={status.mood} size={30} badge={false} />
          {pct !== null ? (
            <span className="font-mono text-[15px] font-semibold text-zinc-100">
              {pct}
              <span className="text-[11px] text-zinc-500">%</span>
            </span>
          ) : status.display ? (
            // Proveedor de saldo prepagado: importe en vez de porcentaje.
            <span className="font-mono text-[14px] font-semibold text-zinc-100">
              {status.display}
            </span>
          ) : (
            <span className="text-[11px] text-zinc-500">Karina</span>
          )}
          <span
            className={cn(
              'h-1.5 w-1.5 rounded-full',
              status.mood === 'exhausted'
                ? 'bg-rose-400'
                : status.mood === 'worried' || status.mood === 'confused'
                  ? 'bg-amber-400'
                  : status.mood === 'thinking'
                    ? 'bg-sky-400 animate-pulse-dot'
                    : status.mood === 'sleeping'
                      ? 'bg-zinc-500'
                      : 'bg-emerald-400',
            )}
          />
          {/* Hora del sistema, con el formato (12/24 h) de Windows */}
          <span className="h-3.5 w-px bg-white/[0.12]" />
          <span className="font-mono text-[13px] tabular-nums text-zinc-300">{clock}</span>
        </div>
      ) : (
        <>
          {/* Barra superior (arrastrable) */}
          <div className="titlebar flex items-center gap-2 px-4 pb-1 pt-2.5">
            <p className="flex flex-1 items-center gap-1.5 text-[10px] font-medium uppercase tracking-widest text-zinc-600">
              <GripHorizontal className="h-3.5 w-3.5" />
              Karina · arrastra para mover
            </p>
            <div className="titlebar-no-drag flex items-center gap-0.5">
              {!docked && (
                <IslandButton
                  title="Volver arriba al centro"
                  onClick={() => void api.resetWidgetPosition().catch(() => undefined)}
                >
                  <ArrowUpToLine className="h-3.5 w-3.5" />
                </IslandButton>
              )}
              <IslandButton title="Actualizar ahora" onClick={() => void manualRefresh()}>
                {updatingAll ? (
                  <Spinner className="h-3.5 w-3.5" />
                ) : (
                  <RefreshCw className="h-3.5 w-3.5" />
                )}
              </IslandButton>
              <IslandButton
                title="Ampliar al panel completo"
                onClick={() => {
                  useStore.getState().setWidgetMode(false);
                  void api.exitWidgetMode().catch(() => undefined);
                }}
              >
                <Expand className="h-3.5 w-3.5" />
              </IslandButton>
              <IslandButton
                title="Ocultar a la bandeja (doble clic en el icono para volver)"
                onClick={() => WindowHide()}
              >
                <Minus className="h-3.5 w-3.5" />
              </IslandButton>
            </div>
          </div>

          <div className="flex min-h-0 flex-1 gap-2.5 px-3 pb-3">
            {/* Mascota */}
            <div className="flex w-[150px] shrink-0 flex-col items-center justify-center rounded-2xl border border-white/[0.06] bg-white/[0.03] px-3 py-3 text-center">
              <Mascot mood={status.mood} size={92} />
              <p className={cn('mt-2 text-[11px] leading-snug', moodTextClass(status.mood))}>
                {status.line}
              </p>
              {followedName && (
                <p className="mt-1.5 text-[10px] leading-snug text-zinc-600">
                  {status.pinned ? 'Siguiendo' : 'Automático'}: {followedName}
                </p>
              )}
            </div>

            {/* Proveedores */}
            {/* Rejilla que se ensancha: la ventana crece a lo ancho (dos
                proveedores por columna) y aquí caben tantas columnas como dé
                el ancho que le dio el backend (ver islandPanelSize). */}
            <div className="grid min-w-0 flex-1 auto-rows-min grid-cols-[repeat(auto-fit,minmax(300px,1fr))] content-start gap-2 overflow-y-auto">
              {enabled.length === 0 && (
                <p className="px-2 py-8 text-center text-xs text-zinc-600">
                  Sin proveedores conectados
                </p>
              )}
              {ordered.map((m) => (
                <WidgetRow
                  key={m.id}
                  meta={m}
                  state={states[m.id]}
                  followed={m.id === status.providerId}
                />
              ))}
            </div>
          </div>
        </>
      )}
      </div>
    </div>
  );
}

/** Colores del contorno según el ánimo: los de la app, o aviso si hay límite cerca. */
function ringStyle(mood: MascotMood): React.CSSProperties {
  const [a, b, c] =
    mood === 'exhausted'
      ? ['#fb7185', '#f43f5e', '#fda4af']
      : mood === 'worried' || mood === 'confused'
        ? ['#fbbf24', '#f59e0b', '#fde68a']
        : ['#a78bfa', '#7dd3fc', '#f0abfc'];
  return { '--ring-a': a, '--ring-b': b, '--ring-c': c } as React.CSSProperties;
}

function IslandButton({
  title,
  onClick,
  children,
}: {
  title: string;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      title={title}
      className="rounded-md p-1.5 text-zinc-500 transition-colors hover:bg-white/[0.08] hover:text-zinc-200"
    >
      {children}
    </button>
  );
}

function WidgetRow({
  meta,
  state,
  followed,
}: {
  meta: ProviderMeta;
  state?: ProviderState;
  /** Es el proveedor cuyo porcentaje muestra la mascota. */
  followed: boolean;
}) {
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
    <div
      className={cn(
        'rounded-2xl border px-2.5 py-2',
        followed
          ? 'border-violet-400/30 bg-violet-500/[0.07]'
          : 'border-white/[0.06] bg-white/[0.03]',
      )}
    >
      <div className="flex items-center gap-2">
        <ProviderMark id={meta.id} brand={meta.brand} size="sm" className="h-6 w-6 rounded-md" />
        <span className="min-w-0 flex-1 truncate text-[12px] font-medium text-zinc-200">
          {meta.name}
        </span>
        {followed && (
          <span className="shrink-0 rounded-md border border-violet-400/30 px-1.5 py-px text-[9px] font-medium uppercase tracking-wide text-violet-200">
            Mascota
          </span>
        )}
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

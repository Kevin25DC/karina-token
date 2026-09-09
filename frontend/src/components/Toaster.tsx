import { useEffect } from 'react';
import { AlertTriangle, CheckCircle2, Info, X } from 'lucide-react';
import { useStore } from '@/store';
import { cn } from '@/lib/hooks';

const ICON = {
  success: CheckCircle2,
  error: AlertTriangle,
  info: Info,
};

const ICON_COLOR = {
  success: 'text-emerald-300',
  error: 'text-rose-300',
  info: 'text-sky-300',
};

export function Toaster() {
  const toasts = useStore((s) => s.toasts);
  const dismiss = useStore((s) => s.dismiss);

  return (
    <div className="pointer-events-none fixed right-4 top-4 z-[70] flex w-80 flex-col gap-2">
      {toasts.map((t) => {
        const Icon = ICON[t.kind];
        return (
          <div
            key={t.id}
            className="pointer-events-auto flex animate-rise-in items-start gap-2.5 rounded-xl border border-white/[0.08] bg-ink-800/95 p-3 shadow-glow backdrop-blur"
          >
            <Icon className={cn('mt-0.5 h-4 w-4 shrink-0', ICON_COLOR[t.kind])} />
            <p className="flex-1 text-[13px] leading-snug text-zinc-200">{t.message}</p>
            <button
              onClick={() => dismiss(t.id)}
              className="rounded p-0.5 text-zinc-500 hover:text-zinc-200"
            >
              <X className="h-3.5 w-3.5" />
            </button>
            <AutoDismiss id={t.id} dismiss={dismiss} />
          </div>
        );
      })}
    </div>
  );
}

function AutoDismiss({ id, dismiss }: { id: number; dismiss: (id: number) => void }) {
  useEffect(() => {
    const timer = window.setTimeout(() => dismiss(id), 4600);
    return () => window.clearTimeout(timer);
  }, [id, dismiss]);
  return null;
}

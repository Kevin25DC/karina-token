import { useEffect, type ReactNode, type ButtonHTMLAttributes } from 'react';
import { X, Loader2 } from 'lucide-react';
import { cn } from '@/lib/hooks';

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'subtle';

const VARIANT: Record<Variant, string> = {
  primary:
    'bg-zinc-100 text-zinc-900 hover:bg-white disabled:hover:bg-zinc-100 shadow-sm',
  secondary:
    'bg-white/[0.06] text-zinc-100 hover:bg-white/[0.1] border border-white/[0.08]',
  ghost: 'text-zinc-400 hover:text-zinc-100 hover:bg-white/[0.06]',
  danger:
    'bg-rose-500/10 text-rose-300 hover:bg-rose-500/20 border border-rose-500/20',
  subtle: 'text-zinc-500 hover:text-zinc-200',
};

export function Spinner({ className }: { className?: string }) {
  return <Loader2 className={cn('h-4 w-4 animate-spin', className)} />;
}

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  loading?: boolean;
  icon?: ReactNode;
}

export function Button({
  variant = 'secondary',
  loading,
  icon,
  className,
  children,
  disabled,
  ...rest
}: ButtonProps) {
  return (
    <button
      disabled={disabled || loading}
      className={cn(
        'inline-flex items-center justify-center gap-2 rounded-lg px-3.5 py-2 text-sm font-medium',
        'transition-all duration-150 outline-none',
        'focus-visible:ring-2 focus-visible:ring-white/40',
        'disabled:cursor-not-allowed disabled:opacity-50 active:scale-[0.98]',
        VARIANT[variant],
        className,
      )}
      {...rest}
    >
      {loading ? <Spinner className="h-4 w-4" /> : icon}
      {children}
    </button>
  );
}

export function StatusDot({ className, pulse }: { className?: string; pulse?: boolean }) {
  return (
    <span className="relative inline-flex h-2 w-2">
      {pulse && (
        <span
          className={cn(
            'absolute inline-flex h-full w-full rounded-full opacity-60',
            className,
          )}
        />
      )}
      <span className={cn('relative inline-flex h-2 w-2 rounded-full', className)} />
    </span>
  );
}

export function Badge({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full border border-white/[0.07] bg-white/[0.04] px-2 py-0.5 text-[11px] font-medium text-zinc-300',
        className,
      )}
    >
      {children}
    </span>
  );
}

export function Card({
  children,
  className,
  onClick,
}: {
  children: ReactNode;
  className?: string;
  onClick?: () => void;
}) {
  return (
    <div
      onClick={onClick}
      className={cn(
        'rounded-2xl border border-white/[0.06] bg-gradient-to-b from-white/[0.045] to-white/[0.015]',
        'shadow-card backdrop-blur-sm',
        className,
      )}
    >
      {children}
    </div>
  );
}

export function Skeleton({ className }: { className?: string }) {
  return <div className={cn('skeleton rounded-md', className)} />;
}

/** Rounded, borderless window surface for the frameless/translucent shell. */
export function Surface({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return <div className={cn('surface-frame flex flex-col', className)}>{children}</div>;
}

export function Kbd({ children }: { children: ReactNode }) {
  return (
    <kbd className="rounded border border-white/10 bg-white/[0.05] px-1.5 py-0.5 font-mono text-[10px] text-zinc-400">
      {children}
    </kbd>
  );
}

export function Modal({
  open,
  onClose,
  children,
  title,
  wide,
}: {
  open: boolean;
  onClose: () => void;
  children: ReactNode;
  title?: ReactNode;
  wide?: boolean;
}) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [open, onClose]);

  if (!open) return null;
  return (
    <div
      className="fixed inset-0 z-[80] flex items-center justify-center p-6 animate-fade-in"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" />
      <div
        className={cn(
          'relative z-10 w-full animate-scale-in rounded-2xl border border-white/10 bg-ink-850 p-6 shadow-glow',
          wide ? 'max-w-2xl' : 'max-w-md',
        )}
      >
        {title && (
          <div className="mb-5 flex items-start justify-between">
            <h3 className="text-base font-semibold text-zinc-100">{title}</h3>
            <button
              onClick={onClose}
              className="rounded-lg p-1 text-zinc-500 transition-colors hover:bg-white/[0.06] hover:text-zinc-200"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        )}
        {children}
      </div>
    </div>
  );
}

import { brandTheme } from '@/lib/theme';
import { cn } from '@/lib/hooks';

/** Animated usage bar. Width transitions smoothly between polls. */
export function UsageBar({
  percent,
  brand,
  className,
  height = 'h-2',
}: {
  percent: number;
  brand: string;
  className?: string;
  height?: string;
}) {
  const theme = brandTheme(brand);
  const width = Number.isFinite(percent)
    ? Math.min(100, Math.max(0, percent))
    : 0;
  return (
    <div
      className={cn(
        'w-full overflow-hidden rounded-full bg-white/[0.06] ring-1 ring-inset ring-white/[0.04]',
        height,
        className,
      )}
    >
      <div
        className={cn('h-full rounded-full bg-gradient-to-r transition-all duration-700 ease-out', theme.bar)}
        style={{ width: `${width}%` }}
      />
    </div>
  );
}

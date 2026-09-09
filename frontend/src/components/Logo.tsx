import logoUrl from '@/assets/logo.png';
import { cn } from '@/lib/hooks';

export function Logo({
  className = 'h-9 w-9 rounded-xl',
}: {
  className?: string;
}) {
  return (
    <img
      src={logoUrl}
      alt="Karina"
      draggable={false}
      className={cn('shrink-0 border border-white/10 object-cover', className)}
    />
  );
}

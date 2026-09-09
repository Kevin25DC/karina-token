import {
  Aperture,
  FlaskConical,
  Gem,
  Sparkles,
  Waves,
} from 'lucide-react';
import type { ProviderID } from '@/lib/types';
import { brandTheme } from '@/lib/theme';
import { cn } from '@/lib/hooks';

const ICONS: Record<string, typeof Sparkles> = {
  anthropic: Sparkles,
  openai: Aperture,
  gemini: Gem,
  deepseek: Waves,
  demo: FlaskConical,
};

const TONES: Record<string, string> = {
  amber: 'text-amber-300',
  emerald: 'text-emerald-300',
  blue: 'text-sky-300',
  sky: 'text-cyan-300',
  violet: 'text-violet-300',
};

export function ProviderMark({
  id,
  brand,
  size = 'md',
  className,
}: {
  id: ProviderID;
  brand?: string;
  size?: 'sm' | 'md' | 'lg';
  className?: string;
}) {
  const Icon = ICONS[id] ?? Sparkles;
  const theme = brandTheme(brand ?? 'zinc');
  const tone = TONES[brand ?? ''] ?? 'text-zinc-300';
  const sizes = {
    sm: 'h-8 w-8 rounded-lg',
    md: 'h-10 w-10 rounded-xl',
    lg: 'h-14 w-14 rounded-2xl',
  };
  return (
    <div
      className={cn(
        'flex shrink-0 items-center justify-center bg-gradient-to-br ring-1 ring-inset ring-white/10',
        theme.tile,
        sizes[size],
        className,
      )}
    >
      <Icon className={cn('h-[52%] w-[52%]', tone)} strokeWidth={1.75} />
    </div>
  );
}

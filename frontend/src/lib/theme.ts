// Brand theming. Classes are written as full literal strings so Tailwind can
// detect them during the build (no dynamic class construction).

export interface BrandTheme {
  /** subtle accent text color */
  text: string;
  /** solid accent for the icon tile gradient */
  tile: string;
  bar: string;
  glow: string;
  dot: string;
}

const THEMES: Record<string, BrandTheme> = {
  amber: {
    text: 'text-amber-300',
    tile: 'from-amber-400/25 to-orange-600/25',
    bar: 'from-amber-400 to-orange-500',
    glow: 'shadow-[0_0_24px_-6px_rgba(245,158,11,0.55)]',
    dot: 'bg-amber-400',
  },
  emerald: {
    text: 'text-emerald-300',
    tile: 'from-emerald-400/25 to-teal-600/25',
    bar: 'from-emerald-400 to-teal-500',
    glow: 'shadow-[0_0_24px_-6px_rgba(16,185,129,0.55)]',
    dot: 'bg-emerald-400',
  },
  blue: {
    text: 'text-sky-300',
    tile: 'from-sky-400/25 to-blue-600/25',
    bar: 'from-sky-400 to-blue-500',
    glow: 'shadow-[0_0_24px_-6px_rgba(56,189,248,0.55)]',
    dot: 'bg-sky-400',
  },
  sky: {
    text: 'text-sky-300',
    tile: 'from-cyan-400/25 to-sky-600/25',
    bar: 'from-cyan-300 to-sky-500',
    glow: 'shadow-[0_0_24px_-6px_rgba(34,211,238,0.5)]',
    dot: 'bg-cyan-300',
  },
  violet: {
    text: 'text-violet-300',
    tile: 'from-violet-400/25 to-fuchsia-600/25',
    bar: 'from-violet-400 to-fuchsia-500',
    glow: 'shadow-[0_0_24px_-6px_rgba(167,139,250,0.55)]',
    dot: 'bg-violet-400',
  },
  zinc: {
    text: 'text-zinc-300',
    tile: 'from-zinc-400/20 to-zinc-600/20',
    bar: 'from-zinc-300 to-zinc-500',
    glow: 'shadow-[0_0_24px_-6px_rgba(161,161,170,0.4)]',
    dot: 'bg-zinc-400',
  },
};

export function brandTheme(brand: string): BrandTheme {
  return THEMES[brand] ?? THEMES.zinc;
}

export const STATUS_COLORS: Record<string, { dot: string; text: string }> = {
  connected: { dot: 'bg-emerald-400', text: 'text-emerald-300' },
  updating: { dot: 'bg-sky-400 animate-pulse-dot', text: 'text-sky-300' },
  disconnected: { dot: 'bg-zinc-500', text: 'text-zinc-400' },
  invalid_credentials: { dot: 'bg-rose-400', text: 'text-rose-300' },
  rate_limited: { dot: 'bg-amber-400', text: 'text-amber-300' },
  usage_unavailable: { dot: 'bg-zinc-500', text: 'text-zinc-400' },
  error: { dot: 'bg-rose-400', text: 'text-rose-300' },
};

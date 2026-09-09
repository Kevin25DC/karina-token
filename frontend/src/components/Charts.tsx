import { useMemo } from 'react';
import type { HistoryPoint } from '@/lib/types';
import { formatTokens } from '@/lib/format';

/** Lightweight, dependency-free SVG line/area chart. */
export function AreaChart({
  points,
  value,
  color = '#a78bfa',
  height = 220,
  showDots = true,
}: {
  points: HistoryPoint[];
  value: (p: HistoryPoint) => number | undefined;
  color?: string;
  height?: number;
  showDots?: boolean;
}) {
  const gradientId = useMemo(() => `g-${Math.random().toString(36).slice(2, 8)}`, []);
  const model = useMemo(() => {
    const vals = points
      .map((p) => ({ at: new Date(p.at).getTime(), v: value(p) ?? NaN }))
      .filter((x) => Number.isFinite(x.v));
    if (vals.length < 2) return null;
    const min = Math.min(...vals.map((v) => v.v));
    const max = Math.max(...vals.map((v) => v.v));
    const span = max - min || 1;
    const tMin = Math.min(...vals.map((v) => v.at));
    const tMax = Math.max(...vals.map((v) => v.at));
    const tSpan = tMax - tMin || 1;
    const W = 600;
    const H = height;
    const PAD = 6;
    const x = (t: number) => PAD + ((t - tMin) / tSpan) * (W - PAD * 2);
    const y = (v: number) => H - PAD - ((v - min) / span) * (H - PAD * 2 - 8) - 4;
    const pts = vals.map((v) => [x(v.at), y(v.v)] as const);
    const line = pts
      .map(([px, py], i) => `${i === 0 ? 'M' : 'L'}${px.toFixed(1)},${py.toFixed(1)}`)
      .join(' ');
    const area = `${line} L${pts[pts.length - 1][0].toFixed(1)},${H} L${pts[0][0].toFixed(1)},${H} Z`;
    return { pts, line, area, min, max, tMin, tMax };
  }, [points, value, height]);

  if (!model) return null;

  return (
    <svg
      viewBox={`0 0 600 ${height}`}
      className="h-auto w-full"
      preserveAspectRatio="none"
      role="img"
    >
      <defs>
        <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor={color} stopOpacity="0.28" />
          <stop offset="100%" stopColor={color} stopOpacity="0" />
        </linearGradient>
      </defs>
      {[0.25, 0.5, 0.75].map((f) => (
        <line
          key={f}
          x1="6"
          x2="594"
          y1={4 + (height - 8) * f}
          y2={4 + (height - 8) * f}
          stroke="rgba(255,255,255,0.05)"
          strokeDasharray="3 5"
        />
      ))}
      <path d={model.area} fill={`url(#${gradientId})`} />
      <path
        d={model.line}
        fill="none"
        stroke={color}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      {showDots &&
        model.pts.map(([px, py], i) => (
          <circle
            key={i}
            cx={px}
            cy={py}
            r={i === model.pts.length - 1 ? 3 : 2}
            fill={color}
            stroke="#0b0b0f"
            strokeWidth="1.2"
          />
        ))}
    </svg>
  );
}

export function ChartTooltipValue({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-white/10 bg-ink-800/95 px-2.5 py-1.5 shadow-lg">
      <p className="text-[10px] uppercase tracking-wide text-zinc-500">{label}</p>
      <p className="font-mono text-sm text-zinc-100">{value}</p>
    </div>
  );
}

export function MiniLegend({ items }: { items: Array<{ label: string; color: string }> }) {
  return (
    <div className="flex items-center gap-4 text-[11px] text-zinc-400">
      {items.map((it) => (
        <span key={it.label} className="inline-flex items-center gap-1.5">
          <span className="h-1.5 w-1.5 rounded-full" style={{ background: it.color }} />
          {it.label}
        </span>
      ))}
    </div>
  );
}

/** A horizontal sparkline used on provider cards. */
export function Sparkline({
  points,
  value,
  color = '#fff',
}: {
  points: HistoryPoint[];
  value: (p: HistoryPoint) => number | undefined;
  color?: string;
}) {
  return (
    <AreaChart
      points={points}
      value={value}
      color={color}
      height={40}
      showDots={false}
    />
  );
}

export function formatChartAxis(iso: string, span: string): string {
  const d = new Date(iso);
  if (!Number.isFinite(d.getTime())) return '';
  if (span === 'today') {
    return d.toLocaleTimeString('es-ES', { hour: '2-digit', minute: '2-digit' });
  }
  return d.toLocaleDateString('es-ES', { day: 'numeric', month: 'short' });
}

export function formatTokensAxis(n: number): string {
  return formatTokens(n);
}

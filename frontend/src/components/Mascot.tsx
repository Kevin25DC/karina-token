import { useEffect, useId, useMemo, useRef, useState } from 'react';
import { useStore } from '@/store';
import { cn } from '@/lib/hooks';

/**
 * Kari, la mascota de Karina: un personaje propio (morado, con orejas y panza
 * azul) que refleja el estado del consumo. Es SVG vectorial, sin imágenes ni
 * dependencias.
 */

export type MascotMood =
  | 'sleeping' // sin proveedores o sin datos todavía
  | 'idle' // consumo normal
  | 'happy' // consumo muy bajo (ventana recién reiniciada)
  | 'thinking' // actualizando
  | 'worried' // cerca del umbral de alerta
  | 'exhausted' // límite alcanzado
  | 'confused'; // algún proveedor falló

export interface MascotStatus {
  mood: MascotMood;
  /** Frase corta que dice la mascota. */
  line: string;
  /** Mayor porcentaje de uso entre los proveedores, o -1 si no hay dato. */
  percent: number;
}

const HAPPY_BELOW = 20;

/** Deriva el estado de ánimo del consumo que Karina ya conoce. */
export function useMascotStatus(): MascotStatus {
  const meta = useStore((s) => s.meta);
  const states = useStore((s) => s.states);
  const updatingAll = useStore((s) => s.updatingAll);
  const threshold = useStore((s) => s.config?.alert_threshold_percent ?? 85);
  const budgets = useStore((s) => s.budgets);

  return useMemo(() => {
    let percent = -1;
    let hottest = '';
    let failed = '';
    let any = false;
    for (const m of meta) {
      if (!m.enabled) continue;
      any = true;
      const st = states[m.id];
      if (!st) continue;
      if (st.status === 'error' || st.status === 'rate_limited' || st.status === 'invalid_credentials') {
        failed = failed || m.name;
      }
      const candidates: Array<{ pct: number; label: string }> = [];
      for (const w of st.windows ?? []) {
        candidates.push({ pct: w.percent, label: `${m.name} · ${w.label}` });
      }
      if (!candidates.length && st.usage_available && st.limit_tokens > 0) {
        candidates.push({ pct: (st.used_tokens / st.limit_tokens) * 100, label: m.name });
      }
      for (const c of candidates) {
        if (c.pct > percent) {
          percent = c.pct;
          hottest = c.label;
        }
      }
    }
    const pct = Math.round(percent);

    if (!any) return { mood: 'sleeping', line: 'Conecta un proveedor y despierto', percent: -1 };
    if (percent >= 100) return { mood: 'exhausted', line: `Límite alcanzado: ${hottest}`, percent };
    if (percent >= threshold) return { mood: 'worried', line: `Ojo, ${pct}% en ${hottest}`, percent };
    // Presupuestos por cliente (vienen ordenados del más consumido al menos).
    const tight = budgets.find((b) => b.percent >= 80);
    if (tight) {
      const line =
        tight.percent >= 100
          ? `${tight.name} superó su presupuesto del mes`
          : `${tight.name} va al ${Math.round(tight.percent)}% de su presupuesto`;
      return { mood: 'worried', line, percent };
    }
    if (updatingAll) return { mood: 'thinking', line: 'Consultando el consumo…', percent };
    if (failed) return { mood: 'confused', line: `No pude leer ${failed}`, percent };
    if (percent < 0) return { mood: 'sleeping', line: 'Aún no hay datos de uso', percent };
    if (percent < HAPPY_BELOW) return { mood: 'happy', line: `Vas sobrado: ${pct}%`, percent };
    return { mood: 'idle', line: `Todo tranquilo: ${pct}%`, percent };
  }, [meta, states, updatingAll, threshold, budgets]);
}

const INK = '#1e1b4b';

/** Color del distintivo de estado (la bolita de la esquina). */
const BADGE: Record<MascotMood, string> = {
  sleeping: '#71717a',
  idle: '#34d399',
  happy: '#34d399',
  thinking: '#38bdf8',
  worried: '#fbbf24',
  exhausted: '#fb7185',
  confused: '#fbbf24',
};

const stroke = {
  fill: 'none',
  stroke: INK,
  strokeWidth: 3.4,
  strokeLinecap: 'round' as const,
  strokeLinejoin: 'round' as const,
};

function Eyes({ mood, look }: { mood: MascotMood; look: [number, number] }) {
  if (mood === 'sleeping') {
    return (
      <g {...stroke}>
        <path d="M29 49 q6 5 12 0" />
        <path d="M59 49 q6 5 12 0" />
      </g>
    );
  }
  if (mood === 'happy') {
    return (
      <g {...stroke}>
        <path d="M29 51 q6 -8 12 0" />
        <path d="M59 51 q6 -8 12 0" />
      </g>
    );
  }
  if (mood === 'exhausted') {
    return (
      <g {...stroke}>
        <path d="M30 44 l10 10 M40 44 l-10 10" />
        <path d="M60 44 l10 10 M70 44 l-10 10" />
      </g>
    );
  }
  const [dx, dy] = mood === 'thinking' ? [1, -1] : look;
  return (
    <>
      <g className="kari-eyes" transform={`translate(${dx * 2.2} ${dy * 1.8})`}>
        <g className="kari-blink">
          <ellipse cx={35} cy={49} rx={4.4} ry={mood === 'confused' ? 4.4 : 6} fill={INK} />
          <ellipse cx={65} cy={49} rx={4.4} ry={mood === 'confused' ? 7.4 : 6} fill={INK} />
          <circle cx={36.4} cy={46.6} r={1.3} fill="#fff" />
          <circle cx={66.4} cy={46.6} r={1.3} fill="#fff" />
        </g>
      </g>
      {mood === 'worried' && (
        <g {...stroke} strokeWidth={3}>
          <path d="M27 40 l12 -4" />
          <path d="M73 40 l-12 -4" />
        </g>
      )}
    </>
  );
}

const MOUTH: Record<MascotMood, string> = {
  sleeping: 'M46 62 q4 2 8 0',
  idle: 'M44 60 q6 6 12 0',
  happy: 'M41 59 q9 11 18 0',
  thinking: 'M46 62 h9',
  worried: 'M43 64 q3.5 -5 7 0 q3.5 5 7 0',
  exhausted: 'M42 63 h16',
  confused: 'M44 63 q6 -5 12 1',
};

const STYLES = `
.kari { overflow: visible; }
.kari-eyes { transition: transform 140ms ease-out; }
.kari-body { transform-box: fill-box; transform-origin: 50% 100%; }
.kari-blink { transform-box: fill-box; transform-origin: 50% 50%; animation: kari-blink 5s infinite; }
.kari-idle .kari-body, .kari-thinking .kari-body, .kari-confused .kari-body { animation: kari-breathe 3.2s ease-in-out infinite; }
.kari-happy .kari-body { animation: kari-hop 1.2s ease-in-out infinite; }
.kari-worried .kari-body { animation: kari-shake 0.45s linear infinite; }
.kari-sleeping .kari-body { animation: kari-breathe 5.5s ease-in-out infinite; }
.kari-exhausted .kari-body { transform: scale(1.05, 0.9); filter: saturate(0.3) brightness(0.85); }
.kari-poke .kari-body { animation: kari-hop 0.45s ease-out 1; }
.kari-float { animation: kari-float 2.6s ease-in-out infinite; }
.kari-drop { animation: kari-drop 1.3s ease-in infinite; }
.kari-dots circle { animation: kari-dot 1.2s infinite; }
.kari-dots circle:nth-child(2) { animation-delay: 0.2s; }
.kari-dots circle:nth-child(3) { animation-delay: 0.4s; }
@keyframes kari-blink { 0%, 94%, 100% { transform: scaleY(1); } 97% { transform: scaleY(0.08); } }
@keyframes kari-breathe { 0%, 100% { transform: scale(1, 1); } 50% { transform: scale(1.025, 0.965); } }
@keyframes kari-hop { 0%, 100% { transform: translateY(0) scale(1, 1); } 35% { transform: translateY(-9px) scale(0.96, 1.05); } 70% { transform: translateY(0) scale(1.05, 0.93); } }
@keyframes kari-shake { 0%, 100% { transform: translateX(0); } 25% { transform: translateX(-1.4px); } 75% { transform: translateX(1.4px); } }
@keyframes kari-float { 0% { transform: translateY(4px); opacity: 0; } 30% { opacity: 1; } 100% { transform: translateY(-8px); opacity: 0; } }
@keyframes kari-drop { 0% { transform: translateY(0); opacity: 1; } 100% { transform: translateY(12px); opacity: 0; } }
@keyframes kari-dot { 0%, 100% { opacity: 0.25; } 40% { opacity: 1; } }
@media (prefers-reduced-motion: reduce) {
  .kari *, .kari { animation: none !important; transition: none !important; }
}
`;

/** El personaje. `size` es el ancho en píxeles de pantalla. */
export function Mascot({
  mood,
  size = 64,
  badge = true,
  className,
}: {
  mood: MascotMood;
  size?: number;
  /** Muestra la bolita de estado en la esquina. */
  badge?: boolean;
  className?: string;
}) {
  const ref = useRef<SVGSVGElement>(null);
  const uid = useId().replace(/:/g, '');
  const [look, setLook] = useState<[number, number]>([0, 0]);
  const [poke, setPoke] = useState(false);

  // Los ojos siguen al cursor.
  useEffect(() => {
    const onMove = (e: MouseEvent) => {
      const el = ref.current;
      if (!el) return;
      const r = el.getBoundingClientRect();
      const dx = e.clientX - (r.left + r.width / 2);
      const dy = e.clientY - (r.top + r.height / 2);
      const step = (v: number) => (Math.abs(v) < 20 ? 0 : v > 0 ? 1 : -1);
      setLook((prev) => {
        const next: [number, number] = [step(dx), step(dy)];
        return prev[0] === next[0] && prev[1] === next[1] ? prev : next;
      });
    };
    window.addEventListener('mousemove', onMove);
    return () => window.removeEventListener('mousemove', onMove);
  }, []);

  return (
    <svg
      ref={ref}
      viewBox="-4 -6 108 100"
      width={size}
      height={(size * 100) / 108}
      role="img"
      aria-label={`Kari, la mascota de Karina (${mood})`}
      onClick={() => {
        setPoke(true);
        window.setTimeout(() => setPoke(false), 460);
      }}
      style={{ filter: 'drop-shadow(0 0 9px rgb(167 139 250 / 0.45))' }}
      className={cn('kari cursor-pointer select-none', `kari-${mood}`, poke && 'kari-poke', className)}
    >
      <style>{STYLES}</style>
      <defs>
        <linearGradient id={`${uid}-body`} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor="#ddd6fe" />
          <stop offset="0.55" stopColor="#a78bfa" />
          <stop offset="1" stopColor="#7c5cf0" />
        </linearGradient>
        <clipPath id={`${uid}-clip`}>
          <rect x={6} y={20} width={88} height={68} rx={31} />
        </clipPath>
      </defs>

      <g className="kari-body">
        {/* Orejas */}
        <path d="M14 40 Q8 4 40 22 Z" fill={`url(#${uid}-body)`} />
        <path d="M86 40 Q92 4 60 22 Z" fill={`url(#${uid}-body)`} />
        <path d="M19 31 Q17 15 31 23 Z" fill="#f0abfc" opacity={0.75} />
        <path d="M81 31 Q83 15 69 23 Z" fill="#f0abfc" opacity={0.75} />
        {/* Cuerpo */}
        <rect x={6} y={20} width={88} height={68} rx={31} fill={`url(#${uid}-body)`} />
        <g clipPath={`url(#${uid}-clip)`}>
          <ellipse cx={50} cy={86} rx={33} ry={17} fill="#7dd3fc" opacity={0.9} />
          <ellipse cx={30} cy={30} rx={20} ry={8} fill="#fff" opacity={0.28} />
        </g>
        {(mood === 'happy' || mood === 'idle') && (
          <>
            <ellipse cx={23} cy={59} rx={5.5} ry={3.4} fill="#f0abfc" opacity={0.8} />
            <ellipse cx={77} cy={59} rx={5.5} ry={3.4} fill="#f0abfc" opacity={0.8} />
          </>
        )}
        <Eyes mood={mood} look={look} />
        <path d={MOUTH[mood]} {...stroke} strokeWidth={3} />
      </g>

      {mood === 'sleeping' && (
        <text className="kari-float" x={80} y={10} fontSize={16} fontWeight={700} fill="#a1a1aa">
          z
        </text>
      )}
      {mood === 'worried' && (
        <path className="kari-drop" d="M90 30 q-5 8 0 11 q5 -3 0 -11 Z" fill="#7dd3fc" />
      )}
      {mood === 'confused' && (
        <text x={82} y={14} fontSize={20} fontWeight={800} fill="#fbbf24">
          ?
        </text>
      )}

      {badge && (
        <g>
          <circle cx={90} cy={80} r={10} fill={BADGE[mood]} stroke="#0b0b0f" strokeWidth={3} />
          {mood === 'thinking' && (
            <g className="kari-dots" fill="#0b0b0f">
              <circle cx={85.6} cy={80} r={1.5} />
              <circle cx={90} cy={80} r={1.5} />
              <circle cx={94.4} cy={80} r={1.5} />
            </g>
          )}
        </g>
      )}
    </svg>
  );
}

/** Mascota con su frase, lista para colocar en la barra lateral. */
export function MascotCard({ className, size = 56 }: { className?: string; size?: number }) {
  const status = useMascotStatus();
  return (
    <div className={cn('flex items-center gap-3', className)}>
      <Mascot mood={status.mood} size={size} className="shrink-0" />
      <p className={cn('min-w-0 text-[11px] leading-snug', moodTextClass(status.mood))}>
        {status.line}
      </p>
    </div>
  );
}

/** Color del texto que acompaña a la mascota según su ánimo. */
export function moodTextClass(mood: MascotMood): string {
  return mood === 'exhausted'
    ? 'text-rose-300'
    : mood === 'worried' || mood === 'confused'
      ? 'text-amber-300'
      : 'text-zinc-400';
}

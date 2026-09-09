// Formatting helpers shared across the UI.

export function formatTokens(n: number): string {
  if (!Number.isFinite(n)) return '—';
  if (n >= 1_000_000) {
    const v = n / 1_000_000;
    return `${trim(v)}M`;
  }
  if (n >= 1000) {
    const v = n / 1000;
    return `${trim(v)}K`;
  }
  return `${Math.round(n)}`;
}

function trim(v: number): string {
  const rounded = Math.round(v * 10) / 10;
  return Number.isInteger(rounded) ? String(rounded) : String(rounded);
}

export function formatTokensFull(n: number): string {
  return Math.round(n).toLocaleString('en-US');
}

export function formatPercent(used: number, limit: number): number {
  if (!limit || limit <= 0) return -1;
  const p = (used / limit) * 100;
  return Math.min(100, Math.max(0, Math.round(p * 10) / 10));
}

export function formatMoney(n: number, currency?: string): string {
  const v = new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: currency || 'USD',
    maximumFractionDigits: 2,
  }).format(n);
  return v;
}

export function timeAgo(iso?: string, now = Date.now()): string {
  if (!iso) return 'nunca';
  const t = new Date(iso).getTime();
  if (!Number.isFinite(t)) return 'nunca';
  const diff = Math.max(0, now - t);
  const s = Math.floor(diff / 1000);
  if (s < 5) return 'ahora mismo';
  if (s < 60) return `hace ${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `hace ${m}m`;
  const h = Math.floor(m / 60);
  if (h < 24) return `hace ${h}h`;
  return `hace ${Math.floor(h / 24)}d`;
}

export function formatTime(iso?: string): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (!Number.isFinite(d.getTime())) return '—';
  return d.toLocaleTimeString('es-ES', {
    hour: 'numeric',
    minute: '2-digit',
    second: '2-digit',
  });
}

export function countdown(iso?: string, now = Date.now()): string | null {
  if (!iso) return null;
  const t = new Date(iso).getTime();
  if (!Number.isFinite(t)) return null;
  const diff = t - now;
  if (diff <= 0) return null;
  const h = Math.floor(diff / 3_600_000);
  const m = Math.floor((diff % 3_600_000) / 60_000);
  const s = Math.floor((diff % 60_000) / 1000);
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

export function clamp(n: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, n));
}

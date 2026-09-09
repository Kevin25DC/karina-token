import { useEffect, useState } from 'react';

/** Re-renders the caller every second so "x seconds ago" stays fresh. */
export function useNow(intervalMs = 1000): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), intervalMs);
    return () => window.clearInterval(id);
  }, [intervalMs]);
  return now;
}

/** True while `active` is false and at least `delayMs` of time have passed. */
export function useDelayed(active: boolean, delayMs = 140): boolean {
  const [shown, setShown] = useState(false);
  useEffect(() => {
    if (active) {
      setShown(true);
      return;
    }
    const id = window.setTimeout(() => setShown(false), delayMs);
    return () => window.clearTimeout(id);
  }, [active, delayMs]);
  return shown;
}

export function cn(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(' ');
}

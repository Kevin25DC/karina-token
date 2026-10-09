import { useEffect, useMemo } from 'react';
import {
  BarChart3,
  Plus,
  RefreshCw,
  Settings,
  SlidersHorizontal,
  Terminal,
} from 'lucide-react';
import { useStore, type View } from '@/store';
import { cn } from '@/lib/hooks';
import { Dashboard } from '@/pages/Dashboard';
import { History } from '@/pages/History';
import { ClaudeCodeUsage } from '@/pages/ClaudeCodeUsage';
import { Settings as SettingsPage } from '@/pages/Settings';
import { Onboarding } from '@/pages/Onboarding';
import { ConnectSheet } from '@/components/ConnectSheet';
import { AddProviderModal } from '@/components/AddProviderModal';
import { Toaster } from '@/components/Toaster';
import { WidgetMode } from '@/components/WidgetMode';
import { TitleBar } from '@/components/TitleBar';
import { Logo } from '@/components/Logo';
import { MascotCard } from '@/components/Mascot';
import { Kbd, Surface } from '@/components/primitives';

const NAV: Array<{ id: View; label: string; icon: typeof BarChart3 }> = [
  { id: 'dashboard', label: 'Panel', icon: SlidersHorizontal },
  { id: 'history', label: 'Historial de uso', icon: BarChart3 },
  { id: 'claudecode', label: 'Agentes de código', icon: Terminal },
  { id: 'settings', label: 'Ajustes', icon: Settings },
];

// Proveedores de Claude: añadir cualquiera habilita la sección de agentes.
const CLAUDE_PROVIDERS = ['anthropic', 'claude_subscription'];

export default function App() {
  // La sección de agentes de código (Claude Code, Codex CLI, Gemini CLI) solo
  // se ofrece si alguno se ha usado en este equipo o si el usuario
  // añadió Claude como proveedor.
  const showClaudeCode = useStore(
    (s) =>
      s.claudeCodeDetected ||
      s.meta.some((m) => m.enabled && CLAUDE_PROVIDERS.includes(m.id)),
  );
  const nav = useMemo(
    () => NAV.filter((item) => item.id !== 'claudecode' || showClaudeCode),
    [showClaudeCode],
  );
  const booted = useStore((s) => s.booted);
  const config = useStore((s) => s.config);
  const widgetMode = useStore((s) => s.widgetMode);
  const view = useStore((s) => s.view);
  const setView = useStore((s) => s.setView);
  const updatingAll = useStore((s) => s.updatingAll);
  const refreshing = useStore((s) => s.refreshing);
  const manualRefresh = useStore((s) => s.manualRefresh);
  const setAdding = useStore((s) => s.setAdding);
  const info = useStore((s) => s.info);
  const boot = useStore((s) => s.boot);

  useEffect(() => {
    void boot();
  }, [boot]);

  // Atajos de teclado globales.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      const typing =
        target &&
        (target.tagName === 'INPUT' ||
          target.tagName === 'TEXTAREA' ||
          target.isContentEditable);
      if (typing) return;
      const item = nav[Number(e.key) - 1];
      if (item) setView(item.id);
      else if (e.key === 'r' || e.key === 'R') void manualRefresh();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [nav, setView, manualRefresh]);

  // Si la sección deja de estar disponible (se quitó el proveedor), volver al panel.
  useEffect(() => {
    if (view === 'claudecode' && !showClaudeCode) setView('dashboard');
  }, [view, showClaudeCode, setView]);

  if (!booted || !config) {
    return (
      <Surface>
        <div className="flex h-full items-center justify-center">
          <div className="flex flex-col items-center gap-3">
            <Logo className="h-12 w-12 rounded-2xl animate-pulse" />
            <p className="text-xs text-zinc-600">Iniciando Karina…</p>
          </div>
        </div>
      </Surface>
    );
  }

  if (!config.onboarding_done) {
    return (
      <>
        <Surface className="relative">
          <Onboarding />
        </Surface>
        <ConnectSheet />
        <Toaster />
      </>
    );
  }

  if (widgetMode) {
    return (
      <>
        <WidgetMode />
        <Toaster />
      </>
    );
  }

  return (
    <>
      <Surface>
        <TitleBar />
        <div className="flex min-h-0 flex-1">
          <aside className="flex w-60 shrink-0 flex-col border-r border-white/[0.06] bg-ink-900/60">
            {/* Navegación */}
            <nav className="flex-1 space-y-1 px-3 pt-4">
          {nav.map((item, i) => {
            const Icon = item.icon;
            const active = view === item.id;
            return (
              <button
                key={item.id}
                onClick={() => setView(item.id)}
                className={cn(
                  'group flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition-all',
                  active
                    ? 'bg-white/[0.07] text-zinc-50 shadow-sm'
                    : 'text-zinc-400 hover:bg-white/[0.04] hover:text-zinc-100',
                )}
              >
                <Icon className={cn('h-4 w-4', active && 'text-violet-300')} />
                <span className="flex-1 text-left">{item.label}</span>
                <Kbd>{i + 1}</Kbd>
              </button>
            );
          })}
        </nav>

        {/* Acciones + pie */}
        <div className="space-y-3 px-4 pb-5 pt-3">
          <MascotCard className="rounded-xl border border-white/[0.06] bg-white/[0.02] px-3 py-2.5" />
          <div className="grid grid-cols-2 gap-2">
            <button
              onClick={() => void manualRefresh()}
              disabled={refreshing}
              title="Actualizar ahora (R)"
              className="flex items-center justify-center gap-2 rounded-xl border border-white/[0.07] bg-white/[0.03] py-2 text-xs font-medium text-zinc-300 transition-all hover:bg-white/[0.07] disabled:opacity-60"
            >
              <RefreshCw
                className={cn('h-3.5 w-3.5', (refreshing || updatingAll) && 'animate-spin')}
              />
              Actualizar
            </button>
            <button
              onClick={() => setAdding(true)}
              title="Añadir un proveedor"
              className="flex items-center justify-center gap-2 rounded-xl bg-zinc-100 py-2 text-xs font-semibold text-zinc-900 transition-all hover:bg-white"
            >
              <Plus className="h-3.5 w-3.5" /> Añadir
            </button>
          </div>
          <div className="flex items-center justify-between border-t border-white/[0.05] pt-3 text-[10px] text-zinc-600">
            <span className="inline-flex items-center gap-1.5">
              <span
                className={cn(
                  'h-1.5 w-1.5 rounded-full',
                  updatingAll ? 'bg-sky-400 animate-pulse-dot' : 'bg-emerald-400',
                )}
              />
              {updatingAll ? 'Actualizando' : 'En línea'}
            </span>
            <span>v{info?.version ?? '0.1.0'}</span>
          </div>
        </div>
      </aside>

      <main className="relative flex-1 overflow-y-auto bg-ink-950">
        {/* resplandor ambiental */}
        <div className="pointer-events-none absolute -top-24 right-0 h-64 w-96 rounded-full bg-violet-500/[0.05] blur-3xl" />
        <div className="relative">
          {view === 'dashboard' && <Dashboard />}
          {view === 'history' && <History />}
          {view === 'claudecode' && showClaudeCode && <ClaudeCodeUsage />}
          {view === 'settings' && <SettingsPage />}
        </div>
      </main>
        </div>
      </Surface>

      <ConnectSheet />
      <AddProviderModal />
      <Toaster />
      </>
  );
}

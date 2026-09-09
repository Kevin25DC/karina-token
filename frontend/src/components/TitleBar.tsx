import { Minus, PictureInPicture2, Square } from 'lucide-react';
import { WindowHide, WindowMinimise, WindowToggleMaximise } from '../../wailsjs/runtime/runtime';
import { api } from '@/lib/api';
import { Logo } from '@/components/Logo';

export function TitleBar() {
  return (
    <div className="titlebar flex h-10 shrink-0 items-center justify-between border-b border-white/[0.06] bg-ink-900/80 pl-4 pr-1.5">
      <div className="flex items-center gap-2.5">
        <Logo className="h-6 w-6 rounded-lg" />
        <span className="text-[13px] font-semibold tracking-tight text-zinc-100">Karina</span>
        <span className="ml-1 text-[10px] uppercase tracking-widest text-zinc-600">
          monitor de uso de IA
        </span>
      </div>
      <div className="titlebar-no-drag flex items-center gap-0.5">
        <button
          onClick={() => WindowMinimise()}
          title="Minimizar"
          className="rounded-md p-1.5 text-zinc-500 transition-colors hover:bg-white/[0.06] hover:text-zinc-200"
        >
          <Minus className="h-4 w-4" />
        </button>
        <button
          onClick={() => WindowToggleMaximise()}
          title="Maximizar / restaurar"
          className="rounded-md p-1.5 text-zinc-500 transition-colors hover:bg-white/[0.06] hover:text-zinc-200"
        >
          <Square className="h-3.5 w-3.5" />
        </button>
        <button
          onClick={() => api.enterWidgetMode().catch(() => undefined)}
          title="Modo widget (mini, fijo en la esquina)"
          className="rounded-md p-1.5 text-zinc-500 transition-colors hover:bg-white/[0.06] hover:text-zinc-200"
        >
          <PictureInPicture2 className="h-4 w-4" />
        </button>
        <button
          onClick={() => WindowHide()}
          title="Ocultar a la bandeja (Karina sigue en segundo plano)"
          className="rounded-md p-1.5 text-zinc-500 transition-colors hover:bg-rose-500/15 hover:text-rose-200"
        >
          <span className="text-[16px] leading-none">×</span>
        </button>
      </div>
    </div>
  );
}

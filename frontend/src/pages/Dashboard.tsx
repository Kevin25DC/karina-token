import { Plug, RefreshCw, ShieldCheck, Zap } from 'lucide-react';
import { useStore } from '@/store';
import { ProviderCard } from '@/components/ProviderCard';
import { Button, Spinner } from '@/components/primitives';
import { Logo } from '@/components/Logo';

export function Dashboard() {
  const meta = useStore((s) => s.meta);
  const states = useStore((s) => s.states);
  const updatingAll = useStore((s) => s.updatingAll);
  const setAdding = useStore((s) => s.setAdding);
  const config = useStore((s) => s.config);

  const enabled = meta.filter((m) => m.enabled);

  return (
    <div className="mx-auto w-full max-w-6xl px-8 pb-14">
      <header className="flex flex-wrap items-end justify-between gap-4 py-8">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-zinc-50">Uso de IA</h1>
          <p className="mt-1 text-sm text-zinc-500">
            Tu consumo actual de IA en todos tus proveedores
          </p>
        </div>
        <div className="flex items-center gap-2 text-xs text-zinc-500">
          {updatingAll && (
            <span className="inline-flex items-center gap-1.5 text-sky-300">
              <Spinner className="h-3.5 w-3.5" /> Actualizando…
            </span>
          )}
          {config && (
            <span className="inline-flex items-center gap-1.5">
              <Zap className="h-3.5 w-3.5" />
              Auto-actualización cada {config.refresh_interval_seconds}s
            </span>
          )}
          <span
            className="inline-flex items-center gap-1.5"
            title="Solo se muestran datos reales que expone cada proveedor; lo que no está disponible se indica, nunca se inventa."
          >
            <ShieldCheck className="h-3.5 w-3.5" />
            Actualización automática
          </span>
        </div>
      </header>

      {enabled.length === 0 ? (
        <EmptyDashboard onAdd={() => setAdding(true)} />
      ) : (
        <div className="grid grid-cols-1 gap-5 md:grid-cols-2 2xl:grid-cols-3">
          {enabled.map((m) => (
            <ProviderCard key={m.id} meta={m} state={states[m.id]} />
          ))}
        </div>
      )}
    </div>
  );
}

function EmptyDashboard({ onAdd }: { onAdd: () => void }) {
  return (
    <div className="flex flex-col items-center justify-center rounded-3xl border border-dashed border-white/[0.08] py-24 text-center animate-fade-in">
      <div className="relative mb-6">
        <div className="absolute -inset-8 rounded-full bg-violet-500/10 blur-2xl" />
        <Logo className="relative h-16 w-16 rounded-2xl shadow-glow" />
      </div>
      <h2 className="text-lg font-semibold text-zinc-100">No hay proveedores conectados</h2>
      <p className="mt-1.5 max-w-sm text-sm leading-relaxed text-zinc-500">
        Conecta un proveedor de IA para empezar a monitorizar su consumo real de
        tokens, límites de peticiones y facturación en tiempo casi real.
      </p>
      <div className="mt-7 flex items-center gap-3">
        <Button onClick={onAdd} className="h-10 px-4">
          <Plug className="h-4 w-4" /> Añadir proveedor
        </Button>
        <Button
          variant="ghost"
          className="h-10 px-4"
          onClick={() => useStore.getState().openConnect('demo')}
        >
          <RefreshCw className="h-4 w-4 text-violet-300" /> Previsualizar con datos demo
        </Button>
      </div>
    </div>
  );
}

import { useState } from 'react';
import { Activity, ArrowRight, ShieldCheck, Sparkles } from 'lucide-react';
import { api } from '@/lib/api';
import { useStore } from '@/store';
import { Button } from '@/components/primitives';
import { Logo } from '@/components/Logo';
import { ProviderMark } from '@/components/ProviderMark';
import { cn } from '@/lib/hooks';

export function Onboarding() {
  const meta = useStore((s) => s.meta);
  const reload = useStore((s) => s.reload);
  const openConnect = useStore((s) => s.openConnect);
  const [step, setStep] = useState(0);

  const enabledCount = meta.filter((m) => m.enabled).length;

  async function finish() {
    try {
      await api.completeOnboarding();
      await reload();
    } catch {
      /* se continúa aunque el guardado del avance falle */
    }
  }

  return (
    <div className="absolute inset-0 overflow-y-auto">
      {/* resplandores ambientales */}
      <div className="pointer-events-none absolute -top-32 left-1/2 h-96 w-[42rem] -translate-x-1/2 rounded-full bg-violet-500/[0.12] blur-3xl" />
      <div className="pointer-events-none absolute bottom-0 right-0 h-72 w-96 rounded-full bg-sky-500/[0.07] blur-3xl" />

      <div className="relative mx-auto flex min-h-full w-full max-w-2xl flex-col items-center justify-center px-8 py-14">
        <Logo className="mb-6 h-14 w-14 rounded-2xl shadow-glow" />

        {step === 0 ? (
          <div className="w-full animate-rise-in text-center">
            <h1 className="text-3xl font-semibold tracking-tight text-zinc-50">
              Bienvenido a <span className="text-violet-300">Karina</span>
            </h1>
            <p className="mx-auto mt-3 max-w-md text-[15px] leading-relaxed text-zinc-400">
              Monitorea tu uso de IA en un solo lugar — consumo real de tokens,
              límites de peticiones en vivo y facturación de cada proveedor,
              actualizado automáticamente.
            </p>
            <div className="mx-auto mt-8 grid max-w-md grid-cols-1 gap-3 text-left sm:grid-cols-3">
              <Feature
                icon="real"
                title="Datos reales"
                desc="Nunca inventados. Si el proveedor no puede reportarlos, lo decimos."
              />
              <Feature
                icon="local"
                title="Local-first"
                desc="Las claves viven en tu llavero del sistema. Nada sale de tu equipo."
              />
              <Feature
                icon="live"
                title="Casi en tiempo real"
                desc="Consultas configurables con retroceso elegante ante errores."
              />
            </div>
            <Button onClick={() => setStep(1)} className="mt-9 h-11 px-6 text-sm">
              Comenzar <ArrowRight className="h-4 w-4" />
            </Button>
          </div>
        ) : (
          <div className="w-full animate-rise-in">
            <div className="flex items-center justify-between">
              <div>
                <h1 className="text-2xl font-semibold tracking-tight text-zinc-50">
                  Conecta tus proveedores de IA
                </h1>
                <p className="mt-1 text-sm text-zinc-500">
                  Añade uno, varios o ninguno — siempre podrás cambiarlo después.
                </p>
              </div>
              <span className="rounded-full border border-white/[0.08] bg-white/[0.03] px-3 py-1 text-xs text-zinc-400">
                {enabledCount} conectado{enabledCount === 1 ? '' : 's'}
              </span>
            </div>

            <div className="mt-7 space-y-3">
              {meta.map((m) => (
                <button
                  key={m.id}
                  onClick={() => openConnect(m.id)}
                  className={cn(
                    'group flex w-full items-center gap-4 rounded-2xl border p-4 text-left transition-all',
                    m.enabled
                      ? 'border-emerald-400/15 bg-emerald-400/[0.04]'
                      : 'border-white/[0.07] bg-white/[0.02] hover:border-white/[0.14] hover:bg-white/[0.05]',
                  )}
                >
                  <ProviderMark id={m.id} brand={m.brand} size="lg" />
                  <div className="min-w-0 flex-1">
                    <p className="flex items-center gap-2 text-[15px] font-semibold text-zinc-100">
                      {m.name}
                      {m.demo && (
                        <span className="rounded-full border border-violet-400/20 bg-violet-400/10 px-2 py-0.5 text-[10px] font-medium text-violet-300">
                          Demo
                        </span>
                      )}
                    </p>
                    <p className="mt-1 text-xs leading-relaxed text-zinc-500">
                      {m.demo
                        ? 'Datos simulados para previsualizar Karina.'
                        : m.description}
                    </p>
                  </div>
                  {m.enabled ? (
                    <span className="rounded-full border border-emerald-400/20 bg-emerald-400/10 px-3 py-1 text-xs font-medium text-emerald-300">
                      Conectado
                    </span>
                  ) : (
                    <span className="rounded-full border border-white/[0.1] px-3 py-1 text-xs text-zinc-400 transition-colors group-hover:border-white/[0.2] group-hover:text-zinc-200">
                      Conectar
                    </span>
                  )}
                </button>
              ))}
            </div>

            <div className="mt-7 flex flex-wrap items-center justify-between gap-3">
              <button
                onClick={() => setStep(0)}
                className="text-xs text-zinc-500 transition-colors hover:text-zinc-300"
              >
                Atrás
              </button>
              <Button onClick={() => void finish()} className="h-10 px-5">
                {enabledCount ? 'Continuar al panel' : 'Omitir por ahora'}{' '}
                <ArrowRight className="h-4 w-4" />
              </Button>
            </div>
            <p className="mt-6 flex items-center justify-center gap-1.5 text-center text-[11px] text-zinc-600">
              <ShieldCheck className="h-3.5 w-3.5" /> Las claves se guardan en el
              administrador de credenciales de tu sistema y solo se envían al
              proveedor que conectes.
            </p>
          </div>
        )}
      </div>
    </div>
  );
}

function Feature({ icon, title, desc }: { icon: string; title: string; desc: string }) {
  return (
    <div className="rounded-xl border border-white/[0.06] bg-white/[0.02] p-3.5">
      {icon === 'real' && <Sparkles className="h-4 w-4 text-amber-300" />}
      {icon === 'local' && <ShieldCheck className="h-4 w-4 text-emerald-300" />}
      {icon === 'live' && <Activity className="h-4 w-4 text-sky-300" />}
      <p className="mt-2 text-sm font-semibold text-zinc-200">{title}</p>
      <p className="mt-1 text-xs leading-relaxed text-zinc-500">{desc}</p>
    </div>
  );
}

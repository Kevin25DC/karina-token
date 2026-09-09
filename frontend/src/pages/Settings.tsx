import { useState, type ReactNode } from 'react';
import {
  ExternalLink,
  Folder,
  Gauge,
  KeyRound,
  PictureInPicture2,
  Power,
  ShieldCheck,
} from 'lucide-react';
import { api } from '@/lib/api';
import { useStore } from '@/store';
import { cn } from '@/lib/hooks';
import type { ProviderMeta, ProviderState } from '@/lib/types';
import { Badge, Button, Card } from '@/components/primitives';
import { Logo } from '@/components/Logo';
import { ProviderMark } from '@/components/ProviderMark';
import { STATUS_COLORS } from '@/lib/theme';

const INTERVALS = [10, 15, 30, 60, 120, 300];

const CAP_LABEL: Record<string, string> = {
  token_usage: 'Uso de tokens',
  rate_limits: 'Límites de peticiones',
  billing: 'Facturación',
};

const CAP_NOTES: Record<string, string> = {
  anthropic:
    'Claves estándar: conexión + límites de peticiones en vivo. El uso de tokens requiere una clave Admin (reporte de uso de la organización). Sin API de facturación.',
  openai:
    'Claves estándar: conexión + límites de peticiones en vivo. El uso de tokens requiere una clave Admin (uso de la organización). Sin API de facturación para claves estándar.',
  gemini:
    'Solo conexión. La API de Gemini no expone endpoints de uso, cuota ni facturación para claves de API.',
  deepseek:
    'Conexión + saldo monetario prepagado (facturación). No existe endpoint de uso de tokens.',
  demo: 'Uso simulado para previsualizar la app. No es un proveedor real.',
};

export function Settings() {
  const config = useStore((s) => s.config);
  const meta = useStore((s) => s.meta);
  const info = useStore((s) => s.info);
  const reload = useStore((s) => s.reload);
  const notify = useStore((s) => s.notify);
  const openConnect = useStore((s) => s.openConnect);
  const states = useStore((s) => s.states);

  const [savingInterval, setSavingInterval] = useState(false);

  async function changeInterval(seconds: number) {
    setSavingInterval(true);
    try {
      await api.setRefreshInterval(seconds);
      await reload();
      notify('success', `Auto-actualización configurada cada ${seconds}s`);
    } catch (e) {
      notify('error', (e as Error).message);
    } finally {
      setSavingInterval(false);
    }
  }

  async function toggleStartup() {
    const next = !config?.start_with_system;
    try {
      await api.setStartWithSystem(next);
      await reload();
      notify(
        'success',
        next ? 'Karina se iniciará junto con el sistema' : 'Inicio automático desactivado',
      );
    } catch (e) {
      notify('error', (e as Error).message);
    }
  }

  return (
    <div className="mx-auto w-full max-w-4xl px-8 pb-14">
      <header className="py-8">
        <h1 className="text-2xl font-semibold tracking-tight text-zinc-50">Ajustes</h1>
        <p className="mt-1 text-sm text-zinc-500">General, proveedores y seguridad</p>
      </header>

      <div className="space-y-6">
        {/* General */}
        <section>
          <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-zinc-500">
            General
          </h2>
          <Card className="divide-y divide-white/[0.05] p-1">
            <Row
              icon={<Gauge className="h-4 w-4" />}
              title="Intervalo de auto-actualización"
              desc="Cada cuánto consulta Karina a tus proveedores. Sé amable con las APIs — para solo-monitoreo, mejor un valor alto."
            >
              <div className="flex items-center gap-1 rounded-xl border border-white/[0.06] bg-white/[0.02] p-1">
                {INTERVALS.map((s) => (
                  <button
                    key={s}
                    disabled={savingInterval}
                    onClick={() => void changeInterval(s)}
                    className={cn(
                      'rounded-lg px-2.5 py-1.5 font-mono text-xs transition-all disabled:opacity-50',
                      config?.refresh_interval_seconds === s
                        ? 'bg-white/[0.12] text-zinc-100'
                        : 'text-zinc-500 hover:text-zinc-200',
                    )}
                  >
                    {s < 60 ? `${s}s` : `${s / 60}m`}
                  </button>
                ))}
              </div>
            </Row>
            <Row
              icon={<Power className="h-4 w-4" />}
              title="Iniciar Karina al arrancar el sistema"
              desc="Registra una entrada de auto-inicio por usuario (sin permisos de administrador)."
            >
              <Toggle
                on={config?.start_with_system ?? false}
                onChange={() => void toggleStartup()}
              />
            </Row>
            <Row
              icon={<PictureInPicture2 className="h-4 w-4" />}
              title="Modo widget (mini)"
              desc="Ventana compacta, siempre visible y fija en una esquina, con las barras de porcentaje de consumo."
            >
              <Button
                className="h-9"
                onClick={() => {
                  useStore.getState().setWidgetMode(true);
                  void api.enterWidgetMode().catch(() => undefined);
                }}
              >
                <PictureInPicture2 className="h-3.5 w-3.5" /> Activar widget
              </Button>
            </Row>
            <Row
              icon={<Folder className="h-4 w-4" />}
              title="Directorio de datos"
              desc="Dónde se guardan la configuración y las observaciones locales de historial."
            >
              <span className="max-w-[280px] truncate font-mono text-xs text-zinc-500">
                {config?.data_dir}
              </span>
            </Row>
          </Card>
        </section>

        {/* Seguridad */}
        <section>
          <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-zinc-500">
            Seguridad
          </h2>
          <Card className="flex items-start gap-3 p-5">
            <div className="rounded-xl border border-emerald-400/20 bg-emerald-400/10 p-2 text-emerald-300">
              <ShieldCheck className="h-4 w-4" />
            </div>
            <div className="text-sm leading-relaxed text-zinc-400">
              <p className="font-medium text-zinc-200">Las claves se quedan en este equipo</p>
              <p className="mt-1 text-xs leading-relaxed text-zinc-500">
                Las API keys se guardan en el almacén seguro del sistema (Administrador
                de credenciales de Windows / Llavero de macOS / Secret Service en Linux),
                nunca en archivos de configuración, logs ni el frontend. Las peticiones van
                directamente de Karina a cada proveedor.
              </p>
            </div>
          </Card>
        </section>

        {/* Proveedores */}
        <section>
          <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-zinc-500">
            Proveedores
          </h2>
          <div className="space-y-3">
            {meta.map((m) => (
              <ProviderSetting
                key={m.id}
                meta={m}
                state={states[m.id]}
                onManage={() => openConnect(m.id)}
              />
            ))}
          </div>
        </section>

        {/* Acerca de */}
        <section>
          <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-zinc-500">
            Acerca de
          </h2>
          <Card className="flex items-center gap-3 p-5">
            <Logo className="h-10 w-10 rounded-xl" />
            <div className="flex-1">
              <p className="text-sm font-semibold text-zinc-100">
                Karina{' '}
                <span className="font-normal text-zinc-500">v{info?.version}</span>
              </p>
              <p className="text-xs text-zinc-500">
                Monitor de uso de IA en tiempo real · local-first · Go + Wails
              </p>
            </div>
            <Badge>
              <KeyRound className="h-3 w-3" /> Almacén seguro del SO
            </Badge>
          </Card>
        </section>
      </div>
    </div>
  );
}

function Row({
  icon,
  title,
  desc,
  children,
}: {
  icon: ReactNode;
  title: string;
  desc: string;
  children: ReactNode;
}) {
  return (
    <div className="flex items-center justify-between gap-6 p-4">
      <div className="flex min-w-0 items-start gap-3.5">
        <div className="mt-0.5 text-zinc-500">{icon}</div>
        <div className="min-w-0">
          <p className="text-sm font-medium text-zinc-100">{title}</p>
          <p className="mt-0.5 text-xs leading-relaxed text-zinc-500">{desc}</p>
        </div>
      </div>
      <div className="shrink-0">{children}</div>
    </div>
  );
}

function Toggle({ on, onChange }: { on: boolean; onChange: () => void }) {
  return (
    <button
      role="switch"
      aria-checked={on}
      onClick={onChange}
      className={cn(
        'relative h-6 w-11 rounded-full transition-colors duration-200',
        on ? 'bg-emerald-400/80' : 'bg-white/[0.1]',
      )}
    >
      <span
        className={cn(
          'absolute top-0.5 h-5 w-5 rounded-full bg-white shadow transition-all duration-200',
          on ? 'left-[22px]' : 'left-0.5',
        )}
      />
    </button>
  );
}

function ProviderSetting({
  meta,
  state,
  onManage,
}: {
  meta: ProviderMeta;
  state?: ProviderState;
  onManage: () => void;
}) {
  const sc = STATUS_COLORS[state?.status ?? 'disconnected'] ?? STATUS_COLORS.disconnected;
  const statusText = !state
    ? meta.enabled
      ? 'Conectando…'
      : 'Sin conectar'
    : state.status === 'connected'
      ? 'Conectado'
      : state.status_msg || state.status;

  return (
    <Card className="flex flex-col gap-4 p-4 sm:flex-row sm:items-center">
      <ProviderMark id={meta.id} brand={meta.brand} />
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-sm font-semibold text-zinc-100">{meta.name}</p>
          {meta.has_key && meta.enabled && (
            <span className={cn('inline-flex items-center gap-1.5 text-[11px]', sc.text)}>
              <span className={cn('h-1.5 w-1.5 rounded-full', sc.dot)} /> {statusText}
            </span>
          )}
        </div>
        <p className="mt-1 text-xs leading-relaxed text-zinc-500">
          {CAP_NOTES[meta.id] ?? meta.description}
        </p>
        <div className="mt-2 flex flex-wrap items-center gap-1.5">
          {(meta.capabilities.length ? meta.capabilities : []).map((c) => (
            <Badge key={c} className="border-white/[0.05] text-[10px] text-zinc-500">
              {CAP_LABEL[c] ?? c}
            </Badge>
          ))}
          {meta.capabilities.length === 0 && !meta.demo && (
            <Badge className="border-white/[0.05] text-[10px] text-zinc-500">
              Solo conexión
            </Badge>
          )}
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        {meta.docs_url && (
          <a
            href={meta.docs_url}
            target="_blank"
            rel="noreferrer"
            className="rounded-lg p-2 text-zinc-500 transition-colors hover:text-zinc-200"
            title="Documentación del proveedor"
          >
            <ExternalLink className="h-4 w-4" />
          </a>
        )}
        <Button
          variant={meta.has_key ? 'secondary' : 'primary'}
          onClick={onManage}
          className="h-9"
        >
          {meta.has_key ? 'Administrar' : 'Conectar'}
        </Button>
      </div>
    </Card>
  );
}

import { useState } from 'react';
import { Check, ExternalLink, KeyRound, Plug, Sparkles, Unplug } from 'lucide-react';
import { api } from '@/lib/api';
import { useStore } from '@/store';
import type { ProviderMeta, ProviderState } from '@/lib/types';
import { cn } from '@/lib/hooks';
import { Button, Modal } from './primitives';
import { ProviderMark } from './ProviderMark';

const KEY_HINTS: Record<string, { prefix: string; tip: string }> = {
  anthropic: {
    prefix: 'sk-ant-…',
    tip: 'Crea una clave en la consola de Anthropic (platform.claude.com/settings/keys). Para leer el uso de tokens de tu organización necesitas una Admin API key (sk-ant-admin…).',
  },
  openai: {
    prefix: 'sk-… o sk-proj-…',
    tip: 'Crea una clave en platform.openai.com/api-keys. Las claves estándar validan y exponen límites de peticiones en vivo; para leer el uso de la organización se requiere una OpenAI Admin API key.',
  },
  gemini: {
    prefix: 'AIza…',
    tip: 'Crea una clave en Google AI Studio (aistudio.google.com/apikey). La API de Gemini no expone endpoints de uso/cuota para claves de API — el uso solo se ve en la consola de AI Studio / Cloud.',
  },
  deepseek: {
    prefix: 'sk-…',
    tip: 'Crea una clave en platform.deepseek.com. DeepSeek solo expone tu saldo monetario prepagado vía API, no el consumo de tokens.',
  },
  demo: {
    prefix: 'no requiere clave',
    tip: 'El modo demo genera uso simulado para que explores Karina antes de conectar un proveedor real.',
  },
};

export function ConnectSheet() {
  const connectFor = useStore((s) => s.connectFor);
  const openConnect = useStore((s) => s.openConnect);
  const reload = useStore((s) => s.reload);
  const meta = useStore((s) =>
    connectFor ? s.meta.find((m) => m.id === connectFor) ?? null : null,
  );

  const open = meta !== null && connectFor === meta.id;
  const onClose = () => openConnect(null);

  return (
    <Modal open={open} onClose={onClose} title={meta ? `Conectar ${meta.name}` : ''} wide>
      {meta && (
        <SheetBody
          key={meta.id}
          meta={meta}
          onClose={onClose}
          onDone={reload}
          notify={useStore.getState().notify}
        />
      )}
    </Modal>
  );
}

function SheetBody({
  meta,
  onClose,
  onDone,
  notify,
}: {
  meta: ProviderMeta;
  onClose: () => void;
  onDone: () => Promise<void> | void;
  notify: (kind: 'success' | 'error' | 'info', msg: string) => void;
}) {
  const connected = meta.has_key;
  const preview = useStore((s) => s.redacted);
  const [key, setKey] = useState('');
  const [testing, setTesting] = useState(false);
  const [result, setResult] = useState<ProviderState | null>(null);
  const [ok, setOk] = useState(false);
  const [saving, setSaving] = useState(false);

  const hint = KEY_HINTS[meta.id] ?? KEY_HINTS.demo;

  if (meta.manual) {
    return <ManualBody meta={meta} onClose={onClose} onDone={onDone} notify={notify} />;
  }

  async function test() {
    setTesting(true);
    setResult(null);
    setOk(false);
    try {
      const res = await api.testProvider(meta.id, key.trim());
      setResult(res.state);
      setOk(res.ok);
    } catch (e) {
      notify('error', (e as Error).message);
    } finally {
      setTesting(false);
    }
  }

  async function save() {
    setSaving(true);
    try {
      if (meta.id === 'demo') {
        await api.setProviderEnabled('demo', true);
      } else {
        await api.saveProviderKey(meta.id, key.trim());
      }
      await onDone();
      notify('success', `${meta.name} conectado`);
      onClose();
    } catch (e) {
      notify('error', (e as Error).message);
    } finally {
      setSaving(false);
    }
  }

  async function remove() {
    try {
      await api.removeProvider(meta.id);
      await onDone();
      notify('info', `${meta.name} desconectado`);
      onClose();
    } catch (e) {
      notify('error', (e as Error).message);
    }
  }

  const canSave = meta.id === 'demo' ? true : ok && key.trim().length > 0;

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 rounded-xl border border-white/[0.06] bg-white/[0.02] p-3.5">
        <ProviderMark id={meta.id} brand={meta.brand} size="md" />
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium text-zinc-100">{meta.name}</p>
          <p className="text-xs text-zinc-500">{meta.description}</p>
        </div>
        {connected && !meta.demo && (
          <span className="inline-flex items-center gap-1.5 rounded-full border border-emerald-400/20 bg-emerald-400/10 px-2.5 py-1 text-[11px] font-medium text-emerald-300">
            <span className="h-1.5 w-1.5 rounded-full bg-emerald-400" /> Conectado
            {preview[meta.id] && (
              <span className="font-mono text-emerald-400/70">({preview[meta.id]})</span>
            )}
          </span>
        )}
      </div>

      <div>
        <label className="mb-1.5 block text-xs font-medium text-zinc-400">Clave API</label>
        <div className="flex items-center gap-2">
          <div className="relative flex-1">
            <KeyRound className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-500" />
            <input
              type="password"
              autoFocus
              value={key}
              onChange={(e) => {
                setKey(e.target.value);
                setResult(null);
                setOk(false);
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && key.trim() && !testing) void test();
              }}
              placeholder={hint.prefix}
              spellCheck={false}
              autoComplete="off"
              className="w-full rounded-xl border border-white/[0.08] bg-white/[0.03] py-2.5 pl-10 pr-3 font-mono text-sm text-zinc-100 placeholder:text-zinc-600 outline-none transition-colors focus:border-white/20 focus:bg-white/[0.05]"
            />
          </div>
          <Button onClick={() => void test()} loading={testing} className="h-11">
            Probar conexión
          </Button>
        </div>
        <p className="mt-2 flex gap-1.5 text-xs leading-relaxed text-zinc-500">
          <span className="mt-0.5 text-zinc-600">•</span>
          {hint.tip}
        </p>
      </div>

      {result && (
        <div
          className={cn(
            'flex animate-rise-in items-start gap-2.5 rounded-xl border p-3.5',
            ok
              ? 'border-emerald-400/20 bg-emerald-400/[0.06]'
              : 'border-rose-400/20 bg-rose-500/[0.06]',
          )}
        >
          <span
            className={cn('mt-1 h-2 w-2 rounded-full', ok ? 'bg-emerald-400' : 'bg-rose-400')}
          />
          <div className="min-w-0">
            <p className={cn('text-[13px] font-medium', ok ? 'text-emerald-200' : 'text-rose-200')}>
              {ok ? 'Conectado correctamente' : result.status_msg || 'Falló la conexión'}
            </p>
            {!ok && result.error && (
              <p className="mt-1 text-xs leading-relaxed text-zinc-400">{result.error}</p>
            )}
            {ok && result.note && (
              <p className="mt-1 text-xs leading-relaxed text-zinc-400">{result.note}</p>
            )}
          </div>
        </div>
      )}

      <div className="flex items-center justify-between gap-3 border-t border-white/[0.06] pt-4">
        <div className="flex items-center gap-1">
          {connected && !meta.demo && (
            <Button variant="danger" className="h-9 text-xs" onClick={() => void remove()}>
              <Unplug className="h-3.5 w-3.5" /> Desconectar
            </Button>
          )}
        </div>
        <div className="flex items-center gap-2">
          {meta.docs_url && (
            <a
              href={meta.docs_url}
              target="_blank"
              rel="noreferrer"
              className="inline-flex h-9 items-center gap-1.5 rounded-lg px-2 text-xs text-zinc-500 transition-colors hover:text-zinc-200"
            >
              <ExternalLink className="h-3.5 w-3.5" /> Documentación
            </a>
          )}
          <Button variant="ghost" onClick={onClose} className="h-9 text-xs">
            Cancelar
          </Button>
          {meta.demo ? (
            <Button onClick={() => void save()} loading={saving} className="h-9">
              <Plug className="h-3.5 w-3.5" /> Activar datos demo
            </Button>
          ) : (
            <Button
              onClick={() => void save()}
              disabled={!canSave}
              loading={saving}
              className="h-9"
            >
              <Check className="h-3.5 w-3.5" />
              {connected ? 'Guardar clave' : 'Conectar'}
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}

// ManualBody lets the user record the usage percentage shown by claude.ai for
// their Pro/Max subscription (there is no official API for it).
const MANUAL_WINDOWS = [
  'Ventana de 5 horas (sesión)',
  'Ventana semanal (7 días)',
];

function ManualBody({
  meta,
  onClose,
  onDone,
  notify,
}: {
  meta: ProviderMeta;
  onClose: () => void;
  onDone: () => Promise<void> | void;
  notify: (kind: 'success' | 'error' | 'info', msg: string) => void;
}) {
  const states = useStore((s) => s.states);
  const existing = states[meta.id];
  const initial = existing?.usage_available
    ? Math.min(100, existing.used_tokens)
    : 50;

  const [pct, setPct] = useState<number>(initial);
  const [windowLabel, setWindowLabel] = useState<string>(
    existing?.usage_window || MANUAL_WINDOWS[0],
  );
  const [saving, setSaving] = useState(false);
  const [autoLoading, setAutoLoading] = useState(false);
  const [autoMsg, setAutoMsg] = useState<string | null>(null);
  const [autoErr, setAutoErr] = useState<string | null>(null);

  async function tryAuto() {
    setAutoLoading(true);
    setAutoMsg(null);
    setAutoErr(null);
    try {
      const res = await api.experimentalClaudeSubscription();
      if (res.found) {
        const used = Math.max(0, Math.min(100, Math.round(res.used)));
        setPct(used);
        if (res.window) {
          const w = String(res.window);
          if (/7/.test(w)) setWindowLabel(MANUAL_WINDOWS[1]);
          else setWindowLabel(MANUAL_WINDOWS[0]);
        }
        setAutoMsg(
          `Lectura automática obtenida: ${used}% (ventana ${res.window || '?'}, origen: ${res.source}). Revisa y guarda.`,
        );
      } else {
        setAutoErr(res.error || 'No se pudo leer automáticamente.');
      }
    } catch (e) {
      setAutoErr((e as Error).message);
    } finally {
      setAutoLoading(false);
    }
  }

  async function save() {
    setSaving(true);
    try {
      await api.setManualUsage(meta.id, Math.round(pct), 100, windowLabel);
      await onDone();
      notify('success', 'Lectura guardada en el historial');
      onClose();
    } catch (e) {
      notify('error', (e as Error).message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 rounded-xl border border-white/[0.06] bg-white/[0.02] p-3.5">
        <ProviderMark id={meta.id} brand={meta.brand} size="md" />
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium text-zinc-100">{meta.name}</p>
          <p className="text-xs text-zinc-500">{meta.description}</p>
        </div>
      </div>

      <div className="rounded-xl border border-amber-400/15 bg-amber-400/[0.06] p-3.5 text-xs leading-relaxed text-amber-100/80">
        claude.ai <strong>no expone una API oficial</strong> para leer el uso de la
        suscripción. Introduce aquí el porcentaje que ves en tu cuenta de claude.ai
        (Ajustes → Uso). Karina lo registra como lectura manual y lo guarda en el
        historial. No toca tu sesión del navegador.
      </div>

      <div className="rounded-xl border border-violet-400/15 bg-violet-500/[0.06] p-3.5">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="text-xs font-medium text-violet-100/90">
            Lectura automática (experimental)
          </p>
          <Button
            variant="secondary"
            className="h-8 px-3 text-xs"
            loading={autoLoading}
            onClick={() => void tryAuto()}
          >
            <Sparkles className="h-3.5 w-3.5" /> Probar con Claude Code
          </Button>
        </div>
        <p className="mt-1.5 text-[11px] leading-relaxed text-violet-100/60">
          Experimental: reutiliza la sesión de Claude Code en este equipo
          (requiere haber iniciado sesión una vez con <code className="font-mono">claude /login</code>).
          Endpoint no oficial: puede fallar o cambiar; úsalo bajo tu responsabilidad.
        </p>
        {autoMsg && (
          <p className="mt-2 text-[11px] font-medium text-emerald-300">{autoMsg}</p>
        )}
        {autoErr && (
          <p className="mt-2 text-[11px] text-amber-300">{autoErr}</p>
        )}
      </div>

      <div>
        <label className="mb-1.5 block text-xs font-medium text-zinc-400">
          Ventana de uso
        </label>
        <select
          value={windowLabel}
          onChange={(e) => setWindowLabel(e.target.value)}
          className="w-full rounded-xl border border-white/[0.08] bg-white/[0.03] px-3 py-2.5 text-sm text-zinc-100 outline-none transition-colors focus:border-white/20"
        >
          {MANUAL_WINDOWS.map((w) => (
            <option key={w} value={w} className="bg-ink-800">
              {w}
            </option>
          ))}
        </select>
      </div>

      <div>
        <div className="mb-2 flex items-end justify-between">
          <label className="text-xs font-medium text-zinc-400">Uso actual</label>
          <p className="font-mono text-2xl font-semibold text-zinc-50">
            {Math.round(pct)}
            <span className="text-base text-zinc-500">%</span>
          </p>
        </div>
        <input
          type="range"
          min={0}
          max={100}
          step={1}
          value={pct}
          onChange={(e) => setPct(Number(e.target.value))}
          className="w-full accent-violet-500"
        />
        <p className="mt-1 text-xs text-zinc-500">
          Queda {100 - Math.round(pct)}% de la ventana ({windowLabel.toLowerCase()})
        </p>
      </div>

      <div className="flex items-center justify-end gap-2 border-t border-white/[0.06] pt-4">
        <Button variant="ghost" onClick={onClose} className="h-9 text-xs">
          Cancelar
        </Button>
        <Button onClick={() => void save()} loading={saving} className="h-9">
          <Check className="h-3.5 w-3.5" /> Guardar lectura
        </Button>
      </div>
    </div>
  );
}

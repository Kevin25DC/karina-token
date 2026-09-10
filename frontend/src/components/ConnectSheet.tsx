import { useState } from 'react';
import { Check, ExternalLink, KeyRound, LogIn, Plug, Sparkles, Unplug } from 'lucide-react';
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

// ManualBody (automatic mode) reads the claude.ai subscription usage. The
// manual slider was removed at the author's request; there is still a fallback
// token field for advanced users.
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

  const [token, setToken] = useState('');
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const [oauthStarted, setOauthStarted] = useState(false);
  const [oauthCode, setOauthCode] = useState('');
  const [oauthLoading, setOauthLoading] = useState(false);
  const [oauthMsg, setOauthMsg] = useState<string | null>(null);
  const [oauthErr, setOauthErr] = useState<string | null>(null);

  async function persist(used: number, win: string) {
    const value = Math.max(0, Math.min(100, Math.round(used)));
    await api.setManualUsage(meta.id, value, 100, win || 'Sesión (5 h)');
    await onDone();
    notify('success', `${meta.name} conectado · ${value}%`);
    onClose();
  }

  async function tryAuto() {
    setLoading(true);
    setErr(null);
    try {
      const res = await api.experimentalClaudeSubscription(token.trim());
      if (res.found) {
        await persist(res.used, res.window);
      } else {
        setErr(res.error || 'No se pudo leer automáticamente.');
      }
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setLoading(false);
    }
  }

  async function startOAuth() {
    setOauthLoading(true);
    setOauthErr(null);
    setOauthMsg(null);
    try {
      await api.claudeOAuthStart();
      setOauthStarted(true);
      setOauthMsg('Se abrió el navegador. Autoriza y pega aquí el código que muestre Claude.');
    } catch (e) {
      setOauthErr((e as Error).message);
    } finally {
      setOauthLoading(false);
    }
  }

  async function completeOAuth() {
    setOauthLoading(true);
    setOauthErr(null);
    try {
      const res = await api.claudeOAuthComplete(oauthCode.trim());
      if (res.found) {
        await persist(res.used, res.window);
      } else {
        setOauthErr(res.error || 'No se pudo leer el uso.');
      }
    } catch (e) {
      setOauthErr((e as Error).message);
    } finally {
      setOauthLoading(false);
    }
  }

  const current = existing?.windows?.length
    ? existing.windows.map((w) => `${w.label}: ${Math.round(w.percent)}%`).join(' · ')
    : existing?.usage_available
      ? `${Math.round(existing.used_tokens)}%`
      : null;

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3 rounded-xl border border-white/[0.06] bg-white/[0.02] p-3.5">
        <ProviderMark id={meta.id} brand={meta.brand} size="md" />
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium text-zinc-100">{meta.name}</p>
          <p className="text-xs text-zinc-500">{meta.description}</p>
        </div>
        {current && (
          <span className="font-mono text-xs text-emerald-300">{current}</span>
        )}
      </div>

      <div className="rounded-xl border border-amber-400/15 bg-amber-400/[0.06] p-3.5 text-xs leading-relaxed text-amber-100/80">
        claude.ai no expone una API oficial para el uso de la suscripción. Karina
        lo lee reutilizando el OAuth de Claude Code (experimental, endpoint no
        oficial) y se actualiza solo cada intervalo. Si falla, prueba el login.
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <Button onClick={() => void tryAuto()} loading={loading} className="h-9">
          <Sparkles className="h-3.5 w-3.5" /> Leer uso ahora
        </Button>
        <Button
          variant="secondary"
          className="h-9"
          loading={oauthLoading && !oauthStarted}
          onClick={() => void startOAuth()}
        >
          <LogIn className="h-3.5 w-3.5" /> Iniciar sesión con Claude
        </Button>
      </div>
      {err && <p className="text-[11px] text-amber-300">{err}</p>}

      <div className="rounded-xl border border-violet-400/15 bg-violet-500/[0.06] p-3.5">
        <p className="text-[11px] font-medium text-violet-100/90">
          ⚠️ Login OAuth experimental (mismo flujo que Claude Code)
        </p>
        <p className="mt-1 text-[11px] leading-relaxed text-violet-100/50">
          Puede contravenir los términos de Anthropic y conllevar suspensión de la
          cuenta. Úsalo bajo tu responsabilidad.
        </p>
        {oauthStarted && (
          <div className="mt-2 flex items-center gap-2">
            <input
              type="text"
              value={oauthCode}
              onChange={(e) => setOauthCode(e.target.value)}
              placeholder="Pega el código que muestra Claude"
              spellCheck={false}
              autoComplete="off"
              className="flex-1 rounded-lg border border-white/[0.08] bg-white/[0.03] px-3 py-2 font-mono text-[11px] text-zinc-200 placeholder:text-zinc-600 outline-none focus:border-white/20"
            />
            <Button className="h-8 px-3 text-xs" loading={oauthLoading} onClick={() => void completeOAuth()}>
              <Check className="h-3.5 w-3.5" /> Completar
            </Button>
          </div>
        )}
        {oauthMsg && <p className="mt-2 text-[11px] font-medium text-emerald-300">{oauthMsg}</p>}
        {oauthErr && <p className="mt-2 text-[11px] text-amber-300">{oauthErr}</p>}
        <input
          type="password"
          value={token}
          onChange={(e) => setToken(e.target.value)}
          placeholder="Avanzado: pega un token OAuth (sk-ant-oat…)"
          spellCheck={false}
          autoComplete="off"
          className="mt-2.5 w-full rounded-lg border border-white/[0.08] bg-white/[0.03] px-3 py-2 font-mono text-[11px] text-zinc-200 placeholder:text-zinc-600 outline-none focus:border-white/20"
        />
      </div>

      <div className="flex items-center justify-end gap-2 border-t border-white/[0.06] pt-4">
        <Button variant="ghost" onClick={onClose} className="h-9 text-xs">
          Cerrar
        </Button>
      </div>
    </div>
  );
}

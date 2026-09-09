import { useStore } from '@/store';
import { ProviderMark } from './ProviderMark';
import { Badge, Modal } from './primitives';
import { CheckCircle2, Plus } from 'lucide-react';

export function AddProviderModal() {
  const meta = useStore((s) => s.meta);
  const adding = useStore((s) => s.adding);
  const setAdding = useStore((s) => s.setAdding);
  const openConnect = useStore((s) => s.openConnect);

  const candidates = meta.filter((m) => !m.enabled);

  return (
    <Modal open={adding} onClose={() => setAdding(false)} title="Añadir un proveedor" wide>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        {candidates.map((m) => (
          <button
            key={m.id}
            onClick={() => openConnect(m.id)}
            className="group flex items-center gap-3.5 rounded-2xl border border-white/[0.06] bg-white/[0.02] p-4 text-left transition-all hover:border-white/[0.14] hover:bg-white/[0.05]"
          >
            <ProviderMark id={m.id} brand={m.brand} />
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <p className="truncate text-sm font-semibold text-zinc-100">{m.name}</p>
                {m.demo && <Badge className="text-violet-300">Demo</Badge>}
              </div>
              <p className="mt-0.5 line-clamp-2 text-xs leading-relaxed text-zinc-500">
                {m.description}
              </p>
            </div>
            {m.has_key ? (
              <CheckCircle2 className="h-4 w-4 shrink-0 text-zinc-600" />
            ) : (
              <Plus className="h-4 w-4 shrink-0 text-zinc-600 transition-colors group-hover:text-zinc-200" />
            )}
          </button>
        ))}
      </div>
      <p className="mt-4 text-xs text-zinc-600">
        Las claves se guardan de forma segura en el almacén de credenciales de tu
        sistema operativo y solo se envían al propio proveedor.
      </p>
    </Modal>
  );
}

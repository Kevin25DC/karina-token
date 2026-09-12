import { create } from 'zustand';
import { api, onEvent, onWidgetMode } from './lib/api';
import type {
  AppInfo,
  ConfigSnapshot,
  EventPayload,
  ProviderID,
  ProviderMeta,
  ProviderState,
} from './lib/types';

export type View = 'dashboard' | 'history' | 'settings';

export interface Toast {
  id: number;
  kind: 'success' | 'error' | 'info';
  message: string;
}

interface AppState {
  meta: ProviderMeta[];
  states: Record<ProviderID, ProviderState>;
  redacted: Record<ProviderID, string>;
  config: ConfigSnapshot | null;
  info: AppInfo | null;
  booted: boolean;
  cycleCount: number;
  widgetMode: boolean;
  view: View;
  updatingAll: boolean;
  refreshing: boolean;
  connectFor: ProviderID | null;
  adding: boolean;
  toasts: Toast[];

  boot: () => Promise<void>;
  reload: () => Promise<void>;
  applyEvent: (e: EventPayload) => void;
  setView: (v: View) => void;
  setWidgetMode: (active: boolean) => void;
  openConnect: (id: ProviderID | null) => void;
  setAdding: (open: boolean) => void;
  manualRefresh: () => Promise<void>;
  notify: (kind: Toast['kind'], message: string) => void;
  dismiss: (id: number) => void;
}

async function loadRedacted(meta: ProviderMeta[]): Promise<Record<string, string>> {
  const out: Record<string, string> = {};
  await Promise.all(
    meta
      .filter((m) => m.has_key)
      .map(async (m) => {
        try {
          out[m.id] = await api.keyPreview(m.id);
        } catch {
          /* ignore */
        }
      }),
  );
  return out;
}

let toastId = 0;
let bootStarted = false;

export const useStore = create<AppState>((set, get) => ({
  meta: [],
  states: {},
  redacted: {},
  config: null,
  info: null,
  booted: false,
  cycleCount: 0,
  widgetMode: false,
  view: 'dashboard',
  updatingAll: false,
  refreshing: false,
  connectFor: null,
  adding: false,
  toasts: [],

  boot: async () => {
    if (bootStarted) return;
    bootStarted = true;
    const [info, config, meta, states] = await Promise.all([
      api.info(),
      api.config(),
      api.listProviders(),
      api.states(),
    ]);
    const map: Record<ProviderID, ProviderState> = {};
    for (const s of states) map[s.provider] = s;
    const redacted = await loadRedacted(meta);
    set({ info, config, meta, states: map, redacted, booted: true });

    // Live events pushed from the Go side.
    onEvent('provider:update', (e) => get().applyEvent(e));
    onEvent('alert:threshold', (e) => {
      if (e.message) get().notify('error', e.message);
    });
    onWidgetMode((active) => set({ widgetMode: active }));
    onEvent('cycle:start', () => set({ updatingAll: true }));
    onEvent('cycle:end', (e) => {
      set((prev) => ({
        updatingAll: false,
        cycleCount: prev.cycleCount + 1,
      }));
      if (e.all_states && e.all_states.length) {
        const map2: Record<ProviderID, ProviderState> = {};
        for (const s of e.all_states) map2[s.provider] = s;
        set({ states: { ...get().states, ...map2 } });
      }
    });
  },

  reload: async () => {
    const [config, meta, states] = await Promise.all([
      api.config(),
      api.listProviders(),
      api.states(),
    ]);
    const map: Record<ProviderID, ProviderState> = {};
    for (const s of states) map[s.provider] = s;
    const redacted = await loadRedacted(meta);
    set({ config, meta, states: map, redacted });
  },

  applyEvent: (e) => {
    if (e.state) {
      set({ states: { ...get().states, [e.state.provider]: e.state } });
      return;
    }
    if (e.provider) {
      // A provider-scoped event without a state means it was removed.
      const states = { ...get().states };
      delete states[e.provider];
      set({ states });
    }
  },

  setView: (v) => set({ view: v }),
  setWidgetMode: (active) => set({ widgetMode: active }),
  openConnect: (id) => set({ connectFor: id, adding: false }),
  setAdding: (open) => set({ adding: open }),

  manualRefresh: async () => {
    set({ refreshing: true });
    try {
      await api.refreshNow();
    } finally {
      window.setTimeout(() => set({ refreshing: false }), 900);
    }
  },

  notify: (kind, message) => {
    const id = ++toastId;
    set({ toasts: [...get().toasts, { id, kind, message }] });
  },

  dismiss: (id) => set({ toasts: get().toasts.filter((t) => t.id !== id) }),
}));

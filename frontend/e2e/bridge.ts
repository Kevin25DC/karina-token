import type { Page } from '@playwright/test';
import type {
  ClaudeCodeUsageSummary,
  ConfigSnapshot,
  HistoryResult,
  ProviderMeta,
  ProviderState,
  UpdateInfo,
} from '../src/lib/types';

/** Datos que devuelve el backend simulado; cada test puede sobreescribirlos. */
export interface BridgeData {
  config: ConfigSnapshot;
  providers: ProviderMeta[];
  states: ProviderState[];
  update: UpdateInfo;
  history: HistoryResult;
  claudeCode: ClaudeCodeUsageSummary;
  claudeCodeDetected: boolean;
}

const zeroTokens = {
  input_tokens: 0,
  output_tokens: 0,
  cache_creation_tokens: 0,
  cache_read_tokens: 0,
};

export const anthropicMeta: ProviderMeta = {
  id: 'anthropic',
  name: 'Anthropic Claude',
  description: 'API de Anthropic',
  docs_url: 'https://docs.anthropic.com',
  brand: 'anthropic',
  capabilities: ['token_usage', 'rate_limits'],
  enabled: true,
  has_key: true,
};

export const anthropicState: ProviderState = {
  provider: 'anthropic',
  display_name: 'Anthropic Claude',
  status: 'connected',
  updated_at: '2026-01-01T12:00:00Z',
  capabilities: ['token_usage', 'rate_limits'],
  usage_available: true,
  used_tokens: 250_000,
  limit_tokens: 1_000_000,
  remaining_tokens: 750_000,
  usage_window: 'mes',
  rate_limit: {},
};

export const defaults: BridgeData = {
  config: {
    refresh_interval_seconds: 300,
    start_with_system: false,
    onboarding_done: true,
    alerts_enabled: false,
    alert_threshold_percent: 80,
    webhook_configured: false,
    webhook_preview: '',
    data_dir: '/tmp/karina-e2e',
    subscription_monthly_usd: 0,
  },
  providers: [],
  states: [],
  update: {
    current: '9.9.9',
    latest: '9.9.9',
    has_update: false,
    release_url: '',
    download_url: '',
    notes: '',
  },
  history: {
    provider: 'anthropic',
    span: 'today',
    has_usage: false,
    has_billing: false,
    points: [],
  },
  claudeCode: {
    available: false,
    projects: null,
    days: null,
    models: null,
    total: zeroTokens,
    cost_usd: 0,
    unpriced_tokens: 0,
    prices_as_of: '2026-01-01',
    clients: null,
    client_names: null,
    client_folders: null,
  },
  claudeCodeDetected: false,
};

/**
 * Inyecta window.go / window.runtime antes de que cargue la app, igual que
 * hace Wails en la app de escritorio. Los métodos sin respuesta definida
 * resuelven a undefined (setters, acciones).
 */
export async function installBridge(page: Page, overrides: Partial<BridgeData> = {}) {
  const data: BridgeData = { ...defaults, ...overrides };
  await page.addInitScript((d: BridgeData) => {
    const replies: Record<string, unknown> = {
      GetAppInfo: { name: 'Karina', version: '9.9.9' },
      Config: d.config,
      ListProviders: d.providers,
      States: d.states,
      KeyPreview: 'sk-…e2e',
      CheckForUpdate: d.update,
      History: d.history,
      ClaudeCodeUsage: d.claudeCode,
      ClaudeCodeDetected: d.claudeCodeDetected,
      ExportHistory: '/tmp/karina-e2e/historial.csv',
    };
    const w = window as unknown as Record<string, unknown>;
    w.go = {
      main: {
        App: new Proxy(
          {},
          { get: (_t, method: string) => () => Promise.resolve(replies[method]) },
        ),
      },
    };
    w.runtime = new Proxy({}, { get: () => () => undefined });
  }, data);
}

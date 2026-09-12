// Typed bridge to the Go backend via the generated Wails bindings.
import * as App from '../../wailsjs/go/main/App';
import { EventsOn } from '../../wailsjs/runtime/runtime';

import type {
  AppInfo,
  ClaudeOAuthStart,
  ConfigSnapshot,
  EventPayload,
  ExperimentalSubRead,
  HistoryResult,
  HistorySpan,
  ProviderID,
  ProviderMeta,
  ProviderState,
  TestResult,
  UpdateInfo,
} from './types';

async function call<T>(fn: () => Promise<T>): Promise<T> {
  await waitForBridge();
  try {
    return await fn();
  } catch (e) {
    const message = e instanceof Error ? e.message : String(e);
    throw new Error(message);
  }
}

/**
 * The Go bridge (window.go) is injected by Wails. Wait briefly for it so an
 * early call does not fail with "cannot read properties of undefined".
 */
async function waitForBridge(): Promise<void> {
  const w = window as unknown as {
    go?: { main?: { App?: unknown } };
  };
  for (let i = 0; i < 100; i++) {
    if (w.go && w.go.main && w.go.main.App) return;
    await new Promise((r) => window.setTimeout(r, 50));
  }
  throw new Error(
    'No se pudo conectar con el motor de Karina. Abre la aplicación desde Karina.exe (la app de escritorio), no en un navegador.',
  );
}

export const api = {
  info: () => call<AppInfo>(() => App.GetAppInfo()),
  checkForUpdate: () => call<UpdateInfo>(() => App.CheckForUpdate()),
  listProviders: () => call<ProviderMeta[]>(() => App.ListProviders()),
  states: () => call<ProviderState[]>(() => App.States()),
  config: () => call<ConfigSnapshot>(() => App.Config()),
  keyPreview: (provider: ProviderID) =>
    call<string>(() => App.KeyPreview(provider)),
  testProvider: (provider: ProviderID, key: string) =>
    call<TestResult>(() => App.TestProvider(provider, key)),
  saveProviderKey: (provider: ProviderID, key: string) =>
    call<void>(() => App.SaveProviderKey(provider, key)),
  removeProvider: (provider: ProviderID) =>
    call<void>(() => App.RemoveProvider(provider)),
  setProviderEnabled: (provider: ProviderID, enabled: boolean) =>
    call<void>(() => App.SetProviderEnabled(provider, enabled)),
  setManualUsage: (provider: ProviderID, used: number, limit: number, window: string) =>
    call<void>(() => App.SetManualUsage(provider, used, limit, window)),
  experimentalClaudeSubscription: (token: string) =>
    call<ExperimentalSubRead>(() => App.ExperimentalClaudeSubscription(token)),
  claudeOAuthStart: () => call<ClaudeOAuthStart>(() => App.ClaudeOAuthStart()),
  claudeOAuthComplete: (code: string) =>
    call<ExperimentalSubRead>(() => App.ClaudeOAuthComplete(code)),
  setRefreshInterval: (seconds: number) =>
    call<void>(() => App.SetRefreshInterval(seconds)),
  setStartWithSystem: (enabled: boolean) =>
    call<void>(() => App.SetStartWithSystem(enabled)),
  setAlertsEnabled: (enabled: boolean) =>
    call<void>(() => App.SetAlertsEnabled(enabled)),
  setAlertThreshold: (percent: number) =>
    call<void>(() => App.SetAlertThreshold(percent)),
  completeOnboarding: () => call<void>(() => App.CompleteOnboarding()),
  enterWidgetMode: () => call<void>(() => App.EnterWidgetMode()),
  exitWidgetMode: () => call<void>(() => App.ExitWidgetMode()),
  history: (provider: ProviderID, span: HistorySpan) =>
    call<HistoryResult>(() => App.History(provider, span)),
  refreshNow: () => call<void>(() => App.RefreshNow()),
};

/** Register a backend event listener (keyed by event kind). */
export function onEvent(kind: string, cb: (payload: EventPayload) => void): void {
  EventsOn(kind, (data: unknown) => cb(data as EventPayload));
}

/** Listen for widget-mode toggles emitted by the backend. */
export function onWidgetMode(cb: (active: boolean) => void): void {
  EventsOn('mode:widget', (data: unknown) => cb(Boolean(data)));
}

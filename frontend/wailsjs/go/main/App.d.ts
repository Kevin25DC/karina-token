// Type declarations for the Wails App bindings. Mirrors the Go method set.

import type {
  AppInfo,
  ClaudeOAuthStart,
  ConfigSnapshot,
  ExperimentalSubRead,
  HistoryResult,
  HistorySpan,
  ProviderID,
  ProviderMeta,
  ProviderState,
  TestResult,
  UpdateInfo,
} from '../../../src/lib/types';

export function GetAppInfo(): Promise<AppInfo>;
export function CheckForUpdate(): Promise<UpdateInfo>;
export function RefreshNow(): Promise<void>;
export function ListProviders(): Promise<ProviderMeta[]>;
export function States(): Promise<ProviderState[]>;
export function Config(): Promise<ConfigSnapshot>;
export function KeyPreview(provider: ProviderID): Promise<string>;
export function TestProvider(provider: ProviderID, key: string): Promise<TestResult>;
export function SaveProviderKey(provider: ProviderID, key: string): Promise<void>;
export function RemoveProvider(provider: ProviderID): Promise<void>;
export function SetProviderEnabled(provider: ProviderID, enabled: boolean): Promise<void>;
export function SetManualUsage(
  provider: ProviderID,
  used: number,
  limit: number,
  window: string,
): Promise<void>;
export function ExperimentalClaudeSubscription(token: string): Promise<ExperimentalSubRead>;
export function ClaudeOAuthStart(): Promise<ClaudeOAuthStart>;
export function ClaudeOAuthComplete(code: string): Promise<ExperimentalSubRead>;
export function SetRefreshInterval(seconds: number): Promise<void>;
export function SetStartWithSystem(enabled: boolean): Promise<void>;
export function CompleteOnboarding(): Promise<void>;
export function EnterWidgetMode(): Promise<void>;
export function ExitWidgetMode(): Promise<void>;
export function History(provider: ProviderID, span: HistorySpan): Promise<HistoryResult>;

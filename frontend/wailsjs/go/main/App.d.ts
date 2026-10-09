// Type declarations for the Wails App bindings. Mirrors the Go method set.

import type {
  AppInfo,
  ClaudeCodeUsageSummary,
  ClientBudget,
  ClientReport,
  ReportPeriod,
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
export function LogClientError(message: string, stack: string): Promise<void>;
export function SetRefreshInterval(seconds: number): Promise<void>;
export function SetStartWithSystem(enabled: boolean): Promise<void>;
export function SetAlertsEnabled(enabled: boolean): Promise<void>;
export function SetAlertThreshold(percent: number): Promise<void>;
export function SetWebhookURL(url: string): Promise<void>;
export function TestWebhook(): Promise<void>;
export function CompleteOnboarding(): Promise<void>;
export function EnterWidgetMode(): Promise<void>;
export function ExitWidgetMode(): Promise<void>;
export function SetWidgetExpanded(expanded: boolean): Promise<void>;
export function ResetWidgetPosition(): Promise<void>;
export function History(provider: ProviderID, span: HistorySpan): Promise<HistoryResult>;
export function ExportHistory(provider: ProviderID, span: HistorySpan): Promise<string>;
export function ClaudeCodeDetected(): Promise<boolean>;
export function SetProjectClient(path: string, client: string): Promise<void>;
export function SetFolderClient(folder: string, client: string): Promise<void>;
export function SetSubscriptionPrice(usd: number): Promise<void>;
export function ExportClientReport(period: ReportPeriod): Promise<string>;
export function ClientReport(period: ReportPeriod): Promise<ClientReport>;
export function ClientBudgets(): Promise<ClientBudget[]>;
export function SetClientBudget(client: string, usd: number): Promise<void>;
export function SetIdleGapMinutes(minutes: number): Promise<void>;
export function SetSubscriptionInterval(seconds: number): Promise<void>;
export function SetClientRate(client: string, usd: number): Promise<void>;
export function SetReportBusinessName(name: string): Promise<void>;
export function ClaudeCodeUsage(span: HistorySpan): Promise<ClaudeCodeUsageSummary>;

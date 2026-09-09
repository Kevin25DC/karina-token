// Types mirror the Go backend JSON payloads (internal/domain and internal/core).

export type ProviderID = string;

export interface ProviderMeta {
  id: ProviderID;
  name: string;
  description: string;
  docs_url: string;
  brand: string;
  capabilities: Capability[];
  demo?: boolean;
  enabled: boolean;
  has_key: boolean;
}

export type Capability = 'token_usage' | 'rate_limits' | 'billing';

export type ProviderStatus =
  | 'connected'
  | 'updating'
  | 'disconnected'
  | 'invalid_credentials'
  | 'rate_limited'
  | 'usage_unavailable'
  | 'error';

export interface RateBucket {
  limit: number;
  remaining: number;
  reset_at?: string;
  window?: string;
}

export interface RateLimitInfo {
  requests?: RateBucket;
  tokens?: RateBucket;
  input_tokens?: RateBucket;
  output_tokens?: RateBucket;
}

export interface Balance {
  currency: string;
  total: number;
  granted?: number;
  topped_up?: number;
  is_available: boolean;
}

export interface ProviderState {
  provider: ProviderID;
  display_name: string;
  status: ProviderStatus;
  status_msg?: string;
  updated_at: string;
  capabilities: Capability[];
  usage_available: boolean;
  used_tokens: number;
  limit_tokens: number;
  remaining_tokens: number;
  usage_window?: string;
  reset_at?: string;
  rate_limit: RateLimitInfo;
  balance?: Balance;
  note?: string;
  error?: string;
}

export interface HistoryPoint {
  at: string;
  provider?: ProviderID;
  usage_available?: boolean;
  used_tokens?: number;
  limit_tokens?: number;
  remaining_tokens?: number;
  balance_total?: number;
  balance_currency?: string;
  rate_limit_remaining?: number;
  rate_limit_limit?: number;
}

export interface HistoryResult {
  provider: ProviderID;
  span: HistorySpan;
  has_usage: boolean;
  has_billing: boolean;
  points: HistoryPoint[];
}

export type HistorySpan = 'today' | '7d' | '30d';

export interface ConfigSnapshot {
  refresh_interval_seconds: number;
  start_with_system: boolean;
  onboarding_done: boolean;
  data_dir: string;
}

export interface TestResult {
  state: ProviderState;
  ok: boolean;
  error?: string;
}

export interface EventPayload {
  kind: string;
  provider?: ProviderID;
  state?: ProviderState;
  all_states?: ProviderState[];
}

export interface AppInfo {
  name: string;
  version: string;
}

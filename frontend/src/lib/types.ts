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
  manual?: boolean;
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

export interface UsageWindow {
  label: string;
  used: number;
  limit: number;
  remaining: number;
  percent: number;
  reset_at?: string;
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
  windows?: UsageWindow[];
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

export interface UsageTokens {
  input_tokens: number;
  output_tokens: number;
  cache_creation_tokens: number;
  cache_read_tokens: number;
}

export interface ClaudeCodeProjectUsage {
  path: string;
  label: string;
  sessions: number;
  tokens: UsageTokens;
  /** Costo estimado a precios de lista de la API. */
  cost_usd: number;
  /** Tiempo de trabajo estimado en el proyecto, en segundos. */
  active_seconds: number;
  /** Cliente o etiqueta asignado por el usuario ('' si no tiene). */
  client: string;
  /** true si el cliente viene de una regla por carpeta y no del proyecto. */
  client_inherited: boolean;
  models: ClaudeCodeModelUsage[] | null;
}

/** Regla que asigna a un cliente todos los proyectos bajo una carpeta. */
export interface ClaudeCodeFolderRule {
  path: string;
  client: string;
  /** Proyectos del periodo a los que aplica. */
  projects: number;
}

export interface ClaudeCodeClientUsage {
  /** '' agrupa los proyectos sin cliente. */
  name: string;
  projects: number;
  sessions: number;
  tokens: UsageTokens;
  cost_usd: number;
  /** Suma del tiempo activo de sus proyectos, en segundos. */
  active_seconds: number;
}

/** Periodo de un reporte por cliente. */
export type ReportPeriod = HistorySpan | 'this_month' | 'last_month';

export interface ClientReport {
  period: ReportPeriod;
  period_label: string;
  business_name: string;
  generated_at: string;
  summary: ClaudeCodeUsageSummary;
}

/** Presupuesto mensual de un cliente y lo consumido en el mes en curso. */
export interface ClientBudget {
  name: string;
  budget_usd: number;
  spent_usd: number;
  percent: number;
}

export interface ClaudeCodeModelUsage {
  /** Id del modelo tal cual lo registra Claude Code. */
  model: string;
  /** Respuestas del asistente contadas para este modelo. */
  turns: number;
  tokens: UsageTokens;
  cost_usd: number;
  /** false si Karina no tiene precio para este modelo. */
  priced: boolean;
}

export interface ClaudeCodeDayUsage {
  date: string;
  tokens: UsageTokens;
  cost_usd: number;
}

export interface ClaudeCodeUsageSummary {
  available: boolean;
  projects: ClaudeCodeProjectUsage[] | null;
  days: ClaudeCodeDayUsage[] | null;
  models: ClaudeCodeModelUsage[] | null;
  total: UsageTokens;
  /** Costo estimado del total a precios de lista de la API. */
  cost_usd: number;
  /** Tokens de modelos sin precio conocido (no entran en cost_usd). */
  unpriced_tokens: number;
  prices_as_of: string;
  /** Tiempo activo sumado por proyecto, en segundos. */
  active_seconds: number;
  /** Pausa (min) a partir de la cual se considera que dejaste de trabajar. */
  idle_gap_minutes: number;
  clients: ClaudeCodeClientUsage[] | null;
  client_names: string[] | null;
  client_folders: ClaudeCodeFolderRule[] | null;
}

export interface ConfigSnapshot {
  refresh_interval_seconds: number;
  /** Cada cuánto se consulta Claude Pro/Max (aparte y más despacio). */
  subscription_interval_seconds: number;
  start_with_system: boolean;
  onboarding_done: boolean;
  alerts_enabled: boolean;
  alert_threshold_percent: number;
  webhook_configured: boolean;
  webhook_preview: string;
  data_dir: string;
  /** Precio mensual del plan de Claude del usuario (0 = sin definir). */
  subscription_monthly_usd: number;
  /** Pausa (min) que corta un tramo de trabajo al medir horas. */
  idle_gap_minutes: number;
  /** Nombre que encabeza los reportes por cliente. */
  report_business_name: string;
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
  message?: string;
}

export interface AppInfo {
  name: string;
  version: string;
}

export interface UpdateInfo {
  current: string;
  latest: string;
  has_update: boolean;
  release_url: string;
  download_url: string;
  notes: string;
  error?: string;
}

export interface ExperimentalSubRead {
  found: boolean;
  source: string;
  window: string;
  used: number;
  limit: number;
  percent: number;
  reset_at?: string;
  error?: string;
}

export interface ClaudeOAuthStart {
  url: string;
}

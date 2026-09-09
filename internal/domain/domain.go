// Package domain defines the core, provider-agnostic domain models shared
// across Karina. It has no external dependencies on purpose.
package domain

import (
	"math"
	"time"
)

// ProviderID is a stable machine identifier for a provider (e.g. "openai").
type ProviderID string

// Capability describes a category of *real* information a provider exposes.
// Adapters must never claim a capability they cannot actually serve.
type Capability string

const (
	// CapTokenUsage means the adapter can report global token consumption
	// (used vs limit) for the account through the provider API.
	CapTokenUsage Capability = "token_usage"
	// CapRateLimits means the adapter can report current rate-limit state
	// (remaining/limit/reset) observed from API response headers.
	CapRateLimits Capability = "rate_limits"
	// CapBilling means the adapter can report monetary billing/balance data.
	CapBilling Capability = "billing"
)

// ProviderStatus is the lifecycle status of a provider from Karina's
// point of view. It is derived exclusively from real API interactions.
type ProviderStatus string

const (
	StatusUpdating           ProviderStatus = "updating"
	StatusConnected          ProviderStatus = "connected"
	StatusDisconnected       ProviderStatus = "disconnected"
	StatusInvalidCredentials ProviderStatus = "invalid_credentials"
	StatusRateLimited        ProviderStatus = "rate_limited"
	StatusUsageUnavailable   ProviderStatus = "usage_unavailable"
	StatusError              ProviderStatus = "error"
)

// HistorySpan identifies a time range used for history queries.
type HistorySpan string

const (
	SpanToday HistorySpan = "today"
	Span7d    HistorySpan = "7d"
	Span30d   HistorySpan = "30d"
)

// ProviderMeta is static catalog metadata about a supported provider.
type ProviderMeta struct {
	ID           ProviderID   `json:"id"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	DocsURL      string       `json:"docs_url"`
	Brand        string       `json:"brand"` // UI accent key, e.g. "amber"
	Capabilities []Capability `json:"capabilities"`
	Demo         bool         `json:"demo,omitempty"`
	Manual       bool         `json:"manual,omitempty"` // usage entered by user, no API

	// Runtime fields.
	Enabled bool `json:"enabled"`
	HasKey  bool `json:"has_key"`
}

// RateBucket describes one rate-limit bucket (requests or tokens) as exposed
// by provider response headers.
type RateBucket struct {
	Limit     int64     `json:"limit"`
	Remaining int64     `json:"remaining"`
	ResetAt   time.Time `json:"reset_at,omitempty"`
	Window    string    `json:"window,omitempty"`
}

// Used returns how many units were consumed in the current window.
func (b RateBucket) Used() int64 {
	if b.Limit <= 0 {
		return 0
	}
	u := b.Limit - b.Remaining
	if u < 0 {
		u = 0
	}
	return u
}

// Percent returns the consumed percentage in [0,100], or -1 when not known.
func (b RateBucket) Percent() float64 {
	return Percent(b.Used(), b.Limit)
}

// RateLimitInfo groups the rate-limit buckets a provider can report.
type RateLimitInfo struct {
	Requests     *RateBucket `json:"requests,omitempty"`
	Tokens       *RateBucket `json:"tokens,omitempty"` // total token bucket
	InputTokens  *RateBucket `json:"input_tokens,omitempty"`
	OutputTokens *RateBucket `json:"output_tokens,omitempty"`
}

// HasData reports whether any bucket is present.
func (r RateLimitInfo) HasData() bool {
	return r.Requests != nil || r.Tokens != nil || r.InputTokens != nil || r.OutputTokens != nil
}

// Balance is a monetary balance (e.g. DeepSeek prepaid balance).
type Balance struct {
	Currency    string  `json:"currency"`
	Total       float64 `json:"total"`
	Granted     float64 `json:"granted,omitempty"`
	ToppedUp    float64 `json:"topped_up,omitempty"`
	IsAvailable bool    `json:"is_available"`
}

// ProviderState is the snapshot of everything Karina knows about a single
// provider at a point in time. Every field is real data or explicitly unset.
type ProviderState struct {
	Provider     ProviderID     `json:"provider"`
	DisplayName  string         `json:"display_name"`
	Status       ProviderStatus `json:"status"`
	StatusMsg    string         `json:"status_msg,omitempty"`
	UpdatedAt    time.Time      `json:"updated_at"`
	Capabilities []Capability   `json:"capabilities"`

	// Token usage. Only meaningful when UsageAvailable is true.
	UsageAvailable  bool      `json:"usage_available"`
	UsedTokens      int64     `json:"used_tokens"`
	LimitTokens     int64     `json:"limit_tokens"`
	RemainingTokens int64     `json:"remaining_tokens"`
	UsageWindow     string    `json:"usage_window,omitempty"`
	ResetAt         time.Time `json:"reset_at,omitempty"`

	RateLimit RateLimitInfo `json:"rate_limit"`
	Balance   *Balance      `json:"balance,omitempty"`

	// Note is a human-readable, honest explanation of what the provider can
	// and cannot report. Error carries the last failure message.
	Note  string `json:"note,omitempty"`
	Error string `json:"error,omitempty"`
}

// HistoryPoint is one recorded observation used for charts/history.
type HistoryPoint struct {
	At                 time.Time  `json:"at"`
	Provider           ProviderID `json:"provider"`
	UsageAvailable     bool       `json:"usage_available"`
	UsedTokens         int64      `json:"used_tokens"`
	LimitTokens        int64      `json:"limit_tokens"`
	RemainingTokens    int64      `json:"remaining_tokens"`
	BalanceTotal       float64    `json:"balance_total,omitempty"`
	BalanceCurrency    string     `json:"balance_currency,omitempty"`
	RateLimitRemaining int64      `json:"rate_limit_remaining,omitempty"`
	RateLimitLimit     int64      `json:"rate_limit_limit,omitempty"`
}

// Percent returns used/limit in [0,100] rounded to one decimal, or -1 when it
// cannot be computed (limit <= 0).
func Percent(used, limit int64) float64 {
	if limit <= 0 || used < 0 {
		return -1
	}
	if used >= limit {
		return 100
	}
	p := float64(used) / float64(limit) * 100
	return math.Round(p*10) / 10
}

// Remaining returns limit-used (never negative), or -1 when unknown.
func Remaining(used, limit int64) int64 {
	if limit <= 0 {
		return -1
	}
	r := limit - used
	if r < 0 {
		return 0
	}
	return r
}

// HistorySpans returns the supported history spans in order.
func HistorySpans() []HistorySpan { return []HistorySpan{SpanToday, Span7d, Span30d} }

// Start returns the inclusive start time for a span relative to now.
func (s HistorySpan) Start(now time.Time) time.Time {
	switch s {
	case SpanToday:
		y, m, d := now.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	case Span7d:
		return now.AddDate(0, 0, -7)
	case Span30d:
		return now.AddDate(0, 0, -30)
	default:
		return now
	}
}

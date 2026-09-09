// Package anthropic implements the Provider interface for Anthropic (Claude).
//
// REAL CAPABILITIES (verified against Anthropic/Claude platform docs):
//   - Any API key can list models (GET /v1/models) and exposes current
//     rate-limit state through anthropic-ratelimit-* response headers.
//   - Global/organization token usage is ONLY available through the Admin
//     API (GET /v1/organizations/usage_report/messages), which requires an
//     Admin API key (sk-ant-admin...). There is no public billing/credit
//     balance endpoint. The adapter uses Admin keys when detected and
//     otherwise honestly reports "usage unavailable through API".
package anthropic

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"karina/internal/domain"
	"karina/internal/providers/api"
)

const (
	id          = "anthropic"
	baseURL     = "https://api.anthropic.com/v1"
	version     = "2023-06-01"
	usageWindow = 5 * time.Minute
)

// Provider talks to the Anthropic API.
type Provider struct {
	mu       sync.Mutex
	lastHit  time.Time
	adminOK  bool
	lastUsed int64
}

// New returns an Anthropic provider adapter.
func New() *Provider { return &Provider{} }

// ID implements api.Provider.
func (p *Provider) ID() string { return id }

// DisplayName implements api.Provider.
func (p *Provider) DisplayName() string { return "Anthropic Claude" }

// Capabilities implements api.Provider.
func (p *Provider) Capabilities() []domain.Capability {
	return []domain.Capability{domain.CapTokenUsage, domain.CapRateLimits}
}

func headers(key string) map[string]string {
	return map[string]string{
		"x-api-key":         key,
		"anthropic-version": version,
	}
}

// Refresh implements api.Provider. See the package comment for what is real.
func (p *Provider) Refresh(ctx context.Context, cfg api.Config) (domain.ProviderState, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = baseURL
	}
	st := domain.ProviderState{
		Provider:     id,
		DisplayName:  p.DisplayName(),
		UpdatedAt:    time.Now(),
		Capabilities: []domain.Capability{domain.CapRateLimits},
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return st, api.NewError(api.KindInvalidCredentials, 0, "falta la clave API", nil)
	}

	resp, _, err := api.GET(ctx, cfg, "/models", headers(cfg.APIKey))
	if err != nil {
		return st, err
	}
	rateLimit := api.ParseAnthropicHeaders(resp.Header)

	switch resp.StatusCode {
	case 200:
		st.Status = domain.StatusConnected
		st.StatusMsg = "Conectado"
		st.RateLimit = rateLimit

		isAdmin := strings.HasPrefix(cfg.APIKey, "sk-ant-admin")
		if isAdmin && !cfg.ValidationOnly {
			if used, ok := p.fetchOrgUsage(ctx, cfg); ok {
				st.Capabilities = append(st.Capabilities, domain.CapTokenUsage)
				st.UsageAvailable = true
				st.UsedTokens = used
				st.UsageWindow = "Last 7 days"
				st.Note = "Uso de tokens de la organización (últimos 7 días) vía la Anthropic Admin API."
			} else {
				st.Note = "Clave admin detectada, pero no se pudo leer el informe de uso de la organización. Solo se muestran límites de peticiones."
			}
		} else {
			st.Note = "Uso de tokens no disponible vía API: leer el uso de la organización requiere una Anthropic Admin API key (sk-ant-admin). Una clave estándar solo expone límites de peticiones. No existe un endpoint público de saldo/facturación."
		}
		return st, nil

	case 401, 403:
		// An Anthropic Admin key is sometimes not accepted on the standard
		// /models endpoint. If the key looks like an Admin key, validate via
		// the Admin usage report before declaring the key invalid.
		if strings.HasPrefix(cfg.APIKey, "sk-ant-admin") {
			if used, ok := p.fetchOrgUsage(ctx, cfg); ok {
				st.Status = domain.StatusConnected
				st.StatusMsg = "Conectado (clave admin)"
				st.Capabilities = append(st.Capabilities, domain.CapTokenUsage)
				st.UsageAvailable = true
				st.UsedTokens = used
				st.UsageWindow = "Last 7 days"
				st.Note = "Uso de tokens de la organización (últimos 7 días) vía la Anthropic Admin API."
				return st, nil
			}
		}
		return api.StateForError(p, api.NewError(api.KindInvalidCredentials, resp.StatusCode, "clave API no válida o caducada", nil), st), nil

	default:
		return api.StateForError(p, api.ErrorFromResponse(resp.StatusCode, api.ParseRetryAfter(resp)), st), nil
	}
}

// fetchOrgUsage queries the Admin usage report for the last 7 days and sums
// token usage. Results are cached for usageWindow between polls.
func (p *Provider) fetchOrgUsage(ctx context.Context, cfg api.Config) (int64, bool) {
	p.mu.Lock()
	if p.adminOK && time.Since(p.lastHit) < usageWindow {
		v := p.lastUsed
		p.mu.Unlock()
		return v, true
	}
	p.mu.Unlock()

	start := time.Now().AddDate(0, 0, -7).UTC().Format(time.RFC3339)
	end := time.Now().UTC().Format(time.RFC3339)
	path := fmt.Sprintf("/organizations/usage_report/messages?start_time=%s&end_time=%s&bucket_width=1d", start, end)
	resp, body, err := api.GET(ctx, cfg, path, headers(cfg.APIKey))
	if err != nil {
		return 0, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastHit = time.Now()
	if resp.StatusCode != 200 {
		p.adminOK = false
		return 0, false
	}
	var parsed anthropicUsageResponse
	if err := api.DecodeJSON(body, &parsed); err != nil {
		p.adminOK = false
		return 0, false
	}
	var total int64
	valid := false
	for _, b := range parsed.Data {
		if b.StartTime == "" {
			continue
		}
		valid = true
		total += b.UncachedInputTokens + b.CacheReadInputTokens + b.OutputTokens
		total += b.CacheCreation.Ephemeral5mInputTokens + b.CacheCreation.Ephemeral1hInputTokens
	}
	if !valid {
		p.adminOK = false
		return 0, false
	}
	p.adminOK = true
	p.lastUsed = total
	return total, true
}

type anthropicUsageResponse struct {
	Data []struct {
		StartTime            string `json:"start_time"`
		UncachedInputTokens  int64  `json:"uncached_input_tokens"`
		CacheReadInputTokens int64  `json:"cache_read_input_tokens"`
		OutputTokens         int64  `json:"output_tokens"`
		CacheCreation        struct {
			Ephemeral5mInputTokens int64 `json:"ephemeral_5m_input_tokens"`
			Ephemeral1hInputTokens int64 `json:"ephemeral_1h_input_tokens"`
		} `json:"cache_creation"`
	} `json:"data"`
}

var _ api.Provider = (*Provider)(nil)

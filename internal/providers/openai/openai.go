// Package openai implements the Provider interface for OpenAI.
//
// REAL CAPABILITIES (verified against OpenAI platform docs):
//   - A standard API key can list models (GET /v1/models) and exposes current
//     rate-limit state through x-ratelimit-* response headers. Standard keys
//     CANNOT read organization token usage.
//   - Organization usage (GET /v1/organization/usage/completions) requires an
//     OpenAI Admin API key. The adapter auto-detects admin keys and then
//     reports real token usage for the last 30 days; otherwise it honestly
//     reports "usage unavailable through API".
package openai

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"
	"sync"
	"time"

	"karina/internal/domain"
	"karina/internal/providers/api"
)

const (
	id         = "openai"
	baseURL    = "https://api.openai.com/v1"
	usageRetry = 5 * time.Minute
)

// Provider talks to the OpenAI API.
type Provider struct {
	mu             sync.Mutex
	keyTag         string
	lastUsage      time.Time
	adminKnown     bool
	lastTokens     int64
	usageAttempted bool
}

// New returns an OpenAI provider adapter.
func New() *Provider { return &Provider{} }

// ID implements api.Provider.
func (p *Provider) ID() string { return id }

// DisplayName implements api.Provider.
func (p *Provider) DisplayName() string { return "OpenAI" }

// Capabilities implements api.Provider.
func (p *Provider) Capabilities() []domain.Capability {
	return []domain.Capability{domain.CapTokenUsage, domain.CapRateLimits}
}

func (p *Provider) tag(key string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return fmt.Sprintf("%x-%s", h.Sum64(), key[len(key)-4:])
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
	p.mu.Lock()
	keyChanged := p.keyTag != p.tag(cfg.APIKey)
	if keyChanged {
		p.keyTag = p.tag(cfg.APIKey)
		p.adminKnown = false
		p.lastUsage = time.Time{}
	}
	p.mu.Unlock()

	// 1) Validate through GET /v1/models (free of charge).
	resp, _, err := api.GET(ctx, cfg, "/models", map[string]string{
		"Authorization": "Bearer " + cfg.APIKey,
	})
	if err != nil {
		return st, err
	}
	rateLimit := api.ParseOpenAIHeaders(resp.Header)

	switch resp.StatusCode {
	case 200:
		st.Status = domain.StatusConnected
		st.StatusMsg = "Conectado"
		st.RateLimit = rateLimit
		// Standard key validated. Organization usage may still work if the
		// key is an Admin key; probe occasionally.
		if !cfg.ValidationOnly && p.shouldProbeUsage() {
			if u, ok := p.fetchOrgUsage(ctx, cfg); ok {
				st.Capabilities = append(st.Capabilities, domain.CapTokenUsage)
				st.UsageAvailable = true
				st.UsedTokens = u
				st.UsageWindow = "Last 30 days"
				st.Note = "Uso de tokens de la organización (últimos 30 días) vía la Admin API."
			} else {
				st.Note = "Uso de tokens no disponible vía API: leer el uso de la organización requiere una OpenAI Admin API key (propietario de la organización). Tu clave actual solo puede exponer límites de peticiones."
			}
		} else {
			st.Note = "Uso de tokens no disponible vía API: leer el uso de la organización requiere una OpenAI Admin API key (propietario de la organización). Tu clave actual solo puede exponer límites de peticiones."
		}
		return st, nil

	case 401:
		return api.StateForError(p, api.NewError(api.KindInvalidCredentials, 401, "clave API no válida o caducada", nil), st), nil

	case 403:
		// A non-admin key can be 403 here; but valid Admin keys are also not
		// allowed on non-administration endpoints. Fall back to probing the
		// organization usage endpoint to validate Admin keys.
		if u, ok := p.fetchOrgUsage(ctx, cfg); ok {
			st.Status = domain.StatusConnected
			st.StatusMsg = "Conectado (clave admin)"
			st.RateLimit = api.ParseOpenAIHeaders(resp.Header)
			st.Capabilities = append(st.Capabilities, domain.CapTokenUsage)
			st.UsageAvailable = true
			st.UsedTokens = u
			st.UsageWindow = "Last 30 days"
			st.Note = "Uso de tokens de la organización (últimos 30 días) vía la Admin API."
			return st, nil
		}
		return api.StateForError(p, api.NewError(api.KindInvalidCredentials, 403, "la API rechazó esta clave en este endpoint", nil), st), nil

	default:
		return api.StateForError(p, api.ErrorFromResponse(resp.StatusCode, api.ParseRetryAfter(resp)), st), nil
	}
}

func (p *Provider) shouldProbeUsage() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.usageAttempted || time.Since(p.lastUsage) > usageRetry {
		p.usageAttempted = true
		return true
	}
	return false
}

// fetchOrgUsage returns the sum of input+output tokens over the last 30 days.
// ok=false means the key is not an admin key (or the endpoint is unreachable).
// Once an admin key is confirmed, results are cached and reused within the
// retry window to avoid hammering the endpoint on every poll.
func (p *Provider) fetchOrgUsage(ctx context.Context, cfg api.Config) (int64, bool) {
	p.mu.Lock()
	if p.adminKnown && time.Since(p.lastUsage) < usageRetry {
		v := p.lastTokens
		p.mu.Unlock()
		return v, true
	}
	p.mu.Unlock()

	start := time.Now().AddDate(0, 0, -30).Unix()
	path := fmt.Sprintf("/organization/usage/completions?start_time=%d&bucket_width=1d", start)
	resp, body, err := api.GET(ctx, cfg, path, map[string]string{
		"Authorization": "Bearer " + cfg.APIKey,
	})
	if err != nil {
		return 0, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastUsage = time.Now()
	if resp.StatusCode != 200 {
		p.adminKnown = false
		return 0, false
	}
	var parsed oaiUsageResponse
	if err := api.DecodeJSON(body, &parsed); err != nil {
		p.adminKnown = false
		return 0, false
	}
	if len(parsed.Data) == 0 {
		p.adminKnown = true
		return 0, true
	}
	var total int64
	valid := false
	for _, b := range parsed.Data {
		if b.BucketStart == "" {
			continue
		}
		valid = true
		if b.InputTokens != nil {
			total += *b.InputTokens
		}
		if b.InputCachedTokens != nil {
			total += *b.InputCachedTokens
		}
		if b.InputCacheWriteTokens != nil {
			total += *b.InputCacheWriteTokens
		}
		if b.OutputTokens != nil {
			total += *b.OutputTokens
		}
	}
	if !valid {
		p.adminKnown = false
		return 0, false
	}
	p.adminKnown = true
	p.lastTokens = total
	return total, true
}

type oaiUsageResponse struct {
	Data []struct {
		BucketStart           string `json:"bucket_start"`
		InputTokens           *int64 `json:"input_tokens"`
		InputCachedTokens     *int64 `json:"input_cached_tokens"`
		InputCacheWriteTokens *int64 `json:"input_cache_write_tokens"`
		OutputTokens          *int64 `json:"output_tokens"`
		NumModelRequests      *int64 `json:"num_model_requests"`
	} `json:"data"`
	HasMore bool `json:"has_more"`
}

var _ api.Provider = (*Provider)(nil)

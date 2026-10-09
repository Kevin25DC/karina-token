// Package keyonly implements the Provider interface for providers whose API
// key can be validated but that expose NO usage, quota or billing endpoint
// for that key. Karina then reports the connection honestly and explains
// where the usage can be seen instead of inventing a figure.
//
// REAL CAPABILITIES (checked against each provider's docs, 2026-10-08):
//   - xAI (api.x.ai): GET /v1/api-key returns information about the key in
//     use (name, team, blocked/disabled flags). Balance and usage live in the
//     console; programmatic usage needs a separate Management API key.
//   - Mistral (api.mistral.ai): GET /v1/models validates the key. No usage
//     endpoint; rate-limit headers only come back on inference calls.
//   - Groq (api.groq.com): GET /openai/v1/models validates the key. No usage
//     or billing endpoint; x-ratelimit-* headers only on inference calls.
//
// Karina never makes inference calls (they cost money), so the rate-limit
// headers of these providers are out of reach by design.
package keyonly

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"karina/internal/domain"
	"karina/internal/providers/api"
)

// Provider is a key-validation-only adapter.
type Provider struct {
	id      string
	name    string
	baseURL string
	path    string
	note    string
	// inspect optionally reads the validation response and returns a problem
	// to report (e.g. the key is disabled). Empty means all good.
	inspect func(body []byte) string
	// badKeyStatuses are extra HTTP statuses that mean "invalid key" for this
	// provider, besides 401.
	badKeyStatuses []int
}

// NewXAI returns the xAI (Grok) adapter.
func NewXAI() *Provider {
	return &Provider{
		id:      "xai",
		name:    "xAI (Grok)",
		baseURL: "https://api.x.ai",
		path:    "/v1/api-key",
		note:    "xAI no expone el consumo ni el saldo para una clave de API normal: se ven en console.x.ai (Usage y Billing). Karina solo comprueba que la clave está activa.",
		inspect: inspectXAI,
		// xAI documents 400 for an invalid key on this endpoint.
		badKeyStatuses: []int{400},
	}
}

// NewMistral returns the Mistral adapter.
func NewMistral() *Provider {
	return &Provider{
		id:      "mistral",
		name:    "Mistral AI",
		baseURL: "https://api.mistral.ai",
		path:    "/v1/models",
		note:    "La API de Mistral no expone el consumo ni la facturación para una clave de API: se ven en console.mistral.ai. Karina solo comprueba que la clave funciona.",
	}
}

// NewGroq returns the Groq adapter.
func NewGroq() *Provider {
	return &Provider{
		id:      "groq",
		name:    "Groq",
		baseURL: "https://api.groq.com",
		path:    "/openai/v1/models",
		note:    "Groq no tiene un endpoint de consumo ni de facturación: se ven en console.groq.com. Sus límites solo llegan en las cabeceras de las llamadas de inferencia, que Karina no hace.",
	}
}

// ID implements api.Provider.
func (p *Provider) ID() string { return p.id }

// DisplayName implements api.Provider.
func (p *Provider) DisplayName() string { return p.name }

// Capabilities implements api.Provider: none, on purpose.
func (p *Provider) Capabilities() []domain.Capability { return nil }

// Refresh implements api.Provider.
func (p *Provider) Refresh(ctx context.Context, cfg api.Config) (domain.ProviderState, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = p.baseURL
	}
	st := domain.ProviderState{
		Provider:     domain.ProviderID(p.id),
		DisplayName:  p.name,
		UpdatedAt:    time.Now(),
		Capabilities: p.Capabilities(),
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return st, api.NewError(api.KindInvalidCredentials, 0, "falta la clave API", nil)
	}

	resp, body, err := api.GET(ctx, cfg, p.path, map[string]string{
		"Authorization": "Bearer " + cfg.APIKey,
	})
	if err != nil {
		return st, err
	}
	if resp.StatusCode != 200 {
		for _, bad := range p.badKeyStatuses {
			if resp.StatusCode == bad {
				return api.StateForError(p, api.NewError(api.KindInvalidCredentials, resp.StatusCode, "clave API no válida o caducada", nil), st), nil
			}
		}
		return api.StateForError(p, api.ErrorFromResponse(resp.StatusCode, api.ParseRetryAfter(resp)), st), nil
	}

	if p.inspect != nil {
		if problem := p.inspect(body); problem != "" {
			return api.StateForError(p, api.NewError(api.KindPermission, resp.StatusCode, problem, nil), st), nil
		}
	}
	st.Status = domain.StatusConnected
	st.StatusMsg = "Conectado"
	st.Note = p.note
	return st, nil
}

// inspectXAI reports a key or team that xAI has blocked or disabled.
func inspectXAI(body []byte) string {
	var info struct {
		APIKeyBlocked  bool `json:"api_key_blocked"`
		APIKeyDisabled bool `json:"api_key_disabled"`
		TeamBlocked    bool `json:"team_blocked"`
	}
	if json.Unmarshal(body, &info) != nil {
		return ""
	}
	switch {
	case info.APIKeyDisabled:
		return "la clave está desactivada en la consola de xAI"
	case info.APIKeyBlocked:
		return "xAI ha bloqueado esta clave"
	case info.TeamBlocked:
		return "xAI ha bloqueado el equipo de esta clave (revisa la facturación)"
	}
	return ""
}

var _ api.Provider = (*Provider)(nil)

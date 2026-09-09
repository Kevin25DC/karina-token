// Package gemini implements the Provider interface for Google Gemini API.
//
// REAL CAPABILITIES (verified against Google AI docs):
//   - A Gemini API key (AIza...) can list models (GET /v1beta/models), which
//     is used to validate credentials.
//   - The Gemini API does NOT expose a quota/usage/token-consumption endpoint
//     for an API key, nor rate-limit headers, nor billing. Usage is only
//     visible in the Google AI Studio / Google Cloud consoles (or requires a
//     full Vertex AI + Cloud Monitoring setup with service-account
//     credentials, which is out of scope for a personal API key).
//
// Therefore this adapter reports connection status only and honestly marks
// usage, rate limits and billing as unavailable through the public API.
package gemini

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"karina/internal/domain"
	"karina/internal/providers/api"
)

const (
	id      = "gemini"
	baseURL = "https://generativelanguage.googleapis.com/v1beta"
)

// Provider talks to the Google Gemini API.
type Provider struct{}

// New returns a Gemini provider adapter.
func New() *Provider { return &Provider{} }

// ID implements api.Provider.
func (p *Provider) ID() string { return id }

// DisplayName implements api.Provider.
func (p *Provider) DisplayName() string { return "Google Gemini" }

// Capabilities implements api.Provider. Gemini exposes no usage/billing data
// through the public key API, so the list is intentionally empty.
func (p *Provider) Capabilities() []domain.Capability { return nil }

// Refresh implements api.Provider.
func (p *Provider) Refresh(ctx context.Context, cfg api.Config) (domain.ProviderState, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = baseURL
	}
	st := domain.ProviderState{
		Provider:     id,
		DisplayName:  p.DisplayName(),
		UpdatedAt:    time.Now(),
		Capabilities: p.Capabilities(),
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return st, api.NewError(api.KindInvalidCredentials, 0, "falta la clave API", nil)
	}

	resp, body, err := api.GET(ctx, cfg, "/models", map[string]string{
		"x-goog-api-key": cfg.APIKey,
	})
	if err != nil {
		return st, err
	}

	if resp.StatusCode == 200 {
		st.Status = domain.StatusConnected
		st.StatusMsg = "Conectado"
		st.Note = "La API de Gemini no expone uso de tokens, cuota ni facturación para una clave de API. El uso solo está disponible en la consola de Google AI Studio, o vía Google Cloud (Vertex AI + Cloud Monitoring) con una cuenta de servicio."
		return st, nil
	}

	msg := googleError(body)
	switch resp.StatusCode {
	case 400, 401, 403:
		if strings.Contains(msg, "API key not valid") || strings.Contains(msg, "API key not found") {
			return api.StateForError(p, api.NewError(api.KindInvalidCredentials, resp.StatusCode, "clave API no válida o caducada", nil), st), nil
		}
		// 403 on list-models with a valid key usually means the key has no
		// permission on the API. Treating that as credentials failure is
		// misleading, so keep it a plain error state.
		return api.StateForError(p, api.ErrorFromResponse(resp.StatusCode, api.ParseRetryAfter(resp)), st), nil
	default:
		return api.StateForError(p, api.ErrorFromResponse(resp.StatusCode, api.ParseRetryAfter(resp)), st), nil
	}
}

func googleError(body []byte) string {
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		return ""
	}
	return e.Error.Message
}

var _ api.Provider = (*Provider)(nil)

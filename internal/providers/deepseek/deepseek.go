// Package deepseek implements the Provider interface for DeepSeek.
//
// REAL CAPABILITIES (verified against api-docs.deepseek.com):
//   - The DeepSeek API (OpenAI-compatible) can list models (GET /models) and
//     exposes exactly one account endpoint: GET /user/balance, which returns
//     the prepaid MONETARY balance (CNY or USD), not token usage.
//   - There is no endpoint for global/historical token consumption and no
//     documented rate-limit headers.
//
// The adapter therefore reports connectivity plus the real monetary balance
// and clearly explains that token usage is unavailable through the API.
package deepseek

import (
	"context"
	"strconv"
	"strings"
	"time"

	"karina/internal/domain"
	"karina/internal/providers/api"
)

const (
	id      = "deepseek"
	baseURL = "https://api.deepseek.com"
)

// Provider talks to the DeepSeek API.
type Provider struct{}

// New returns a DeepSeek provider adapter.
func New() *Provider { return &Provider{} }

// ID implements api.Provider.
func (p *Provider) ID() string { return id }

// DisplayName implements api.Provider.
func (p *Provider) DisplayName() string { return "DeepSeek" }

// Capabilities implements api.Provider.
func (p *Provider) Capabilities() []domain.Capability {
	return []domain.Capability{domain.CapBilling}
}

func authHeader(key string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + key}
}

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

	resp, _, err := api.GET(ctx, cfg, "/models", authHeader(cfg.APIKey))
	if err != nil {
		return st, err
	}
	if resp.StatusCode != 200 {
		return api.StateForError(p, api.ErrorFromResponse(resp.StatusCode, api.ParseRetryAfter(resp)), st), nil
	}

	// Only the balance endpoint is real, and only when not validating.
	if cfg.ValidationOnly {
		st.Status = domain.StatusConnected
		st.StatusMsg = "Conectado"
		st.Note = "DeepSeek no expone el uso de tokens por API. Su único endpoint de cuenta reporta el saldo monetario prepagado."
		return st, nil
	}

	balResp, balBody, err := api.GET(ctx, cfg, "/user/balance", authHeader(cfg.APIKey))
	if err != nil {
		return st, err
	}
	if balResp.StatusCode != 200 {
		// Balance endpoint is optional extra data: a failure here must not
		// break a provider that otherwise validated.
		st.Status = domain.StatusConnected
		st.StatusMsg = "Conectado"
		st.Note = "Conectado, pero no se pudo leer el endpoint de saldo. DeepSeek no expone el uso de tokens."
		return st, nil
	}
	var parsed deepSeekBalance
	if err := api.DecodeJSON(balBody, &parsed); err != nil {
		st.Status = domain.StatusConnected
		st.StatusMsg = "Conectado"
		st.Note = "Conectado, pero no se pudo interpretar la respuesta del saldo. DeepSeek no expone el uso de tokens."
		return st, nil
	}

	st.Status = domain.StatusConnected
	st.StatusMsg = "Conectado"
	if len(parsed.BalanceInfos) > 0 {
		b := parsed.BalanceInfos[0]
		st.Balance = &domain.Balance{
			Currency:    b.Currency,
			Total:       parseMoney(b.TotalBalance),
			Granted:     parseMoney(b.GrantedBalance),
			ToppedUp:    parseMoney(b.ToppedUpBalance),
			IsAvailable: parsed.IsAvailable,
		}
	}
	st.Note = "DeepSeek solo expone el saldo monetario prepagado (facturación). No tiene API de consumo de tokens, por lo que no se muestra barra de uso."
	return st, nil
}

func parseMoney(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

type deepSeekBalance struct {
	IsAvailable  bool `json:"is_available"`
	BalanceInfos []struct {
		Currency        string `json:"currency"`
		TotalBalance    string `json:"total_balance"`
		GrantedBalance  string `json:"granted_balance"`
		ToppedUpBalance string `json:"topped_up_balance"`
	} `json:"balance_infos"`
}

var _ api.Provider = (*Provider)(nil)

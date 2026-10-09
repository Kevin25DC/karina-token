// Package openrouter implements the Provider interface for OpenRouter.
//
// REAL CAPABILITIES (checked against openrouter.ai/docs, 2026-10-08):
//   - GET /api/v1/key returns the key in use: its credit limit (or null),
//     credits remaining, and credits spent all time / this month / this week
//     / today. Credits are US dollars. Works with a normal API key.
//   - GET /api/v1/credits returns the ACCOUNT totals (purchased and used) but
//     only accepts a Management key; a normal key gets a 403.
//   - There is no token-count endpoint: OpenRouter accounts in dollars.
//
// So the adapter shows money, not tokens: the key's remaining limit when it
// has one, otherwise the account balance when the key is allowed to read it,
// and always what the key has spent.
package openrouter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"karina/internal/domain"
	"karina/internal/providers/api"
)

const (
	id      = "openrouter"
	baseURL = "https://openrouter.ai/api/v1"
)

// Provider talks to the OpenRouter API.
type Provider struct{}

// New returns an OpenRouter provider adapter.
func New() *Provider { return &Provider{} }

// ID implements api.Provider.
func (p *Provider) ID() string { return id }

// DisplayName implements api.Provider.
func (p *Provider) DisplayName() string { return "OpenRouter" }

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

	resp, body, err := api.GET(ctx, cfg, "/key", authHeader(cfg.APIKey))
	if err != nil {
		return st, err
	}
	if resp.StatusCode != 200 {
		return api.StateForError(p, api.ErrorFromResponse(resp.StatusCode, api.ParseRetryAfter(resp)), st), nil
	}
	st.Status = domain.StatusConnected
	st.StatusMsg = "Conectado"

	var key keyResponse
	if err := api.DecodeJSON(body, &key); err != nil {
		st.Note = "Conectado, pero no se pudo interpretar la información de la clave."
		return st, nil
	}
	d := key.Data
	spent := fmt.Sprintf("Gastado con esta clave: $%.2f este mes, $%.2f hoy, $%.2f en total.", d.UsageMonthly, d.UsageDaily, d.Usage)

	switch {
	case d.LimitRemaining != nil:
		// The key has its own credit limit: that is what can run out.
		st.Balance = &domain.Balance{Currency: "USD", Total: *d.LimitRemaining, IsAvailable: *d.LimitRemaining > 0}
		st.Note = "Saldo = crédito que le queda a esta clave según su límite. " + spent
	case !cfg.ValidationOnly:
		// No per-key limit: the account balance, if this key may read it.
		if total, used, ok := p.credits(ctx, cfg); ok {
			left := total - used
			st.Balance = &domain.Balance{Currency: "USD", Total: left, ToppedUp: total, IsAvailable: left > 0}
			st.Note = "Saldo = créditos de la cuenta. " + spent
		} else {
			st.Note = spent + " La clave no tiene límite propio; para ver el saldo de la cuenta OpenRouter exige una clave de gestión (Management key)."
		}
	default:
		st.Note = spent
	}
	return st, nil
}

// credits reads the account totals. It only works with a Management key, so
// any failure simply means "not available" and never breaks the provider.
func (p *Provider) credits(ctx context.Context, cfg api.Config) (total, used float64, ok bool) {
	resp, body, err := api.GET(ctx, cfg, "/credits", authHeader(cfg.APIKey))
	if err != nil || resp.StatusCode != 200 {
		return 0, 0, false
	}
	var c creditsResponse
	if api.DecodeJSON(body, &c) != nil {
		return 0, 0, false
	}
	return c.Data.TotalCredits, c.Data.TotalUsage, true
}

type keyResponse struct {
	Data struct {
		Label          string   `json:"label"`
		Limit          *float64 `json:"limit"`
		LimitRemaining *float64 `json:"limit_remaining"`
		Usage          float64  `json:"usage"`
		UsageDaily     float64  `json:"usage_daily"`
		UsageMonthly   float64  `json:"usage_monthly"`
	} `json:"data"`
}

type creditsResponse struct {
	Data struct {
		TotalCredits float64 `json:"total_credits"`
		TotalUsage   float64 `json:"total_usage"`
	} `json:"data"`
}

var _ api.Provider = (*Provider)(nil)

// Package mock provides a simulated provider used ONLY for development,
// previewing the UI and running tests. It must never appear as a real
// provider in production paths unless explicitly toggled as "demo".
package mock

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"karina/internal/domain"
	"karina/internal/providers/api"
)

const id = "demo"

// Provider simulates an AI provider with a monthly token budget so the
// dashboard and animations can be exercised without real credentials.
type Provider struct {
	mu      sync.Mutex
	limit   int64
	used    int64
	rng     *rand.Rand
	updated time.Time
}

// New returns a demo provider with the given monthly token limit.
func New(limit int64) *Provider {
	if limit <= 0 {
		limit = 100_000
	}
	return &Provider{limit: limit, rng: rand.New(rand.NewSource(time.Now().UnixNano())), updated: time.Now()}
}

// SetUsed forces the current used value (used by tests).
func (p *Provider) SetUsed(v int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if v < 0 {
		v = 0
	}
	p.used = v
}

// SetLimit forces the monthly limit (used by tests).
func (p *Provider) SetLimit(v int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.limit = v
}

// ID implements api.Provider.
func (p *Provider) ID() string { return id }

// DisplayName implements api.Provider.
func (p *Provider) DisplayName() string { return "Demo provider" }

// Capabilities implements api.Provider.
func (p *Provider) Capabilities() []domain.Capability {
	return []domain.Capability{domain.CapTokenUsage}
}

// Refresh implements api.Provider producing a realistic random-walk usage.
func (p *Provider) Refresh(_ context.Context, cfg api.Config) (domain.ProviderState, error) {
	st := domain.ProviderState{
		Provider:     id,
		DisplayName:  p.DisplayName(),
		Capabilities: p.Capabilities(),
		UpdatedAt:    time.Now(),
	}
	if cfg.APIKey == "" {
		return st, api.NewError(api.KindInvalidCredentials, 0, "demo requires any non-empty key", nil)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Random walk: drift up a little most of the time, occasionally reset.
	step := int64(p.rng.Float64()*float64(p.limit)*0.004) + 50
	if p.rng.Float64() < 0.08 {
		step = -int64(float64(p.used) * 0.03)
	}
	p.used += step
	if p.used < 0 {
		p.used = 0
	}
	max := int64(float64(p.limit) * 0.97)
	if p.used > max {
		p.used = max
	}

	now := time.Now()
	st.Status = domain.StatusConnected
	st.StatusMsg = "Simulado"
	st.UsageAvailable = true
	st.UsedTokens = p.used
	st.LimitTokens = p.limit
	st.RemainingTokens = domain.Remaining(p.used, p.limit)
	st.UsageWindow = "Monthly demo budget"
	st.ResetAt = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, 1, 0)
	st.Note = "Proveedor demo. Estos datos son simulados para que previsualices la interfaz. Conecta un proveedor real para monitorear el uso real."
	return st, nil
}

var _ api.Provider = (*Provider)(nil)

// UsageOf is a test helper for the provider's demo value.
func (p *Provider) UsageOf() (used, limit int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.used, p.limit
}

// String implements fmt.Stringer for nicer logging.
func (p *Provider) String() string {
	used, limit := p.UsageOf()
	return fmt.Sprintf("demo(used=%d limit=%d)", used, limit)
}

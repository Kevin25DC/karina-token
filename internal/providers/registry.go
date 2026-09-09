// Package providers is the registry of supported AI providers. It owns the
// static catalog (metadata) and the factory that maps a provider id to an
// adapter implementing api.Provider.
package providers

import (
	"fmt"

	"karina/internal/domain"
	"karina/internal/providers/anthropic"
	"karina/internal/providers/api"
	"karina/internal/providers/deepseek"
	"karina/internal/providers/gemini"
	"karina/internal/providers/mock"
	"karina/internal/providers/openai"
)

var catalog = []domain.ProviderMeta{
	{
		ID:          "anthropic",
		Name:        "Anthropic Claude",
		Description: "Modelos Claude de Anthropic",
		DocsURL:     "https://platform.claude.com/docs",
		Brand:       "amber",
		Capabilities: []domain.Capability{
			domain.CapTokenUsage, // org usage report (Admin API)
			domain.CapRateLimits, // anthropic-ratelimit-* headers
		},
	},
	{
		ID:          "openai",
		Name:        "OpenAI",
		Description: "Modelos de la plataforma OpenAI / ChatGPT",
		DocsURL:     "https://platform.openai.com/docs",
		Brand:       "emerald",
		Capabilities: []domain.Capability{
			domain.CapTokenUsage, // organization usage (Admin API)
			domain.CapRateLimits, // x-ratelimit-* headers
		},
	},
	{
		ID:           "gemini",
		Name:         "Google Gemini",
		Description:  "Modelos Gemini (Google AI Studio)",
		DocsURL:      "https://ai.google.dev/gemini-api/docs",
		Brand:        "blue",
		Capabilities: []domain.Capability{},
	},
	{
		ID:           "deepseek",
		Name:         "DeepSeek",
		Description:  "Modelos de chat y razonamiento de DeepSeek",
		DocsURL:      "https://api-docs.deepseek.com",
		Brand:        "sky",
		Capabilities: []domain.Capability{domain.CapBilling}, // /user/balance
	},
	{
		ID:           "demo",
		Name:         "Proveedor demo",
		Description:  "Uso simulado para previsualizar Karina",
		DocsURL:      "",
		Brand:        "violet",
		Demo:         true,
		Capabilities: []domain.Capability{domain.CapTokenUsage},
	},
}

var order = []domain.ProviderID{"anthropic", "openai", "gemini", "deepseek", "demo"}

// Catalog returns the ordered, immutable provider catalog.
func Catalog() []domain.ProviderMeta {
	out := make([]domain.ProviderMeta, len(order))
	for i, id := range order {
		out[i] = metaOf(id)
	}
	return out
}

func metaOf(id domain.ProviderID) domain.ProviderMeta {
	for _, m := range catalog {
		if m.ID == id {
			return m
		}
	}
	return domain.ProviderMeta{ID: id, Name: string(id)}
}

// Get returns the metadata for a provider id.
func Get(id domain.ProviderID) (domain.ProviderMeta, bool) {
	for _, m := range catalog {
		if m.ID == id {
			return m, true
		}
	}
	return domain.ProviderMeta{}, false
}

// New builds a fresh adapter for the given provider id.
func New(id domain.ProviderID) (api.Provider, error) {
	switch id {
	case "anthropic":
		return anthropic.New(), nil
	case "openai":
		return openai.New(), nil
	case "gemini":
		return gemini.New(), nil
	case "deepseek":
		return deepseek.New(), nil
	case "demo":
		return mock.New(100_000), nil
	default:
		return nil, fmt.Errorf("unknown provider %q", id)
	}
}

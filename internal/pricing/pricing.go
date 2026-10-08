// Package pricing estimates what a number of tokens would cost on the public
// Anthropic API. It is an ESTIMATE: list prices change, and a claude.ai
// subscription is not billed per token at all — the figure answers "what
// would this usage have cost on the API", nothing else.
package pricing

import "strings"

// AsOf is the date the price table below was last checked.
const AsOf = "2026-10-06"

// Price holds USD per million tokens.
type Price struct {
	Input        float64
	Output       float64
	CacheRead    float64
	CacheWrite5m float64
	CacheWrite1h float64
}

// price builds a Price from the list input/output rates. Cache writes follow
// the standard multipliers (1.25x input for 5 minutes, 2x for 1 hour). Cache
// reads are 0.1x input unless the model publishes its own rate.
func price(input, output, cacheRead float64) Price {
	if cacheRead == 0 {
		cacheRead = input * 0.1
	}
	return Price{
		Input:        input,
		Output:       output,
		CacheRead:    cacheRead,
		CacheWrite5m: input * 1.25,
		CacheWrite1h: input * 2,
	}
}

// table maps a model id prefix to its price. Lookups pick the longest
// matching prefix, so dated or suffixed ids ("claude-opus-5-5-2026…",
// "claude-sonnet-5[1m]") resolve to their family.
var table = map[string]Price{
	"claude-fable-5-1":  price(10, 50, 0.25),
	"claude-mythos-5-1": price(10, 50, 0.25),
	"claude-fable-5":    price(10, 50, 0),
	"claude-opus-5-5":   price(4, 20, 0.20),
	"claude-opus-5":     price(5, 25, 0),
	"claude-opus-4-8":   price(5, 25, 0),
	"claude-opus-4-7":   price(5, 25, 0),
	"claude-opus-4-6":   price(5, 25, 0),
	"claude-sonnet-5-5": price(2, 10, 0.20),
	"claude-sonnet-5":   price(2, 10, 0),
	"claude-sonnet-4-6": price(3, 15, 0),
	"claude-haiku-5-5":  price(0.10, 0.50, 0),
	"claude-haiku-4-5":  price(1, 5, 0),
}

// For returns the price of a model id, or false when Karina has no price
// for it (the caller must then report the usage as not priced, never guess).
func For(model string) (Price, bool) {
	best := ""
	for prefix := range table {
		if strings.HasPrefix(model, prefix) && len(prefix) > len(best) {
			best = prefix
		}
	}
	if best == "" {
		return Price{}, false
	}
	return table[best], true
}

// Cost returns the USD cost of the given token counts.
func (p Price) Cost(input, output, cacheWrite5m, cacheWrite1h, cacheRead int64) float64 {
	const perMillion = 1e6
	return (float64(input)*p.Input +
		float64(output)*p.Output +
		float64(cacheWrite5m)*p.CacheWrite5m +
		float64(cacheWrite1h)*p.CacheWrite1h +
		float64(cacheRead)*p.CacheRead) / perMillion
}

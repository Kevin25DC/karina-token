package api

import (
	"net/http"
	"strconv"
	"time"

	"karina/internal/domain"
)

func parseBucket(resetHeader string, resetInHeader string, limit, remaining int64, window string) *domain.RateBucket {
	if limit <= 0 {
		return nil
	}
	b := &domain.RateBucket{Limit: limit, Remaining: remaining, Window: window}
	if reset := time.Now().Add(time.Duration(atof(resetHeader)) * time.Second); resetHeader != "" {
		b.ResetAt = reset
	}
	if resetInHeader != "" {
		if t, err := http.ParseTime(resetInHeader); err == nil {
			b.ResetAt = t
		}
	}
	return b
}

func atof(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func atoi(s string) int64 {
	i, _ := strconv.ParseInt(s, 10, 64)
	return i
}

// ParseOpenAIHeaders extracts the x-ratelimit-* buckets from a response.
// OpenAI documents: limit-requests, limit-tokens, remaining-requests,
// remaining-tokens, reset-requests, reset-tokens (seconds).
func ParseOpenAIHeaders(h http.Header) domain.RateLimitInfo {
	info := domain.RateLimitInfo{}
	reqLimit := atoi(h.Get("x-ratelimit-limit-requests"))
	if reqLimit > 0 {
		info.Requests = parseBucket(h.Get("x-ratelimit-reset-requests"), "",
			reqLimit, atoi(h.Get("x-ratelimit-remaining-requests")), "minute")
	}
	tokLimit := atoi(h.Get("x-ratelimit-limit-tokens"))
	if tokLimit > 0 {
		info.Tokens = parseBucket(h.Get("x-ratelimit-reset-tokens"), "",
			tokLimit, atoi(h.Get("x-ratelimit-remaining-tokens")), "minute")
	}
	return info
}

// ParseAnthropicHeaders extracts the anthropic-ratelimit-* buckets.
// Anthropic documents limit/remaining/reset for requests, input-tokens,
// output-tokens and a combined tokens bucket.
func ParseAnthropicHeaders(h http.Header) domain.RateLimitInfo {
	info := domain.RateLimitInfo{}
	req := parseBucket("", h.Get("anthropic-ratelimit-requests-reset"),
		atoi(h.Get("anthropic-ratelimit-requests-limit")),
		atoi(h.Get("anthropic-ratelimit-requests-remaining")), "requests")
	if req != nil {
		info.Requests = req
	}
	in := parseBucket("", h.Get("anthropic-ratelimit-input-tokens-reset"),
		atoi(h.Get("anthropic-ratelimit-input-tokens-limit")),
		atoi(h.Get("anthropic-ratelimit-input-tokens-remaining")), "input tokens/min")
	if in != nil {
		info.InputTokens = in
	}
	out := parseBucket("", h.Get("anthropic-ratelimit-output-tokens-reset"),
		atoi(h.Get("anthropic-ratelimit-output-tokens-limit")),
		atoi(h.Get("anthropic-ratelimit-output-tokens-remaining")), "output tokens/min")
	if out != nil {
		info.OutputTokens = out
	}
	tok := parseBucket("", h.Get("anthropic-ratelimit-tokens-reset"),
		atoi(h.Get("anthropic-ratelimit-tokens-limit")),
		atoi(h.Get("anthropic-ratelimit-tokens-remaining")), "tokens/min")
	if tok != nil {
		info.Tokens = tok
	}
	return info
}

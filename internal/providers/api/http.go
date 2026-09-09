package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DefaultClient returns the shared HTTP client used for all provider calls.
// Timeouts keep a dead provider from blocking the scheduler for long.
func DefaultClient() *http.Client {
	return &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			DialContext:         (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
			TLSHandshakeTimeout: 10 * time.Second,
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
		},
	}
}

// maxBody caps response bodies so a runaway endpoint cannot exhaust memory.
const maxBody = 4 << 20 // 4 MiB

// GET performs an authenticated GET and returns the raw response. Transport
// level errors are wrapped as KindNetwork / KindTimeout.
func GET(ctx context.Context, cfg Config, path string, headers map[string]string) (*http.Response, []byte, error) {
	url := strings.TrimRight(cfg.BaseURL, "/") + "/" + strings.TrimLeft(path, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, NewError(KindUnexpected, 0, "build request", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := cfg.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, NewError(KindTimeout, 0, ctx.Err().Error(), ctx.Err())
		}
		return nil, nil, NewError(KindNetwork, 0, "network error", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, nil, NewError(KindNetwork, resp.StatusCode, "read response", err)
	}
	return resp, body, nil
}

// DecodeJSON parses a JSON payload, surfacing malformed responses clearly.
func DecodeJSON(payload []byte, out any) error {
	if err := json.Unmarshal(payload, out); err != nil {
		return NewError(KindUnexpected, 0, "invalid JSON response", err)
	}
	return nil
}

// Classify maps an HTTP status onto the shared taxonomy. A 401/403 on the
// validation endpoint means the key itself is not accepted.
func Classify(status int, retryAfter time.Duration) *ProviderError {
	switch status {
	case 401:
		return NewError(KindInvalidCredentials, status, "invalid or expired API key", nil)
	case 402:
		return NewError(KindPayment, status, "billing/payment error on provider account", nil)
	case 403:
		return NewError(KindPermission, status, "key lacks permission for this resource", nil)
	case 404:
		return NewError(KindNotFound, status, "resource not found", nil)
	case 408, 504:
		return NewError(KindTimeout, status, "provider request timed out", nil)
	case 429:
		return NewError(KindRateLimited, status, "rate limited", nil)
	default:
		if status >= 500 {
			return NewError(KindServer, status, "provider server error", nil)
		}
		return NewError(KindUnexpected, status, "unexpected status", nil)
	}
}

// ParseRetryAfter extracts the Retry-After header if present.
func ParseRetryAfter(resp *http.Response) time.Duration {
	raw := resp.Header.Get("Retry-After")
	if raw == "" {
		raw = resp.Header.Get("retry-after")
	}
	if raw == "" {
		return 0
	}
	if secs, err := strconv.Atoi(raw); err == nil {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(raw); err == nil {
		return time.Until(t)
	}
	return 0
}

// ErrorFromResponse builds a classified error from an HTTP response body that
// was already read.
func ErrorFromResponse(status int, retryAfter time.Duration) *ProviderError {
	pe := Classify(status, retryAfter)
	pe.RetryAfter = retryAfter
	return pe
}

// JSONBody is a helper used in tests to render JSON responses.
func JSONBody(v any) io.Reader {
	data, _ := json.Marshal(v)
	return bytes.NewReader(data)
}

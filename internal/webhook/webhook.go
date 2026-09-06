// Package webhook posts threshold-alert notifications to a user-configured
// HTTP endpoint (Slack incoming webhook, Discord webhook, or any custom
// receiver).
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Details carries the structured facts behind an alert, for receivers that
// want to parse them instead of (or alongside) the human-readable text.
type Details struct {
	Provider        string  `json:"provider"`
	ProviderDisplay string  `json:"provider_display"`
	Window          string  `json:"window"`
	Percent         float64 `json:"percent"`
	ResetAt         string  `json:"reset_at,omitempty"`
	FiredAt         string  `json:"fired_at"`
}

// Payload is intentionally shaped to work out of the box with the two most
// common destinations without any per-provider configuration:
//   - Discord webhooks read "content".
//   - Slack incoming webhooks read "text".
//
// Both fields carry the same human-readable message; unknown receivers can
// ignore them and read the structured "karina" object instead.
type Payload struct {
	Content string  `json:"content"`
	Text    string  `json:"text"`
	Karina  Details `json:"karina"`
}

// Send POSTs the payload as JSON to url. A blank url is a no-op (returns
// nil) so callers don't need to guard every call site on whether a webhook
// is configured.
func Send(ctx context.Context, client *http.Client, url string, payload Payload) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil
	}
	if client == nil {
		client = http.DefaultClient
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook responded with status %d", resp.StatusCode)
	}
	return nil
}

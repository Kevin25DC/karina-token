// Package claudesub is an EXPERIMENTAL reader for the usage of a claude.ai
// subscription (Pro/Max).
//
// WARNING: Anthropic does not provide an official API for per-person
// subscription usage. This reader reuses the OAuth token that the user's own
// Claude Code installation already stores locally and calls an UNDOCUMENTED
// endpoint (GET https://api.anthropic.com/api/oauth/usage). Consequences:
//   - It may break at any time without notice.
//   - Using Claude.ai subscription OAuth outside native Anthropic apps can
//     violate the terms of service (enforcement/bans were reported in 2026).
//
// The token is only read from the local machine, never stored by Karina and
// never sent anywhere except to Anthropic itself.
package claudesub

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/zalando/go-keyring"

	"karina/internal/credentials"
)

// usageURL is a var so tests can point it at an httptest server.
var usageURL = "https://api.anthropic.com/api/oauth/usage"

// Result is what the experimental reader found.
type Result struct {
	Found   bool    `json:"found"`
	Source  string  `json:"source"`
	Window  string  `json:"window"`
	Used    int64   `json:"used"`
	Limit   int64   `json:"limit"`
	Percent float64 `json:"percent"`
	ResetAt string  `json:"reset_at,omitempty"`
	Error   string  `json:"error,omitempty"`
}

// Reader discovers the local Claude Code token and queries the usage
// endpoint. HTTP is injectable for tests.
type Reader struct {
	HTTP *http.Client
	// OverrideHome lets tests redirect the .claude discovery directory.
	OverrideHome string
	// OverrideToken overrides the env var (tests).
	OverrideToken string
	// SkipKeyring disables the OS credential-store lookup (tests).
	SkipKeyring bool
}

func (r *Reader) http() *http.Client {
	if r.HTTP != nil {
		return r.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}

// Read returns the current subscription usage window when it can.
func (r *Reader) Read(ctx context.Context) Result {
	token, source, err := r.discoverToken()
	if err != nil {
		return Result{Found: false, Error: err.Error()}
	}
	if token == "" {
		return Result{
			Found: false,
			Error: "No se encontró una sesión de Claude Code. Abre claude.ai o ejecuta `claude /login` una vez y vuelve a intentarlo.",
		}
	}

	res := Result{Found: true, Source: source}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, usageURL, nil)
	if err != nil {
		res.Found = false
		res.Error = err.Error()
		return res
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	// Required to accept an OAuth Bearer token on Anthropic endpoints.
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("User-Agent", "Karina/0.2-experimental")

	httpResp, err := r.http().Do(req)
	if err != nil {
		res.Found = false
		res.Error = "No se pudo contactar a Anthropic: " + err.Error()
		return res
	}
	defer httpResp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(httpResp.Body, 2<<20))
	if err != nil {
		res.Found = false
		res.Error = "Error leyendo respuesta"
		return res
	}
	if httpResp.StatusCode != http.StatusOK {
		res.Found = false
		res.Error = fmt.Sprintf("Anthropic respondió HTTP %d (el endpoint es experimental)", httpResp.StatusCode)
		return res
	}

	if !applyUsage(&res, body) {
		res.Found = false
		res.Error = "La respuesta del endpoint no se pudo interpretar (cambió el formato). Sigue disponible la lectura manual."
	}
	return res
}

// discoverToken looks for the token Claude Code stores locally.
func (r *Reader) discoverToken() (token, source string, err error) {
	if tok := strings.TrimSpace(r.OverrideToken); tok != "" {
		return tok, "token proporcionado", nil
	}
	if tok := strings.TrimSpace(os.Getenv("KARINA_CLAUDE_TOKEN")); tok != "" {
		return tok, "variable de entorno KARINA_CLAUDE_TOKEN", nil
	}

	home := r.OverrideHome
	if home == "" {
		home, err = os.UserHomeDir()
		if err != nil {
			return "", "", err
		}
	}
	appData := os.Getenv("APPDATA")
	configDir, _ := os.UserConfigDir()
	candidates := []string{
		filepath.Join(home, ".claude", ".credentials.json"),
		filepath.Join(home, ".claude", "credentials.json"),
		filepath.Join(home, ".claude", ".credentials"),
		filepath.Join(configDir, "claude", ".credentials.json"),
		filepath.Join(configDir, "Claude", ".credentials.json"),
	}
	if appData != "" {
		candidates = append(candidates,
			filepath.Join(appData, "claude", ".credentials.json"),
			filepath.Join(appData, "Claude", ".credentials.json"),
		)
	}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if tok := readTokenFile(p); tok != "" {
			return tok, p, nil
		}
	}

	// Windows Credential Manager / macOS Keychain / Secret Service.
	if !r.SkipKeyring {
		if tok := readFromKeyring(); tok != "" {
			return tok, "almacén de credenciales del sistema", nil
		}
	}
	return "", "", nil
}

func readFromKeyring() string {
	// Token stored by Karina's own experimental OAuth login.
	if secret, err := keyring.Get(credentials.Service, "claude_subscription_oauth"); err == nil && secret != "" {
		var tok Token
		if json.Unmarshal([]byte(secret), &tok) == nil && tok.AccessToken != "" {
			return tok.AccessToken
		}
		if looksLikeToken(secret) {
			return strings.TrimSpace(secret)
		}
	}

	services := []string{"Claude Code-credentials", "Claude Code", "claude-code", "claude.ai"}
	accounts := []string{"", "claude", "Claude Code"}
	for _, svc := range services {
		for _, acc := range accounts {
			secret, err := keyring.Get(svc, acc)
			if err != nil || secret == "" {
				continue
			}
			var m map[string]any
			if json.Unmarshal([]byte(secret), &m) == nil {
				if tok := findToken(m, 0); tok != "" {
					return tok
				}
			}
			if looksLikeToken(secret) {
				return strings.TrimSpace(secret)
			}
		}
	}
	return ""
}

func readTokenFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var m map[string]any
	if json.Unmarshal(data, &m) != nil {
		return ""
	}
	return findToken(m, 0)
}

// tokenKeys are the JSON field names Claude Code has used across versions.
var tokenKeys = map[string]bool{
	"token":         true,
	"accessToken":   true,
	"access_token":  true,
	"oauthToken":    true,
	"oauth_token":   true,
	"claudeAiOauth": true,
}

// findToken does a bounded DFS looking for a token-like string value.
func findToken(v any, depth int) string {
	if depth > 6 {
		return ""
	}
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			if s, ok := child.(string); ok {
				if looksLikeToken(s) || (tokenKeys[k] && len(s) > 20) {
					return s
				}
			}
			if s := findToken(child, depth+1); s != "" {
				return s
			}
		}
	case []any:
		for _, child := range t {
			if s := findToken(child, depth+1); s != "" {
				return s
			}
		}
	}
	return ""
}

// looksLikeToken reports whether a string looks like a Claude OAuth token.
func looksLikeToken(s string) bool {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "sk-ant-") {
		return true
	}
	return false
}

// windowLabels maps the known legacy window keys to friendly names.
var windowLabels = map[string]string{
	"five_hour":            "Ventana de 5 horas",
	"seven_day":            "Semanal (7 días)",
	"seven_day_opus":       "Semanal Opus",
	"seven_day_sonnet":     "Semanal Sonnet",
	"seven_day_oauth_apps": "Semanal (apps OAuth)",
	"seven_day_cowork":     "Semanal (Cowork)",
}

type usageCandidate struct {
	name    string
	percent float64
	reset   string
}

// applyUsage parses the real (undocumented) usage payload:
//   - legacy windows: {"five_hour":{"utilization":25.0,"resets_at":"..."}}
//   - new "Claude 5" format: {"limits":[{"kind":"session","percent":25.0,...}]}
//   - overage: {"extra_usage":{"utilization":18.68,...}}
func applyUsage(res *Result, body []byte) bool {
	var root map[string]any
	if json.Unmarshal(body, &root) != nil {
		return false
	}

	var candidates []usageCandidate

	// Legacy window objects.
	for key, raw := range root {
		obj, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		pct, ok := numOf(obj["utilization"])
		if !ok {
			if pct, ok = numOf(obj["used_percentage"]); !ok {
				continue
			}
		}
		name := windowLabels[key]
		if name == "" {
			name = key
		}
		reset, _ := obj["resets_at"].(string)
		candidates = append(candidates, usageCandidate{name: name, percent: pct, reset: reset})
	}

	// New limits[] array.
	if arr, ok := root["limits"].([]any); ok {
		for _, item := range arr {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			pct, ok := numOf(m["percent"])
			if !ok {
				continue
			}
			name := labelForLimit(m)
			reset, _ := m["resets_at"].(string)
			candidates = append(candidates, usageCandidate{name: name, percent: pct, reset: reset})
		}
	}

	// Extra usage / overage.
	if ex, ok := root["extra_usage"].(map[string]any); ok {
		if pct, ok := numOf(ex["utilization"]); ok {
			candidates = append(candidates, usageCandidate{name: "Uso extra (overage)", percent: pct})
		}
	}

	if len(candidates) == 0 {
		return false
	}

	// Show the most consumed window (that is what matters for the user).
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.percent > best.percent {
			best = c
		}
	}
	res.Window = best.name
	res.Used = int64(best.percent + 0.5)
	res.Limit = 100
	res.Percent = best.percent
	res.ResetAt = best.reset
	return true
}

func labelForLimit(m map[string]any) string {
	if scope, ok := m["scope"].(map[string]any); ok {
		if model, ok := scope["model"].(map[string]any); ok {
			if dn, ok := model["display_name"].(string); ok && dn != "" {
				return "Semanal · " + dn
			}
		}
	}
	switch kind, _ := m["kind"].(string); kind {
	case "session":
		return "Ventana de 5 horas"
	case "weekly_all":
		return "Semanal (7 días)"
	case "weekly_scoped":
		return "Semanal (modelo)"
	}
	return "Límite"
}

func numOf(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		s := strings.TrimSpace(t)
		s = strings.TrimSuffix(s, "%")
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

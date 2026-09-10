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
	if tok := readFromKeyring(); tok != "" {
		return tok, "almacén de credenciales del sistema", nil
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

// applyUsage parses the (undocumented) usage payload tolerantly.
func applyUsage(res *Result, body []byte) bool {
	var root map[string]any
	if json.Unmarshal(body, &root) != nil {
		return false
	}
	for window, raw := range root {
		obj, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		used, hasUsed := numOf(obj["used"])
		pct, hasPct := numOf(obj["used_percent"])
		limit, hasLimit := numOf(obj["limit"])
		if !hasUsed && !hasPct {
			continue
		}
		// If the payload reports only a percentage, show a 0..100 gauge.
		if !hasUsed && hasPct {
			used = pct
			limit = 100
			hasUsed = true
		}
		if !hasLimit {
			limit = 100
		}
		res.Window = window
		res.Used = int64(used)
		res.Limit = int64(limit)
		res.Percent = pct
		if res.Percent <= 0 && res.Limit > 0 {
			res.Percent = used / float64(res.Limit) * 100
		}
		return true
	}
	return false
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

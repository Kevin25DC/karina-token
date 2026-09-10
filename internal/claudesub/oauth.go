package claudesub

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// OAuthConfig holds the parameters of the Claude Code OAuth flow.
//
// WARNING: these are the client_id/endpoints of Anthropic's official Claude
// Code app, NOT a third-party OAuth registration (none exists). Reusing them
// from another application can violate Anthropic's terms of service and lead
// to account enforcement. This is an explicit, opt-in experimental feature.
type OAuthConfig struct {
	ClientID     string
	AuthorizeURL string
	TokenURL     string
	RedirectURI  string
	Scopes       string
}

// DefaultOAuthConfig returns the current Claude Code OAuth parameters.
func DefaultOAuthConfig() OAuthConfig {
	return OAuthConfig{
		ClientID:     "9d1c250a-e61b-44d9-88ed-5944d1962f5e",
		AuthorizeURL: "https://claude.com/cai/oauth/authorize",
		TokenURL:     "https://platform.claude.com/v1/oauth/token",
		RedirectURI:  "https://platform.claude.com/oauth/code/callback",
		Scopes:       "org:create_api_key user:profile user:inference user:sessions:claude_code user:mcp_servers user:file_upload",
	}
}

// Token is the OAuth token response.
type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

// PKCE holds a verifier and its S256 challenge.
type PKCE struct {
	Verifier  string
	Challenge string
}

// NewPKCE generates a PKCE verifier/challenge pair.
func NewPKCE() (PKCE, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return PKCE{}, err
	}
	verifier := base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return PKCE{Verifier: verifier, Challenge: challenge}, nil
}

// AuthorizeURL builds the URL the user must open in the browser. The PKCE
// verifier is also used as the `state` value, matching the Claude Code flow.
func AuthorizeURL(cfg OAuthConfig, pkce PKCE) string {
	q := url.Values{}
	q.Set("code", "true")
	q.Set("client_id", cfg.ClientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", cfg.RedirectURI)
	q.Set("scope", cfg.Scopes)
	q.Set("code_challenge", pkce.Challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", pkce.Verifier)
	return cfg.AuthorizeURL + "?" + q.Encode()
}

// ParseCallbackCode accepts the code in several formats the user may paste:
// a full URL, a query string, "code#state" or a bare code.
func ParseCallbackCode(input string) string {
	s := strings.TrimSpace(input)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		if u, err := url.Parse(s); err == nil {
			if c := u.Query().Get("code"); c != "" {
				return c
			}
		}
	}
	if i := strings.Index(s, "code="); i >= 0 {
		s = s[i+len("code="):]
		if j := strings.IndexAny(s, "&# \n\r\t"); j >= 0 {
			s = s[:j]
		}
		return s
	}
	if i := strings.Index(s, "#"); i >= 0 {
		return s[:i]
	}
	return s
}

// ExchangeCode trades the authorization code for tokens.
func ExchangeCode(ctx context.Context, client *http.Client, cfg OAuthConfig, code, verifier string) (Token, error) {
	if client == nil {
		client = &http.Client{}
	}
	payload := map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     cfg.ClientID,
		"code":          code,
		"state":         verifier,
		"redirect_uri":  cfg.RedirectURI,
		"code_verifier": verifier,
	}
	data, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, bytes.NewReader(data))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return Token{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Token{}, fmt.Errorf("intercambio de token falló (HTTP %d)", resp.StatusCode)
	}
	var tok Token
	if err := json.Unmarshal(body, &tok); err != nil {
		return Token{}, fmt.Errorf("respuesta de token inválida: %w", err)
	}
	if tok.AccessToken == "" {
		return Token{}, fmt.Errorf("la respuesta no incluyó access_token")
	}
	return tok, nil
}

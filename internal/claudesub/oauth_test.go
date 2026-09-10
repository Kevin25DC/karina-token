package claudesub

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestNewPKCE(t *testing.T) {
	p, err := NewPKCE()
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(p.Verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if p.Challenge != want {
		t.Fatalf("challenge mismatch")
	}
	if strings.ContainsAny(p.Verifier, "+/=") {
		t.Fatalf("verifier not base64url: %q", p.Verifier)
	}
}

func TestAuthorizeURLParams(t *testing.T) {
	cfg := DefaultOAuthConfig()
	p := PKCE{Verifier: "verifier123", Challenge: "challenge123"}
	raw := AuthorizeURL(cfg, p)
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	checks := map[string]string{
		"client_id":             cfg.ClientID,
		"response_type":         "code",
		"redirect_uri":          cfg.RedirectURI,
		"code_challenge":        "challenge123",
		"code_challenge_method": "S256",
		"state":                 "verifier123",
		"code":                  "true",
	}
	for k, want := range checks {
		if q.Get(k) != want {
			t.Fatalf("param %s = %q want %q", k, q.Get(k), want)
		}
	}
	if !strings.Contains(q.Get("scope"), "user:inference") {
		t.Fatalf("scope missing: %q", q.Get("scope"))
	}
}

func TestParseCallbackCode(t *testing.T) {
	cases := map[string]string{
		"abc123":              "abc123",
		"abc123#state":        "abc123",
		"code=abc123&state=x": "abc123",
		"https://platform.claude.com/cb?code=abc123&state=x": "abc123",
		"  abc123  ": "abc123",
	}
	for in, want := range cases {
		if got := ParseCallbackCode(in); got != want {
			t.Fatalf("ParseCallbackCode(%q)=%q want %q", in, got, want)
		}
	}
}

func TestExchangeCode(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		fmt.Fprint(w, `{"access_token":"sk-ant-oat01-xyz","refresh_token":"r1","expires_in":3600,"scope":"user:inference"}`)
	}))
	defer srv.Close()

	cfg := DefaultOAuthConfig()
	cfg.TokenURL = srv.URL
	tok, err := ExchangeCode(context.Background(), srv.Client(), cfg, "thecode", "theverifier")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "sk-ant-oat01-xyz" {
		t.Fatalf("token %q", tok.AccessToken)
	}
	if gotBody["code"] != "thecode" || gotBody["code_verifier"] != "theverifier" {
		t.Fatalf("body %+v", gotBody)
	}
	if gotBody["grant_type"] != "authorization_code" {
		t.Fatalf("grant_type %q", gotBody["grant_type"])
	}
}

func TestExchangeCodeError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
	}))
	defer srv.Close()
	cfg := DefaultOAuthConfig()
	cfg.TokenURL = srv.URL
	if _, err := ExchangeCode(context.Background(), srv.Client(), cfg, "c", "v"); err == nil {
		t.Fatal("expected error")
	}
}

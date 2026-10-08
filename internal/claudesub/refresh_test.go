package claudesub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"karina/internal/credentials"
)

type memBackend map[string]string

func (m memBackend) Set(service, account, secret string) error {
	m[service+"/"+account] = secret
	return nil
}

func (m memBackend) Get(service, account string) (string, error) {
	s, ok := m[service+"/"+account]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return s, nil
}

func (m memBackend) Delete(service, account string) error {
	delete(m, service+"/"+account)
	return nil
}

// ownTokenServer serves the token and usage endpoints. The usage endpoint
// only accepts the access token named in validAccess.
func ownTokenServer(t *testing.T, validAccess string, refreshes *int) (*httptest.Server, *Reader) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["grant_type"] != "refresh_token" || body["refresh_token"] != "r1" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			*refreshes++
			fmt.Fprintf(w, `{"access_token":%q,"refresh_token":"r2","expires_in":3600}`, validAccess)
		default:
			if r.Header.Get("Authorization") != "Bearer "+validAccess {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			fmt.Fprint(w, `{"five_hour":{"utilization":25.0}}`)
		}
	}))
	t.Cleanup(srv.Close)

	old := usageURL
	usageURL = srv.URL + "/usage"
	t.Cleanup(func() { usageURL = old })
	t.Setenv("KARINA_CLAUDE_TOKEN", "")

	cfg := DefaultOAuthConfig()
	cfg.TokenURL = srv.URL + "/token"
	return srv, &Reader{HTTP: srv.Client(), OverrideHome: t.TempDir(), OAuth: &cfg}
}

func useMemKeyring(t *testing.T, tok Token) memBackend {
	t.Helper()
	mem := memBackend{}
	credentials.SetBackend(mem)
	if err := SaveOwnToken(tok); err != nil {
		t.Fatal(err)
	}
	return mem
}

func TestReadRenewsExpiredOwnToken(t *testing.T) {
	refreshes := 0
	_, r := ownTokenServer(t, "sk-ant-oat01-new", &refreshes)
	useMemKeyring(t, Token{
		AccessToken:  "sk-ant-oat01-old",
		RefreshToken: "r1",
		ExpiresAt:    time.Now().Add(-time.Hour).Unix(),
	})

	res := r.Read(context.Background())
	if !res.Found {
		t.Fatalf("found=false error=%s", res.Error)
	}
	if refreshes != 1 {
		t.Fatalf("refreshes=%d", refreshes)
	}
	saved, ok := LoadOwnToken()
	if !ok || saved.AccessToken != "sk-ant-oat01-new" || saved.RefreshToken != "r2" || saved.ExpiresAt == 0 {
		t.Fatalf("rotated token not stored: %+v ok=%v", saved.ExpiresAt, ok)
	}
}

func TestReadRenewsOwnTokenOn401(t *testing.T) {
	refreshes := 0
	_, r := ownTokenServer(t, "sk-ant-oat01-new", &refreshes)
	// No expiry recorded (token stored by an older Karina).
	useMemKeyring(t, Token{AccessToken: "sk-ant-oat01-old", RefreshToken: "r1"})

	res := r.Read(context.Background())
	if !res.Found || refreshes != 1 {
		t.Fatalf("found=%v refreshes=%d error=%s", res.Found, refreshes, res.Error)
	}
}

func TestReadDoesNotRenewValidOwnToken(t *testing.T) {
	refreshes := 0
	_, r := ownTokenServer(t, "sk-ant-oat01-ok", &refreshes)
	useMemKeyring(t, Token{
		AccessToken:  "sk-ant-oat01-ok",
		RefreshToken: "r1",
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
	})

	res := r.Read(context.Background())
	if !res.Found || refreshes != 0 {
		t.Fatalf("found=%v refreshes=%d error=%s", res.Found, refreshes, res.Error)
	}
}

func TestOwnTokenWinsOverClaudeCodeFile(t *testing.T) {
	refreshes := 0
	_, r := ownTokenServer(t, "sk-ant-oat01-own", &refreshes)
	useMemKeyring(t, Token{AccessToken: "sk-ant-oat01-own", RefreshToken: "r1"})
	writeCredentials(t, r.OverrideHome, `{"claudeAiOauth":{"accessToken":"sk-ant-oat01-claudecodeclaudecode"}}`)

	tok, _, _ := r.DiscoverToken()
	if tok != "sk-ant-oat01-own" {
		t.Fatalf("token %q", tok)
	}
}

func TestReadSessionExpired(t *testing.T) {
	refreshes := 0
	_, r := ownTokenServer(t, "sk-ant-oat01-new", &refreshes)
	useMemKeyring(t, Token{
		AccessToken:  "sk-ant-oat01-old",
		RefreshToken: "revoked",
		ExpiresAt:    time.Now().Add(-time.Hour).Unix(),
	})

	res := r.Read(context.Background())
	if res.Found || res.Error != ErrSessionExpired.Error() {
		t.Fatalf("found=%v error=%q", res.Found, res.Error)
	}
}

func TestReadRateLimitedReportsRetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1800")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	old := usageURL
	usageURL = srv.URL
	defer func() { usageURL = old }()

	res := (&Reader{OverrideToken: "sk-ant-oat01-x"}).Read(context.Background())
	if res.Found || res.HTTPStatus != http.StatusTooManyRequests || res.RetryAfter != 30*time.Minute {
		t.Fatalf("found=%v status=%d retry=%s", res.Found, res.HTTPStatus, res.RetryAfter)
	}
}

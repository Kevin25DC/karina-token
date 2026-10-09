package keyonly

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"karina/internal/domain"
	"karina/internal/providers/api"
)

func refresh(t *testing.T, p *Provider, status int, body string) domain.ProviderState {
	t.Helper()
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	defer srv.Close()
	st, err := p.Refresh(context.Background(), api.Config{APIKey: "key-123", BaseURL: srv.URL, HTTP: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != p.path || gotAuth != "Bearer key-123" {
		t.Fatalf("request = %s %q", gotPath, gotAuth)
	}
	return st
}

func TestValidKeyConnectsWithoutInventingUsage(t *testing.T) {
	for _, p := range []*Provider{NewXAI(), NewMistral(), NewGroq()} {
		st := refresh(t, p, 200, `{"object":"list","data":[]}`)
		if st.Status != domain.StatusConnected || st.Note == "" {
			t.Fatalf("%s: %+v", p.ID(), st)
		}
		if st.UsageAvailable || st.Balance != nil || st.RateLimit.HasData() {
			t.Fatalf("%s must not report usage, balance or rate limits", p.ID())
		}
	}
}

func TestInvalidKey(t *testing.T) {
	for _, p := range []*Provider{NewXAI(), NewMistral(), NewGroq()} {
		if st := refresh(t, p, 401, `{}`); st.Status != domain.StatusInvalidCredentials {
			t.Fatalf("%s 401 -> %v", p.ID(), st.Status)
		}
	}
	// xAI answers 400 for an invalid key on /v1/api-key.
	if st := refresh(t, NewXAI(), 400, `{}`); st.Status != domain.StatusInvalidCredentials {
		t.Fatalf("xai 400 -> %v", st.Status)
	}
	if st := refresh(t, NewGroq(), 400, `{}`); st.Status == domain.StatusInvalidCredentials {
		t.Fatal("a 400 is not an invalid key for Groq")
	}
}

func TestXAIBlockedKeyIsNotConnected(t *testing.T) {
	st := refresh(t, NewXAI(), 200, `{"name":"k","api_key_blocked":false,"api_key_disabled":true,"team_blocked":false}`)
	if st.Status == domain.StatusConnected {
		t.Fatalf("disabled key reported as connected: %+v", st)
	}
	st = refresh(t, NewXAI(), 200, `{"name":"k","api_key_blocked":false,"api_key_disabled":false,"team_blocked":false}`)
	if st.Status != domain.StatusConnected {
		t.Fatalf("healthy key: %+v", st)
	}
}

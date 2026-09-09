package openai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"karina/internal/domain"
	"karina/internal/providers/api"
)

func newTestClient(handler http.Handler) (api.Config, *httptest.Server) {
	srv := httptest.NewServer(handler)
	return api.Config{
		APIKey:  "sk-test-123",
		BaseURL: srv.URL + "/v1",
		HTTP:    srv.Client(),
	}, srv
}

func withModelsHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ratelimit-limit-requests", "10000")
		w.Header().Set("x-ratelimit-remaining-requests", "9990")
		w.Header().Set("x-ratelimit-reset-requests", "3")
		w.Header().Set("x-ratelimit-limit-tokens", "2000000")
		w.Header().Set("x-ratelimit-remaining-tokens", "1500000")
		w.Header().Set("x-ratelimit-reset-tokens", "5")
		h.ServeHTTP(w, r)
	})
}

func TestStandardKeyConnectedWithRateLimits(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test-123" {
			w.WriteHeader(401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"object":"list","data":[{"id":"gpt-4o"}]}`)
	})
	mux.HandleFunc("/v1/organization/usage/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	})
	cfg, srv := newTestClient(withModelsHeaders(mux))
	defer srv.Close()

	p := New()
	st, err := p.Refresh(context.Background(), cfg)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if st.Status != domain.StatusConnected {
		t.Fatalf("status %v", st.Status)
	}
	if st.UsageAvailable {
		t.Fatal("standard key must not report usage as available")
	}
	if st.RateLimit.Requests == nil || st.RateLimit.Tokens == nil {
		t.Fatal("expected rate limit buckets from headers")
	}
	if st.RateLimit.Requests.Remaining != 9990 {
		t.Fatalf("requests remaining %d", st.RateLimit.Requests.Remaining)
	}
	if st.RateLimit.Tokens.Limit != 2000000 {
		t.Fatalf("token limit %d", st.RateLimit.Tokens.Limit)
	}
	if !strings.Contains(st.Note, "Admin API key") {
		t.Fatalf("note should explain admin requirement, got: %s", st.Note)
	}
}

func TestInvalidKey(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		fmt.Fprint(w, `{"error":{"message":"Incorrect API key provided"}}`)
	})
	cfg, srv := newTestClient(mux)
	defer srv.Close()

	st, err := New().Refresh(context.Background(), cfg)
	if err != nil {
		t.Fatalf("should not error on invalid key, got %v", err)
	}
	if st.Status != domain.StatusInvalidCredentials {
		t.Fatalf("status %v", st.Status)
	}
}

func TestAdminKeyReportsUsage(t *testing.T) {
	mux := http.NewServeMux()
	var usageCalls int32
	// Admin keys are rejected by /models with 403 in this simulation.
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		fmt.Fprint(w, `{"error":{"message":"Admin API keys cannot be used for this endpoint"}}`)
	})
	mux.HandleFunc("/v1/organization/usage/completions", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&usageCalls, 1)
		if r.URL.Query().Get("bucket_width") != "1d" {
			t.Errorf("expected bucket_width=1d")
		}
		if r.URL.Query().Get("start_time") == "" {
			t.Errorf("expected start_time")
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"object":"page","data":[
			{"bucket_start":"2026-08-10T00:00:00Z","input_tokens":1000,"input_cached_tokens":200,"output_tokens":300},
			{"bucket_start":"2026-08-11T00:00:00Z","input_tokens":500,"output_tokens":150}
		]}`)
	})
	cfg, srv := newTestClient(mux)
	defer srv.Close()

	p := New()
	st, err := p.Refresh(context.Background(), cfg)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if st.Status != domain.StatusConnected {
		t.Fatalf("status %v", st.Status)
	}
	if !st.UsageAvailable {
		t.Fatal("admin key should report usage")
	}
	if st.UsedTokens != 2150 {
		t.Fatalf("used tokens %d want 2150", st.UsedTokens)
	}
	if atomic.LoadInt32(&usageCalls) != 1 {
		t.Fatalf("usage calls %d", usageCalls)
	}
}

func TestNetworkError(t *testing.T) {
	cfg := api.Config{APIKey: "sk-test", BaseURL: "http://127.0.0.1:1/v1", HTTP: api.DefaultClient()}
	_, err := New().Refresh(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected network error")
	}
}

func TestValidationOnlySkipsUsage(t *testing.T) {
	var usageCalls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"object":"list","data":[]}`)
	})
	mux.HandleFunc("/v1/organization/usage/", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&usageCalls, 1)
		w.WriteHeader(403)
	})
	cfg, srv := newTestClient(mux)
	defer srv.Close()

	cfg.ValidationOnly = true
	st, err := New().Refresh(context.Background(), cfg)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if atomic.LoadInt32(&usageCalls) != 0 {
		t.Fatal("validation-only must not hit usage endpoint")
	}
	_ = st
}

func TestUsageEndpointMalformedJSONIsTolerated(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"object":"list","data":[]}`)
	})
	mux.HandleFunc("/v1/organization/usage/completions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{broken`)
	})
	cfg, srv := newTestClient(mux)
	defer srv.Close()

	p := New()
	// force immediate probe
	p.usageAttempted = false
	st, err := p.Refresh(context.Background(), cfg)
	if err != nil {
		t.Fatalf("refresh must not fail on malformed usage body: %v", err)
	}
	if st.UsageAvailable {
		t.Fatal("malformed usage must not be reported as available")
	}
	if st.Status != domain.StatusConnected {
		t.Fatalf("status %v", st.Status)
	}
	_ = time.Now()
}

package anthropic

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"karina/internal/domain"
	"karina/internal/providers/api"
)

func withRateHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("anthropic-ratelimit-requests-limit", "50")
		w.Header().Set("anthropic-ratelimit-requests-remaining", "40")
		w.Header().Set("anthropic-ratelimit-requests-reset", "2026-09-09T12:00:00Z")
		w.Header().Set("anthropic-ratelimit-input-tokens-limit", "8000")
		w.Header().Set("anthropic-ratelimit-input-tokens-remaining", "6000")
		w.Header().Set("anthropic-ratelimit-input-tokens-reset", "2026-09-09T12:00:00Z")
		h.ServeHTTP(w, r)
	})
}

func TestStandardKeyConnected(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "sk-ant-test" {
			w.WriteHeader(401)
			return
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("missing anthropic-version header")
		}
		fmt.Fprint(w, `{"data":[{"id":"claude-3-5-sonnet-latest","type":"model"}],"has_more":false}`)
	})
	cfg := api.Config{APIKey: "sk-ant-test", BaseURL: "PLACEHOLDER", HTTP: nil}
	srv := httptest.NewServer(withRateHeaders(mux))
	defer srv.Close()
	cfg.BaseURL = srv.URL + "/v1"
	cfg.HTTP = srv.Client()

	st, err := New().Refresh(context.Background(), cfg)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if st.Status != domain.StatusConnected {
		t.Fatalf("status %v", st.Status)
	}
	if st.UsageAvailable {
		t.Fatal("standard key must not report org usage")
	}
	if st.RateLimit.Requests == nil || st.RateLimit.InputTokens == nil {
		t.Fatal("expected rate limit buckets")
	}
	if st.RateLimit.Requests.Remaining != 40 {
		t.Fatalf("remaining %d", st.RateLimit.Requests.Remaining)
	}
	if st.RateLimit.InputTokens.Limit != 8000 {
		t.Fatalf("input token limit %d", st.RateLimit.InputTokens.Limit)
	}
}

func TestInvalidKey(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		fmt.Fprint(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cfg := api.Config{APIKey: "sk-ant-bad", BaseURL: srv.URL + "/v1", HTTP: srv.Client()}

	st, err := New().Refresh(context.Background(), cfg)
	if err != nil {
		t.Fatalf("should not error on invalid key, got %v", err)
	}
	if st.Status != domain.StatusInvalidCredentials {
		t.Fatalf("status %v", st.Status)
	}
}

func TestAdminKeyUsageReport(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[]}`)
	})
	mux.HandleFunc("/v1/organizations/usage_report/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("start_time") == "" || r.URL.Query().Get("end_time") == "" {
			t.Error("missing time range")
		}
		fmt.Fprint(w, `{"data":[
			{"start_time":"2026-09-01T00:00:00Z","uncached_input_tokens":1000,"cache_read_input_tokens":200,"output_tokens":300,"cache_creation":{"ephemeral_5m_input_tokens":50,"ephemeral_1h_input_tokens":0}},
			{"start_time":"2026-09-02T00:00:00Z","uncached_input_tokens":100,"output_tokens":50}
		]}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cfg := api.Config{APIKey: "sk-ant-admin-xyz", BaseURL: srv.URL + "/v1", HTTP: srv.Client()}

	st, err := New().Refresh(context.Background(), cfg)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if !st.UsageAvailable {
		t.Fatal("admin key should report usage")
	}
	want := int64(1000 + 200 + 300 + 50 + 100 + 50)
	if st.UsedTokens != want {
		t.Fatalf("used %d want %d", st.UsedTokens, want)
	}
}

func TestAdminUsageReportEndpointErrorTolerated(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[]}`)
	})
	mux.HandleFunc("/v1/organizations/usage_report/messages", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		fmt.Fprint(w, `{"type":"error","error":{"type":"invalid_request_error","message":"bad params"}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cfg := api.Config{APIKey: "sk-ant-admin-xyz", BaseURL: srv.URL + "/v1", HTTP: srv.Client()}

	st, err := New().Refresh(context.Background(), cfg)
	if err != nil {
		t.Fatalf("refresh must tolerate report failure: %v", err)
	}
	if st.UsageAvailable {
		t.Fatal("usage must not be marked available on report failure")
	}
	if st.Status != domain.StatusConnected {
		t.Fatalf("status %v", st.Status)
	}
}

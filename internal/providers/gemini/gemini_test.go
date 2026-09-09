package gemini

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"karina/internal/domain"
	"karina/internal/providers/api"
)

func TestValidKeyConnected(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1beta/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "AIza-test" {
			w.WriteHeader(400)
			fmt.Fprint(w, `{"error":{"code":400,"message":"API key not valid. Please pass a valid API key.","status":"INVALID_ARGUMENT"}}`)
			return
		}
		fmt.Fprint(w, `{"models":[{"name":"models/gemini-2.0-flash"}]}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := api.Config{APIKey: "AIza-test", BaseURL: srv.URL + "/v1beta", HTTP: srv.Client()}
	st, err := New().Refresh(context.Background(), cfg)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if st.Status != domain.StatusConnected {
		t.Fatalf("status %v", st.Status)
	}
	if st.UsageAvailable {
		t.Fatal("gemini must not report usage availability")
	}
	if st.RateLimit.HasData() {
		t.Fatal("gemini has no rate limit data")
	}
	if len(st.Capabilities) != 0 {
		t.Fatalf("gemini capabilities should be empty, got %v", st.Capabilities)
	}
	if st.Note == "" {
		t.Fatal("expected explanatory note")
	}
}

func TestInvalidKey(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1beta/models", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		fmt.Fprint(w, `{"error":{"code":400,"message":"API key not valid. Please pass a valid API key.","status":"INVALID_ARGUMENT"}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := api.Config{APIKey: "AIza-bad", BaseURL: srv.URL + "/v1beta", HTTP: srv.Client()}
	st, err := New().Refresh(context.Background(), cfg)
	if err != nil {
		t.Fatalf("should not error, got %v", err)
	}
	if st.Status != domain.StatusInvalidCredentials {
		t.Fatalf("status %v", st.Status)
	}
}

package deepseek

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"karina/internal/domain"
	"karina/internal/providers/api"
)

func TestConnectedWithBalance(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-ds-test" {
			w.WriteHeader(401)
			fmt.Fprint(w, `{"error":"Authentication Fails"}`)
			return
		}
		fmt.Fprint(w, `{"object":"list","data":[{"id":"deepseek-chat"}]}`)
	})
	mux.HandleFunc("/user/balance", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"is_available":true,"balance_infos":[{"currency":"USD","total_balance":"42.50","granted_balance":"2.50","topped_up_balance":"40.00"}]}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := api.Config{APIKey: "sk-ds-test", BaseURL: srv.URL, HTTP: srv.Client()}
	st, err := New().Refresh(context.Background(), cfg)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if st.Status != domain.StatusConnected {
		t.Fatalf("status %v", st.Status)
	}
	if st.Balance == nil {
		t.Fatal("expected balance")
	}
	if st.Balance.Currency != "USD" || st.Balance.Total != 42.50 {
		t.Fatalf("balance %+v", st.Balance)
	}
	if !st.Balance.IsAvailable {
		t.Fatal("balance should be available")
	}
	if st.UsageAvailable {
		t.Fatal("deepseek must not report token usage")
	}
}

func TestBalanceEndpointDownStillConnected(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/models", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"object":"list","data":[]}`)
	})
	mux.HandleFunc("/user/balance", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := api.Config{APIKey: "sk-ds-test", BaseURL: srv.URL, HTTP: srv.Client()}
	st, err := New().Refresh(context.Background(), cfg)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if st.Status != domain.StatusConnected {
		t.Fatalf("status %v want connected", st.Status)
	}
}

func TestInvalidKey(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/models", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		fmt.Fprint(w, `{"error":"Authentication Fails"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := api.Config{APIKey: "sk-ds-bad", BaseURL: srv.URL, HTTP: srv.Client()}
	st, err := New().Refresh(context.Background(), cfg)
	if err != nil {
		t.Fatalf("should not error, got %v", err)
	}
	if st.Status != domain.StatusInvalidCredentials {
		t.Fatalf("status %v", st.Status)
	}
}

func TestValidationOnlySkipsBalance(t *testing.T) {
	var balanceCalls int
	mux := http.NewServeMux()
	mux.HandleFunc("/models", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"object":"list","data":[]}`)
	})
	mux.HandleFunc("/user/balance", func(w http.ResponseWriter, r *http.Request) {
		balanceCalls++
		fmt.Fprint(w, `{"is_available":true,"balance_infos":[]}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := api.Config{APIKey: "sk-ds-test", BaseURL: srv.URL, HTTP: srv.Client(), ValidationOnly: true}
	_, err := New().Refresh(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if balanceCalls != 0 {
		t.Fatalf("validation-only should skip balance, calls=%d", balanceCalls)
	}
}

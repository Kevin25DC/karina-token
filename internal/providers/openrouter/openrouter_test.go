package openrouter

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"karina/internal/domain"
	"karina/internal/providers/api"
)

func serve(t *testing.T, keyBody string, creditsStatus int) api.Config {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/key", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-or-test" {
			w.WriteHeader(401)
			return
		}
		fmt.Fprint(w, keyBody)
	})
	mux.HandleFunc("/credits", func(w http.ResponseWriter, r *http.Request) {
		if creditsStatus != 200 {
			w.WriteHeader(creditsStatus)
			fmt.Fprint(w, `{"error":{"code":403,"message":"Only management keys can perform this operation"}}`)
			return
		}
		fmt.Fprint(w, `{"data":{"total_credits":100.5,"total_usage":25.75}}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return api.Config{APIKey: "sk-or-test", BaseURL: srv.URL, HTTP: srv.Client()}
}

func TestKeyWithLimitShowsRemaining(t *testing.T) {
	cfg := serve(t, `{"data":{"label":"k","limit":50,"limit_remaining":12.5,"usage":37.5,"usage_daily":1.25,"usage_monthly":20}}`, 403)
	st, err := New().Refresh(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != domain.StatusConnected || st.Balance == nil || st.Balance.Total != 12.5 || st.Balance.Currency != "USD" {
		t.Fatalf("state = %+v balance = %+v", st.Status, st.Balance)
	}
	if !strings.Contains(st.Note, "$20.00 este mes") || st.UsageAvailable {
		t.Fatalf("note = %q", st.Note)
	}
}

func TestUnlimitedKeyWithoutManagementAccess(t *testing.T) {
	cfg := serve(t, `{"data":{"label":"k","limit":null,"limit_remaining":null,"usage":3,"usage_daily":0,"usage_monthly":3}}`, 403)
	st, err := New().Refresh(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != domain.StatusConnected || st.Balance != nil {
		t.Fatalf("a normal key must not show an account balance: %+v", st.Balance)
	}
	if !strings.Contains(st.Note, "Management key") {
		t.Fatalf("note = %q", st.Note)
	}
}

func TestManagementKeyShowsAccountCredits(t *testing.T) {
	cfg := serve(t, `{"data":{"label":"k","limit":null,"limit_remaining":null,"usage":3,"usage_daily":0,"usage_monthly":3}}`, 200)
	st, err := New().Refresh(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if st.Balance == nil || st.Balance.Total != 74.75 || st.Balance.ToppedUp != 100.5 {
		t.Fatalf("balance = %+v", st.Balance)
	}
}

func TestInvalidKey(t *testing.T) {
	cfg := serve(t, `{}`, 403)
	cfg.APIKey = "wrong"
	st, err := New().Refresh(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != domain.StatusInvalidCredentials {
		t.Fatalf("status = %v", st.Status)
	}
}

package claudesub

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func writeCredentials(t *testing.T, dir, content string) {
	t.Helper()
	p := filepath.Join(dir, ".claude", ".credentials.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverTokenFromClaudeCodeFile(t *testing.T) {
	dir := t.TempDir()
	writeCredentials(t, dir, `{"primaryAccount":{"oauthAccount":{"token":"3-abc123verylongtokentoken"}}}`)
	r := &Reader{OverrideHome: dir, SkipKeyring: true}
	tok, src, err := r.discoverToken()
	if err != nil || tok == "" {
		t.Fatalf("token=%q src=%q err=%v", tok, src, err)
	}
	if tok != "3-abc123verylongtokentoken" {
		t.Fatalf("unexpected token %q", tok)
	}
}

func TestDiscoverTokenMissing(t *testing.T) {
	r := &Reader{OverrideHome: t.TempDir(), SkipKeyring: true}
	tok, _, _ := r.discoverToken()
	if tok != "" {
		t.Fatalf("expected no token, got %q", tok)
	}
}

func TestReadUsageLegacyWindows(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("anthropic-beta") != "oauth-2025-04-20" {
			t.Errorf("missing anthropic-beta header")
		}
		fmt.Fprint(w, `{"five_hour":{"utilization":25.0,"resets_at":"2026-09-09T20:00:00Z"},"seven_day":{"utilization":45.0,"resets_at":"2026-09-13T00:00:00Z"},"seven_day_opus":null}`)
	}))
	defer srv.Close()
	old := usageURL
	usageURL = srv.URL
	defer func() { usageURL = old }()

	r := &Reader{OverrideToken: "sk-ant-oat01-x"}
	res := r.Read(context.Background())
	if !res.Found {
		t.Fatalf("found=false error=%s", res.Error)
	}
	if res.Used != 45 || res.Limit != 100 {
		t.Fatalf("used/limit = %d/%d", res.Used, res.Limit)
	}
	if res.Window != "Semanal (7 días)" {
		t.Fatalf("window %q", res.Window)
	}
	if res.ResetAt == "" {
		t.Fatal("expected reset_at")
	}
}

func TestReadUsageNewLimitsFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"five_hour":null,"seven_day":null,"limits":[
			{"kind":"session","group":"session","percent":25.0},
			{"kind":"weekly_all","group":"weekly","percent":48.5},
			{"kind":"weekly_scoped","group":"weekly","percent":12.0,"scope":{"model":{"display_name":"Fable"}}}
		]}`)
	}))
	defer srv.Close()
	old := usageURL
	usageURL = srv.URL
	defer func() { usageURL = old }()

	res := (&Reader{OverrideToken: "sk-ant-oat01-x"}).Read(context.Background())
	if !res.Found {
		t.Fatalf("found=false error=%s", res.Error)
	}
	if res.Percent != 48.5 {
		t.Fatalf("percent %v", res.Percent)
	}
	if res.Window != "Semanal (7 días)" {
		t.Fatalf("window %q", res.Window)
	}
}

func TestReadUsageExtraOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"extra_usage":{"is_enabled":true,"utilization":18.68}}`)
	}))
	defer srv.Close()
	old := usageURL
	usageURL = srv.URL
	defer func() { usageURL = old }()

	res := (&Reader{OverrideToken: "sk-ant-oat01-x"}).Read(context.Background())
	if !res.Found || res.Used != 19 {
		t.Fatalf("res %+v", res)
	}
}

func TestReadUsageUnknownFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"totally":"different","shape":true}`)
	}))
	defer srv.Close()
	old := usageURL
	usageURL = srv.URL
	defer func() { usageURL = old }()

	res := (&Reader{OverrideToken: "sk-ant-oat01-x"}).Read(context.Background())
	if res.Found {
		t.Fatal("should be found=false for unknown shape")
	}
	if res.Error == "" {
		t.Fatal("expected explanatory error")
	}
}

func TestReadHTTPErrorTolerated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	old := usageURL
	usageURL = srv.URL
	defer func() { usageURL = old }()

	r := &Reader{OverrideToken: "3-x"}
	res := r.Read(context.Background())
	if res.Found {
		t.Fatal("should be found=false")
	}
	if res.Error == "" {
		t.Fatal("expected error message")
	}
}

func TestDiscoverNewClaudeAiOauthFormat(t *testing.T) {
	dir := t.TempDir()
	writeCredentials(t, dir, `{"claudeAiOauth":{"accessToken":"sk-ant-oat01-abcdefghijklmnopqrstuvwxyz","refreshToken":"sk-ant-ort01-zzz"}}`)
	r := &Reader{OverrideHome: dir, SkipKeyring: true}
	tok, src, err := r.discoverToken()
	if err != nil || tok != "sk-ant-oat01-abcdefghijklmnopqrstuvwxyz" {
		t.Fatalf("token=%q src=%q err=%v", tok, src, err)
	}
}

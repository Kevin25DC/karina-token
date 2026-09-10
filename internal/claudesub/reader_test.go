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
	tok, src, err := r.DiscoverToken()
	if err != nil || tok == "" {
		t.Fatalf("token=%q src=%q err=%v", tok, src, err)
	}
	if tok != "3-abc123verylongtokentoken" {
		t.Fatalf("unexpected token %q", tok)
	}
}

func TestDiscoverTokenMissing(t *testing.T) {
	r := &Reader{OverrideHome: t.TempDir(), SkipKeyring: true}
	tok, _, _ := r.DiscoverToken()
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
	// Session window (5h) is the primary figure.
	if res.Used != 25 || res.Limit != 100 {
		t.Fatalf("used/limit = %d/%d", res.Used, res.Limit)
	}
	if res.Window != "Ventana de 5 horas" {
		t.Fatalf("window %q", res.Window)
	}
	if len(res.Windows) != 2 {
		t.Fatalf("expected 2 windows, got %d", len(res.Windows))
	}
	if res.Windows[0].Label != "Ventana de 5 horas" || res.Windows[1].Label != "Semanal (7 días)" {
		t.Fatalf("windows order %+v", res.Windows)
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
	// Session (kind=session) is primary even though weekly is higher.
	if res.Percent != 25.0 {
		t.Fatalf("percent %v", res.Percent)
	}
	if res.Window != "Ventana de 5 horas" {
		t.Fatalf("window %q", res.Window)
	}
	if len(res.Windows) != 3 {
		t.Fatalf("expected 3 windows, got %d", len(res.Windows))
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
	tok, src, err := r.DiscoverToken()
	if err != nil || tok != "sk-ant-oat01-abcdefghijklmnopqrstuvwxyz" {
		t.Fatalf("token=%q src=%q err=%v", tok, src, err)
	}
}

func TestReadUsageDeduplicatesWindows(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"five_hour":{"utilization":13.0,"resets_at":"2026-09-10T06:10:00Z"},
			"seven_day":{"utilization":44.0,"resets_at":"2026-09-12T16:00:00Z"},
			"nimbus_quill":{"utilization":0},
			"limits":[
				{"kind":"session","percent":13.0},
				{"kind":"weekly_all","percent":44.0}
			]
		}`)
	}))
	defer srv.Close()
	old := usageURL
	usageURL = srv.URL
	defer func() { usageURL = old }()

	res := (&Reader{OverrideToken: "sk-ant-oat01-x"}).Read(context.Background())
	if !res.Found {
		t.Fatalf("found=false error=%s", res.Error)
	}
	if len(res.Windows) != 2 {
		t.Fatalf("expected 2 deduped windows, got %d: %+v", len(res.Windows), res.Windows)
	}
	if res.Windows[0].Label != "Ventana de 5 horas" || res.Windows[0].Percent != 13 {
		t.Fatalf("session window wrong: %+v", res.Windows[0])
	}
	if res.Windows[1].Label != "Semanal (7 días)" || res.Windows[1].Percent != 44 {
		t.Fatalf("weekly window wrong: %+v", res.Windows[1])
	}
}

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
	r := &Reader{OverrideHome: dir}
	tok, src, err := r.discoverToken()
	if err != nil || tok == "" {
		t.Fatalf("token=%q src=%q err=%v", tok, src, err)
	}
	if tok != "3-abc123verylongtokentoken" {
		t.Fatalf("unexpected token %q", tok)
	}
}

func TestDiscoverTokenMissing(t *testing.T) {
	r := &Reader{OverrideHome: t.TempDir()}
	tok, _, _ := r.discoverToken()
	if tok != "" {
		t.Fatalf("expected no token, got %q", tok)
	}
}

func TestReadUsagePercentOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer 3-test-token-000" {
			w.WriteHeader(401)
			return
		}
		fmt.Fprint(w, `{"5h":{"used_percent":67}}`)
	}))
	defer srv.Close()
	old := usageURL
	usageURL = srv.URL
	defer func() { usageURL = old }()

	r := &Reader{OverrideToken: "3-test-token-000"}
	res := r.Read(context.Background())
	if !res.Found {
		t.Fatalf("found=false error=%s", res.Error)
	}
	if res.Used != 67 || res.Limit != 100 {
		t.Fatalf("used/limit = %d/%d", res.Used, res.Limit)
	}
	if res.Percent != 67 {
		t.Fatalf("percent %v", res.Percent)
	}
	if res.Window != "5h" {
		t.Fatalf("window %q", res.Window)
	}
}

func TestReadUsageAbsolute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"7d":{"used":42000,"limit":100000}}`)
	}))
	defer srv.Close()
	old := usageURL
	usageURL = srv.URL
	defer func() { usageURL = old }()

	r := &Reader{OverrideToken: "3-x"}
	res := r.Read(context.Background())
	if !res.Found || res.Used != 42000 || res.Limit != 100000 {
		t.Fatalf("res %+v", res)
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

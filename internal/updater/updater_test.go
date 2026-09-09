package updater

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLatestParsesRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"tag_name": "v0.3.0",
			"html_url": "https://github.com/Kevin25DC/karina-token/releases/tag/v0.3.0",
			"body": "notas de la version",
			"assets": [
			  {"name": "Karina-Windows-v0.3.0.zip", "browser_download_url": "https://example.com/k.zip"},
			  {"name": "logo.png", "browser_download_url": "https://example.com/logo.png"}
			]
		}`)
	}))
	defer srv.Close()

	c := &Checker{HTTP: srv.Client(), APIURL: srv.URL}
	rel, err := c.Latest(context.Background())
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	if rel.TagName != "v0.3.0" {
		t.Fatalf("tag %q", rel.TagName)
	}
	if rel.AssetURL != "https://example.com/k.zip" {
		t.Fatalf("asset %q", rel.AssetURL)
	}

	info := Check("0.2.0", rel)
	if !info.HasUpdate {
		t.Fatal("expected update available")
	}
	if info.Latest != "0.3.0" {
		t.Fatalf("latest normalized %q", info.Latest)
	}
}

func TestLatestAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	c := &Checker{HTTP: srv.Client(), APIURL: srv.URL}
	if _, err := c.Latest(context.Background()); err == nil {
		t.Fatal("expected error on http 500")
	}
}

func TestMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{nope`)
	}))
	defer srv.Close()
	c := &Checker{HTTP: srv.Client(), APIURL: srv.URL}
	if _, err := c.Latest(context.Background()); err == nil {
		t.Fatal("expected error on malformed json")
	}
}

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.3.0", "0.2.0", true},
		{"v0.2.1", "0.2.0", true},
		{"0.2.0", "0.2.0", false},
		{"0.2.0", "0.10.0", false},
		{"1.0", "0.9.9", true},
		{"0.2.0", "0.2.0-rc1", false},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Fatalf("Newer(%q,%q)=%v want %v", c.a, c.b, got, c.want)
		}
	}
}

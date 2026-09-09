package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func writeTemp(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "config.toml")
}

func TestDefaults(t *testing.T) {
	cfg := Default()
	if cfg.RefreshInterval() != 30*time.Second {
		t.Fatalf("default interval wrong")
	}
	if cfg.Enabled("openai") {
		t.Fatalf("provider should default to disabled")
	}
}

func TestRoundTrip(t *testing.T) {
	path := writeTemp(t)
	cfg := Default()
	cfg.SetEnabled("openai", true)
	cfg.SetEnabled("gemini", true)
	cfg.RefreshIntervalSeconds = 15
	cfg.StartWithSystem = true
	cfg.OnboardingDone = true

	if err := Save(path, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !got.Enabled("openai") || !got.Enabled("gemini") {
		t.Fatalf("enabled flags lost")
	}
	if got.Enabled("anthropic") {
		t.Fatalf("unexpected enabled")
	}
	if got.RefreshIntervalSeconds != 15 || !got.StartWithSystem || !got.OnboardingDone {
		t.Fatalf("fields lost: %+v", got)
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}
	if cfg.RefreshInterval() != 30*time.Second {
		t.Fatalf("expected defaults")
	}
}

func TestLoadCorruptReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.toml")
	if err := os.WriteFile(path, []byte("not toml {{{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatalf("expected error for corrupt config")
	}
}

func TestSaveDoesNotLeakMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows does not honor posix permission bits")
	}
	path := writeTemp(t)
	cfg := Default()
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Config must not be world readable.
	if fi.Mode().Perm()&0o044 != 0 {
		t.Fatalf("config is group/other readable: %v", fi.Mode().Perm())
	}
}

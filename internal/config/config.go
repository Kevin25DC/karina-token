// Package config loads and saves the local Karina configuration.
//
// Security contract: this file NEVER contains API keys. Keys are stored in
// the OS credential store (see internal/credentials).
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const (
	defaultRefreshSeconds  = 30
	defaultSnapshotSeconds = 60
	defaultAlertThreshold  = 85
)

// ProviderEntry holds per-provider local settings.
type ProviderEntry struct {
	Enabled bool `toml:"enabled"`
}

// Config is the local application configuration.
type Config struct {
	RefreshIntervalSeconds int                      `toml:"refresh_interval_seconds"`
	SnapshotEverySeconds   int                      `toml:"snapshot_every_seconds"`
	StartWithSystem        bool                     `toml:"start_with_system"`
	OnboardingDone         bool                     `toml:"onboarding_done"`
	AlertsEnabled          bool                     `toml:"alerts_enabled"`
	AlertThresholdPercent  int                      `toml:"alert_threshold_percent"`
	Providers              map[string]ProviderEntry `toml:"providers"`
}

// Default returns the default configuration.
func Default() *Config {
	return &Config{
		RefreshIntervalSeconds: defaultRefreshSeconds,
		SnapshotEverySeconds:   defaultSnapshotSeconds,
		AlertsEnabled:          true,
		AlertThresholdPercent:  defaultAlertThreshold,
		Providers:              map[string]ProviderEntry{},
	}
}

// Threshold returns the configured alert threshold, clamped to a sane range.
func (c *Config) Threshold() int {
	t := c.AlertThresholdPercent
	if t <= 0 {
		t = defaultAlertThreshold
	}
	if t > 100 {
		t = 100
	}
	return t
}

// RefreshInterval returns the configured polling interval.
func (c *Config) RefreshInterval() time.Duration {
	s := c.RefreshIntervalSeconds
	if s <= 0 {
		s = defaultRefreshSeconds
	}
	return time.Duration(s) * time.Second
}

// SnapshotInterval returns how often a value change is persisted to history.
func (c *Config) SnapshotInterval() time.Duration {
	s := c.SnapshotEverySeconds
	if s <= 0 {
		s = defaultSnapshotSeconds
	}
	return time.Duration(s) * time.Second
}

// Enabled reports whether a provider is enabled in the configuration.
func (c *Config) Enabled(id string) bool {
	e, ok := c.Providers[id]
	return ok && e.Enabled
}

// SetEnabled toggles a provider.
func (c *Config) SetEnabled(id string, enabled bool) {
	e := c.Providers[id]
	e.Enabled = enabled
	c.Providers[id] = e
}

// BaseDir returns the directory where Karina keeps config and data.
func BaseDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}
	dir := filepath.Join(base, "Karina")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	return dir, nil
}

// Path returns the full path of the configuration file.
func Path() (string, error) {
	dir, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// Load reads the config from disk, returning defaults when the file does not
// exist yet. A corrupt file returns an error (it is never silently wiped).
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := Default()
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if cfg.Providers == nil {
		cfg.Providers = map[string]ProviderEntry{}
	}
	return cfg, nil
}

// Save atomically persists the config.
func Save(path string, cfg *Config) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

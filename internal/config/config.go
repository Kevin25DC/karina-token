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
	defaultIdleGapMinutes  = 10
	// 5 minutes is what usage monitors commonly use for this endpoint.
	defaultSubscriptionSeconds = 300
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
	// WidgetMoved is true once the user has dragged the widget; WidgetCenterX
	// and WidgetTopY are then the top-centre point where they left it.
	WidgetMoved   bool `toml:"widget_moved"`
	WidgetCenterX int  `toml:"widget_center_x"`
	WidgetTopY    int  `toml:"widget_top_y"`
	// SubscriptionIntervalSeconds is how often the Claude subscription usage
	// is read (0 = default). Its endpoint rate-limits frequent polling, so it
	// has its own, slower pace than RefreshIntervalSeconds.
	SubscriptionIntervalSeconds int `toml:"subscription_interval_seconds"`
	// IdleGapMinutes is the pause that ends a stretch of work when measuring
	// time worked on Claude Code projects (0 = default).
	IdleGapMinutes int `toml:"idle_gap_minutes"`
	// ReportBusinessName is printed at the top of client reports.
	ReportBusinessName string `toml:"report_business_name"`
	// ClientBudgets maps a client to its monthly budget in USD. BudgetAlerts
	// remembers the highest alert level already sent, keyed "YYYY-MM|client".
	ClientBudgets map[string]float64 `toml:"client_budgets"`
	// ClientRates maps a client to what the user charges per hour, in USD.
	ClientRates  map[string]float64 `toml:"client_rates"`
	BudgetAlerts map[string]int     `toml:"budget_alerts"`
	// SubscriptionMonthlyUSD is what the user pays per month for their Claude
	// plan; 0 means not set. Used only to compare against API-equivalent cost.
	SubscriptionMonthlyUSD float64 `toml:"subscription_monthly_usd"`
	// ProjectClients maps a Claude Code project path to the client/label the
	// user assigned to it.
	ProjectClients map[string]string `toml:"project_clients"`
	// FolderClients maps a parent folder to the client of every project
	// under it. An entry in ProjectClients overrides it for that project.
	FolderClients map[string]string `toml:"folder_clients"`
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

// MinSubscriptionSeconds is the fastest the Claude subscription may be read.
const MinSubscriptionSeconds = 120

// SubscriptionInterval returns how often the Claude subscription is read.
func (c *Config) SubscriptionInterval() time.Duration {
	s := c.SubscriptionIntervalSeconds
	if s <= 0 {
		s = defaultSubscriptionSeconds
	}
	if s < MinSubscriptionSeconds {
		s = MinSubscriptionSeconds
	}
	return time.Duration(s) * time.Second
}

// IdleGap returns, in minutes, the pause that ends a stretch of work.
func (c *Config) IdleGap() int {
	if c.IdleGapMinutes <= 0 {
		return defaultIdleGapMinutes
	}
	return c.IdleGapMinutes
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

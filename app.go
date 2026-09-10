package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"karina/internal/claudesub"
	"karina/internal/core"
	"karina/internal/credentials"
	"karina/internal/domain"
	"karina/internal/platform"
	"karina/internal/updater"
)

const claudeOAuthAccount = "claude_subscription_oauth"

// Widget (ventana compacta) geometry.
const (
	widgetWidth  = 336
	widgetMargin = 20
	widgetRow    = 108
	widgetHeader = 150
	widgetMaxH   = 680
	normalWidth  = 1180
	normalHeight = 780
)

// App is the Wails-bound application object. Every exported method is a
// small, typed bridge to the core service; the frontend never talks to the
// providers directly.
type App struct {
	ctx         context.Context
	svc         *core.Service
	log         *slog.Logger
	unsubscribe func()
	icon        []byte
	trayOK      bool
	widgetMode  bool

	oauthMu    sync.Mutex
	oauthPKCE  claudesub.PKCE
	oauthReady bool
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.unsubscribe = a.svc.Subscribe(func(e core.Event) {
		if a.ctx == nil {
			return
		}
		// Events are forwarded to the webview under their kind name.
		runtime.EventsEmit(a.ctx, e.Kind, e)
		// Keep the tray tooltip useful.
		if e.Kind == core.EventCycleEnd {
			a.updateTraySummary(e.AllStates)
		}
	})
	a.log.Info("application started", "version", appVersion)
	if err := a.svc.Start(ctx); err != nil {
		a.log.Error("failed to start polling", "error", err.Error())
	}
	// Resident system tray (Windows).
	if platform.TrayAvailable {
		if err := platform.StartTray(a.icon, platform.TrayActions{
			OnShow:    a.ShowMain,
			OnHide:    func() { runtime.WindowHide(a.ctx) },
			OnRefresh: func() { a.svc.RefreshNow() },
			OnWidget:  func() { _ = a.EnterWidgetMode() },
			OnQuit:    func() { runtime.Quit(a.ctx) },
		}); err != nil {
			a.log.Warn("system tray unavailable", "error", err.Error())
		} else {
			a.trayOK = true
			a.log.Info("system tray ready")
		}
	}
}

func (a *App) shutdown(_ context.Context) {
	if a.unsubscribe != nil {
		a.unsubscribe()
	}
	if a.trayOK {
		platform.StopTray()
	}
	a.svc.Close()
}

// updateTraySummary shows how many providers are connected over the icon.
func (a *App) updateTraySummary(all []domain.ProviderState) {
	connected := 0
	for _, st := range all {
		if st.Status == domain.StatusConnected {
			connected++
		}
	}
	if connected == 0 {
		platform.SetTrayTooltip("Karina · sin proveedores conectados")
		return
	}
	platform.SetTrayTooltip(fmt.Sprintf("Karina · %d proveedor(es) conectado(s)", connected))
}

// AppInfo is shown in the Settings/About area.
type AppInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// GetAppInfo returns basic application information.
func (a *App) GetAppInfo() AppInfo {
	return AppInfo{Name: appName, Version: appVersion}
}

// CheckForUpdate asks GitHub Releases whether a newer version exists.
func (a *App) CheckForUpdate() updater.Info {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	rel, err := updater.NewChecker().Latest(ctx)
	if err != nil {
		a.log.Info("update check failed", "error", err.Error())
		return updater.Info{Current: appVersion, Error: err.Error()}
	}
	info := updater.Check(appVersion, rel)
	a.log.Info("update check done", "latest", info.Latest, "has_update", info.HasUpdate)
	return info
}

// EnterWidgetMode shrinks the window into a compact, always-on-top widget
// pinned to the bottom-right corner, showing the live usage bars.
func (a *App) EnterWidgetMode() error {
	if a.ctx == nil {
		return errors.New("aplicación no iniciada")
	}
	n := 0
	for _, m := range a.svc.ListProviders() {
		if m.Enabled {
			n++
		}
	}
	if n < 1 {
		n = 1
	}
	h := widgetHeader + n*widgetRow
	if h > widgetMaxH {
		h = widgetMaxH
	}

	// A maximised window cannot be shrunk; unmaximise first.
	runtime.WindowUnmaximise(a.ctx)
	runtime.WindowSetAlwaysOnTop(a.ctx, true)
	runtime.WindowSetMinSize(a.ctx, 300, 160)
	runtime.WindowSetSize(a.ctx, widgetWidth, h)

	if screens, err := runtime.ScreenGetAll(a.ctx); err == nil && len(screens) > 0 {
		sc := screens[0]
		for _, s := range screens {
			if s.IsPrimary {
				sc = s
				break
			}
		}
		sw, sh := sc.Size.Width, sc.Size.Height
		if sw <= 0 {
			sw = sc.Width
		}
		if sh <= 0 {
			sh = sc.Height
		}
		x := sw - widgetWidth - widgetMargin
		y := sh - h - widgetMargin
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		runtime.WindowSetPosition(a.ctx, x, y)
	}

	a.widgetMode = true
	runtime.EventsEmit(a.ctx, "mode:widget", true)
	a.log.Info("widget mode enabled", "height", h)
	return nil
}

// ExitWidgetMode restores the normal dashboard window.
func (a *App) ExitWidgetMode() error {
	if a.ctx == nil {
		return errors.New("aplicación no iniciada")
	}
	a.widgetMode = false
	runtime.WindowSetAlwaysOnTop(a.ctx, false)
	runtime.WindowSetMinSize(a.ctx, 860, 600)
	runtime.WindowSetSize(a.ctx, normalWidth, normalHeight)
	runtime.WindowCenter(a.ctx)
	runtime.EventsEmit(a.ctx, "mode:widget", false)
	a.log.Info("widget mode disabled")
	return nil
}

// ShowMain shows the window. If it is in widget mode it first restores the
// normal dashboard so the user is never stranded in a broken widget.
func (a *App) ShowMain() {
	if a.ctx == nil {
		return
	}
	if a.widgetMode {
		_ = a.ExitWidgetMode()
	}
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

// Quit closes the application.
func (a *App) Quit() {
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}

// RefreshNow triggers an immediate polling cycle.
func (a *App) RefreshNow() {
	a.svc.RefreshNow()
}

// ListProviders returns the provider catalog with runtime state.
func (a *App) ListProviders() []domain.ProviderMeta {
	return a.svc.ListProviders()
}

// States returns the latest snapshots for all known providers.
func (a *App) States() []domain.ProviderState {
	return a.svc.States()
}

// Config returns a key-free view of the current configuration.
func (a *App) Config() core.ConfigSnapshot {
	return a.svc.Config()
}

// KeyPreview returns a redacted preview of the stored key for a provider.
func (a *App) KeyPreview(provider string) string {
	return a.svc.KeyPreview(domain.ProviderID(provider))
}

// TestProvider validates credentials without persisting them.
func (a *App) TestProvider(provider string, key string) core.TestResult {
	return a.svc.TestProvider(domain.ProviderID(provider), key)
}

// SaveProviderKey stores validated credentials and enables the provider.
func (a *App) SaveProviderKey(provider string, key string) error {
	return a.svc.SaveProviderKey(domain.ProviderID(provider), key)
}

// RemoveProvider disconnects a provider and clears its data.
func (a *App) RemoveProvider(provider string) error {
	return a.svc.RemoveProvider(domain.ProviderID(provider))
}

// SetProviderEnabled enables or disables a provider.
func (a *App) SetProviderEnabled(provider string, enabled bool) error {
	return a.svc.SetProviderEnabled(domain.ProviderID(provider), enabled)
}

// SetManualUsage stores a user-entered reading for a manual provider
// (e.g. Claude subscription percentage shown in claude.ai).
func (a *App) SetManualUsage(provider string, used int64, limit int64, window string) error {
	return a.svc.SetManualUsage(domain.ProviderID(provider), used, limit, window)
}

// ExperimentalClaudeSubscription attempts an automated reading of the
// claude.ai subscription usage (experimental, local Claude Code token).
// Pass an empty token to auto-detect it from Claude Code.
func (a *App) ExperimentalClaudeSubscription(token string) claudesub.Result {
	return a.svc.ExperimentalClaudeSubscription(token)
}

// ClaudeOAuthStart builds the Claude authorization URL, opens it in the
// browser and remembers the PKCE verifier for the completion step.
func (a *App) ClaudeOAuthStart() (ClaudeOAuthStart, error) {
	pkce, err := claudesub.NewPKCE()
	if err != nil {
		return ClaudeOAuthStart{}, err
	}
	a.oauthMu.Lock()
	a.oauthPKCE = pkce
	a.oauthReady = true
	a.oauthMu.Unlock()

	cfg := claudesub.DefaultOAuthConfig()
	authURL := claudesub.AuthorizeURL(cfg, pkce)
	if a.ctx != nil {
		runtime.BrowserOpenURL(a.ctx, authURL)
	}
	a.log.Info("claude oauth started")
	return ClaudeOAuthStart{URL: authURL}, nil
}

// ClaudeOAuthComplete exchanges the pasted authorization code for a token,
// stores it in the OS credential store and immediately reads the usage.
func (a *App) ClaudeOAuthComplete(codeInput string) claudesub.Result {
	code := claudesub.ParseCallbackCode(codeInput)
	if code == "" {
		return claudesub.Result{Error: "Pega el código que te mostró Claude."}
	}
	a.oauthMu.Lock()
	pkce := a.oauthPKCE
	ready := a.oauthReady
	a.oauthMu.Unlock()
	if !ready {
		return claudesub.Result{Error: "Pulsa primero «Iniciar sesión con Claude»."}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tok, err := claudesub.ExchangeCode(ctx, &http.Client{Timeout: 30 * time.Second}, claudesub.DefaultOAuthConfig(), code, pkce.Verifier)
	if err != nil {
		a.log.Warn("claude oauth exchange failed", "error", err.Error())
		return claudesub.Result{Error: err.Error()}
	}
	a.oauthMu.Lock()
	a.oauthReady = false
	a.oauthMu.Unlock()

	// Persist the token in the OS credential store (never logged).
	if payload, err := json.Marshal(tok); err == nil {
		if err := credentials.NewStore().Save(claudeOAuthAccount, string(payload)); err != nil {
			a.log.Warn("could not store claude oauth token", "error", err.Error())
		}
	}

	res := a.svc.ExperimentalClaudeSubscription(tok.AccessToken)
	if res.Found {
		res.Source = "login con Claude (token guardado)"
	}
	return res
}

// ClaudeOAuthStart is the response of ClaudeOAuthStart.
type ClaudeOAuthStart struct {
	URL string `json:"url"`
}

// SetRefreshInterval updates the polling cadence (seconds).
func (a *App) SetRefreshInterval(seconds int) error {
	return a.svc.SetRefreshInterval(seconds)
}

// SetStartWithSystem toggles autostart at logon.
func (a *App) SetStartWithSystem(enabled bool) error {
	return a.svc.SetStartWithSystem(enabled)
}

// CompleteOnboarding marks the first-run onboarding as finished.
func (a *App) CompleteOnboarding() error {
	return a.svc.CompleteOnboarding()
}

// History returns bucketed local observations for a provider and span.
func (a *App) History(provider string, span string) (core.HistoryResult, error) {
	return a.svc.History(domain.ProviderID(provider), domain.HistorySpan(span))
}

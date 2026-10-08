package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"karina/internal/claudesub"
	"karina/internal/core"
	"karina/internal/domain"
	"karina/internal/platform"
	"karina/internal/transcripts"
	"karina/internal/updater"
)

// Widget (ventana compacta) geometry.
const (
	// The widget is an "island" hanging from the top-centre of the screen: a
	// small pill that expands into a panel while the pointer is over it.
	islandPillW  = 240
	islandPillH  = 46
	islandWidth  = 500
	islandHeader = 64 // toolbar + padding
	islandRow    = 82
	islandMinH   = 214
	islandMaxH   = 560
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

	// Island (widget) placement. The anchor is its top-centre point; Last is
	// where Karina last put the window, to tell a user drag apart.
	islandPlaced             bool
	islandCX, islandY        int
	islandLastX, islandLastY int

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

// EnterWidgetMode shrinks the window into an always-on-top "island" hanging
// from the top-centre of the screen. It starts collapsed (see
// SetWidgetExpanded).
func (a *App) EnterWidgetMode() error {
	if a.ctx == nil {
		return errors.New("aplicación no iniciada")
	}
	// A maximised window cannot be shrunk; unmaximise first.
	runtime.WindowUnmaximise(a.ctx)
	runtime.WindowSetAlwaysOnTop(a.ctx, true)
	runtime.WindowSetMinSize(a.ctx, 160, 40)
	a.widgetMode = true
	a.placeIsland(false)

	runtime.EventsEmit(a.ctx, "mode:widget", true)
	a.log.Info("widget mode enabled")
	return nil
}

// SetWidgetExpanded switches the island between its collapsed pill and the
// expanded panel. The frontend calls it when the pointer enters or leaves.
func (a *App) SetWidgetExpanded(expanded bool) error {
	if a.ctx == nil {
		return errors.New("aplicación no iniciada")
	}
	if !a.widgetMode {
		return nil
	}
	a.placeIsland(expanded)
	return nil
}

// placeIsland sizes the widget window and puts it at its anchor: the
// top-centre of the screen by default, or wherever the user dragged it.
func (a *App) placeIsland(expanded bool) {
	w, h := islandPillW, islandPillH
	if expanded {
		n := 0
		for _, m := range a.svc.ListProviders() {
			if m.Enabled {
				n++
			}
		}
		if n < 1 {
			n = 1
		}
		w = islandWidth
		h = islandHeader + n*islandRow
		if h < islandMinH {
			h = islandMinH
		}
		if h > islandMaxH {
			h = islandMaxH
		}
	}
	sw, sh := a.screenSize()
	if !a.islandPlaced {
		// First placement of this widget session: where the user last left
		// it, or the top-centre of the screen.
		cx, y, moved := a.svc.WidgetAnchor()
		if !moved {
			cx, y = sw/2, 0
		}
		a.islandCX, a.islandY = cx, y
		a.islandPlaced = true
	} else {
		a.captureIslandDrag()
	}

	runtime.WindowSetSize(a.ctx, w, h)

	// The anchor is the island's top-centre, so the pill and the panel grow
	// from the same point. Keep the whole window on screen.
	x, y := a.islandCX-w/2, a.islandY
	if sw > 0 && x > sw-w {
		x = sw - w
	}
	if sh > 0 && y > sh-h {
		y = sh - h
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	runtime.WindowSetPosition(a.ctx, x, y)
	a.islandLastX, a.islandLastY = x, y
	// Docked at the top edge the island keeps square top corners.
	runtime.EventsEmit(a.ctx, "widget:docked", y == 0)
}

// captureIslandDrag notices that the user dragged the island since Karina
// last positioned it, and adopts (and remembers) the new spot.
func (a *App) captureIslandDrag() {
	x, y := runtime.WindowGetPosition(a.ctx)
	if x == a.islandLastX && y == a.islandLastY {
		return
	}
	w, _ := runtime.WindowGetSize(a.ctx)
	a.islandCX, a.islandY = x+w/2, y
	a.islandLastX, a.islandLastY = x, y
	if err := a.svc.SetWidgetAnchor(a.islandCX, a.islandY); err != nil {
		a.log.Warn("could not save widget position", "error", err.Error())
	}
}

// ResetWidgetPosition sends the island back to the top-centre of the screen.
func (a *App) ResetWidgetPosition() error {
	if a.ctx == nil {
		return errors.New("aplicación no iniciada")
	}
	if !a.widgetMode {
		return nil
	}
	if err := a.svc.ClearWidgetAnchor(); err != nil {
		return err
	}
	a.islandPlaced = false
	a.placeIsland(true)
	return nil
}

// screenSize returns the size of the screen the window is on.
func (a *App) screenSize() (w, h int) {
	screens, err := runtime.ScreenGetAll(a.ctx)
	if err != nil || len(screens) == 0 {
		return 0, 0
	}
	sc := screens[0]
	for _, s := range screens {
		if s.IsPrimary {
			sc = s
		}
	}
	for _, s := range screens {
		if s.IsCurrent {
			sc = s
		}
	}
	w, h = sc.Size.Width, sc.Size.Height
	if w <= 0 {
		w = sc.Width
	}
	if h <= 0 {
		h = sc.Height
	}
	return w, h
}

// ExitWidgetMode restores the normal dashboard window.
func (a *App) ExitWidgetMode() error {
	if a.ctx == nil {
		return errors.New("aplicación no iniciada")
	}
	if a.widgetMode && a.islandPlaced {
		a.captureIslandDrag()
	}
	a.islandPlaced = false
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
	if err := claudesub.SaveOwnToken(tok); err != nil {
		a.log.Warn("could not store claude oauth token", "error", err.Error())
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

// LogClientError records a frontend error to a local file (diagnostics).
func (a *App) LogClientError(message string, stack string) {
	if err := a.svc.LogClientError(message, stack); err != nil {
		a.log.Warn("could not write client error log", "error", err.Error())
	}
}

// SetRefreshInterval updates the polling cadence (seconds).
func (a *App) SetRefreshInterval(seconds int) error {
	return a.svc.SetRefreshInterval(seconds)
}

// SetStartWithSystem toggles autostart at logon.
func (a *App) SetStartWithSystem(enabled bool) error {
	return a.svc.SetStartWithSystem(enabled)
}

// SetAlertsEnabled toggles usage-threshold alert notifications.
func (a *App) SetAlertsEnabled(enabled bool) error {
	return a.svc.SetAlertsEnabled(enabled)
}

// SetAlertThreshold updates the usage percentage that triggers an alert.
func (a *App) SetAlertThreshold(percent int) error {
	return a.svc.SetAlertThreshold(percent)
}

// SetWebhookURL stores (or, given an empty string, clears) the webhook that
// receives a POST whenever a usage threshold alert fires.
func (a *App) SetWebhookURL(url string) error {
	return a.svc.SetWebhookURL(url)
}

// TestWebhook sends a sample alert to the configured webhook.
func (a *App) TestWebhook() error {
	return a.svc.TestWebhook()
}

// CompleteOnboarding marks the first-run onboarding as finished. It also
// asks the OS for notification permission right away, on platforms that
// require it (macOS): this is the first moment the user is actively engaged
// with the app, a better time for that system prompt to appear than
// silently the first time a threshold alert happens to fire.
func (a *App) CompleteOnboarding() error {
	platform.RequestNotificationPermission()
	return a.svc.CompleteOnboarding()
}

// History returns bucketed local observations for a provider and span.
func (a *App) History(provider string, span string) (core.HistoryResult, error) {
	return a.svc.History(domain.ProviderID(provider), domain.HistorySpan(span))
}

// ClaudeCodeDetected reports whether Claude Code has been used on this
// machine (local transcripts exist).
func (a *App) ClaudeCodeDetected() bool {
	return a.svc.ClaudeCodeDetected()
}

// ClaudeCodeUsage returns real local Claude Code token usage per project
// and per day, read straight from ~/.claude/projects, for the given span.
func (a *App) ClaudeCodeUsage(span string) (transcripts.Summary, error) {
	return a.svc.ClaudeCodeUsage(domain.HistorySpan(span))
}

// SetProjectClient assigns a Claude Code project to a client/label ("" to
// remove the assignment).
func (a *App) SetProjectClient(path string, client string) error {
	return a.svc.SetProjectClient(path, client)
}

// SetFolderClient assigns every project under a folder to a client/label
// ("" to remove the rule).
func (a *App) SetFolderClient(folder string, client string) error {
	return a.svc.SetFolderClient(folder, client)
}

// SetSubscriptionPrice stores the monthly price of the user's Claude plan.
func (a *App) SetSubscriptionPrice(usd float64) error {
	return a.svc.SetSubscriptionPrice(usd)
}

// ClientReport returns the Claude Code usage of a period ("this_month",
// "last_month", "today", "7d", "30d") grouped by client, for the printable
// report.
func (a *App) ClientReport(period string) (core.ClientReport, error) {
	return a.svc.ClientReport(period)
}

// ClientBudgets returns each client's monthly budget and month-to-date spend.
func (a *App) ClientBudgets() ([]core.ClientBudget, error) {
	return a.svc.ClientBudgets()
}

// SetClientBudget sets a client's monthly budget in USD (0 removes it).
func (a *App) SetClientBudget(client string, usd float64) error {
	return a.svc.SetClientBudget(client, usd)
}

// SetSubscriptionInterval sets how often the Claude subscription usage is
// read (seconds). It is separate from, and slower than, the general interval.
func (a *App) SetSubscriptionInterval(seconds int) error {
	return a.svc.SetSubscriptionInterval(seconds)
}

// SetIdleGapMinutes sets the pause that ends a stretch of work when
// measuring time worked.
func (a *App) SetIdleGapMinutes(minutes int) error {
	return a.svc.SetIdleGapMinutes(minutes)
}

// SetReportBusinessName sets the name printed at the top of client reports.
func (a *App) SetReportBusinessName(name string) error {
	return a.svc.SetReportBusinessName(name)
}

// ExportClientReport opens a native "save as" dialog and writes the Claude
// Code usage of a period, grouped by client and project, to a CSV file.
// Returns the saved path, or "" if the user cancels the dialog.
func (a *App) ExportClientReport(span string) (string, error) {
	if a.ctx == nil {
		return "", errors.New("aplicación no iniciada")
	}
	data, err := a.svc.ExportClientReportCSV(span)
	if err != nil {
		return "", err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Exportar reporte por cliente",
		DefaultFilename: fmt.Sprintf("karina-clientes-%s-%s.csv", span, time.Now().Format("2006-01-02")),
		Filters: []runtime.FileFilter{
			{DisplayName: "CSV (*.csv)", Pattern: "*.csv"},
		},
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("guardar archivo: %w", err)
	}
	a.log.Info("client report exported", "span", span, "path", path)
	return path, nil
}

// ExportHistory opens a native "save as" dialog and writes the raw local
// usage history for a provider/span to a CSV file. Returns the saved path,
// or "" if the user cancels the dialog.
func (a *App) ExportHistory(provider string, span string) (string, error) {
	if a.ctx == nil {
		return "", errors.New("aplicación no iniciada")
	}
	data, err := a.svc.ExportHistoryCSV(domain.ProviderID(provider), domain.HistorySpan(span))
	if err != nil {
		return "", err
	}

	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Exportar historial",
		DefaultFilename: fmt.Sprintf("karina-%s-%s-%s.csv", provider, span, time.Now().Format("2006-01-02")),
		Filters: []runtime.FileFilter{
			{DisplayName: "CSV (*.csv)", Pattern: "*.csv"},
		},
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("guardar archivo: %w", err)
	}
	a.log.Info("history exported", "provider", provider, "span", span, "path", path)
	return path, nil
}

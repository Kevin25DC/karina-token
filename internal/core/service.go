// Package core wires the application together: configuration, credential
// storage, provider adapters, the polling scheduler and local history. It is
// UI-framework agnostic (no Wails imports) so it stays fully testable.
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"karina/internal/claudesub"
	"karina/internal/config"
	"karina/internal/credentials"
	"karina/internal/domain"
	"karina/internal/logging"
	"karina/internal/providers"
	"karina/internal/providers/api"
	"karina/internal/scheduler"
	"karina/internal/storage"
)

const (
	refreshTimeout     = 25 * time.Second
	minIntervalSeconds = 10
	maxIntervalSeconds = 3600
)

// Event kinds pushed to subscribers (and from there to the UI).
const (
	EventCycleStart     = "cycle:start"
	EventProviderUpdate = "provider:update"
	EventCycleEnd       = "cycle:end"
)

// Event is an immutable notification for subscribers.
type Event struct {
	Kind      string                 `json:"kind"`
	Provider  domain.ProviderID      `json:"provider,omitempty"`
	State     *domain.ProviderState  `json:"state,omitempty"`
	AllStates []domain.ProviderState `json:"all_states,omitempty"`
}

// Options configure a Service (tests inject fake pieces here).
type Options struct {
	DataDir         string
	HTTP            *http.Client
	Logger          *slog.Logger
	ProviderFactory func(domain.ProviderID) (api.Provider, error)
	Autostart       func(enabled bool) error
}

// manualReading is a user-entered usage reading for a manual provider
// (e.g. Claude subscription percentage shown in claude.ai).
type manualReading struct {
	Used      int64     `json:"used"`
	Limit     int64     `json:"limit"`
	Window    string    `json:"window"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Service is the application core.
type Service struct {
	logger *slog.Logger
	opts   Options

	cfg     *config.Config
	cfgPath string
	baseDir string

	creds *credentials.Store
	store *storage.Store
	http  *http.Client

	newProvider func(domain.ProviderID) (api.Provider, error)

	mu       sync.Mutex
	adapters map[domain.ProviderID]api.Provider
	states   map[domain.ProviderID]domain.ProviderState
	nextTry  map[domain.ProviderID]time.Time
	backoff  map[domain.ProviderID]time.Duration
	lastSnap map[domain.ProviderID]time.Time
	lastPt   map[domain.ProviderID]*domain.HistoryPoint

	manualPath string
	manual     map[domain.ProviderID]manualReading

	listeners map[int]func(Event)
	nextID    int

	runMu      sync.Mutex
	running    bool
	closeOnce  sync.Once
	rootCtx    context.Context
	cancelRoot context.CancelFunc
	sched      *scheduler.Scheduler
}

// New creates a service with the given logger.
func New(logger *slog.Logger) *Service {
	if logger == nil {
		logger = logging.New(slog.LevelInfo)
	}
	return &Service{
		logger:    logger,
		adapters:  map[domain.ProviderID]api.Provider{},
		states:    map[domain.ProviderID]domain.ProviderState{},
		nextTry:   map[domain.ProviderID]time.Time{},
		backoff:   map[domain.ProviderID]time.Duration{},
		lastSnap:  map[domain.ProviderID]time.Time{},
		lastPt:    map[domain.ProviderID]*domain.HistoryPoint{},
		manual:    map[domain.ProviderID]manualReading{},
		listeners: map[int]func(Event){},
	}
}

// Open loads configuration and prepares storage. Safe to call once.
func (s *Service) Open(opts Options) error {
	if opts.Logger != nil {
		s.logger = opts.Logger
	}
	if opts.DataDir != "" {
		s.baseDir = opts.DataDir
		if err := os.MkdirAll(s.baseDir, 0o700); err != nil {
			return err
		}
	} else {
		dir, err := config.BaseDir()
		if err != nil {
			return err
		}
		s.baseDir = dir
	}
	s.cfgPath = filepath.Join(s.baseDir, "config.toml")
	cfg, err := config.Load(s.cfgPath)
	if err != nil {
		return err
	}
	s.cfg = cfg
	s.opts = opts
	s.http = opts.HTTP
	if s.http == nil {
		s.http = api.DefaultClient()
	}
	s.newProvider = opts.ProviderFactory
	if s.newProvider == nil {
		s.newProvider = providers.New
	}
	s.creds = credentials.NewStore()
	s.store, err = storage.New(s.baseDir)
	if err != nil {
		return err
	}

	// Seed placeholder states for enabled providers so the UI is not empty
	// on startup before the first cycle completes.
	for id, e := range cfg.Providers {
		if e.Enabled {
			if m, ok := providers.Get(domain.ProviderID(id)); ok {
				s.seedState(m)
			}
		}
	}

	// Manual providers (e.g. Claude subscription) have no adapter: restore
	// their last user-entered readings.
	s.manualPath = filepath.Join(s.baseDir, "manual.json")
	if err := s.loadManualLocked(); err != nil {
		return err
	}
	s.mu.Lock()
	for id, r := range s.manual {
		s.setStatus(s.manualState(id, r))
	}
	s.mu.Unlock()

	s.logger.Info("service opened", "dir", s.baseDir, "interval", cfg.RefreshInterval().String())
	return nil
}

func (s *Service) loadManualLocked() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.manualPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := json.Unmarshal(data, &s.manual); err != nil {
		return fmt.Errorf("parse manual readings: %w", err)
	}
	if s.manual == nil {
		s.manual = map[domain.ProviderID]manualReading{}
	}
	return nil
}

func (s *Service) saveManualLocked() error {
	data, err := json.Marshal(s.manual)
	if err != nil {
		return err
	}
	return os.WriteFile(s.manualPath, data, 0o600)
}

func (s *Service) manualState(id domain.ProviderID, r manualReading) domain.ProviderState {
	m, _ := providers.Get(id)
	if r.Limit <= 0 {
		r.Limit = 100
	}
	return domain.ProviderState{
		Provider:        id,
		DisplayName:     m.Name,
		Status:          domain.StatusConnected,
		StatusMsg:       "Lectura manual",
		UpdatedAt:       r.UpdatedAt,
		Capabilities:    []domain.Capability{domain.CapTokenUsage},
		UsageAvailable:  true,
		UsedTokens:      r.Used,
		LimitTokens:     r.Limit,
		RemainingTokens: domain.Remaining(r.Used, r.Limit),
		UsageWindow:     r.Window,
		Note:            "Lectura introducida manualmente desde claude.ai (no existe API oficial para la suscripción). Actualízala cuando cambie tu uso.",
	}
}

// SetManualUsage stores a user-entered reading for a manual provider (used
// is expressed in the same unit as limit, e.g. percentage when limit = 100).
func (s *Service) SetManualUsage(id domain.ProviderID, used, limit int64, window string) error {
	if !providers.IsManual(id) {
		return fmt.Errorf("el proveedor %q no admite lectura manual", id)
	}
	if used < 0 {
		used = 0
	}
	if limit <= 0 {
		limit = 100
	}
	if used > limit {
		used = limit
	}
	r := manualReading{Used: used, Limit: limit, Window: window, UpdatedAt: time.Now()}

	s.mu.Lock()
	s.manual[id] = r
	err := s.saveManualLocked()
	state := s.manualState(id, r)
	s.mu.Unlock()
	if err != nil {
		return err
	}

	s.cfg.SetEnabled(string(id), true)
	_ = config.Save(s.cfgPath, s.cfg)

	s.setStatus(state)
	s.persistSnapshot(id, state)
	s.logger.Info("manual usage saved", "provider", id, "used", used, "limit", limit)
	s.emit(Event{Kind: EventProviderUpdate, Provider: id, State: s.cloneState(state)})
	return nil
}

func (s *Service) seedState(m domain.ProviderMeta) {
	st := domain.ProviderState{
		Provider:     m.ID,
		DisplayName:  m.Name,
		Status:       domain.StatusDisconnected,
		StatusMsg:    "Esperando la primera actualización…",
		Capabilities: m.Capabilities,
	}
	s.setStatus(st)
}

// Close stops the scheduler and releases resources.
func (s *Service) Close() {
	s.closeOnce.Do(func() {
		if s.sched != nil {
			s.sched.Stop()
		}
		if s.cancelRoot != nil {
			s.cancelRoot()
		}
		s.logger.Info("service closed")
	})
}

// Start begins the polling loop.
func (s *Service) Start(ctx context.Context) error {
	s.rootCtx, s.cancelRoot = context.WithCancel(ctx)
	sched := scheduler.New(scheduler.Options{
		Interval:   s.cfg.RefreshInterval(),
		MaxBackoff: s.cfg.RefreshInterval() * 8,
		Jitter:     s.cfg.RefreshInterval() / 10,
	})
	s.sched = sched
	sched.Start(s.rootCtx, s.refreshCycle)
	// Kick off the first refresh almost immediately.
	go func() {
		select {
		case <-time.After(300 * time.Millisecond):
			sched.Trigger()
		case <-s.rootCtx.Done():
		}
	}()
	s.logger.Info("polling started", "interval", s.cfg.RefreshInterval().String())
	return nil
}

// Subscribe registers a listener for events. Returns an unsubscribe func.
func (s *Service) Subscribe(listener func(Event)) func() {
	s.mu.Lock()
	id := s.nextID
	s.nextID++
	s.listeners[id] = listener
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		delete(s.listeners, id)
		s.mu.Unlock()
	}
}

func (s *Service) emit(e Event) {
	s.mu.Lock()
	ls := make([]func(Event), 0, len(s.listeners))
	for _, l := range s.listeners {
		ls = append(ls, l)
	}
	s.mu.Unlock()
	for _, l := range ls {
		l(e)
	}
}

// RefreshNow asks the scheduler for an immediate cycle.
func (s *Service) RefreshNow() {
	if s.sched != nil {
		s.sched.Trigger()
	}
}

func (s *Service) refreshCycle(ctx context.Context) error {
	if !s.runMu.TryLock() {
		s.logger.Debug("cycle skipped: previous still running")
		return nil
	}
	defer s.runMu.Unlock()

	s.mu.Lock()
	ids := s.enabledReadyIDsLocked()
	s.mu.Unlock()
	if len(ids) == 0 {
		return nil
	}

	s.emit(Event{Kind: EventCycleStart})
	s.logger.Info("refresh cycle start", "providers", len(ids))

	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id domain.ProviderID) {
			defer wg.Done()
			s.refreshOne(ctx, id)
		}(id)
	}
	wg.Wait()

	s.emit(Event{Kind: EventCycleEnd, AllStates: s.States()})
	s.logger.Info("refresh cycle end")
	return nil
}

func (s *Service) enabledReadyIDsLocked() []domain.ProviderID {
	var out []domain.ProviderID
	for id, e := range s.cfg.Providers {
		if !e.Enabled {
			continue
		}
		// Manual providers have no adapter to poll.
		if providers.IsManual(domain.ProviderID(id)) {
			continue
		}
		if s.hasCredentialLocked(domain.ProviderID(id)) {
			out = append(out, domain.ProviderID(id))
		}
	}
	return out
}

func (s *Service) hasCredentialLocked(id domain.ProviderID) bool {
	if id == "demo" {
		return true
	}
	key, err := s.creds.Get(string(id))
	return err == nil && key != ""
}

func (s *Service) refreshOne(ctx context.Context, id domain.ProviderID) {
	s.mu.Lock()
	next := s.nextTry[id]
	s.mu.Unlock()
	if !next.IsZero() && time.Now().Before(next) {
		s.logger.Debug("provider skipped by backoff", "provider", id, "retry_at", next)
		return
	}

	adapter, err := s.adapterFor(id)
	if err != nil {
		s.stateError(id, err)
		return
	}

	prev := s.currentState(id)
	updating := prev
	updating.Status = domain.StatusUpdating
	updating.StatusMsg = "Actualizando…"
	updating.UpdatedAt = time.Now()
	s.setStatus(updating)
	s.emit(Event{Kind: EventProviderUpdate, Provider: id, State: s.cloneState(updating)})

	key := "demo"
	if id != "demo" {
		key, err = s.creds.Get(string(id))
		if err != nil {
			s.markFailed(id, adapter, fmt.Errorf("credentials: %w", err))
			return
		}
	}

	cfg := api.Config{APIKey: key, HTTP: s.http, Logger: s.logger}

	runCtx, cancel := context.WithTimeout(ctx, refreshTimeout)
	defer cancel()
	state, rerr := adapter.Refresh(runCtx, cfg)

	if rerr != nil {
		s.markFailed(id, adapter, rerr)
		return
	}
	state.Provider = id
	if state.DisplayName == "" {
		state.DisplayName = adapter.DisplayName()
	}
	if state.Capabilities == nil {
		state.Capabilities = adapter.Capabilities()
	}
	state.UpdatedAt = time.Now()
	if state.Status == "" {
		state.Status = domain.StatusConnected
	}

	s.mu.Lock()
	delete(s.backoff, id)
	delete(s.nextTry, id)
	s.mu.Unlock()

	s.setStatus(state)
	s.persistSnapshot(id, state)
	s.logger.Info("provider refreshed", "provider", id, "status", state.Status)
	s.emit(Event{Kind: EventProviderUpdate, Provider: id, State: s.cloneState(state)})
}

// markFailed records an error state and schedules an exponential backoff.
func (s *Service) markFailed(id domain.ProviderID, adapter api.Provider, err error) {
	prev := s.currentState(id)
	state := api.StateForError(adapter, err, prev)
	state.Provider = id
	state.DisplayName = adapter.DisplayName()

	base := s.cfg.RefreshInterval()
	s.mu.Lock()
	d := s.backoff[id]
	if d <= 0 {
		d = base
	}
	d *= 2
	if d > base*16 {
		d = base * 16
	}
	s.backoff[id] = d
	s.nextTry[id] = time.Now().Add(d)
	s.mu.Unlock()

	s.setStatus(state)
	s.logger.Warn("provider request failed", "provider", id, "error", err.Error())
	s.emit(Event{Kind: EventProviderUpdate, Provider: id, State: s.cloneState(state)})
}

func (s *Service) adapterFor(id domain.ProviderID) (api.Provider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.adapters[id]; ok {
		return p, nil
	}
	p, err := s.newProvider(id)
	if err != nil {
		return nil, err
	}
	s.adapters[id] = p
	return p, nil
}

func (s *Service) stateError(id domain.ProviderID, err error) {
	st := domain.ProviderState{
		Provider:  id,
		Status:    domain.StatusError,
		StatusMsg: "Proveedor no disponible",
		Error:     err.Error(),
		UpdatedAt: time.Now(),
	}
	s.setStatus(st)
	s.logger.Error("provider failed", "provider", id, "error", err.Error())
	s.emit(Event{Kind: EventProviderUpdate, Provider: id, State: s.cloneState(st)})
}

// TestResult is the outcome of a credential test.
type TestResult struct {
	State domain.ProviderState `json:"state"`
	OK    bool                 `json:"ok"`
	Error string               `json:"error,omitempty"`
}

// TestProvider validates a key without storing it. It returns the resulting
// state and whether the credentials were accepted.
func (s *Service) TestProvider(id domain.ProviderID, key string) TestResult {
	adapter, err := s.adapterFor(id)
	if err != nil {
		return TestResult{State: domain.ProviderState{Provider: id, Status: domain.StatusError, Error: err.Error()}}
	}
	ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
	defer cancel()
	state, rerr := adapter.Refresh(ctx, api.Config{APIKey: key, HTTP: s.http, Logger: s.logger, ValidationOnly: true})
	if rerr != nil {
		state = api.StateForError(adapter, rerr, state)
	}
	if state.DisplayName == "" {
		state.DisplayName = adapter.DisplayName()
	}
	ok := state.Status == domain.StatusConnected
	if !ok {
		s.logger.Warn("provider test failed", "provider", id, "status", state.Status)
	}
	return TestResult{State: state, OK: ok, Error: state.Error}
}

// SaveProviderKey stores a key (after the UI has tested it) and enables the
// provider.
func (s *Service) SaveProviderKey(id domain.ProviderID, key string) error {
	if err := s.creds.Save(string(id), key); err != nil {
		return fmt.Errorf("store credentials: %w", err)
	}
	s.cfg.SetEnabled(string(id), true)
	if err := config.Save(s.cfgPath, s.cfg); err != nil {
		return err
	}
	s.logger.Info("provider credentials saved", "provider", id)
	s.RefreshNow()
	return nil
}

// RemoveProvider deletes the key, disables the provider and clears history.
func (s *Service) RemoveProvider(id domain.ProviderID) error {
	_ = s.creds.Delete(string(id))
	s.cfg.SetEnabled(string(id), false)
	if err := config.Save(s.cfgPath, s.cfg); err != nil {
		return err
	}
	if s.store != nil {
		_ = s.store.DropProvider(id)
	}
	s.mu.Lock()
	delete(s.states, id)
	delete(s.lastSnap, id)
	delete(s.lastPt, id)
	delete(s.nextTry, id)
	delete(s.backoff, id)
	if providers.IsManual(id) {
		delete(s.manual, id)
		_ = s.saveManualLocked()
	}
	s.mu.Unlock()
	s.logger.Info("provider removed", "provider", id)
	s.emit(Event{Kind: EventProviderUpdate, Provider: id})
	return nil
}

// ExperimentalClaudeSubscription attempts an automated reading of the
// claude.ai subscription usage by reusing the local Claude Code OAuth token.
// If token is non-empty it is used directly; otherwise Karina looks for the
// token saved by Claude Code on this machine. Experimental: undocumented
// endpoint, use at your own risk.
func (s *Service) ExperimentalClaudeSubscription(token string) claudesub.Result {
	r := &claudesub.Reader{HTTP: s.http, OverrideToken: token}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	res := r.Read(ctx)
	s.logger.Info("experimental claude subscription read", "found", res.Found, "window", res.Window)
	return res
}

// SetProviderEnabled toggles a provider without touching credentials.
func (s *Service) SetProviderEnabled(id domain.ProviderID, enabled bool) error {
	s.cfg.SetEnabled(string(id), enabled)
	if err := config.Save(s.cfgPath, s.cfg); err != nil {
		return err
	}
	if enabled {
		s.RefreshNow()
	}
	return nil
}

// SetRefreshInterval updates the polling cadence.
func (s *Service) SetRefreshInterval(seconds int) error {
	if seconds < minIntervalSeconds {
		seconds = minIntervalSeconds
	}
	if seconds > maxIntervalSeconds {
		seconds = maxIntervalSeconds
	}
	s.cfg.RefreshIntervalSeconds = seconds
	if err := config.Save(s.cfgPath, s.cfg); err != nil {
		return err
	}
	s.restartScheduler()
	s.logger.Info("refresh interval updated", "interval", s.cfg.RefreshInterval().String())
	return nil
}

func (s *Service) restartScheduler() {
	if s.rootCtx == nil {
		return
	}
	if s.sched != nil {
		s.sched.Stop()
		s.sched = nil
	}
	sched := scheduler.New(scheduler.Options{
		Interval:   s.cfg.RefreshInterval(),
		MaxBackoff: s.cfg.RefreshInterval() * 8,
		Jitter:     s.cfg.RefreshInterval() / 10,
	})
	s.sched = sched
	sched.Start(s.rootCtx, s.refreshCycle)
	s.sched.Trigger()
}

// SetStartWithSystem persists the preference and invokes the platform hook.
func (s *Service) SetStartWithSystem(enabled bool) error {
	s.cfg.StartWithSystem = enabled
	if err := config.Save(s.cfgPath, s.cfg); err != nil {
		return err
	}
	if s.opts.Autostart != nil {
		if err := s.opts.Autostart(enabled); err != nil {
			return err
		}
	}
	return nil
}

// CompleteOnboarding marks the onboarding as finished.
func (s *Service) CompleteOnboarding() error {
	s.cfg.OnboardingDone = true
	return config.Save(s.cfgPath, s.cfg)
}

// ConfigSnapshot is a safe, key-free view of the configuration for the UI.
type ConfigSnapshot struct {
	RefreshIntervalSeconds int    `json:"refresh_interval_seconds"`
	StartWithSystem        bool   `json:"start_with_system"`
	OnboardingDone         bool   `json:"onboarding_done"`
	DataDir                string `json:"data_dir"`
}

// Config returns a snapshot of the configuration (never contains keys).
func (s *Service) Config() ConfigSnapshot {
	return ConfigSnapshot{
		RefreshIntervalSeconds: s.cfg.RefreshIntervalSeconds,
		StartWithSystem:        s.cfg.StartWithSystem,
		OnboardingDone:         s.cfg.OnboardingDone,
		DataDir:                s.baseDir,
	}
}

// KeyPreview returns a redacted preview of the stored key, or "" when none.
// The raw key never leaves the credential store.
func (s *Service) KeyPreview(id domain.ProviderID) string {
	if id == "demo" {
		return "demo"
	}
	if !s.creds.Has(string(id)) {
		return ""
	}
	key, err := s.creds.Get(string(id))
	if err != nil {
		return ""
	}
	return credentials.Preview(key)
}

// ListProviders returns the catalog annotated with runtime state.
func (s *Service) ListProviders() []domain.ProviderMeta {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := providers.Catalog()
	for i := range out {
		m := &out[i]
		m.Enabled = s.cfg.Enabled(string(m.ID))
		m.HasKey = s.hasCredentialLocked(m.ID)
	}
	return out
}

// States returns a snapshot of all known provider states, in catalog order.
func (s *Service) States() []domain.ProviderState {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]domain.ProviderState, 0, len(s.states))
	for _, m := range providers.Catalog() {
		if st, ok := s.states[m.ID]; ok {
			out = append(out, st)
		}
	}
	return out
}

func (s *Service) setStatus(st domain.ProviderState) {
	s.mu.Lock()
	s.states[st.Provider] = st
	s.mu.Unlock()
}

func (s *Service) currentState(id domain.ProviderID) domain.ProviderState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.states[id]
}

func (s *Service) cloneState(st domain.ProviderState) *domain.ProviderState {
	c := st
	return &c
}

// persistSnapshot writes an observation to local history when the real
// numbers changed (or every ~15 minutes as a heartbeat) but never faster
// than the configured snapshot interval.
func (s *Service) persistSnapshot(id domain.ProviderID, st domain.ProviderState) {
	if !st.UsageAvailable && st.Balance == nil {
		return
	}
	pt := pointOf(st)

	s.mu.Lock()
	last := s.lastPt[id]
	lastAt := s.lastSnap[id]
	s.mu.Unlock()

	now := time.Now()
	if !lastAt.IsZero() && now.Sub(lastAt) < s.cfg.SnapshotInterval() {
		return
	}
	changed := last == nil || pointChanged(&pt, last)
	stale := lastAt.IsZero() || now.Sub(lastAt) >= 15*time.Minute
	if !changed && !stale {
		return
	}
	if err := s.store.AppendPoint(id, pt); err != nil {
		s.logger.Warn("history write failed", "provider", id, "error", err.Error())
		return
	}
	s.mu.Lock()
	s.lastSnap[id] = now
	s.lastPt[id] = &pt
	s.mu.Unlock()
}

func pointOf(st domain.ProviderState) domain.HistoryPoint {
	p := domain.HistoryPoint{
		At:              st.UpdatedAt,
		UsageAvailable:  st.UsageAvailable,
		UsedTokens:      st.UsedTokens,
		LimitTokens:     st.LimitTokens,
		RemainingTokens: st.RemainingTokens,
	}
	if st.Balance != nil {
		p.BalanceTotal = st.Balance.Total
		p.BalanceCurrency = st.Balance.Currency
	}
	if st.RateLimit.Tokens != nil {
		p.RateLimitRemaining = st.RateLimit.Tokens.Remaining
		p.RateLimitLimit = st.RateLimit.Tokens.Limit
	}
	return p
}

func pointChanged(a, b *domain.HistoryPoint) bool {
	if a == nil || b == nil {
		return true
	}
	return a.UsedTokens != b.UsedTokens ||
		a.LimitTokens != b.LimitTokens ||
		a.RemainingTokens != b.RemainingTokens ||
		a.BalanceTotal != b.BalanceTotal
}

// HistoryResult is a bucketed, chart-ready series plus meta information.
type HistoryResult struct {
	Provider   domain.ProviderID     `json:"provider"`
	Span       domain.HistorySpan    `json:"span"`
	HasUsage   bool                  `json:"has_usage"`
	HasBilling bool                  `json:"has_billing"`
	Points     []domain.HistoryPoint `json:"points"`
}

// History returns downsampled local observations for a provider and span.
func (s *Service) History(id domain.ProviderID, span domain.HistorySpan) (HistoryResult, error) {
	res := HistoryResult{Provider: id, Span: span}
	if s.store == nil {
		return res, nil
	}
	now := time.Now()
	points, err := s.store.Points(id, span.Start(now))
	if err != nil {
		return res, err
	}
	if len(points) == 0 {
		return res, nil
	}

	// Determine which metrics really exist in the data.
	for _, p := range points {
		if p.UsageAvailable || p.UsedTokens > 0 {
			res.HasUsage = true
		}
		if p.BalanceTotal != 0 {
			res.HasBilling = true
		}
	}
	res.Points = downsample(points, span, now)
	return res, nil
}

// downsample keeps one "last value per bucket" so charts stay small.
func downsample(points []domain.HistoryPoint, span domain.HistorySpan, now time.Time) []domain.HistoryPoint {
	start := span.Start(now)
	width := time.Hour
	if span == domain.Span7d || span == domain.Span30d {
		width = 24 * time.Hour
	}
	type agg struct {
		pt  domain.HistoryPoint
		idx int64
	}
	last := map[int64]domain.HistoryPoint{}
	for _, p := range points {
		idx := int64(p.At.Sub(start) / width)
		if idx < 0 {
			idx = 0
		}
		if prev, ok := last[idx]; !ok || p.At.After(prev.At) {
			last[idx] = p
		}
	}
	// Build in ascending order.
	maxIdx := int64(now.Sub(start) / width)
	if len(last) == 0 {
		return nil
	}
	out := make([]domain.HistoryPoint, 0, len(last))
	for i := int64(0); i <= maxIdx; i++ {
		if p, ok := last[i]; ok {
			bucket := start.Add(time.Duration(i) * width)
			p.At = bucket
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		for _, p := range last {
			out = append(out, p)
		}
	}
	return out
}

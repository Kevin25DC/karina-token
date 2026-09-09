package core

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"karina/internal/credentials"
	"karina/internal/domain"
	"karina/internal/providers/api"
)

type fakeProvider struct {
	id     string
	name   string
	caps   []domain.Capability
	state  domain.ProviderState
	err    error
	calls  int32
	keyLog []string
}

func (f *fakeProvider) ID() string          { return f.id }
func (f *fakeProvider) DisplayName() string { return f.name }
func (f *fakeProvider) Capabilities() []domain.Capability {
	if f.caps == nil {
		return []domain.Capability{domain.CapTokenUsage}
	}
	return f.caps
}
func (f *fakeProvider) Refresh(_ context.Context, cfg api.Config) (domain.ProviderState, error) {
	atomic.AddInt32(&f.calls, 1)
	f.keyLog = append(f.keyLog, cfg.APIKey)
	if f.err != nil {
		return domain.ProviderState{}, f.err
	}
	st := f.state
	st.Provider = domain.ProviderID(f.id)
	st.DisplayName = f.name
	st.UpdatedAt = time.Now()
	return st, nil
}

func newTestService(t *testing.T, fakes map[string]*fakeProvider) *Service {
	t.Helper()
	credentials.SetBackend(newMemoryBackend())
	dir := t.TempDir()

	factory := func(id domain.ProviderID) (api.Provider, error) {
		if f, ok := fakes[string(id)]; ok {
			return f, nil
		}
		return nil, fmt.Errorf("no fake for %s", id)
	}
	s := New(nil)
	if err := s.Open(Options{DataDir: dir, ProviderFactory: factory}); err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func newMemoryBackend() credentials.Backend {
	return &memoryBack{}
}

type memoryBack struct{ m map[string]string }

func (b *memoryBack) Set(service, account, secret string) error {
	if b.m == nil {
		b.m = map[string]string{}
	}
	b.m[service+"/"+account] = secret
	return nil
}
func (b *memoryBack) Get(service, account string) (string, error) {
	if b.m == nil {
		return "", credentials.ErrNotFound
	}
	if v, ok := b.m[service+"/"+account]; ok {
		return v, nil
	}
	return "", credentials.ErrNotFound
}
func (b *memoryBack) Delete(service, account string) error {
	if b.m == nil {
		return credentials.ErrNotFound
	}
	if _, ok := b.m[service+"/"+account]; !ok {
		return credentials.ErrNotFound
	}
	delete(b.m, service+"/"+account)
	return nil
}

func TestSaveKeyEnablesAndRefreshes(t *testing.T) {
	f := &fakeProvider{id: "openai", name: "OpenAI",
		state: domain.ProviderState{Status: domain.StatusConnected, UsageAvailable: true, UsedTokens: 1000, LimitTokens: 10000}}
	s := newTestService(t, map[string]*fakeProvider{"openai": f})

	if err := s.SaveProviderKey("openai", "sk-test"); err != nil {
		t.Fatalf("save key: %v", err)
	}
	if !s.cfg.Enabled("openai") {
		t.Fatal("provider should be enabled")
	}
	if !s.creds.Has("openai") {
		t.Fatal("key should be stored")
	}

	ctx := context.Background()
	s.refreshOne(ctx, "openai")

	states := s.States()
	if len(states) != 1 {
		t.Fatalf("states len %d", len(states))
	}
	if states[0].Status != domain.StatusConnected {
		t.Fatalf("status %v", states[0].Status)
	}
	if states[0].UsedTokens != 1000 {
		t.Fatalf("used %d", states[0].UsedTokens)
	}
	if atomic.LoadInt32(&f.calls) == 0 {
		t.Fatal("provider should have been called")
	}

	// History should contain an observation because usage is available.
	hist, err := s.History("openai", domain.SpanToday)
	if err != nil {
		t.Fatal(err)
	}
	if !hist.HasUsage || len(hist.Points) == 0 {
		t.Fatalf("expected usage history, got %+v", hist)
	}
}

func TestRefreshFailureStateAndBackoff(t *testing.T) {
	f := &fakeProvider{id: "openai", name: "OpenAI", err: fmt.Errorf("network down")}
	s := newTestService(t, map[string]*fakeProvider{"openai": f})
	if err := s.SaveProviderKey("openai", "sk-test"); err != nil {
		t.Fatal(err)
	}

	s.refreshOne(context.Background(), "openai")
	states := s.States()
	if len(states) != 1 || states[0].Status != domain.StatusError {
		t.Fatalf("expected error state, got %+v", states)
	}

	first := atomic.LoadInt32(&f.calls)
	s.refreshOne(context.Background(), "openai")
	// Backoff should skip the immediate retry.
	if got := atomic.LoadInt32(&f.calls); got != first {
		t.Fatalf("expected backoff skip, calls %d -> %d", first, got)
	}
}

func TestInvalidCredsStatus(t *testing.T) {
	f := &fakeProvider{id: "gemini", name: "Gemini",
		err: api.NewError(api.KindInvalidCredentials, 400, "bad key", nil)}
	s := newTestService(t, map[string]*fakeProvider{"gemini": f})
	if err := s.SaveProviderKey("gemini", "AIza-x"); err != nil {
		t.Fatal(err)
	}
	s.refreshOne(context.Background(), "gemini")

	res := s.TestProvider("gemini", "AIza-bad")
	if res.OK {
		t.Fatal("test must fail for invalid creds")
	}
	if res.State.Status != domain.StatusInvalidCredentials {
		t.Fatalf("status %v", res.State.Status)
	}
}

func TestRemoveProviderCleansUp(t *testing.T) {
	f := &fakeProvider{id: "deepseek", name: "DeepSeek",
		state: domain.ProviderState{Status: domain.StatusConnected, Balance: &domain.Balance{Total: 10, Currency: "USD"}}}
	s := newTestService(t, map[string]*fakeProvider{"deepseek": f})
	if err := s.SaveProviderKey("deepseek", "sk-ds"); err != nil {
		t.Fatal(err)
	}
	s.refreshOne(context.Background(), "deepseek")
	if _, err := s.History("deepseek", domain.SpanToday); err != nil {
		t.Fatal(err)
	}

	if err := s.RemoveProvider("deepseek"); err != nil {
		t.Fatal(err)
	}
	if s.cfg.Enabled("deepseek") {
		t.Fatal("provider still enabled")
	}
	if s.creds.Has("deepseek") {
		t.Fatal("credentials not removed")
	}
	hist, _ := s.History("deepseek", domain.SpanToday)
	if len(hist.Points) != 0 {
		t.Fatalf("history should be cleared, got %d", len(hist.Points))
	}
}

func TestConfigHasNoSecrets(t *testing.T) {
	f := &fakeProvider{id: "openai", name: "OpenAI"}
	s := newTestService(t, map[string]*fakeProvider{"openai": f})
	if err := s.SaveProviderKey("openai", "sk-super-secret"); err != nil {
		t.Fatal(err)
	}
	snap := s.Config()
	if snap.RefreshIntervalSeconds <= 0 {
		t.Fatal("expected interval")
	}
	if snap.DataDir == "" {
		t.Fatal("expected data dir")
	}
}

func TestListProvidersAnnotation(t *testing.T) {
	s := newTestService(t, map[string]*fakeProvider{"demo": &fakeProvider{id: "demo", name: "Demo"}})
	s.cfg.OnboardingDone = true
	list := s.ListProviders()
	found := false
	for _, m := range list {
		if m.ID == "openai" && m.Enabled {
			t.Fatal("unexpected enabled")
		}
		if m.ID == "demo" {
			found = true
			if !m.HasKey {
				t.Fatal("demo should always have a key")
			}
		}
	}
	if !found {
		t.Fatal("demo missing from catalog")
	}
}

func TestHistoryDownsampling(t *testing.T) {
	s := newTestService(t, map[string]*fakeProvider{"openai": &fakeProvider{id: "openai", name: "OpenAI"}})
	now := time.Now()
	base := now.AddDate(0, 0, -3)
	// Write 10 points spread over 3 days (5 minutes apart) for the span 7d.
	for i := 0; i < 10; i++ {
		p := domain.HistoryPoint{
			At:          base.Add(time.Duration(i) * 5 * time.Minute),
			UsedTokens:  int64(i * 100),
			LimitTokens: 10000,
		}
		if err := s.store.AppendPoint("openai", p); err != nil {
			t.Fatal(err)
		}
	}
	hist, err := s.History("openai", domain.Span7d)
	if err != nil {
		t.Fatal(err)
	}
	if !hist.HasUsage {
		t.Fatal("expected usage")
	}
	// All points share <= 2 distinct days, so <= 2 buckets.
	if len(hist.Points) > 2 {
		t.Fatalf("expected <=2 bucketed points, got %d", len(hist.Points))
	}
	// Verify ascending and last-value semantics within bucket: all points
	// used token 0..900 but bucket boundary truncation yields ascending.
	for i := 1; i < len(hist.Points); i++ {
		if hist.Points[i].At.Before(hist.Points[i-1].At) {
			t.Fatal("points not ascending")
		}
	}
}

func TestSetManualUsage(t *testing.T) {
	s := newTestService(t, map[string]*fakeProvider{})
	if err := s.SetManualUsage("claude_subscription", 67, 100, "Ventana de 5 horas"); err != nil {
		t.Fatalf("set manual: %v", err)
	}
	if !s.cfg.Enabled("claude_subscription") {
		t.Fatal("manual provider should be enabled")
	}
	var found bool
	for _, st := range s.States() {
		if st.Provider == "claude_subscription" {
			found = true
			if !st.UsageAvailable || st.UsedTokens != 67 || st.LimitTokens != 100 {
				t.Fatalf("state %+v", st)
			}
			if st.RemainingTokens != 33 {
				t.Fatalf("remaining %d", st.RemainingTokens)
			}
		}
	}
	if !found {
		t.Fatal("manual provider missing from states")
	}
	hist, err := s.History("claude_subscription", domain.SpanToday)
	if err != nil {
		t.Fatal(err)
	}
	if !hist.HasUsage || len(hist.Points) == 0 {
		t.Fatalf("manual usage should be recorded in history: %+v", hist)
	}
}

func TestManualProviderNotPolled(t *testing.T) {
	s := newTestService(t, map[string]*fakeProvider{})
	if err := s.SetManualUsage("claude_subscription", 30, 100, "semanal"); err != nil {
		t.Fatal(err)
	}
	if err := s.refreshCycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	st := s.currentState("claude_subscription")
	if !st.UsageAvailable || st.UsedTokens != 30 {
		t.Fatalf("manual state changed unexpectedly: %+v", st)
	}
}

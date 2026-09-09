package mock

import (
	"context"
	"testing"

	"karina/internal/domain"
	"karina/internal/providers/api"
)

func TestMockLifecycle(t *testing.T) {
	p := New(100_000)
	p.SetUsed(50_000)

	st, err := p.Refresh(context.Background(), api.Config{APIKey: "demo-key"})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if st.Status != domain.StatusConnected {
		t.Fatalf("status %v", st.Status)
	}
	if !st.UsageAvailable {
		t.Fatal("usage should be available")
	}
	if st.LimitTokens != 100_000 {
		t.Fatalf("limit %d", st.LimitTokens)
	}
	if st.UsedTokens > st.LimitTokens {
		t.Fatalf("used %d exceeds limit", st.UsedTokens)
	}
	if st.RemainingTokens != domain.Remaining(st.UsedTokens, st.LimitTokens) {
		t.Fatal("remaining mismatch")
	}
	if st.UpdatedAt.IsZero() {
		t.Fatal("updated at zero")
	}
	if len(st.Capabilities) == 0 {
		t.Fatal("capabilities empty")
	}
}

func TestMockRejectsEmptyKey(t *testing.T) {
	p := New(1000)
	_, err := p.Refresh(context.Background(), api.Config{})
	if err == nil {
		t.Fatal("empty key must error")
	}
	if err.Error() == "" {
		t.Fatal("error without message")
	}
}

func TestMockRandomWalkStaysBounded(t *testing.T) {
	p := New(50_000)
	p.SetUsed(20_000)
	cfg := api.Config{APIKey: "demo-key"}
	for i := 0; i < 50; i++ {
		st, err := p.Refresh(context.Background(), cfg)
		if err != nil {
			t.Fatalf("refresh %d: %v", i, err)
		}
		if st.UsedTokens < 0 || st.UsedTokens > st.LimitTokens {
			t.Fatalf("iter %d out of bounds: %d", i, st.UsedTokens)
		}
	}
}

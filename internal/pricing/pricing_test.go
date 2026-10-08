package pricing

import (
	"math"
	"testing"
)

func TestForPicksLongestPrefix(t *testing.T) {
	p, ok := For("claude-opus-5-5-20260901")
	if !ok || p.Input != 4 {
		t.Fatalf("opus 5.5 = %+v ok=%v", p, ok)
	}
	p, ok = For("claude-opus-5")
	if !ok || p.Input != 5 {
		t.Fatalf("opus 5 = %+v ok=%v", p, ok)
	}
	if _, ok := For("<synthetic>"); ok {
		t.Fatal("unknown model must not be priced")
	}
}

func TestCost(t *testing.T) {
	p, _ := For("claude-sonnet-5")
	// 1M of each: input 2 + output 10 + write5m 2.5 + write1h 4 + read 0.2
	got := p.Cost(1e6, 1e6, 1e6, 1e6, 1e6)
	if math.Abs(got-18.7) > 1e-9 {
		t.Fatalf("cost = %v", got)
	}
}

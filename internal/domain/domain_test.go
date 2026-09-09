package domain

import (
	"testing"
	"time"
)

func TestPercent(t *testing.T) {
	cases := []struct {
		name  string
		used  int64
		limit int64
		want  float64
	}{
		{"empty", 0, 100, 0},
		{"half", 50, 100, 50},
		{"typical", 67420, 100000, 67.4},
		{"at limit", 100, 100, 100},
		{"over limit", 150, 100, 100},
		{"unknown limit", 10, 0, -1},
		{"negative limit", 10, -5, -1},
		{"rounding", 1, 3, 33.3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Percent(c.used, c.limit)
			if got != c.want {
				t.Fatalf("Percent(%d,%d)=%v want %v", c.used, c.limit, got, c.want)
			}
		})
	}
}

func TestRemaining(t *testing.T) {
	if got := Remaining(67, 100); got != 33 {
		t.Fatalf("got %d want 33", got)
	}
	if got := Remaining(150, 100); got != 0 {
		t.Fatalf("got %d want 0", got)
	}
	if got := Remaining(10, 0); got != -1 {
		t.Fatalf("got %d want -1", got)
	}
}

func TestRateBucket(t *testing.T) {
	b := RateBucket{Limit: 1000, Remaining: 400}
	if b.Used() != 600 {
		t.Fatalf("Used()=%d want 600", b.Used())
	}
	if p := b.Percent(); p != 60 {
		t.Fatalf("Percent()=%v want 60", p)
	}
}

func TestSpanStart(t *testing.T) {
	now := time.Date(2026, 9, 9, 15, 30, 0, 0, time.UTC)
	if got := SpanToday.Start(now); got.Hour() != 0 || got.Minute() != 0 {
		t.Fatalf("today start wrong: %v", got)
	}
	if got := Span30d.Start(now); got != now.AddDate(0, 0, -30) {
		t.Fatalf("30d start wrong: %v", got)
	}
}

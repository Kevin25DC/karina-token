package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunsOnInterval(t *testing.T) {
	s := New(Options{Interval: 20 * time.Millisecond, MaxBackoff: 100 * time.Millisecond, Jitter: 1 * time.Millisecond})
	var n atomic.Int64
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx, func(ctx context.Context) error {
		n.Add(1)
		return nil
	})
	defer s.Stop()

	deadline := time.Now().Add(250 * time.Millisecond)
	for n.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if n.Load() < 3 {
		t.Fatalf("expected at least 3 runs, got %d", n.Load())
	}
}

func TestBackoffOnError(t *testing.T) {
	s := New(Options{Interval: 10 * time.Millisecond, MaxBackoff: 60 * time.Millisecond, Jitter: 0})
	var n atomic.Int64
	s.Start(context.Background(), func(ctx context.Context) error {
		n.Add(1)
		return errors.New("boom")
	})
	defer s.Stop()

	// With backoff growing 10->20->40->60, we should see far fewer than
	// interval runs over 200ms.
	time.Sleep(200 * time.Millisecond)
	if got := n.Load(); got > 15 {
		t.Fatalf("too many retries without pacing: %d", got)
	}
	if s.ErrCount() == 0 {
		t.Fatal("expected errors recorded")
	}
}

func TestTriggerRunsImmediately(t *testing.T) {
	s := New(Options{Interval: time.Hour, MaxBackoff: time.Hour, Jitter: 0})
	var n atomic.Int64
	s.Start(context.Background(), func(ctx context.Context) error {
		n.Add(1)
		return nil
	})
	defer s.Stop()

	s.Trigger()
	deadline := time.Now().Add(time.Second)
	for n.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if n.Load() != 1 {
		t.Fatalf("trigger did not run job, got %d", n.Load())
	}
}

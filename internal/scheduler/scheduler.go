// Package scheduler implements a polling loop with jitter and exponential
// backoff, designed so a failing provider/network never hammers the APIs and
// a healthy one is always polled on time.
package scheduler

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Options tunes the scheduler.
type Options struct {
	Interval   time.Duration
	MaxBackoff time.Duration
	Jitter     time.Duration // +/- added to every wait
}

// Scheduler periodically runs a job function. A non-nil error returned by the
// job doubles the next delay up to MaxBackoff. A nil error resets it.
type Scheduler struct {
	opts Options

	mu     sync.Mutex
	timer  *time.Timer
	next   time.Time
	delay  time.Duration
	closed bool

	trigger chan struct{}
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	// runCount and errCount are exposed for tests/observability.
	runCount  atomic.Int64
	errCount  atomic.Int64
	lastErr   atomic.Value // string
	lastStart atomic.Int64 // unix nanos
}

// New returns a scheduler that calls run each Interval.
func New(opts Options) *Scheduler {
	if opts.Interval <= 0 {
		opts.Interval = 30 * time.Second
	}
	if opts.MaxBackoff <= opts.Interval {
		opts.MaxBackoff = opts.Interval * 8
	}
	if opts.Jitter <= 0 {
		opts.Jitter = opts.Interval / 10
	}
	return &Scheduler{
		opts:    opts,
		delay:   opts.Interval,
		trigger: make(chan struct{}, 1),
	}
}

// Start launches the loop. It returns immediately.
func (s *Scheduler) Start(ctx context.Context, run func(context.Context) error) {
	ctx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancel = cancel
	s.closed = false
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.loop(ctx, run)
	}()
}

func (s *Scheduler) loop(ctx context.Context, run func(context.Context) error) {
	for {
		wait := s.nextDelay(true)
		s.setTimer(wait)
		select {
		case <-ctx.Done():
			s.stopTimer()
			return
		case <-s.trigger:
			s.stopTimer()
			s.execute(ctx, run)
		case <-s.timerC():
			s.execute(ctx, run)
		}
	}
}

func (s *Scheduler) execute(ctx context.Context, run func(context.Context) error) {
	s.lastStart.Store(time.Now().UnixNano())
	s.runCount.Add(1)

	runCtx, cancel := context.WithTimeout(ctx, s.opts.Interval)
	defer cancel()

	if err := run(runCtx); err != nil {
		s.errCount.Add(1)
		s.lastErr.Store(err.Error())
		// Grow delay (capped) so we stop hammering on persistent failures.
		d := s.delay * 2
		if d > s.opts.MaxBackoff {
			d = s.opts.MaxBackoff
		}
		s.mu.Lock()
		s.delay = d
		s.mu.Unlock()
	} else {
		s.mu.Lock()
		s.delay = s.opts.Interval
		s.mu.Unlock()
	}
}

// Trigger asks for an immediate run. It never blocks and coalesces requests.
func (s *Scheduler) Trigger() {
	select {
	case s.trigger <- struct{}{}:
	default:
	}
}

// Stop halts the loop and waits for it to finish.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.cancel()
	s.mu.Unlock()
	s.wg.Wait()
}

// RunCount returns how many times the job executed.
func (s *Scheduler) RunCount() int64 { return s.runCount.Load() }

// ErrCount returns how many executions errored.
func (s *Scheduler) ErrCount() int64 { return s.errCount.Load() }

// LastErr returns the last job error ("" if none).
func (s *Scheduler) LastErr() string {
	if v, ok := s.lastErr.Load().(string); ok {
		return v
	}
	return ""
}

func (s *Scheduler) nextDelay(resetJitter bool) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.delay
	if resetJitter {
		d += jitter(s.opts.Jitter)
	}
	return d
}

func (s *Scheduler) setTimer(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next = time.Now().Add(d)
	if s.timer == nil {
		s.timer = time.NewTimer(d)
	} else {
		s.stopTimerLocked()
		s.timer = time.NewTimer(d)
	}
}

func (s *Scheduler) timerC() <-chan time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer == nil {
		return nil
	}
	return s.timer.C
}

func (s *Scheduler) stopTimer() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopTimerLocked()
}

func (s *Scheduler) stopTimerLocked() {
	if s.timer != nil {
		if !s.timer.Stop() {
			select {
			case <-s.timer.C:
			default:
			}
		}
		s.timer = nil
	}
}

func jitter(amount time.Duration) time.Duration {
	if amount <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(amount*2))) - amount
}

package limiter

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestGradientLimiter_BasicAcquireAndDone(t *testing.T) {
	cfg := DefaultConfig()
	cfg.InitialLimit = 10
	l := NewGradientLimiter(cfg)

	tok, ok := l.Acquire(context.Background(), PriorityStandard)
	if !ok {
		t.Fatalf("expected acquire to succeed")
	}

	if l.InFlight() != 1 {
		t.Fatalf("expected inFlight to be 1, got %d", l.InFlight())
	}

	tok.Done(10*time.Millisecond, false)

	if l.InFlight() != 0 {
		t.Fatalf("expected inFlight to be 0 after Done, got %d", l.InFlight())
	}
}

func TestGradientLimiter_SheddingUnderSaturation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.InitialLimit = 5
	cfg.MinLimit = 2
	cfg.StandardThreshold = 1.0 // allow standard up to full limit for this test
	l := NewGradientLimiter(cfg)

	tokens := make([]Token, 0, 5)
	for i := 0; i < 5; i++ {
		tok, ok := l.Acquire(context.Background(), PriorityStandard)
		if !ok {
			t.Fatalf("expected acquire #%d to succeed", i)
		}
		tokens = append(tokens, tok)
	}

	// 6th acquire should be shed immediately
	_, ok := l.Acquire(context.Background(), PriorityStandard)
	if ok {
		t.Fatalf("expected 6th acquire to be shed, but it succeeded")
	}

	// Release one token
	tokens[0].Done(10*time.Millisecond, false)

	// Now another acquire should succeed
	tokNew, okNew := l.Acquire(context.Background(), PriorityStandard)
	if !okNew {
		t.Fatalf("expected acquire to succeed after slot released")
	}
	tokNew.Release()

	// Clean up remaining
	for _, tok := range tokens[1:] {
		tok.Release()
	}
}

func TestGradientLimiter_PriorityTieredShedding(t *testing.T) {
	cfg := DefaultConfig()
	cfg.InitialLimit = 100
	cfg.StandardThreshold = 0.80
	cfg.BestEffortThreshold = 0.50
	l := NewGradientLimiter(cfg)

	// Hold 60 tokens
	tokens := make([]Token, 0, 60)
	for i := 0; i < 60; i++ {
		tok, ok := l.Acquire(context.Background(), PriorityCritical)
		if !ok {
			t.Fatalf("failed acquiring critical token #%d", i)
		}
		tokens = append(tokens, tok)
	}

	// At 60 in-flight (> 50% of 100):
	// Best-effort should be shed
	_, okBest := l.Acquire(context.Background(), PriorityBestEffort)
	if okBest {
		t.Fatalf("expected best-effort to be shed at 60%% capacity")
	}

	// Standard should still be accepted (threshold is 80%)
	tokStd, okStd := l.Acquire(context.Background(), PriorityStandard)
	if !okStd {
		t.Fatalf("expected standard to be accepted at 60%% capacity")
	}
	tokStd.Release()

	// Critical should still be accepted
	tokCrit, okCrit := l.Acquire(context.Background(), PriorityCritical)
	if !okCrit {
		t.Fatalf("expected critical to be accepted at 60%% capacity")
	}
	tokCrit.Release()

	for _, tok := range tokens {
		tok.Release()
	}
}

func TestGradientLimiter_DynamicLimitContractionOnLatencySpike(t *testing.T) {
	cfg := DefaultConfig()
	cfg.InitialLimit = 20
	cfg.MinLimit = 5
	cfg.Smoothing = 0.5
	l := NewGradientLimiter(cfg)

	// Establish healthy baseline at 10ms with unqueued requests
	for i := 0; i < 5; i++ {
		tok, ok := l.Acquire(context.Background(), PriorityCritical)
		if !ok {
			t.Fatalf("acquire failed")
		}
		tok.Done(10*time.Millisecond, false)
	}

	initialLimit := l.CurrentLimit()

	// Simulate heavy load testing capacity (> 50% in-flight) with 200ms latency spike
	for round := 0; round < 15; round++ {
		targetInFlight := int(l.CurrentLimit() * 0.75)
		if targetInFlight < 4 {
			targetInFlight = 4
		}

		var active []Token
		for i := 0; i < targetInFlight; i++ {
			if tok, ok := l.Acquire(context.Background(), PriorityCritical); ok {
				active = append(active, tok)
			}
		}

		for _, tok := range active {
			tok.Done(200*time.Millisecond, false)
		}
	}

	contractedLimit := l.CurrentLimit()
	if contractedLimit >= initialLimit {
		t.Fatalf("expected limit to contract under 200ms latency spike (was %.2f, now %.2f)",
			initialLimit, contractedLimit)
	}
}

func TestLimiter_AppLimitedNoGrowth(t *testing.T) {
	cfg := DefaultConfig()
	cfg.InitialLimit = 50
	l := NewGradientLimiter(cfg)

	// 1000 completions at in-flight 1 (well below limit/2) must not grow the limit
	for i := 0; i < 1000; i++ {
		tok, _ := l.Acquire(context.Background(), PriorityCritical)
		tok.Done(10*time.Millisecond, false)
	}

	if l.CurrentLimit() > 50 {
		t.Fatalf("expected limit not to grow when app-limited at in-flight 1, got %.2f", l.CurrentLimit())
	}
}

func TestLimiter_ErrorSamplesDontMoveMinRTT(t *testing.T) {
	cfg := DefaultConfig()
	cfg.InitialLimit = 50
	l := NewGradientLimiter(cfg)

	// Establish healthy baseline at 15ms
	for i := 0; i < 5; i++ {
		tok, _ := l.Acquire(context.Background(), PriorityCritical)
		tok.Done(15*time.Millisecond, false)
	}

	if l.MinRTT() != 15*time.Millisecond {
		t.Fatalf("expected baseline 15ms, got %v", l.MinRTT())
	}

	// Fast 500 error in 100 microseconds
	tokErr, _ := l.Acquire(context.Background(), PriorityCritical)
	tokErr.Done(100*time.Microsecond, true)

	// Baseline MUST NOT be poisoned by the fast error!
	if l.MinRTT() < 10*time.Millisecond {
		t.Fatalf("error sample poisoned minRTT! was 15ms, now %v", l.MinRTT())
	}
}

func TestLimiter_OutlierBoundedDrop(t *testing.T) {
	cfg := DefaultConfig()
	cfg.InitialLimit = 100
	l := NewGradientLimiter(cfg)

	// Establish baseline at 10ms
	for i := 0; i < 5; i++ {
		tok, _ := l.Acquire(context.Background(), PriorityCritical)
		tok.Done(10*time.Millisecond, false)
	}

	// Hold 60 requests in flight (>50% capacity)
	var active []Token
	for i := 0; i < 60; i++ {
		if tok, ok := l.Acquire(context.Background(), PriorityCritical); ok {
			active = append(active, tok)
		}
	}

	// Release 1 outlier with 300ms (30x baseline)
	active[0].Done(300*time.Millisecond, false)

	// Release remaining 59 with normal 10ms
	for _, tok := range active[1:] {
		tok.Done(10*time.Millisecond, false)
	}

	// With limit smoothing, a single outlier should not crater the limit below 60
	if l.CurrentLimit() < 60 {
		t.Fatalf("outlier collapsed limit too aggressively, got %.2f", l.CurrentLimit())
	}
}

func TestLimiter_ConcurrentAcquireDone(t *testing.T) {
	cfg := DefaultConfig()
	cfg.InitialLimit = 100
	cfg.MaxLimit = 500
	l := NewGradientLimiter(cfg)

	var wg sync.WaitGroup
	workers := 16
	requestsPerWorker := 500

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < requestsPerWorker; i++ {
				tok, ok := l.Acquire(context.Background(), PriorityStandard)
				if ok {
					tok.Done(10*time.Millisecond, false)
				}
			}
		}()
	}

	wg.Wait()

	if l.InFlight() != 0 {
		t.Fatalf("expected inFlight == 0 at end of concurrent test, got %d", l.InFlight())
	}
}

func TestLimiter_StaleDoneIsNoop(t *testing.T) {
	cfg := DefaultConfig()
	cfg.InitialLimit = 10
	l := NewGradientLimiter(cfg)

	tok, ok := l.Acquire(context.Background(), PriorityStandard)
	if !ok {
		t.Fatalf("expected acquire to succeed")
	}

	if l.InFlight() != 1 {
		t.Fatalf("expected inFlight == 1, got %d", l.InFlight())
	}

	// First Done() releases the slot
	tok.Done(10*time.Millisecond, false)
	if l.InFlight() != 0 {
		t.Fatalf("expected inFlight == 0, got %d", l.InFlight())
	}

	// Second and third stale Done() calls MUST be no-ops!
	tok.Done(10*time.Millisecond, false)
	tok.Release()

	if l.InFlight() != 0 {
		t.Fatalf("stale Done() decremented in-flight! got %d", l.InFlight())
	}
}

func TestLimiter_SustainedQueueingNoBaselineRise(t *testing.T) {
	currentTime := time.Now()
	clock := func() time.Time { return currentTime }

	cfg := DefaultConfig()
	cfg.InitialLimit = 5
	cfg.MinLimit = 5
	cfg.MinRTTWindow = 10 * time.Second
	cfg.Now = clock
	l := NewGradientLimiter(cfg)

	// Step 1: Establish healthy 15ms baseline
	tok, _ := l.Acquire(context.Background(), PriorityCritical)
	tok.Done(15*time.Millisecond, false)

	if l.MinRTT() != 15*time.Millisecond {
		t.Fatalf("expected baseline 15ms, got %v", l.MinRTT())
	}

	// Step 2: 3 windows of heavy queuing at limit=5 with 4 in flight (80%) and 150ms latency
	for window := 0; window < 3; window++ {
		currentTime = currentTime.Add(11 * time.Second) // Advance clock past window

		var active []Token
		for i := 0; i < 4; i++ {
			t, _ := l.Acquire(context.Background(), PriorityCritical)
			active = append(active, t)
		}

		for _, t := range active {
			t.Done(150*time.Millisecond, false)
		}
	}

	// Baseline must NOT have ratcheted up to 150ms!
	if l.MinRTT() > 35*time.Millisecond {
		t.Fatalf("sustained queuing at floor ratcheted baseline! was 15ms, now %v", l.MinRTT())
	}
}

func TestLimiter_BaselineShiftRecoversWhenSaturated(t *testing.T) {
	currentTime := time.Now()
	clock := func() time.Time { return currentTime }

	cfg := DefaultConfig()
	cfg.InitialLimit = 20
	cfg.MinLimit = 5
	cfg.MinRTTWindow = 10 * time.Second
	cfg.Now = clock
	l := NewGradientLimiter(cfg)

	// Initial baseline 15ms
	tok, _ := l.Acquire(context.Background(), PriorityCritical)
	tok.Done(15*time.Millisecond, false)

	// Saturated traffic at 90% load where backend has permanently moved to 25ms
	for window := 0; window < 4; window++ {
		currentTime = currentTime.Add(11 * time.Second)

		var active []Token
		for i := 0; i < 18; i++ {
			t, ok := l.Acquire(context.Background(), PriorityCritical)
			if ok {
				active = append(active, t)
			}
		}

		for _, t := range active {
			t.Done(25*time.Millisecond, false)
		}
	}

	// Fallback candidate must allow baseline to adapt toward 25ms rather than staying frozen at 15ms
	if l.MinRTT() < 20*time.Millisecond {
		t.Fatalf("expected baseline to adapt toward permanent 25ms shift, got %v", l.MinRTT())
	}
}

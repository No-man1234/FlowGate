package limiter

import (
	"context"
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
	cfg.InitialLimit = 50
	cfg.MinLimit = 5
	cfg.Smoothing = 0.5
	l := NewGradientLimiter(cfg)

	// Establish healthy baseline at 10ms
	for i := 0; i < 10; i++ {
		tok, ok := l.Acquire(context.Background(), PriorityCritical)
		if !ok {
			t.Fatalf("acquire failed")
		}
		tok.Done(10*time.Millisecond, false)
	}

	initialHealthyLimit := l.CurrentLimit()

	// Simulate sudden upstream queueing: latency spikes from 10ms to 200ms
	for i := 0; i < 20; i++ {
		tok, ok := l.Acquire(context.Background(), PriorityCritical)
		if !ok {
			continue // May start shedding
		}
		tok.Done(200*time.Millisecond, false)
	}

	contractedLimit := l.CurrentLimit()
	if contractedLimit >= initialHealthyLimit {
		t.Fatalf("expected limit to contract under 200ms latency spike (was %.2f, now %.2f)",
			initialHealthyLimit, contractedLimit)
	}
}

func TestGradientLimiter_SevereErrorBackoff(t *testing.T) {
	cfg := DefaultConfig()
	cfg.InitialLimit = 100
	cfg.BackoffRatio = 0.5
	l := NewGradientLimiter(cfg)

	tok, ok := l.Acquire(context.Background(), PriorityCritical)
	if !ok {
		t.Fatalf("acquire failed")
	}

	// Report severe error (e.g., 504 Gateway Timeout)
	tok.Done(5*time.Second, true)

	if l.CurrentLimit() > 55 {
		t.Fatalf("expected limit to be reduced by backoff ratio (~50), got %.2f", l.CurrentLimit())
	}
}

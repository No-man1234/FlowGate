package breaker

import (
	"testing"
	"time"
)

func TestCircuitBreaker_StateTransitions(t *testing.T) {
	cfg := Config{
		FailureThreshold:    3,
		CoolOff:             50 * time.Millisecond,
		HalfOpenMaxRequests: 2,
		SuccessThreshold:    2,
	}
	cb := New(cfg)

	if cb.State() != StateClosed {
		t.Fatalf("expected initial state CLOSED, got %s", cb.State())
	}

	// 2 failures - still closed
	_, gen := cb.Allow()
	cb.RecordFailure(gen)
	cb.RecordFailure(gen)
	if cb.State() != StateClosed {
		t.Fatalf("expected state CLOSED after 2 failures, got %s", cb.State())
	}

	// 3rd failure trips the breaker
	cb.RecordFailure(gen)
	if cb.State() != StateOpen {
		t.Fatalf("expected state OPEN after 3 failures, got %s", cb.State())
	}

	allowed, _ := cb.Allow()
	if allowed {
		t.Fatalf("expected Allow() to be false when OPEN")
	}

	// Wait for cool-off
	time.Sleep(60 * time.Millisecond)

	// First request after cool-off should transition to HALF-OPEN
	allowed, genProbe := cb.Allow()
	if !allowed {
		t.Fatalf("expected Allow() to return true after cool-off (probe)")
	}
	if cb.State() != StateHalfOpen {
		t.Fatalf("expected state HALF-OPEN, got %s", cb.State())
	}

	// First success in half-open
	cb.RecordSuccess(genProbe)
	if cb.State() != StateHalfOpen {
		t.Fatalf("expected state still HALF-OPEN after 1 success, got %s", cb.State())
	}

	// Second success reaches SuccessThreshold -> resets to CLOSED
	cb.RecordSuccess(genProbe)
	if cb.State() != StateClosed {
		t.Fatalf("expected state CLOSED after 2 successes in half-open, got %s", cb.State())
	}
}

func TestCircuitBreaker_CancelReleasesHalfOpenSlot(t *testing.T) {
	cfg := Config{
		FailureThreshold:    1,
		CoolOff:             20 * time.Millisecond,
		HalfOpenMaxRequests: 1,
		SuccessThreshold:    1,
	}
	cb := New(cfg)

	_, gen := cb.Allow()
	cb.RecordFailure(gen) // Trips open

	time.Sleep(30 * time.Millisecond)

	// Transition to HalfOpen (occupies 1/1 slot)
	allowed, genProbe := cb.Allow()
	if !allowed {
		t.Fatalf("expected probe allowed")
	}

	// Next allow must fail (slot occupied)
	allowed2, _ := cb.Allow()
	if allowed2 {
		t.Fatalf("expected second allow to fail when max requests reached")
	}

	// Cancel the probe (e.g. shed or client disconnect)
	cb.Cancel(genProbe)

	// Slot should be free now!
	allowed3, genProbe3 := cb.Allow()
	if !allowed3 {
		t.Fatalf("expected probe allowed after cancel")
	}

	cb.RecordSuccess(genProbe3)
	if cb.State() != StateClosed {
		t.Fatalf("expected circuit to close after successful probe")
	}
}

func TestCircuitBreaker_StragglerCannotCloseHalfOpen(t *testing.T) {
	cfg := Config{
		FailureThreshold:    1,
		CoolOff:             20 * time.Millisecond,
		HalfOpenMaxRequests: 1,
		SuccessThreshold:    1,
	}
	cb := New(cfg)

	_, oldGen := cb.Allow() // Gen 1 request admitted

	cb.RecordFailure(oldGen) // Trips open (now Gen 2)

	time.Sleep(30 * time.Millisecond)

	_, probeGen := cb.Allow() // Transition to HalfOpen (now Gen 3)
	if cb.State() != StateHalfOpen {
		t.Fatalf("expected state HALF-OPEN")
	}

	// An old straggler from Gen 1 finishes now with success
	cb.RecordSuccess(oldGen)

	// It must NOT close the breaker!
	if cb.State() != StateHalfOpen {
		t.Fatalf("straggler closed the circuit prematurely! expected HALF-OPEN, got %s", cb.State())
	}

	// The actual probe finishes
	cb.RecordSuccess(probeGen)
	if cb.State() != StateClosed {
		t.Fatalf("expected state CLOSED after valid probe success")
	}
}

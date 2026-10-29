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

	// Initially closed
	if cb.State() != StateClosed {
		t.Fatalf("expected initial state CLOSED, got %s", cb.State())
	}

	// 2 failures - still closed
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != StateClosed {
		t.Fatalf("expected state CLOSED after 2 failures, got %s", cb.State())
	}
	if !cb.Allow() {
		t.Fatalf("expected Allow() to be true when closed")
	}

	// 3rd failure trips the breaker
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Fatalf("expected state OPEN after 3 failures, got %s", cb.State())
	}
	if cb.Allow() {
		t.Fatalf("expected Allow() to be false when OPEN")
	}

	// Wait for cool-off
	time.Sleep(60 * time.Millisecond)

	// First request after cool-off should transition to HALF-OPEN
	if !cb.Allow() {
		t.Fatalf("expected Allow() to return true after cool-off (probe)")
	}
	if cb.State() != StateHalfOpen {
		t.Fatalf("expected state HALF-OPEN, got %s", cb.State())
	}

	// First success in half-open
	cb.RecordSuccess()
	if cb.State() != StateHalfOpen {
		t.Fatalf("expected state still HALF-OPEN after 1 success, got %s", cb.State())
	}

	// Second success reaches SuccessThreshold -> resets to CLOSED
	cb.RecordSuccess()
	if cb.State() != StateClosed {
		t.Fatalf("expected state CLOSED after 2 successes in half-open, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenFailureTripsImmediately(t *testing.T) {
	cfg := Config{
		FailureThreshold:    2,
		CoolOff:             20 * time.Millisecond,
		HalfOpenMaxRequests: 1,
		SuccessThreshold:    2,
	}
	cb := New(cfg)

	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Fatalf("expected state OPEN")
	}

	time.Sleep(30 * time.Millisecond)

	if !cb.Allow() {
		t.Fatalf("expected probe allowed")
	}

	// Failure during probe immediately re-trips
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Fatalf("expected state OPEN immediately after probe failure")
	}
	if cb.Allow() {
		t.Fatalf("expected Allow() to return false when re-tripped")
	}
}

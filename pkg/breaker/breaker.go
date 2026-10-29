package breaker

import (
	"sync"
	"time"
)

// State represents the current lifecycle phase of the circuit breaker.
type State int

const (
	// StateClosed indicates healthy operation; all requests pass through.
	StateClosed State = iota

	// StateOpen indicates severe upstream failure; requests are failed fast (<1ms).
	StateOpen

	// StateHalfOpen indicates recovery probe phase; limited test requests are permitted.
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF-OPEN"
	default:
		return "UNKNOWN"
	}
}

// Config configures the circuit breaker behavior.
type Config struct {
	FailureThreshold    int           // Number of consecutive failures to trip open (default: 5)
	CoolOff             time.Duration // Time to wait before attempting half-open probe (default: 5s)
	HalfOpenMaxRequests int           // Maximum test requests permitted in half-open state (default: 3)
	SuccessThreshold    int           // Consecutive successes in half-open state required to close (default: 2)
}

// DefaultConfig provides sensible circuit breaker defaults.
func DefaultConfig() Config {
	return Config{
		FailureThreshold:    5,
		CoolOff:             5 * time.Second,
		HalfOpenMaxRequests: 3,
		SuccessThreshold:    2,
	}
}

// CircuitBreaker guards upstream services from cascading failure loops.
type CircuitBreaker struct {
	cfg Config

	mu               sync.Mutex
	state            State
	consecutiveFails int
	consecutiveSucc  int
	halfOpenInFlight int
	lastTripTime     time.Time
}

// New creates a new CircuitBreaker.
func New(cfg Config) *CircuitBreaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.CoolOff <= 0 {
		cfg.CoolOff = 5 * time.Second
	}
	if cfg.HalfOpenMaxRequests <= 0 {
		cfg.HalfOpenMaxRequests = 3
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = 2
	}

	return &CircuitBreaker{
		cfg:   cfg,
		state: StateClosed,
	}
}

// Allow reports whether a request is allowed to proceed to upstream.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	switch cb.state {
	case StateClosed:
		return true

	case StateOpen:
		// Check if cool-off period has elapsed to transition to half-open
		if now.Sub(cb.lastTripTime) >= cb.cfg.CoolOff {
			cb.state = StateHalfOpen
			cb.halfOpenInFlight = 1
			cb.consecutiveSucc = 0
			return true
		}
		return false

	case StateHalfOpen:
		// Allow up to HalfOpenMaxRequests concurrently
		if cb.halfOpenInFlight < cb.cfg.HalfOpenMaxRequests {
			cb.halfOpenInFlight++
			return true
		}
		return false
	}

	return true
}

// RecordSuccess records a successful upstream interaction.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		cb.consecutiveFails = 0

	case StateHalfOpen:
		if cb.halfOpenInFlight > 0 {
			cb.halfOpenInFlight--
		}
		cb.consecutiveSucc++
		if cb.consecutiveSucc >= cb.cfg.SuccessThreshold {
			// Successfully verified upstream recovery!
			cb.state = StateClosed
			cb.consecutiveFails = 0
			cb.consecutiveSucc = 0
			cb.halfOpenInFlight = 0
		}
	}
}

// RecordFailure records an upstream failure (e.g. 502 Bad Gateway, 503, connection refused).
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		cb.consecutiveFails++
		if cb.consecutiveFails >= cb.cfg.FailureThreshold {
			// Trip the breaker
			cb.state = StateOpen
			cb.lastTripTime = time.Now()
			cb.consecutiveFails = 0
			cb.consecutiveSucc = 0
		}

	case StateHalfOpen:
		// Any failure during probing immediately re-trips the breaker
		if cb.halfOpenInFlight > 0 {
			cb.halfOpenInFlight--
		}
		cb.state = StateOpen
		cb.lastTripTime = time.Now()
		cb.consecutiveSucc = 0
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

package limiter

import (
	"context"
	"time"
)

// Priority represents the classification tier of an incoming request.
// In overload conditions, lower priorities are shed first to protect higher tiers.
type Priority int

const (
	// PriorityCritical is for essential business operations (e.g., checkout, payments, auth).
	// Dropped only in catastrophic scenarios.
	PriorityCritical Priority = 1

	// PriorityStandard is for normal user-facing requests (e.g., feed, browse, profile).
	PriorityStandard Priority = 2

	// PriorityBestEffort is for background or non-critical tasks (e.g., analytics, webhooks, prefetching).
	// First to be shed under load.
	PriorityBestEffort Priority = 3
)

// String returns the human-readable name of the priority tier.
func (p Priority) String() string {
	switch p {
	case PriorityCritical:
		return "critical"
	case PriorityStandard:
		return "standard"
	case PriorityBestEffort:
		return "best-effort"
	default:
		return "standard"
	}
}

// ParsePriority converts a header string into a Priority enum.
func ParsePriority(s string) Priority {
	switch s {
	case "critical", "1", "high":
		return PriorityCritical
	case "best-effort", "3", "low", "background":
		return PriorityBestEffort
	default:
		return PriorityStandard
	}
}

// Token represents permission granted to execute a request through the gateway.
// When execution finishes, the caller MUST invoke Done() to provide latency feedback
// and release the concurrency slot.
type Token interface {
	// Done marks the request as finished, records RTT, and updates the dynamic limit.
	Done(rtt time.Duration, isError bool)

	// Release releases the token without updating latency metrics (e.g., if dropped before upstream call).
	Release()
}

// Limiter defines the behavior of an adaptive concurrency limiter.
type Limiter interface {
	// Acquire attempts to acquire a token for the given priority tier.
	// Returns (token, true) if accepted, or (nil, false) if shed.
	Acquire(ctx context.Context, priority Priority) (Token, bool)

	// CurrentLimit returns the current dynamically calculated concurrency ceiling.
	CurrentLimit() float64

	// InFlight returns the current number of concurrent requests executing.
	InFlight() int64

	// Stats returns internal telemetry snapshots.
	Stats() Snapshot
}

// Snapshot contains an instantaneous snapshot of limiter metrics.
type Snapshot struct {
	InFlight     int64
	Limit        float64
	MinRTT       time.Duration
	SampleRTT    time.Duration
	Accepted     uint64
	ShedCritical uint64
	ShedStandard uint64
	ShedBest     uint64
}

package limiter

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// GradientConfig controls the behavior of the Gradient adaptive limiter.
type GradientConfig struct {
	InitialLimit float64       // Initial concurrency limit on boot (default: 50)
	MinLimit     float64       // Lower bound for concurrency limit (default: 5)
	MaxLimit     float64       // Upper bound for concurrency limit (default: 10000)
	Smoothing    float64       // EWMA smoothing factor alpha for sampleRTT (0.0 to 1.0, default: 0.2)
	Tolerance    float64       // Tolerance factor for gradient before throttling (default: 1.25, allows 25% jitter)
	BackoffRatio float64       // Multiplicative reduction upon severe timeout/5xx (default: 0.75)
	MinRTTWindow time.Duration // Window to refresh minRTT baseline to adapt to infra shifts (default: 30s)

	// Thresholds for priority tiers (percentage of current limit)
	StandardThreshold   float64 // default: 0.85 (sheds standard when in-flight >= 85% limit)
	BestEffortThreshold float64 // default: 0.70 (sheds best-effort when in-flight >= 70% limit)
}

// DefaultConfig provides recommended production defaults.
func DefaultConfig() GradientConfig {
	return GradientConfig{
		InitialLimit:        50,
		MinLimit:            5,
		MaxLimit:            10000,
		Smoothing:           0.2,
		Tolerance:           1.25,
		BackoffRatio:        0.75,
		MinRTTWindow:        30 * time.Second,
		StandardThreshold:   0.85,
		BestEffortThreshold: 0.70,
	}
}

// GradientLimiter is an adaptive concurrency limiter using a Gradient-style
// controller with a windowed min-RTT baseline, anti-ratchet queuing filter, and limit smoothing.
type GradientLimiter struct {
	cfg GradientConfig

	inFlight atomic.Int64

	// Lock-free metrics counters
	accepted     atomic.Uint64
	shedCritical atomic.Uint64
	shedStandard atomic.Uint64
	shedBest     atomic.Uint64

	// Concurrency state protected by mu
	mu              sync.RWMutex
	currentLimit    float64
	minRTT          time.Duration
	sampleRTT       time.Duration
	candidateMinRTT time.Duration
	windowStart     time.Time
}

var tokenPool = sync.Pool{
	New: func() any {
		return &gradientToken{}
	},
}

// NewGradientLimiter creates a configured GradientLimiter with validated parameters.
func NewGradientLimiter(cfg GradientConfig) *GradientLimiter {
	if cfg.MinLimit <= 0 {
		cfg.MinLimit = 5
	}
	if cfg.MaxLimit < cfg.MinLimit {
		cfg.MaxLimit = 10000
	}
	if cfg.InitialLimit < cfg.MinLimit {
		cfg.InitialLimit = cfg.MinLimit
	}
	if cfg.InitialLimit > cfg.MaxLimit {
		cfg.InitialLimit = cfg.MaxLimit
	}
	if cfg.Smoothing <= 0 || cfg.Smoothing > 1.0 {
		cfg.Smoothing = 0.2
	}
	if cfg.Tolerance <= 0 {
		cfg.Tolerance = 1.25
	}
	if cfg.BackoffRatio <= 0 || cfg.BackoffRatio >= 1.0 {
		cfg.BackoffRatio = 0.75
	}
	if cfg.MinRTTWindow <= 0 {
		cfg.MinRTTWindow = 30 * time.Second
	}
	if cfg.StandardThreshold <= 0 {
		cfg.StandardThreshold = 0.85
	}
	if cfg.BestEffortThreshold <= 0 {
		cfg.BestEffortThreshold = 0.70
	}

	return &GradientLimiter{
		cfg:          cfg,
		currentLimit: cfg.InitialLimit,
		windowStart:  time.Now(),
	}
}

// Acquire tests whether a request of the specified priority can be admitted.
func (g *GradientLimiter) Acquire(ctx context.Context, priority Priority) (Token, bool) {
	if ctx != nil && ctx.Err() != nil {
		return nil, false
	}

	g.mu.RLock()
	limit := g.currentLimit
	g.mu.RUnlock()

	var maxAllowed float64
	switch priority {
	case PriorityCritical:
		maxAllowed = limit
	case PriorityStandard:
		maxAllowed = limit * g.cfg.StandardThreshold
	case PriorityBestEffort:
		maxAllowed = limit * g.cfg.BestEffortThreshold
	default:
		maxAllowed = limit * g.cfg.StandardThreshold
	}

	if maxAllowed < g.cfg.MinLimit {
		maxAllowed = g.cfg.MinLimit
	}

	currentInFlight := g.inFlight.Add(1)

	// If over allowed threshold, fast-fail shed (0 allocs, 61ns)
	if float64(currentInFlight) > maxAllowed {
		g.inFlight.Add(-1)
		switch priority {
		case PriorityCritical:
			g.shedCritical.Add(1)
		case PriorityStandard:
			g.shedStandard.Add(1)
		case PriorityBestEffort:
			g.shedBest.Add(1)
		}
		return nil, false
	}

	g.accepted.Add(1)
	tok := tokenPool.Get().(*gradientToken)
	tok.limiter = g
	tok.done = 0
	return tok, true
}

// onDone updates the limiter's RTT tracking and adjusts the dynamic limit.
func (g *GradientLimiter) onDone(rtt time.Duration, isError bool) {
	inFlightBefore := g.inFlight.Add(-1)
	inFlight := inFlightBefore + 1 // in-flight count while request was executing

	if rtt <= 0 {
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// Rule 1: Errors trigger immediate backoff and MUST NOT poison RTT state or baseline!
	if isError {
		g.currentLimit = math.Max(g.cfg.MinLimit, g.currentLimit*g.cfg.BackoffRatio)
		return
	}

	now := time.Now()

	// Anti-ratchet guard:
	// Only update candidateMinRTT from samples taken when the system is NOT queuing.
	// If in-flight was well below limit (e.g. <= 50% capacity or <= 5 in-flight),
	// this is a valid measurement of unqueued service time.
	isUnqueuedSample := float64(inFlight) <= g.currentLimit*0.5 || inFlight <= 5
	if isUnqueuedSample {
		if g.candidateMinRTT == 0 || rtt < g.candidateMinRTT {
			g.candidateMinRTT = rtt
		}
	}

	if now.Sub(g.windowStart) >= g.cfg.MinRTTWindow || g.minRTT == 0 {
		if g.candidateMinRTT > 0 {
			g.minRTT = g.candidateMinRTT
		} else if g.minRTT == 0 {
			g.minRTT = rtt
		}
		// Reset candidate for next window
		g.candidateMinRTT = 0
		g.windowStart = now
	} else if rtt < g.minRTT {
		g.minRTT = rtt
	}

	// Update Exponentially Weighted Moving Average (EWMA) of sample RTT
	if g.sampleRTT == 0 {
		g.sampleRTT = rtt
	} else {
		alpha := g.cfg.Smoothing
		g.sampleRTT = time.Duration(float64(g.sampleRTT)*(1.0-alpha) + float64(rtt)*alpha)
	}

	if g.sampleRTT <= 0 || g.minRTT <= 0 {
		return
	}

	// App-Limited Guard:
	// When in-flight is below half capacity, the system is client/app-limited, not saturated.
	// We skip limit updates to prevent idle periods from inflating the limit to infinity.
	if float64(inFlight) < g.currentLimit*0.5 {
		return
	}

	// Gradient calculation:
	// rawGradient = (minRTT / sampleRTT) * tolerance
	// gradient = clamp(rawGradient, 0.5, 1.0)
	rawGradient := (float64(g.minRTT) / float64(g.sampleRTT)) * g.cfg.Tolerance
	gradient := math.Max(0.5, math.Min(1.0, rawGradient))

	// Queue headroom probe factor: sqrt(currentLimit)
	probe := math.Sqrt(g.currentLimit)

	targetLimit := g.currentLimit*gradient + probe

	// Limit smoothing (EWMA with alpha=0.2): prevents single outlier from collapsing limit
	smoothedLimit := g.currentLimit*0.8 + targetLimit*0.2
	g.currentLimit = math.Max(g.cfg.MinLimit, math.Min(g.cfg.MaxLimit, smoothedLimit))
}

func (g *GradientLimiter) onRelease() {
	g.inFlight.Add(-1)
}

func (g *GradientLimiter) CurrentLimit() float64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.currentLimit
}

func (g *GradientLimiter) InFlight() int64 {
	return g.inFlight.Load()
}

func (g *GradientLimiter) MinRTT() time.Duration {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.minRTT
}

func (g *GradientLimiter) Stats() Snapshot {
	g.mu.RLock()
	limit := g.currentLimit
	minRTT := g.minRTT
	sampleRTT := g.sampleRTT
	g.mu.RUnlock()

	return Snapshot{
		InFlight:     g.inFlight.Load(),
		Limit:        limit,
		MinRTT:       minRTT,
		SampleRTT:    sampleRTT,
		Accepted:     g.accepted.Load(),
		ShedCritical: g.shedCritical.Load(),
		ShedStandard: g.shedStandard.Load(),
		ShedBest:     g.shedBest.Load(),
	}
}

type gradientToken struct {
	limiter *GradientLimiter
	done    uint32
}

func (t *gradientToken) Done(rtt time.Duration, isError bool) {
	if atomic.CompareAndSwapUint32(&t.done, 0, 1) {
		t.limiter.onDone(rtt, isError)
		tokenPool.Put(t)
	}
}

func (t *gradientToken) Release() {
	if atomic.CompareAndSwapUint32(&t.done, 0, 1) {
		t.limiter.onRelease()
		tokenPool.Put(t)
	}
}

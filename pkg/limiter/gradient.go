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

// GradientLimiter is an adaptive concurrency limiter based on Netflix's Gradient2 algorithm.
// It dynamically tracks RTT drift to detect queuing and prevent upstream collapse.
type GradientLimiter struct {
	cfg GradientConfig

	inFlight atomic.Int64

	// Metrics counters
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

// NewGradientLimiter creates a configured GradientLimiter.
func NewGradientLimiter(cfg GradientConfig) *GradientLimiter {
	if cfg.InitialLimit <= 0 {
		cfg.InitialLimit = 50
	}
	if cfg.MinLimit <= 0 {
		cfg.MinLimit = 5
	}
	if cfg.MaxLimit < cfg.MinLimit {
		cfg.MaxLimit = 10000
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

	// Determine allowed capacity threshold based on priority tier
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

	// Optimistically increment in-flight count
	currentInFlight := g.inFlight.Add(1)

	// If over the allowed threshold, reject immediately (shed load)
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
	return &gradientToken{
		limiter: g,
	}, true
}

// onDone updates the limiter's RTT tracking and adjusts the dynamic limit.
func (g *GradientLimiter) onDone(rtt time.Duration, isError bool) {
	defer g.inFlight.Add(-1)

	if rtt <= 0 {
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()

	// Windowed minRTT tracking with candidate window:
	// Avoids setting minRTT to an anomalous spike if the reset happens during a brownout.
	if g.candidateMinRTT == 0 || rtt < g.candidateMinRTT {
		g.candidateMinRTT = rtt
	}

	if now.Sub(g.windowStart) >= g.cfg.MinRTTWindow || g.minRTT == 0 {
		if g.candidateMinRTT > 0 {
			g.minRTT = g.candidateMinRTT
		} else {
			g.minRTT = rtt
		}
		g.candidateMinRTT = rtt
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

	// Severe errors trigger aggressive multiplicative backoff
	if isError {
		g.currentLimit = math.Max(g.cfg.MinLimit, g.currentLimit*g.cfg.BackoffRatio)
		return
	}

	// Netflix Gradient2 algorithm:
	// rawGradient = (minRTT / sampleRTT) * tolerance
	// gradient = clamp(rawGradient, 0.5, 1.0)
	if g.sampleRTT <= 0 || g.minRTT <= 0 {
		return
	}

	rawGradient := (float64(g.minRTT) / float64(g.sampleRTT)) * g.cfg.Tolerance
	gradient := math.Max(0.5, math.Min(1.0, rawGradient))

	// Queue headroom probe factor: beta = sqrt(currentLimit)
	queueSize := math.Sqrt(g.currentLimit)

	// New Limit = Current * Gradient + QueueSize
	newLimit := g.currentLimit*gradient + queueSize

	// Bound between MinLimit and MaxLimit
	newLimit = math.Max(g.cfg.MinLimit, math.Min(g.cfg.MaxLimit, newLimit))
	g.currentLimit = newLimit
}

// onRelease decrements in-flight without affecting RTT calculations.
func (g *GradientLimiter) onRelease() {
	g.inFlight.Add(-1)
}

// CurrentLimit returns the current dynamic concurrency ceiling.
func (g *GradientLimiter) CurrentLimit() float64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.currentLimit
}

// InFlight returns the number of active requests currently executing.
func (g *GradientLimiter) InFlight() int64 {
	return g.inFlight.Load()
}

// Stats returns a thread-safe snapshot of limiter state.
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
	once    sync.Once
}

func (t *gradientToken) Done(rtt time.Duration, isError bool) {
	t.once.Do(func() {
		t.limiter.onDone(rtt, isError)
	})
}

func (t *gradientToken) Release() {
	t.once.Do(func() {
		t.limiter.onRelease()
	})
}

package metrics

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// Collector records real-time traffic statistics using lock-free atomic counters.
type Collector struct {
	TotalRequests    atomic.Uint64
	AcceptedRequests atomic.Uint64
	ShedRequests     atomic.Uint64
	BreakerTrips     atomic.Uint64
	UpstreamErrors   atomic.Uint64

	// Rates calculated on ticker
	lastTotal    uint64
	lastAccepted uint64
	lastShed     uint64
	lastTick     time.Time

	currentRPS     float64
	currentShedRPS float64
}

// NewCollector creates an initialized metrics collector.
func NewCollector() *Collector {
	return &Collector{
		lastTick: time.Now(),
	}
}

// Tick recalculates short-term rates. Should be called periodically (e.g. every 1s).
func (c *Collector) Tick() {
	now := time.Now()
	elapsed := now.Sub(c.lastTick).Seconds()
	if elapsed <= 0 {
		return
	}

	total := c.TotalRequests.Load()
	shed := c.ShedRequests.Load()

	deltaTotal := total - c.lastTotal
	deltaShed := shed - c.lastShed

	c.currentRPS = float64(deltaTotal) / elapsed
	c.currentShedRPS = float64(deltaShed) / elapsed

	c.lastTotal = total
	c.lastShed = shed
	c.lastTick = now
}

// CurrentRPS returns the calculated requests per second across the last tick window.
func (c *Collector) CurrentRPS() float64 {
	return c.currentRPS
}

// CurrentShedRPS returns the calculated shed requests per second across the last tick window.
func (c *Collector) CurrentShedRPS() float64 {
	return c.currentShedRPS
}

// Summary returns an instantaneous snapshot of system metrics.
type Summary struct {
	TotalRequests   uint64  `json:"total_requests"`
	Accepted        uint64  `json:"accepted_requests"`
	Shed            uint64  `json:"shed_requests"`
	BreakerTrips    uint64  `json:"circuit_breaker_trips"`
	UpstreamErrors  uint64  `json:"upstream_errors"`
	RequestsPerSec  float64 `json:"requests_per_sec"`
	ShedPerSec      float64 `json:"shed_per_sec"`
	InFlight        int64   `json:"in_flight"`
	Limit           float64 `json:"current_limit"`
	MinRTTMs        float64 `json:"min_rtt_ms"`
	SampleRTTMs     float64 `json:"sample_rtt_ms"`
	CircuitState    string  `json:"circuit_state"`
}

// ServeHTTP handles the /metrics JSON endpoint.
func (c *Collector) Handler(getSnapshot func() Summary) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		snap := getSnapshot()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(snap)
	}
}

// FormatDashboard returns a clean ANSI terminal dashboard string.
func FormatDashboard(s Summary) string {
	var healthColor string
	if s.CircuitState == "CLOSED" {
		healthColor = "HEALTHY [CLOSED]"
	} else if s.CircuitState == "HALF-OPEN" {
		healthColor = "PROBING [HALF-OPEN]"
	} else {
		healthColor = "TRIPPED [OPEN]"
	}

	shedPercent := 0.0
	if s.TotalRequests > 0 {
		shedPercent = (float64(s.Shed) / float64(s.TotalRequests)) * 100.0
	}

	return fmt.Sprintf(
		"======================================================================\n"+
			" [FlowGate] Resilient Gateway Status                           \n"+
			"======================================================================\n"+
			" Upstream Circuit : %-20s  In-Flight: %d / %.0f (Limit)\n"+
			" Traffic Inflow   : %-10.1f req/sec        Min RTT  : %.2f ms\n"+
			" Shedding Rate    : %-10.1f req/sec        SampleRTT: %.2f ms\n"+
			" Total Handled    : %-10d                Total Shed: %d (%.1f%%)\n"+
			" Upstream 5xx     : %-10d                Breaker Trips: %d\n"+
			"======================================================================\n",
		healthColor, s.InFlight, s.Limit,
		s.RequestsPerSec, s.MinRTTMs,
		s.ShedPerSec, s.SampleRTTMs,
		s.TotalRequests, s.Shed, shedPercent,
		s.UpstreamErrors, s.BreakerTrips,
	)
}

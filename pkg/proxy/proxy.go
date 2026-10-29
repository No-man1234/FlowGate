package proxy

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"

	"github.com/No-man1234/FlowGate/pkg/breaker"
	"github.com/No-man1234/FlowGate/pkg/limiter"
	"github.com/No-man1234/FlowGate/pkg/metrics"
)

// GatewayConfig holds operational configuration for the reverse proxy.
type GatewayConfig struct {
	UpstreamURL         *url.URL
	PriorityHeader      string        // e.g. "X-Priority"
	TrustClientPriority bool          // If false, untrusted client-supplied priority headers are ignored (default: false)
	ClientTimeout       time.Duration // Max time client request is allowed to wait
	RetryAfterSeconds   int           // Retry-After header value on shed (default: 1)
	MaxIdleConns        int           // Pool capacity (default: 20000)
	MaxIdleConnsPerHost int           // Pool per upstream host (default: 10000)
}

// ResilientGateway integrates Adaptive Concurrency Limiting, Circuit Breaking,
// and tuned HTTP connection pooling.
type ResilientGateway struct {
	cfg     GatewayConfig
	limiter limiter.Limiter
	breaker *breaker.CircuitBreaker
	metrics *metrics.Collector
	proxy   *httputil.ReverseProxy
}

// NewGateway creates and configures a ResilientGateway.
func NewGateway(
	cfg GatewayConfig,
	l limiter.Limiter,
	cb *breaker.CircuitBreaker,
	m *metrics.Collector,
) *ResilientGateway {
	if cfg.PriorityHeader == "" {
		cfg.PriorityHeader = "X-Priority"
	}
	if cfg.RetryAfterSeconds <= 0 {
		cfg.RetryAfterSeconds = 1
	}
	if cfg.MaxIdleConns <= 0 {
		cfg.MaxIdleConns = 20000
	}
	if cfg.MaxIdleConnsPerHost <= 0 {
		cfg.MaxIdleConnsPerHost = 10000
	}

	// High-performance transport tuned to prevent connection churn under 10k+ requests
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}

	rg := &ResilientGateway{
		cfg:     cfg,
		limiter: l,
		breaker: cb,
		metrics: m,
	}

	revProxy := httputil.NewSingleHostReverseProxy(cfg.UpstreamURL)
	revProxy.Transport = transport

	// Custom error handler to detect upstream connection refusal / timeouts
	revProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		rg.metrics.UpstreamErrors.Add(1)
		rg.breaker.RecordFailure()

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", strconv.Itoa(cfg.RetryAfterSeconds))
		w.WriteHeader(http.StatusBadGateway)
		_, _ = fmt.Fprintf(w, `{"error":"upstream unavailable","detail":"%s"}`+"\n", err.Error())
	}

	rg.proxy = revProxy
	return rg
}

// ServeHTTP implements http.Handler, executing the resilient protection pipeline.
func (rg *ResilientGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rg.metrics.TotalRequests.Add(1)

	// Step 1: Circuit Breaker Check (Fast-fail if upstream is down)
	if !rg.breaker.Allow() {
		rg.metrics.BreakerTrips.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", strconv.Itoa(rg.cfg.RetryAfterSeconds*3))
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"circuit open","message":"upstream service is experiencing severe failures"}` + "\n"))
		return
	}

	// Step 2: Adaptive Concurrency Check (Load Shedding / Backpressure)
	priority := limiter.PriorityStandard
	if rg.cfg.TrustClientPriority {
		priorityVal := r.Header.Get(rg.cfg.PriorityHeader)
		priority = limiter.ParsePriority(priorityVal)
	}

	token, ok := rg.limiter.Acquire(r.Context(), priority)
	if !ok {
		// Fast-fail rejection in < 0.2ms
		rg.metrics.ShedRequests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", strconv.Itoa(rg.cfg.RetryAfterSeconds))
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"too many requests","message":"gateway shed load to protect upstream stability"}` + "\n"))
		return
	}

	rg.metrics.AcceptedRequests.Add(1)

	// Step 3: Forward to Upstream with latency timing
	start := time.Now()
	wrappedWriter := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

	rg.proxy.ServeHTTP(wrappedWriter, r)

	rtt := time.Since(start)
	isServerErr := wrappedWriter.statusCode >= 500

	if isServerErr {
		rg.metrics.UpstreamErrors.Add(1)
		rg.breaker.RecordFailure()
	} else {
		rg.breaker.RecordSuccess()
	}

	// Step 4: Feedback loop to Adaptive Limiter
	token.Done(rtt, isServerErr)
}

// statusResponseWriter captures the HTTP status code written to the client.
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

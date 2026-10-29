package proxy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/http/httputil"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/No-man1234/FlowGate/pkg/breaker"
	"github.com/No-man1234/FlowGate/pkg/limiter"
	"github.com/No-man1234/FlowGate/pkg/metrics"
)

// ClassifierFunc inspects an incoming request and determines its priority tier.
type ClassifierFunc func(r *http.Request) limiter.Priority

// GatewayConfig holds operational configuration for the reverse proxy.
type GatewayConfig struct {
	UpstreamURL         *url.URL
	PriorityHeader      string         // e.g. "X-Priority"
	TrustClientPriority bool           // If true, client-supplied priority headers are respected (default: false)
	Classifier          ClassifierFunc // Custom server-side request priority classifier
	CriticalPrefixes    []string       // Configurable path prefixes for critical tier (e.g. ["/checkout", "/auth"])
	BestEffortPrefixes  []string       // Configurable path prefixes for best-effort tier (e.g. ["/analytics", "/logs"])
	ClientTimeout       time.Duration  // Max time to wait for upstream response headers (default: 15s)
	RetryAfterSeconds   int            // Retry-After header value on shed (default: 1)
	MaxIdleConns        int            // Pool capacity (default: 20000)
	MaxIdleConnsPerHost int            // Pool per upstream host (default: 10000)
}

// ResilientGateway integrates Adaptive Concurrency Limiting, Circuit Breaking,
// and tuned HTTP connection pooling.
type ResilientGateway struct {
	cfg        GatewayConfig
	limiter    limiter.Limiter
	breaker    *breaker.CircuitBreaker
	metrics    *metrics.Collector
	proxy      *httputil.ReverseProxy
	classifier ClassifierFunc
}

// hasSegmentPrefix checks if path matches prefix as an exact segment or directory prefix.
func hasSegmentPrefix(p, prefix string) bool {
	prefix = strings.TrimSuffix(prefix, "/")
	return p == prefix || strings.HasPrefix(p, prefix+"/")
}

// BuildDefaultClassifier creates a path-based classifier using exact segment boundaries.
func BuildDefaultClassifier(critical, bestEffort []string) ClassifierFunc {
	return func(r *http.Request) limiter.Priority {
		clean := path.Clean("/" + r.URL.Path)
		p := strings.ToLower(clean)

		for _, pre := range critical {
			if hasSegmentPrefix(p, strings.ToLower(pre)) {
				return limiter.PriorityCritical
			}
		}

		for _, pre := range bestEffort {
			if hasSegmentPrefix(p, strings.ToLower(pre)) {
				return limiter.PriorityBestEffort
			}
		}

		return limiter.PriorityStandard
	}
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
	if cfg.ClientTimeout <= 0 {
		cfg.ClientTimeout = 15 * time.Second
	}

	if len(cfg.CriticalPrefixes) == 0 {
		cfg.CriticalPrefixes = []string{"/checkout", "/payments", "/auth", "/orders"}
	}
	if len(cfg.BestEffortPrefixes) == 0 {
		cfg.BestEffortPrefixes = []string{"/analytics", "/metrics", "/logs", "/webhooks"}
	}

	classifier := cfg.Classifier
	if classifier == nil {
		classifier = BuildDefaultClassifier(cfg.CriticalPrefixes, cfg.BestEffortPrefixes)
	}

	// High-performance transport tuned to prevent connection churn under high concurrency
	transport := &http.Transport{
		Proxy: nil, // Zero-cost direct routing for internal reverse proxies
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		DisableCompression:   true, // Avoid gzip CPU serialization overhead on proxy path
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: cfg.ClientTimeout,
	}

	rg := &ResilientGateway{
		cfg:        cfg,
		limiter:    l,
		breaker:    cb,
		metrics:    m,
		classifier: classifier,
	}

	revProxy := httputil.NewSingleHostReverseProxy(cfg.UpstreamURL)
	revProxy.Transport = transport

	// Custom error handler to detect upstream connection failure without leaking internal addresses
	revProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if sw, ok := w.(*statusResponseWriter); ok {
			sw.proxyErr = err
		}

		statusCode := http.StatusBadGateway
		if errors.Is(err, context.DeadlineExceeded) {
			statusCode = http.StatusGatewayTimeout
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", strconv.Itoa(cfg.RetryAfterSeconds))
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(`{"error":"upstream unavailable"}` + "\n"))
	}

	rg.proxy = revProxy
	return rg
}

// ServeHTTP implements http.Handler, executing the resilient protection pipeline.
func (rg *ResilientGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rg.metrics.TotalRequests.Add(1)

	// Step 1: Circuit Breaker Check (Fast-fail if upstream is down)
	allowed, gen := rg.breaker.Allow()
	if !allowed {
		rg.metrics.BreakerTrips.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", strconv.Itoa(rg.cfg.RetryAfterSeconds*3))
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"circuit open","message":"upstream service is experiencing severe failures"}` + "\n"))
		return
	}

	// Step 2: Clean path and Request Priority Classification
	cleanPath := path.Clean("/" + r.URL.Path)
	rClean := r.Clone(r.Context())
	rClean.URL.Path = cleanPath

	priority := rg.classifier(rClean)
	if rg.cfg.TrustClientPriority {
		if val := r.Header.Get(rg.cfg.PriorityHeader); val != "" {
			priority = limiter.ParsePriority(val)
		}
	}

	// Step 3: Adaptive Concurrency Check (Load Shedding / Backpressure)
	token, ok := rg.limiter.Acquire(r.Context(), priority)
	if !ok {
		// Cancel breaker probe slot so half-open state is never locked out!
		rg.breaker.Cancel(gen)

		rg.metrics.ShedRequests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", strconv.Itoa(rg.cfg.RetryAfterSeconds))
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"too many requests","message":"gateway shed load to protect upstream stability"}` + "\n"))
		return
	}

	rg.metrics.AcceptedRequests.Add(1)

	// Step 4: Forward to Upstream with Time To First Byte (TTFB) latency tracking
	start := time.Now()
	var ttfb atomic.Int64

	trace := &httptrace.ClientTrace{
		GotFirstResponseByte: func() {
			ttfb.Store(int64(time.Since(start)))
		},
	}
	ctxWithTrace := httptrace.WithClientTrace(r.Context(), trace)

	wrappedWriter := &statusResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}

	// Strip incoming priority header and sync cleaned path before proxying
	rOut := rClean.WithContext(ctxWithTrace)
	rOut.Header.Del(rg.cfg.PriorityHeader)

	// Step 5: Panic & Mid-Body Abort Protection:
	// ReverseProxy panics with http.ErrAbortHandler when a client disconnects mid-body.
	// This defer ensures the limiter token and breaker slot are ALWAYS released even on panic!
	settled := false
	defer func() {
		if settled {
			return
		}
		// Mid-body panic or aborted copy
		if r.Context().Err() != nil {
			rg.breaker.Cancel(gen)
			token.Release()
		} else {
			rg.metrics.UpstreamErrors.Add(1)
			rg.breaker.RecordFailure(gen)
			token.Done(time.Since(start), true)
		}
	}()

	rg.proxy.ServeHTTP(wrappedWriter, rOut)
	settled = true

	// Determine latency for feedback
	var rtt time.Duration
	if measuredTTFB := ttfb.Load(); measuredTTFB > 0 {
		rtt = time.Duration(measuredTTFB)
	} else {
		rtt = time.Since(start)
	}

	// Step 6: Handle Results and Segregate Client Aborts from Upstream Failures
	isClientCancel := r.Context().Err() != nil || errors.Is(wrappedWriter.proxyErr, context.Canceled)

	switch {
	case isClientCancel:
		// Client aborted or timed out early:
		// MUST NOT count as an upstream failure! Cancel breaker probe and release token.
		rg.breaker.Cancel(gen)
		token.Release()

	case wrappedWriter.proxyErr != nil || wrappedWriter.statusCode >= 500:
		// Upstream transport error or 5xx: record failure exactly once!
		rg.metrics.UpstreamErrors.Add(1)
		rg.breaker.RecordFailure(gen)
		token.Done(rtt, true)

	default:
		// Healthy interaction:
		rg.breaker.RecordSuccess(gen)

		// If streaming (SSE text/event-stream or WebSocket 101 Switching Protocols),
		// release token without poisoning the latency feedback loop with stream duration!
		if isStreamingResponse(wrappedWriter.Header().Get("Content-Type"), wrappedWriter.statusCode) {
			token.Release()
		} else {
			token.Done(rtt, false)
		}
	}
}

func isStreamingResponse(contentType string, code int) bool {
	if code == http.StatusSwitchingProtocols {
		return true
	}
	return strings.HasPrefix(contentType, "text/event-stream")
}

// statusResponseWriter captures status code and error, and unwraps the underlying writer
// so http.ResponseController, http.Flusher, and http.Hijacker work seamlessly.
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
	proxyErr   error
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// Unwrap exposes the underlying ResponseWriter for Go's ResponseController, Flusher, and Hijacker.
func (w *statusResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

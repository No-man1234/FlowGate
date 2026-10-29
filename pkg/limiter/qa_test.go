package limiter_test

import (
	"context"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/No-man1234/FlowGate/pkg/breaker"
	"github.com/No-man1234/FlowGate/pkg/limiter"
	"github.com/No-man1234/FlowGate/pkg/metrics"
	"github.com/No-man1234/FlowGate/pkg/proxy"
	"github.com/No-man1234/FlowGate/pkg/server"
)

// ============================================================================
// 1. GOROUTINE LEAK AUDIT (runtime.NumGoroutine())
// ============================================================================

// TestQA_Goroutines_ClientAbortAndTimeoutLeakProbe audits goroutine count before and after
// bursts of client timeouts and mid-body aborts.
func TestQA_Goroutines_ClientAbortAndTimeoutLeakProbe(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		_, _ = w.Write([]byte("chunk-1\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// Pause to provoke mid-body client disconnection
		time.Sleep(100 * time.Millisecond)
		_, _ = w.Write([]byte("chunk-2\n"))
	}))
	defer upstream.Close()

	u, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatalf("parse upstream URL: %v", err)
	}

	l := limiter.NewGradientLimiter(limiter.DefaultConfig())
	cb := breaker.New(breaker.DefaultConfig())
	m := metrics.NewCollector()
	gw := proxy.NewGateway(proxy.GatewayConfig{UpstreamURL: u}, l, cb, m)

	ts := httptest.NewServer(gw)
	defer ts.Close()

	// Stabilize and record baseline goroutine count
	for i := 0; i < 3; i++ {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
	baselineG := runtime.NumGoroutine()

	// Fire 50 requests with client aborts/timeouts
	client := &http.Client{
		Timeout: 25 * time.Millisecond,
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Get(ts.URL + "/work")
			if err == nil {
				_, _ = io.ReadAll(resp.Body)
				_ = resp.Body.Close()
			}
		}()
	}
	wg.Wait()
	client.CloseIdleConnections()

	// Wait for upstream work to complete and connections to settle
	time.Sleep(300 * time.Millisecond)
	for i := 0; i < 3; i++ {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
	afterG := runtime.NumGoroutine()

	t.Logf("[Goroutine Leak Probe] Baseline=%d, Post-Load=%d, Net Delta=%d", baselineG, afterG, afterG-baselineG)
	if afterG > baselineG+5 {
		t.Errorf("WARNING: Potential goroutine leak detected on client aborts! Delta: +%d goroutines", afterG-baselineG)
	}

	// Verify limiter slot wasn't leaked
	if l.InFlight() != 0 {
		t.Errorf("Limiter inFlight leaked! inFlight = %d (expected 0)", l.InFlight())
	}
}

// TestQA_Goroutines_ServerShutdownLeakProbe tests whether ListenAndServeWithGracefulShutdown
// leaks background signal monitoring goroutines on early failure or exit.
func TestQA_Goroutines_ServerShutdownLeakProbe(t *testing.T) {
	// Initialize runtime signal loop so it doesn't count as a leaked goroutine
	dummy := make(chan os.Signal, 1)
	signal.Notify(dummy, os.Interrupt)
	signal.Stop(dummy)

	// Pre-bind a port to force ListenAndServe to fail immediately
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}
	addr := ln.Addr().String()
	defer ln.Close()

	for i := 0; i < 3; i++ {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
	baselineG := runtime.NumGoroutine()

	srv := server.New(server.Config{
		Addr: addr,
	})

	err = srv.ListenAndServeWithGracefulShutdown()
	if err == nil {
		t.Fatalf("expected bind error, got nil")
	}

	time.Sleep(50 * time.Millisecond)
	for i := 0; i < 3; i++ {
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
	afterG := runtime.NumGoroutine()

	t.Logf("[Server Goroutine Probe] Baseline=%d, After=%d, Leaked=%d", baselineG, afterG, afterG-baselineG)
	if afterG > baselineG {
		t.Errorf("BUG FOUND: ListenAndServeWithGracefulShutdown leaked %d goroutines on bind failure!", afterG-baselineG)
	}
}

// ============================================================================
// 2. PANIC SAFETY UNDER MALFORMED / NIL INPUTS
// ============================================================================

// TestQA_PanicSafety_NilInputs audits panic safety when nil pointers or malformed inputs are passed.
func TestQA_PanicSafety_NilInputs(t *testing.T) {
	t.Run("ServeHTTP panics when r.URL is nil", func(t *testing.T) {
		u, _ := url.Parse("http://127.0.0.1:9999")
		gw := proxy.NewGateway(
			proxy.GatewayConfig{UpstreamURL: u},
			limiter.NewGradientLimiter(limiter.DefaultConfig()),
			breaker.New(breaker.DefaultConfig()),
			metrics.NewCollector(),
		)

		defer func() {
			r := recover()
			if r != nil {
				t.Errorf("BUG FOUND: ServeHTTP panics on nil r.URL: %v", r)
			}
		}()

		req := &http.Request{Method: "GET", Header: make(http.Header)}
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)
	})

	t.Run("ServeHTTP panics when UpstreamURL is nil", func(t *testing.T) {
		gw := proxy.NewGateway(
			proxy.GatewayConfig{UpstreamURL: nil},
			limiter.NewGradientLimiter(limiter.DefaultConfig()),
			breaker.New(breaker.DefaultConfig()),
			metrics.NewCollector(),
		)

		defer func() {
			r := recover()
			if r != nil {
				t.Errorf("BUG FOUND: ServeHTTP panics when UpstreamURL is nil: %v", r)
			}
		}()

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)
	})

	t.Run("Acquire with nil context does not panic", func(t *testing.T) {
		l := limiter.NewGradientLimiter(limiter.DefaultConfig())
		tok, ok := l.Acquire(nil, limiter.PriorityStandard)
		if !ok || tok == nil {
			t.Errorf("expected Acquire(nil, ...) to succeed or handle gracefully, got ok=%v", ok)
		} else {
			tok.Release()
		}
	})

	t.Run("ParsePriority case-sensitivity and whitespace bugs", func(t *testing.T) {
		tests := []struct {
			input    string
			expected limiter.Priority
		}{
			{"critical", limiter.PriorityCritical},
			{"Critical", limiter.PriorityCritical},
			{"CRITICAL", limiter.PriorityCritical},
			{"high", limiter.PriorityCritical},
			{"HIGH", limiter.PriorityCritical},
			{"  critical  ", limiter.PriorityCritical},
			{"best-effort", limiter.PriorityBestEffort},
			{"Best-Effort", limiter.PriorityBestEffort},
			{"LOW", limiter.PriorityBestEffort},
			{"background", limiter.PriorityBestEffort},
			{"standard", limiter.PriorityStandard},
			{"unknown-priority", limiter.PriorityStandard},
		}

		for _, tc := range tests {
			actual := limiter.ParsePriority(tc.input)
			if actual != tc.expected {
				t.Errorf("BUG FOUND: ParsePriority(%q) = %v; expected %v", tc.input, actual, tc.expected)
			}
		}
	})
}

// ============================================================================
// 3. GRACEFUL SERVER DRAINING UNDER HEAVY SATURATION
// ============================================================================

// TestQA_GracefulDrain_ActiveSaturationDrainsCleanly tests server shutdown under heavy active load.
func TestQA_GracefulDrain_ActiveSaturationDrainsCleanly(t *testing.T) {
	var inFlight atomic.Int64
	var completed atomic.Int64
	var dropped atomic.Int64

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inFlight.Add(1)
		defer inFlight.Add(-1)
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("completed"))
		completed.Add(1)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}
	addr := ln.Addr().String()

	httpSrv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		_ = httpSrv.Serve(ln)
	}()

	// Launch 30 concurrent requests
	var wg sync.WaitGroup
	client := &http.Client{Timeout: 5 * time.Second}

	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Get("http://" + addr + "/work")
			if err != nil {
				dropped.Add(1)
				return
			}
			_, _ = io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				dropped.Add(1)
			}
		}()
	}

	// Give requests time to become in-flight
	time.Sleep(20 * time.Millisecond)

	// Initiate graceful shutdown with 500ms drain timeout (ample for 50ms work)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := httpSrv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown error: %v", err)
	}

	wg.Wait()
	client.CloseIdleConnections()

	t.Logf("[Graceful Drain] In-flight drained: completed=%d, dropped=%d, remaining=%d",
		completed.Load(), dropped.Load(), inFlight.Load())

	if dropped.Load() > 0 {
		t.Errorf("BUG FOUND: %d active requests dropped during graceful draining within timeout!", dropped.Load())
	}
}

// TestQA_GracefulDrain_TimeoutForcesAbruptDrop tests server behavior when upstream is slower than DrainTimeout.
func TestQA_GracefulDrain_TimeoutForcesAbruptDrop(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond) // Exceeds drain timeout
		w.WriteHeader(http.StatusOK)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}
	addr := ln.Addr().String()

	httpSrv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		_ = httpSrv.Serve(ln)
	}()

	var dropped atomic.Int64
	var wg sync.WaitGroup
	client := &http.Client{Timeout: 2 * time.Second}

	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := client.Get("http://" + addr + "/slow")
		if err != nil {
			dropped.Add(1)
			return
		}
		_ = resp.Body.Close()
	}()

	time.Sleep(30 * time.Millisecond)

	// Drain timeout of only 50ms forces abort of 300ms request
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	shutdownErr := httpSrv.Shutdown(ctx)
	if !errors.Is(shutdownErr, context.DeadlineExceeded) {
		t.Logf("Expected DeadlineExceeded, got: %v", shutdownErr)
	} else {
		// When Shutdown times out, net/http leaves connections open unless Close() is called!
		_ = httpSrv.Close()
	}

	wg.Wait()
	client.CloseIdleConnections()
	t.Logf("[Drain Timeout] Saturated requests exceeding drain time dropped=%d (expected 1)", dropped.Load())
	if dropped.Load() != 1 {
		t.Errorf("Expected in-flight request exceeding drain timeout to be dropped upon Close(), got dropped=%d", dropped.Load())
	}
}

// ============================================================================
// 4. CIRCUIT BREAKER EDGE CASES
// ============================================================================

// TestQA_CircuitBreaker_DNSResolutionFailure verifies circuit breaker behavior when DNS fails.
func TestQA_CircuitBreaker_DNSResolutionFailure(t *testing.T) {
	invalidURL, _ := url.Parse("http://nonexistent-domain-flowgate-qa.invalid:8080")

	l := limiter.NewGradientLimiter(limiter.DefaultConfig())
	cb := breaker.New(breaker.Config{
		FailureThreshold: 3,
		CoolOff:          1 * time.Second,
	})
	m := metrics.NewCollector()

	gw := proxy.NewGateway(proxy.GatewayConfig{
		UpstreamURL:   invalidURL,
		ClientTimeout: 500 * time.Millisecond,
	}, l, cb, m)

	// Send 3 requests: should fail DNS and trip breaker
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		gw.ServeHTTP(w, req)
		if w.Code != http.StatusBadGateway {
			t.Errorf("request %d: expected 502, got %d", i, w.Code)
		}
	}

	if cb.State() != breaker.StateOpen {
		t.Errorf("expected circuit breaker to be OPEN after 3 DNS failures, got %v", cb.State())
	}

	// 4th request must be failed fast with 503
	start := time.Now()
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	gw.ServeHTTP(w, req)
	elapsed := time.Since(start)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable when circuit open, got %d", w.Code)
	}
	if elapsed > 10*time.Millisecond {
		t.Errorf("circuit open response took too long: %v (expected <10ms fast fail)", elapsed)
	}
}

// TestQA_CircuitBreaker_HalfOpenPermanentLockout proves that lost probe requests in
// StateHalfOpen permanently wedge the circuit breaker.
func TestQA_CircuitBreaker_HalfOpenPermanentLockout(t *testing.T) {
	cfg := breaker.Config{
		FailureThreshold:    2,
		CoolOff:             50 * time.Millisecond,
		HalfOpenMaxRequests: 2,
		SuccessThreshold:    2,
	}
	cb := breaker.New(cfg)

	// Trip open
	_, g1 := cb.Allow()
	cb.RecordFailure(g1)
	_, g2 := cb.Allow()
	cb.RecordFailure(g2)

	if cb.State() != breaker.StateOpen {
		t.Fatalf("expected StateOpen, got %v", cb.State())
	}

	// Wait for cool-off
	time.Sleep(60 * time.Millisecond)

	// Probe 1 & 2 admitted
	ok1, _ := cb.Allow()
	ok2, _ := cb.Allow()
	if !ok1 || !ok2 {
		t.Fatalf("expected both probes admitted in half-open")
	}

	// Both probes hang/crash without calling RecordSuccess, RecordFailure, or Cancel
	// Advance well past cool-off
	time.Sleep(100 * time.Millisecond)

	// Probe 3 is locked out forever
	ok3, _ := cb.Allow()
	if !ok3 {
		t.Errorf("BUG FOUND: Circuit breaker is permanently wedged in HALF-OPEN when probes drop without reporting!")
	}
}

// TestQA_CircuitBreaker_FlappingBlindness documents the consecutive-failure limitation under flapping.
func TestQA_CircuitBreaker_FlappingBlindness(t *testing.T) {
	cfg := breaker.Config{
		FailureThreshold: 5,
		CoolOff:          1 * time.Second,
	}
	cb := breaker.New(cfg)

	// 80% failure rate: 4 fails, 1 success, repeating
	for cycle := 0; cycle < 20; cycle++ {
		for f := 0; f < 4; f++ {
			ok, gen := cb.Allow()
			if !ok {
				t.Fatalf("unexpected trip at cycle %d", cycle)
			}
			cb.RecordFailure(gen)
		}
		ok, gen := cb.Allow()
		if !ok {
			t.Fatalf("unexpected trip at cycle %d success", cycle)
		}
		cb.RecordSuccess(gen)
	}

	if cb.State() != breaker.StateClosed {
		t.Errorf("expected closed, got %v", cb.State())
	}
	t.Logf("Circuit Breaker remains CLOSED under 80%% error rate because successes continually clear consecutive failure counter.")
}

// ============================================================================
// 5. LIMITER ARITHMETIC & EDGE CASES
// ============================================================================

// TestQA_Limiter_ZeroRTT_ErrorBackoffSkipped proves that onDone returns early when
// rtt <= 0, causing error backoff to be completely skipped on immediate network failures.
func TestQA_Limiter_ZeroRTT_ErrorBackoffSkipped(t *testing.T) {
	cfg := limiter.DefaultConfig()
	cfg.InitialLimit = 50
	cfg.MinLimit = 5
	cfg.BackoffRatio = 0.5
	l := limiter.NewGradientLimiter(cfg)

	initialLimit := l.CurrentLimit()

	// Error with RTT = 0
	tok1, ok1 := l.Acquire(context.Background(), limiter.PriorityCritical)
	if !ok1 {
		t.Fatalf("acquire failed")
	}
	tok1.Done(0, true)

	if l.CurrentLimit() == initialLimit {
		t.Errorf("BUG FOUND: tok.Done(0, true) skipped error backoff! Limit is still %.2f (expected %.2f)",
			l.CurrentLimit(), initialLimit*0.5)
	}

	// Error with negative RTT
	tok2, ok2 := l.Acquire(context.Background(), limiter.PriorityCritical)
	if !ok2 {
		t.Fatalf("acquire failed")
	}
	tok2.Done(-10*time.Millisecond, true)

	if l.CurrentLimit() == initialLimit {
		t.Errorf("BUG FOUND: tok.Done(-10ms, true) skipped error backoff! Limit is still %.2f", l.CurrentLimit())
	}
}

// TestQA_Limiter_BootUnderSaturation_MinRTTParalysis proves that booting under saturation
// leaves minRTT at 0s forever, permanently paralyzing dynamic limit adaptation.
func TestQA_Limiter_BootUnderSaturation_MinRTTParalysis(t *testing.T) {
	cfg := limiter.DefaultConfig()
	cfg.InitialLimit = 10
	cfg.MinLimit = 5
	l := limiter.NewGradientLimiter(cfg)

	// 10 concurrent requests arrive at boot
	tokens := make([]limiter.Token, 10)
	for i := 0; i < 10; i++ {
		tok, ok := l.Acquire(context.Background(), limiter.PriorityCritical)
		if !ok {
			t.Fatalf("acquire failed at %d", i)
		}
		tokens[i] = tok
	}

	// First 5 finish with 20ms while in-flight is > 50% capacity
	for i := 0; i < 5; i++ {
		tokens[i].Done(20*time.Millisecond, false)
	}

	if l.MinRTT() == 0 {
		t.Errorf("BUG FOUND: Saturated boot left minRTT = 0s! Dynamic adaptation is paralyzed.")
	}

	for i := 5; i < 10; i++ {
		tokens[i].Done(20*time.Millisecond, false)
	}
}

// TestQA_Limiter_NaNInfParameters proves that NaN or Inf parameters corrupt the limiter state.
func TestQA_Limiter_NaNInfParameters(t *testing.T) {
	t.Run("NaN InitialLimit disables load shedding", func(t *testing.T) {
		cfg := limiter.DefaultConfig()
		cfg.InitialLimit = math.NaN()
		l := limiter.NewGradientLimiter(cfg)

		if math.IsNaN(l.CurrentLimit()) {
			t.Errorf("BUG FOUND: CurrentLimit() is NaN when initialized with NaN!")
			// Shedding is broken: all requests accepted
			for i := 0; i < 100; i++ {
				_, ok := l.Acquire(context.Background(), limiter.PriorityStandard)
				if !ok {
					t.Fatalf("unexpected shed at %d", i)
				}
			}
			t.Logf("Accepted 100 requests with InFlight = %d because NaN disabled shedding!", l.InFlight())
		}
	})

	t.Run("NaN Tolerance corrupts limit to NaN on sample", func(t *testing.T) {
		cfg := limiter.DefaultConfig()
		cfg.Tolerance = math.NaN()
		cfg.InitialLimit = 10
		l := limiter.NewGradientLimiter(cfg)

		tok, _ := l.Acquire(context.Background(), limiter.PriorityCritical)
		tok.Done(10*time.Millisecond, false)

		var toks []limiter.Token
		for i := 0; i < 6; i++ {
			t, _ := l.Acquire(context.Background(), limiter.PriorityCritical)
			toks = append(toks, t)
		}
		for _, t := range toks {
			t.Done(10*time.Millisecond, false)
		}

		if math.IsNaN(l.CurrentLimit()) {
			t.Errorf("BUG FOUND: CurrentLimit() became NaN due to NaN Tolerance!")
		}
	})

	t.Run("NaN BackoffRatio corrupts limit on error", func(t *testing.T) {
		cfg := limiter.DefaultConfig()
		cfg.BackoffRatio = math.NaN()
		cfg.InitialLimit = 10
		l := limiter.NewGradientLimiter(cfg)

		tok, _ := l.Acquire(context.Background(), limiter.PriorityCritical)
		tok.Done(10*time.Millisecond, true)

		if math.IsNaN(l.CurrentLimit()) {
			t.Errorf("BUG FOUND: CurrentLimit() became NaN on error backoff due to NaN BackoffRatio!")
		}
	})
}

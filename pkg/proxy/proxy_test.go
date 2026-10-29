package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/No-man1234/FlowGate/pkg/breaker"
	"github.com/No-man1234/FlowGate/pkg/limiter"
	"github.com/No-man1234/FlowGate/pkg/metrics"
)

func TestResilientGateway_ProxySuccess(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, `{"status":"ok"}`)
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)

	l := limiter.NewGradientLimiter(limiter.DefaultConfig())
	cb := breaker.New(breaker.DefaultConfig())
	m := metrics.NewCollector()

	gateway := NewGateway(
		GatewayConfig{UpstreamURL: upstreamURL},
		l,
		cb,
		m,
	)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	gateway.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", resp.StatusCode)
	}

	if m.AcceptedRequests.Load() != 1 {
		t.Fatalf("expected 1 accepted request in metrics, got %d", m.AcceptedRequests.Load())
	}
}

func TestResilientGateway_LoadShedding429(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)

	cfg := limiter.DefaultConfig()
	cfg.InitialLimit = 1
	cfg.MinLimit = 1
	cfg.StandardThreshold = 1.0
	l := limiter.NewGradientLimiter(cfg)

	cb := breaker.New(breaker.DefaultConfig())
	m := metrics.NewCollector()

	gateway := NewGateway(
		GatewayConfig{UpstreamURL: upstreamURL},
		l,
		cb,
		m,
	)

	// Hold the 1 available token
	tok, ok := l.Acquire(nil, limiter.PriorityStandard)
	if !ok {
		t.Fatalf("failed acquiring setup token")
	}
	defer tok.Release()

	// Subsequent request must be shed with 429
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	gateway.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 Too Many Requests, got %d", resp.StatusCode)
	}

	if m.ShedRequests.Load() != 1 {
		t.Fatalf("expected 1 shed request in metrics, got %d", m.ShedRequests.Load())
	}

	if resp.Header.Get("Retry-After") == "" {
		t.Fatalf("expected Retry-After header on 429 response")
	}
}

func TestGateway_ShedDoesNotLeakHalfOpenSlot(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)

	// Configure limiter with limit 1
	limCfg := limiter.DefaultConfig()
	limCfg.InitialLimit = 1
	limCfg.MinLimit = 1
	limCfg.StandardThreshold = 1.0
	l := limiter.NewGradientLimiter(limCfg)

	// Breaker with 1 failure threshold, 20ms cooloff, 1 half-open request
	cbCfg := breaker.Config{
		FailureThreshold:    1,
		CoolOff:             20 * time.Millisecond,
		HalfOpenMaxRequests: 1,
		SuccessThreshold:    1,
	}
	cb := breaker.New(cbCfg)
	m := metrics.NewCollector()

	gateway := NewGateway(
		GatewayConfig{UpstreamURL: upstreamURL},
		l,
		cb,
		m,
	)

	// Trip the breaker
	_, gen := cb.Allow()
	cb.RecordFailure(gen)
	if cb.State() != breaker.StateOpen {
		t.Fatalf("expected breaker OPEN")
	}

	// Wait for cool-off
	time.Sleep(30 * time.Millisecond)

	// Occupy the limiter's only slot so next gateway request is shed
	tok, _ := l.Acquire(context.Background(), limiter.PriorityCritical)

	// Fire request through gateway: Allow() grants half-open probe, but Limiter sheds!
	wShed := httptest.NewRecorder()
	gateway.ServeHTTP(wShed, httptest.NewRequest("GET", "/test", nil))
	if wShed.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", wShed.Code)
	}

	// Release the limiter token
	tok.Release()

	// Breaker probe slot MUST NOT be leaked. A new probe request must be admitted through the gateway!
	wProbe := httptest.NewRecorder()
	gateway.ServeHTTP(wProbe, httptest.NewRequest("GET", "/test", nil))
	if wProbe.Code != http.StatusOK {
		t.Fatalf("expected probe to succeed with 200 OK, got %d (breaker locked out!)", wProbe.Code)
	}

	if cb.State() != breaker.StateClosed {
		t.Fatalf("expected circuit breaker to close after successful probe, got %s", cb.State())
	}
}

func TestGateway_TransportErrorCountsOnce(t *testing.T) {
	// Point to unreachable port
	unreachableURL, _ := url.Parse("http://127.0.0.1:54321")

	l := limiter.NewGradientLimiter(limiter.DefaultConfig())
	cb := breaker.New(breaker.Config{
		FailureThreshold: 5,
		CoolOff:          1 * time.Second,
	})
	m := metrics.NewCollector()

	gateway := NewGateway(
		GatewayConfig{UpstreamURL: unreachableURL},
		l,
		cb,
		m,
	)

	w := httptest.NewRecorder()
	gateway.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 Bad Gateway, got %d", w.Code)
	}

	// Must be counted exactly once, not twice!
	if m.UpstreamErrors.Load() != 1 {
		t.Fatalf("transport error double-counted! expected 1, got %d", m.UpstreamErrors.Load())
	}
}

func TestGateway_ClientCancelNotUpstreamFailure(t *testing.T) {
	// Slow upstream that waits for client
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)

	l := limiter.NewGradientLimiter(limiter.DefaultConfig())
	cb := breaker.New(breaker.Config{
		FailureThreshold: 2,
		CoolOff:          1 * time.Second,
	})
	m := metrics.NewCollector()

	gateway := NewGateway(
		GatewayConfig{UpstreamURL: upstreamURL},
		l,
		cb,
		m,
	)

	// Fire 3 cancelled requests
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		req := httptest.NewRequest("GET", "/test", nil).WithContext(ctx)
		w := httptest.NewRecorder()
		gateway.ServeHTTP(w, req)
	}

	// Client cancels MUST NOT count as upstream errors or trip the breaker!
	if m.UpstreamErrors.Load() != 0 {
		t.Fatalf("client cancellations counted as upstream errors! got %d", m.UpstreamErrors.Load())
	}

	if cb.State() != breaker.StateClosed {
		t.Fatalf("client cancellations tripped the circuit breaker! state is %s", cb.State())
	}
}

func TestResilientGateway_ServerSidePathClassification(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)

	cfg := limiter.DefaultConfig()
	cfg.InitialLimit = 10
	cfg.StandardThreshold = 0.80
	cfg.BestEffortThreshold = 0.50
	l := limiter.NewGradientLimiter(cfg)

	cb := breaker.New(breaker.DefaultConfig())
	m := metrics.NewCollector()

	gateway := NewGateway(
		GatewayConfig{
			UpstreamURL:         upstreamURL,
			TrustClientPriority: false, // strictly enforce server-side path rules
		},
		l,
		cb,
		m,
	)

	// Occupy 6 tokens (60% capacity)
	var tokens []limiter.Token
	for i := 0; i < 6; i++ {
		tok, ok := l.Acquire(nil, limiter.PriorityCritical)
		if !ok {
			t.Fatalf("failed acquiring setup token %d", i)
		}
		tokens = append(tokens, tok)
	}
	defer func() {
		for _, tok := range tokens {
			tok.Release()
		}
	}()

	// At 60% capacity:
	// /analytics should be classified as BestEffort and shed (threshold 50%)
	wAnalytics := httptest.NewRecorder()
	gateway.ServeHTTP(wAnalytics, httptest.NewRequest("GET", "/analytics/track", nil))
	if wAnalytics.Code != http.StatusTooManyRequests {
		t.Fatalf("expected /analytics to be shed at 60%% capacity, got %d", wAnalytics.Code)
	}

	// /checkout should be classified as Critical and admitted (threshold 100%)
	wCheckout := httptest.NewRecorder()
	gateway.ServeHTTP(wCheckout, httptest.NewRequest("GET", "/checkout/pay", nil))
	if wCheckout.Code != http.StatusOK {
		t.Fatalf("expected /checkout to be admitted at 60%% capacity, got %d", wCheckout.Code)
	}
}

func TestResilientGateway_CircuitBreaker503(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)

	l := limiter.NewGradientLimiter(limiter.DefaultConfig())
	cb := breaker.New(breaker.Config{
		FailureThreshold: 1,
		CoolOff:          1 * time.Second,
	})
	m := metrics.NewCollector()

	_, gen := cb.Allow()
	cb.RecordFailure(gen)

	gateway := NewGateway(
		GatewayConfig{UpstreamURL: upstreamURL},
		l,
		cb,
		m,
	)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	gateway.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable when circuit open, got %d", resp.StatusCode)
	}

	if m.BreakerTrips.Load() != 1 {
		t.Fatalf("expected 1 breaker trip metric, got %d", m.BreakerTrips.Load())
	}
}

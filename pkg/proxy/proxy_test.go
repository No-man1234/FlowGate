package proxy

import (
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

func TestResilientGateway_Shed429DoesNotTripBreaker(t *testing.T) {
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

	// Configure breaker that trips after 3 failures
	cbCfg := breaker.DefaultConfig()
	cbCfg.FailureThreshold = 3
	cb := breaker.New(cbCfg)

	m := metrics.NewCollector()

	gateway := NewGateway(
		GatewayConfig{UpstreamURL: upstreamURL},
		l,
		cb,
		m,
	)

	// Hold the only slot
	tok, _ := l.Acquire(nil, limiter.PriorityCritical)
	defer tok.Release()

	// Fire 20 consecutive requests that will all get shed with 429
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		gateway.ServeHTTP(w, req)

		if w.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429 on shed request #%d, got %d", i, w.Code)
		}
	}

	// Gateway 429 sheds must NEVER trip the circuit breaker
	if cb.State() != breaker.StateClosed {
		t.Fatalf("expected circuit breaker to remain CLOSED after 20 sheds, got %s", cb.State())
	}
	if !cb.Allow() {
		t.Fatalf("expected circuit breaker Allow() to remain true")
	}
}

func TestResilientGateway_ServerSidePathClassification(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)

	// Setup limiter with limit=10, BestEffortThreshold=0.50 (5), StandardThreshold=0.80 (8)
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

	cb.RecordFailure()

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

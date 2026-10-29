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
	// 1. Setup mock upstream
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

	// 2. Perform test request through gateway
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

	// Configure limiter with limit 1
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

	// Acquire the 1 available slot manually
	tok, ok := l.Acquire(nil, limiter.PriorityStandard)
	if !ok {
		t.Fatalf("failed acquiring setup token")
	}
	defer tok.Release()

	// New request through gateway must be shed with 429
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

	// Trip the circuit breaker
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

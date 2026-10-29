package proxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/No-man1234/FlowGate/pkg/breaker"
	"github.com/No-man1234/FlowGate/pkg/limiter"
	"github.com/No-man1234/FlowGate/pkg/metrics"
)

func BenchmarkGateway_Handler_Shedding(b *testing.B) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)

	cfg := limiter.DefaultConfig()
	cfg.InitialLimit = 1
	cfg.MinLimit = 1
	l := limiter.NewGradientLimiter(cfg)

	// Hold the slot so every request through the gateway is shed
	tok, _ := l.Acquire(context.Background(), limiter.PriorityCritical)
	defer tok.Release()

	cb := breaker.New(breaker.DefaultConfig())
	m := metrics.NewCollector()

	gw := NewGateway(
		GatewayConfig{UpstreamURL: upstreamURL},
		l,
		cb,
		m,
	)

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		w := httptest.NewRecorder()
		for pb.Next() {
			w.Body.Reset()
			gw.ServeHTTP(w, req)
		}
	})
}

func BenchmarkGateway_TCP_Shedding(b *testing.B) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	upstreamURL, _ := url.Parse(upstream.URL)

	cfg := limiter.DefaultConfig()
	cfg.InitialLimit = 1
	cfg.MinLimit = 1
	l := limiter.NewGradientLimiter(cfg)

	// Hold slot so all incoming requests are shed
	tok, _ := l.Acquire(context.Background(), limiter.PriorityCritical)
	defer tok.Release()

	cb := breaker.New(breaker.DefaultConfig())
	m := metrics.NewCollector()

	gw := NewGateway(
		GatewayConfig{UpstreamURL: upstreamURL},
		l,
		cb,
		m,
	)

	// Spin up real HTTP server over TCP socket
	server := httptest.NewServer(gw)
	defer server.Close()

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        1000,
			MaxIdleConnsPerHost: 1000,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := client.Get(server.URL + "/test")
			if err != nil {
				b.Errorf("get error: %v", err)
				return
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
	})
}

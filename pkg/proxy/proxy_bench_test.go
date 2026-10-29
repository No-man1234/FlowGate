package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/No-man1234/FlowGate/pkg/breaker"
	"github.com/No-man1234/FlowGate/pkg/limiter"
	"github.com/No-man1234/FlowGate/pkg/metrics"
)

func BenchmarkGateway_EndToEndShedding(b *testing.B) {
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
	tok, _ := l.Acquire(nil, limiter.PriorityCritical)
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

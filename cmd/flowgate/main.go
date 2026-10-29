package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/No-man1234/FlowGate/pkg/breaker"
	"github.com/No-man1234/FlowGate/pkg/limiter"
	"github.com/No-man1234/FlowGate/pkg/metrics"
	"github.com/No-man1234/FlowGate/pkg/proxy"
	"github.com/No-man1234/FlowGate/pkg/server"
)

func main() {
	port := flag.Int("port", 8080, "Port for FlowGate to listen on")
	upstreamRaw := flag.String("upstream", "http://127.0.0.1:9090", "Upstream target URL")
	initialLimit := flag.Float64("initial-limit", 50, "Initial dynamic concurrency limit")
	minLimit := flag.Float64("min-limit", 5, "Minimum concurrency limit floor")
	maxLimit := flag.Float64("max-limit", 10000, "Maximum concurrency ceiling")
	showDashboard := flag.Bool("dashboard", true, "Render live ASCII status dashboard in terminal")
	flag.Parse()

	upstreamURL, err := url.Parse(*upstreamRaw)
	if err != nil {
		log.Fatalf("Invalid upstream URL: %v", err)
	}

	collector := metrics.NewCollector()

	limiterCfg := limiter.DefaultConfig()
	limiterCfg.InitialLimit = *initialLimit
	limiterCfg.MinLimit = *minLimit
	limiterCfg.MaxLimit = *maxLimit
	adaptLimiter := limiter.NewGradientLimiter(limiterCfg)

	breakerCfg := breaker.DefaultConfig()
	circuitBreaker := breaker.New(breakerCfg)

	gateway := proxy.NewGateway(
		proxy.GatewayConfig{
			UpstreamURL: upstreamURL,
		},
		adaptLimiter,
		circuitBreaker,
		collector,
	)

	// Multiplexer for proxy vs internal endpoints
	mux := http.NewServeMux()

	// Metrics endpoint
	mux.HandleFunc("/metrics", collector.Handler(func() metrics.Summary {
		snap := adaptLimiter.Stats()
		return metrics.Summary{
			TotalRequests:   collector.TotalRequests.Load(),
			Accepted:        collector.AcceptedRequests.Load(),
			Shed:            collector.ShedRequests.Load(),
			BreakerTrips:    collector.BreakerTrips.Load(),
			UpstreamErrors:  collector.UpstreamErrors.Load(),
			RequestsPerSec:  0, // snapshot
			InFlight:        snap.InFlight,
			Limit:           snap.Limit,
			MinRTTMs:        float64(snap.MinRTT) / float64(time.Millisecond),
			SampleRTTMs:     float64(snap.SampleRTT) / float64(time.Millisecond),
			CircuitState:    circuitBreaker.State().String(),
		}
	}))

	// Health check
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK\n"))
	})

	// Forward all other routes through resilient proxy
	mux.Handle("/", gateway)

	// Live terminal status ticker
	if *showDashboard {
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			for range ticker.C {
				collector.Tick()
				snap := adaptLimiter.Stats()

				summary := metrics.Summary{
					TotalRequests:  collector.TotalRequests.Load(),
					Accepted:       collector.AcceptedRequests.Load(),
					Shed:           collector.ShedRequests.Load(),
					BreakerTrips:   collector.BreakerTrips.Load(),
					UpstreamErrors: collector.UpstreamErrors.Load(),
					InFlight:       snap.InFlight,
					Limit:          snap.Limit,
					MinRTTMs:       float64(snap.MinRTT) / float64(time.Millisecond),
					SampleRTTMs:    float64(snap.SampleRTT) / float64(time.Millisecond),
					CircuitState:   circuitBreaker.State().String(),
				}

				fmt.Print(metrics.FormatDashboard(summary))
			}
		}()
	}

	srv := server.New(server.Config{
		Addr:         fmt.Sprintf(":%d", *port),
		Handler:      mux,
		DrainTimeout: 10 * time.Second,
	})

	log.Printf("[FlowGate] Starting Resilient Gateway -> Upstream: %s", upstreamURL.String())
	if err := srv.ListenAndServeWithGracefulShutdown(); err != nil {
		log.Fatalf("Server stopped with error: %v", err)
	}
}

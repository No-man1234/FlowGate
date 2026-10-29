package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/No-man1234/FlowGate/pkg/server"
)

type ChaosState struct {
	mu           sync.RWMutex
	extraLatency time.Duration
	errorRate    float64
}

func main() {
	port := flag.Int("port", 9090, "Port for mock upstream to listen on")
	baseLatencyMs := flag.Int("latency", 15, "Base service latency in ms")
	capacity := flag.Int("capacity", 50, "Capacity threshold where queueing/slowdown occurs")
	flag.Parse()

	chaos := &ChaosState{}
	var activeInFlight atomic.Int64
	var totalServed atomic.Uint64

	mux := http.NewServeMux()

	// The primary workload endpoint
	mux.HandleFunc("/work", func(w http.ResponseWriter, r *http.Request) {
		current := activeInFlight.Add(1)
		defer activeInFlight.Add(-1)

		chaos.mu.RLock()
		extra := chaos.extraLatency
		errRate := chaos.errorRate
		chaos.mu.RUnlock()

		// Simulate chaos error injection
		if errRate > 0 && rand.Float64() < errRate {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"simulated database panic"}` + "\n"))
			return
		}

		// Calculate simulated processing time:
		// Base latency + extra chaos latency + nonlinear queuing penalty if over capacity
		delay := time.Duration(*baseLatencyMs)*time.Millisecond + extra
		if int(current) > *capacity {
			// Simulates CPU/DB saturation queueing
			overage := int(current) - *capacity
			penalty := time.Duration(overage*10) * time.Millisecond
			delay += penalty
		}

		time.Sleep(delay)

		count := totalServed.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"status":"completed","id":%d,"processed_in_ms":%d}`+"\n", count, delay.Milliseconds())
	})

	// Chaos injection: introduce latency spike
	mux.HandleFunc("/chaos/spike", func(w http.ResponseWriter, r *http.Request) {
		latencyStr := r.URL.Query().Get("latency")
		durationStr := r.URL.Query().Get("duration")

		latencyVal, _ := time.ParseDuration(latencyStr)
		if latencyVal == 0 {
			latencyVal = 300 * time.Millisecond
		}
		durationVal, _ := time.ParseDuration(durationStr)
		if durationVal == 0 {
			durationVal = 10 * time.Second
		}

		chaos.mu.Lock()
		chaos.extraLatency = latencyVal
		chaos.mu.Unlock()

		go func() {
			time.Sleep(durationVal)
			chaos.mu.Lock()
			chaos.extraLatency = 0
			chaos.mu.Unlock()
			log.Printf("[Upstream Chaos] Latency spike resolved after %v.", durationVal)
		}()

		log.Printf("[Upstream Chaos] Injected %v latency spike for %v", latencyVal, durationVal)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"message":"injected latency spike of %v for %v"}`+"\n", latencyVal, durationVal)
	})

	// Chaos injection: introduce error rate
	mux.HandleFunc("/chaos/error", func(w http.ResponseWriter, r *http.Request) {
		rateStr := r.URL.Query().Get("rate")
		rate, _ := strconv.ParseFloat(rateStr, 64)

		chaos.mu.Lock()
		chaos.errorRate = rate
		chaos.mu.Unlock()

		log.Printf("[Upstream Chaos] Injected error rate: %.2f", rate)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"message":"error rate set to %.2f"}`+"\n", rate)
	})

	// Reset chaos
	mux.HandleFunc("/chaos/reset", func(w http.ResponseWriter, r *http.Request) {
		chaos.mu.Lock()
		chaos.extraLatency = 0
		chaos.errorRate = 0
		chaos.mu.Unlock()

		log.Printf("[Upstream Chaos] All chaos parameters reset to normal.")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"chaos cleared"}` + "\n"))
	})

	srv := server.New(server.Config{
		Addr: fmt.Sprintf(":%d", *port),
		Handler: mux,
	})

	log.Printf("[Upstream Mock] Running on :%d (Base latency: %dms, Capacity threshold: %d)", *port, *baseLatencyMs, *capacity)
	if err := srv.ListenAndServeWithGracefulShutdown(); err != nil {
		log.Fatalf("Upstream server exited with error: %v", err)
	}
}

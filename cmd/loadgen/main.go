package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type LatencyRecord struct {
	mu        sync.Mutex
	durations []time.Duration
}

func (lr *LatencyRecord) Add(d time.Duration) {
	lr.mu.Lock()
	lr.durations = append(lr.durations, d)
	lr.mu.Unlock()
}

func (lr *LatencyRecord) Percentile(p float64) time.Duration {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	if len(lr.durations) == 0 {
		return 0
	}

	sort.Slice(lr.durations, func(i, j int) bool {
		return lr.durations[i] < lr.durations[j]
	})

	idx := int(float64(len(lr.durations)-1) * p)
	return lr.durations[idx]
}

func main() {
	targetURL := flag.String("url", "http://127.0.0.1:8080/work", "Target URL to benchmark")
	concurrency := flag.Int("concurrency", 500, "Number of concurrent worker goroutines")
	duration := flag.Duration("duration", 10*time.Second, "Test run duration")
	flag.Parse()

	fmt.Printf("[LoadGen] Starting benchmark against: %s\n", *targetURL)
	fmt.Printf("[LoadGen] Concurrency: %d workers | Duration: %v\n", *concurrency, *duration)

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        *concurrency * 2,
			MaxIdleConnsPerHost: *concurrency * 2,
			IdleConnTimeout:     30 * time.Second,
		},
		Timeout: 5 * time.Second,
	}

	var (
		totalRequests atomic.Uint64
		success200    atomic.Uint64
		shed429       atomic.Uint64
		circuit503    atomic.Uint64
		errorsOther   atomic.Uint64

		successLatencies LatencyRecord
		shedLatencies    LatencyRecord
	)

	stopChan := make(chan struct{})
	var wg sync.WaitGroup

	startTime := time.Now()

	// Launch concurrent workers
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-stopChan:
					return
				default:
				}

				// Generate priority mix
				r := rand.Float64()
				var prio string
				if r < 0.15 {
					prio = "critical"
				} else if r < 0.70 {
					prio = "standard"
				} else {
					prio = "best-effort"
				}

				req, err := http.NewRequest("GET", *targetURL, nil)
				if err != nil {
					continue
				}
				req.Header.Set("X-Priority", prio)

				reqStart := time.Now()
				resp, err := client.Do(req)
				elapsed := time.Since(reqStart)

				totalRequests.Add(1)

				if err != nil {
					errorsOther.Add(1)
					continue
				}

				switch resp.StatusCode {
				case http.StatusOK:
					success200.Add(1)
					successLatencies.Add(elapsed)
				case http.StatusTooManyRequests:
					shed429.Add(1)
					shedLatencies.Add(elapsed)
				case http.StatusServiceUnavailable:
					circuit503.Add(1)
				default:
					errorsOther.Add(1)
				}
				_ = resp.Body.Close()
			}
		}()
	}

	time.Sleep(*duration)
	close(stopChan)
	wg.Wait()

	totalTime := time.Since(startTime).Seconds()
	total := totalRequests.Load()
	ok200 := success200.Load()
	sh429 := shed429.Load()
	cr503 := circuit503.Load()
	errs := errorsOther.Load()

	fmt.Println("\n=======================================================")
	fmt.Println("             BENCHMARK RESULTS SUMMARY                 ")
	fmt.Println("=======================================================")
	fmt.Printf(" Total Requests Sent : %d\n", total)
	fmt.Printf(" Throughput (RPS)    : %.1f req/sec\n", float64(total)/totalTime)
	fmt.Println("-------------------------------------------------------")
	fmt.Printf(" 200 OK (Completed)  : %d (%.1f%%)\n", ok200, (float64(ok200)/float64(total))*100.0)
	fmt.Printf(" 429 Shed (Protected): %d (%.1f%%)\n", sh429, (float64(sh429)/float64(total))*100.0)
	fmt.Printf(" 503 Circuit Breaker : %d (%.1f%%)\n", cr503, (float64(cr503)/float64(total))*100.0)
	fmt.Printf(" Other/Failed Errs   : %d (%.1f%%)\n", errs, (float64(errs)/float64(total))*100.0)
	fmt.Println("-------------------------------------------------------")
	fmt.Println(" Latency Distribution (Successful 200 OK):")
	fmt.Printf("   p50 (Median)      : %v\n", successLatencies.Percentile(0.50))
	fmt.Printf("   p90               : %v\n", successLatencies.Percentile(0.90))
	fmt.Printf("   p95               : %v\n", successLatencies.Percentile(0.95))
	fmt.Printf("   p99               : %v\n", successLatencies.Percentile(0.99))
	fmt.Println(" Latency Distribution (Fast-Fail Shed 429):")
	fmt.Printf("   p50 (Median)      : %v\n", shedLatencies.Percentile(0.50))
	fmt.Printf("   p99               : %v\n", shedLatencies.Percentile(0.99))
	fmt.Println("=======================================================")
}

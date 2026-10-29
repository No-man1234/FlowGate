# FlowGate 🛡️

[![CI](https://github.com/No-man1234/FlowGate/actions/workflows/ci.yml/badge.svg)](https://github.com/No-man1234/FlowGate/actions)
[![Go Version](https://img.shields.io/badge/go-1.23%2B-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

**FlowGate** is a high-concurrency, self-tuning resilient reverse proxy and API gateway written in Go.

While enterprise service meshes like Envoy support adaptive concurrency via custom C++ filters, FlowGate is a lightweight, standalone, and embeddable Go implementation of the adaptive concurrency principles pioneered by **Netflix** (`concurrency-limits`) and **Envoy** (`adaptive_concurrency`).

Instead of relying on fragile, static human-configured rate limits (`max_connections = 500`), FlowGate acts as an autonomous shock absorber. It uses **control theory** (Netflix Gradient2) to continuously probe backend round-trip time ($RTT$) and dynamically adjust in-flight request capacity in real time.

When upstream services experience queueing or brownouts, FlowGate instantly sheds excess traffic with sub-microsecond fast-fail responses (`429 Too Many Requests` / `503 Service Unavailable`), protecting the backend from catastrophic failure while keeping $p95$ latencies pristine.

For complete architectural details, queuing theory, and threat models, see [docs/design.md](docs/design.md).

---

## Architecture & System Design

```mermaid
flowchart TD
    Clients["High-Volume Concurrent Requests"] -->|HTTP / Keep-Alive| Listener["Tuned HTTP Listener"]
    
    subgraph FlowGate ["FlowGate Resilient Gateway"]
        Listener --> Graceful["Graceful Shutdown Hook (SIGINT/SIGTERM)"]
        Graceful --> Breaker{"Circuit Breaker (Closed?)"}
        
        Breaker -->|"Open (Severe 5xx)"| Fast503["503 Upstream Circuit Tripped (<0.1ms)"]
        Breaker -->|"Closed / Half-Open"| Limiter{"Adaptive Concurrency Limiter"}
        
        Limiter -->|"In-Flight >= Limit (Shed)"| Fast429["429 Too Many Requests (67ns fast-fail)"]
        Limiter -->|"In-Flight < Capacity (Priority Tier)"| Proxy["httputil.ReverseProxy (Tuned Pool)"]
        
        Proxy --> LatencyProbe["Latency & Error Measurement"]
        LatencyProbe -.->|"Feedback Loop (EWMA RTT)"| Limiter
        LatencyProbe -.->|"Health Feedback"| Breaker
    end
    
    Proxy --> Upstream["Upstream Target Service"]
```

---

## Core Capabilities

### 1. Adaptive Concurrency Limiting (Netflix Gradient2)
FlowGate implements an adaptation of the **Netflix Gradient2** algorithm based on Little's Law:

$$\text{rawGradient} = \left(\frac{RTT_{\text{min}}}{RTT_{\text{sample}}}\right) \times \text{Tolerance}$$

$$\text{clampedGradient} = \max\left(0.5, \min\left(1.0, \text{rawGradient}\right)\right)$$

$$\text{Limit}_{\text{next}} = \text{Limit}_{\text{current}} \times \text{clampedGradient} + \sqrt{\text{Limit}_{\text{current}}}$$

* **$RTT_{\text{min}}$**: Minimum baseline latency observed over a moving evaluation window.
* **$RTT_{\text{sample}}$**: Exponentially Weighted Moving Average (EWMA) of current response times.
* **$\text{Tolerance}$ (default: $1.25$)**: Allows normal network jitter up to 25% above baseline before capacity contracts.
* **$\sqrt{\text{Limit}}$**: Additive increase probing factor to safely discover recovered upstream throughput.

### 2. Priority-Tiered Load Shedding
When capacity contracts, FlowGate does not reject requests indiscriminately. It uses tiered headroom:

* **Critical (`X-Priority: critical`)**: Allowed up to 100% capacity (payments, authentication, checkout).
* **Standard (`X-Priority: standard`)**: Allowed up to 85% capacity (normal browsing, search).
* **Best-Effort (`X-Priority: best-effort`)**: Shed first when in-flight reaches 70% capacity (analytics, webhooks, prefetching).

> **Security Note:** In production, client-supplied priority headers must be stripped at the public edge or verified with authenticated tokens (`-trust-priority=false` by default).

### 3. Fast-Fail Circuit Breaker
If an upstream service suffers repeated crashes ($5\text{xx}$ errors or network timeouts), the circuit breaker trips `OPEN`, rejecting traffic in **$< 0.1\text{ms}$** without touching the upstream network stack. After a cooldown window, it shifts to `HALF-OPEN` to safely probe recovery.

Gateway-generated `429` shed responses never count toward upstream circuit breaker trips.

### 4. Lock-Free Atomic Fast-Path
Evaluation of limits and counters relies entirely on Go's `sync/atomic` primitives and cache-line aligned structures.

#### Microbenchmark Results (`go test -bench=. -benchmem ./pkg/limiter`)
*Measured on AMD Ryzen 7 5800U (Go 1.27):*

| Operation | Time / Op | Memory / Op | Allocations |
| :--- | :--- | :--- | :--- |
| **Shed Path (429 Rejection)** | **67.36 ns/op** | **0 B/op** | **0 allocs/op** |
| **Admitted Path (Acquire + Done)** | **187.10 ns/op** | **24 B/op** | **1 allocs/op** |

---

## Quickstart

### Prerequisites
* Go 1.23+

### 1. Clone & Build
```bash
git clone https://github.com/No-man1234/FlowGate.git
cd FlowGate
go build -o bin/ ./cmd/...
```

### 2. Run the Mock Upstream Server
Simulates a backend service with a baseline latency of 15ms that degrades if overloaded beyond 50 concurrent requests:
```bash
go run ./cmd/upstream -port 9090 -latency 15 -capacity 50
```

### 3. Launch FlowGate
Starts the resilient gateway on port `8080`, proxying traffic to the upstream service with the live dashboard enabled:
```bash
go run ./cmd/flowgate -port 8080 -upstream http://127.0.0.1:9090 -trust-priority=true
```

```text
======================================================================
 [FlowGate] Resilient Gateway Status                           
======================================================================
 Upstream Circuit : HEALTHY [CLOSED]      In-Flight: 48 / 52 (Limit)
 Traffic Inflow   : 4,820.0 req/sec       Min RTT  : 14.80 ms
 Shedding Rate    : 2,340.0 req/sec       SampleRTT: 18.20 ms
 Total Handled    : 48,200                Total Shed: 23,400 (48.5%)
 Upstream 5xx     : 0                     Breaker Trips: 0
======================================================================
```

### 4. Run the Chaos Benchmark
Fire 200 concurrent worker loops sending a mix of Critical, Standard, and Best-Effort requests (generating 50,000+ RPS):
```bash
go run ./cmd/loadgen -url http://127.0.0.1:8080/work -concurrency 200 -duration 10s
```

You can dynamically inject chaos into the upstream server in another terminal:
```bash
# Inject a 300ms latency spike for 10 seconds:
curl.exe "http://127.0.0.1:9090/chaos/spike?duration=10s&latency=300ms"

# Inject 50% simulated database panics:
curl.exe "http://127.0.0.1:9090/chaos/error?rate=0.5"

# Reset chaos:
curl.exe "http://127.0.0.1:9090/chaos/reset"
```

Watch FlowGate contract its concurrency limits in real-time, protect the backend from dying, and recover automatically.

---

## Testing & Benchmarks

Run unit tests with Go's data race detector:
```bash
go test -v -race -cover ./...
```

Run microbenchmarks:
```bash
go test -bench=. -benchmem ./pkg/...
```

---

## License

This project is licensed under the [MIT License](LICENSE).

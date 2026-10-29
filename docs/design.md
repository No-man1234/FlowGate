# FlowGate Architecture & Design Specification

## 1. Problem Statement: The Fallacy of Static Concurrency Limits

Most modern microservices rely on static limits (e.g., `max_connections: 500` or rate limits like `1000 req/s`). These static thresholds fail under real-world production conditions due to **Little's Law**:

$$L = \lambda W$$

Where:
* $L$ is the number of concurrent requests executing in the system.
* $\lambda$ is the arrival rate (throughput in req/sec).
* $W$ is the average response time (latency).

When downstream systems (such as a database or third-party API) degrade—increasing latency $W$ from 15ms to 150ms—a server receiving the same arrival rate $\lambda$ must maintain **10x more concurrent connections** ($L$) to keep up. 

If concurrency is unmanaged:
1. Thread/goroutine pools saturate.
2. Connection pools to databases exhaust.
3. Memory buffers balloon, triggering GC pauses and socket exhaustion.
4. Callers experience client-side timeouts and execute retries, creating an uncontrollable **retry storm** (thundering herd).

FlowGate treats concurrency limit discovery not as a static configuration problem, but as a continuous **closed-loop feedback control problem**.

---

## 2. Mathematical Formulation: Netflix Gradient2

FlowGate implements an adaptation of the **Netflix Gradient2** algorithm, which relates observed latency drift to queuing build-up.

### The Algorithm

1. **Baseline Tracking ($RTT_{\text{min}}$):**
   Represents the latency of the upstream service when it is completely unloaded (pure computation + network propagation without queueing).
2. **Current Smoothing ($RTT_{\text{sample}}$):**
   An Exponentially Weighted Moving Average (EWMA) of observed round-trip times:
   $$RTT_{\text{sample}} = \alpha \times RTT_{\text{observed}} + (1 - \alpha) \times RTT_{\text{sample}}$$
   Where $\alpha \in (0, 1]$ (default: $0.2$).
3. **Gradient Calculation:**
   $$\text{rawGradient} = \left(\frac{RTT_{\text{min}}}{RTT_{\text{sample}}}\right) \times \text{Tolerance}$$
   $$\text{clampedGradient} = \max\left(0.5, \min\left(1.0, \text{rawGradient}\right)\right)$$
4. **Limit Adjustment:**
   $$\text{Limit}_{\text{new}} = \text{Limit}_{\text{current}} \times \text{clampedGradient} + \sqrt{\text{Limit}_{\text{current}}}$$

### Why These Parameters Matter

* **`Tolerance` (default: 1.25):** 
  Because $RTT_{\text{sample}} \ge RTT_{\text{min}}$, the raw ratio $\frac{RTT_{\text{min}}}{RTT_{\text{sample}}}$ is always $\le 1.0$. Without multiplying by `Tolerance`, normal micro-jitter would trigger immediate contraction. A tolerance of `1.25` allows normal latency to drift up to 25% above baseline without dropping capacity.
* **`clamp(0.5, 1.0)`:** 
  Limits downward adjustment to at most 50% in a single calculation step, preventing an isolated outlier from abruptly reducing capacity to minimum floor.
* **`queueSize = sqrt(Limit)`:** 
  The additive increase term (probing factor). It allows the limiter to continuously probe for extra throughput when the upstream service scales or recovers.

---

## 3. Mitigating Stale $RTT_{\text{min}}$ (Baseline Drift)

A fundamental failure mode in latency-based limiters is **stale baseline lock**:
* If $RTT_{\text{min}}$ is fixed at 10ms, and the backend permanently migrates or deploys new features that shift legitimate baseline processing time to 25ms, the limiter would perceive all future traffic as overloaded and permanently throttle.
* Conversely, if $RTT_{\text{min}}$ is reset naively during a brownout, an anomalous 500ms latency spike could become the new baseline, blinding the limiter to queueing.

### FlowGate Solution: Windowed Candidate Minimums
FlowGate tracks a `candidateMinRTT` within an evaluation window (default: 30 seconds):
* Every incoming response updates `candidateMinRTT` if it is lower.
* When the window expires, `RTT_min` shifts to the minimum observed during that window.
* This allows FlowGate to track legitimate infrastructure changes while rejecting brief latency spikes from corrupting the baseline.

---

## 4. Threat Model: Priority Spoofing (`X-Priority`)

### Vulnerability
FlowGate supports priority-tiered load shedding:
* `Critical`: 100% capacity reserved (payments, checkout).
* `Standard`: 85% capacity reserved.
* `Best-Effort`: 70% capacity reserved (background jobs, webhooks).

If client requests are exposed directly to the public internet, malicious or selfish clients could send `X-Priority: critical` on every request to bypass shedding and starve legitimate users.

### Mitigation Architecture
* **`TrustClientPriority` Flag (default: `false`):** 
  By default, FlowGate ignores client-supplied `X-Priority` headers and classifies all incoming requests as `PriorityStandard`.
* **Trusted Edge Deployment:** 
  In production, FlowGate must be placed behind an authenticated edge gateway (or WAF) that either:
  1. Strips all `X-Priority` headers from public internet clients.
  2. Sets `X-Priority` based on authenticated API keys, user tiers, or route paths (e.g. `/api/v1/checkout` -> `critical`, `/api/v1/search` -> `standard`).

---

## 5. Failure Isolation: Gateway Sheds vs. Circuit Breaker

A critical bug in naïve proxy implementations is conflating **gateway load shedding** with **upstream failure**:
* If FlowGate rejects a request with `429 Too Many Requests`, that rejection is generated by FlowGate itself in $< 0.1\mu\text{s}$.
* It **never touches the upstream service** and **never increments the circuit breaker failure counter**.
* The Circuit Breaker only counts actual upstream dial failures, socket drops, or $5\text{xx}$ responses originating from the target server.

---

## 6. HTTP Status Code Choices: 429 vs 503

* **`429 Too Many Requests` (RFC 6585):** Used for adaptive concurrency shedding. Indicates the client should throttle its rate, accompanied by a `Retry-After: 1` header.
* **`503 Service Unavailable` (RFC 9110):** Reserved for when the Circuit Breaker trips `OPEN` due to hard upstream service death.

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

## 2. Algorithm Taxonomy: Gradient with Windowed Min-RTT

### Nomenclature & Netflix Lineage
In Netflix's `concurrency-limits` library:
* **`GradientLimit` (v1):** Compared short-term RTT against a windowed minimum RTT ($RTT_{\text{min}}$).
* **`Gradient2Limit` (v2):** Compared a short-term EWMA against a long-term EWMA, tracking downward drift without storing discrete minimums.

**FlowGate implements a Gradient-style controller with a windowed min-RTT baseline**, enhanced with an **anti-ratchet filter** and an **app-limited guard**:

### Mathematical Formulation

1. **Baseline Tracking ($RTT_{\text{min}}$):**
   Tracks the minimum latency observed over a 30s evaluation window. To prevent sustained queuing from corrupting the baseline, only samples taken during unqueued conditions ($\text{InFlight} \le 0.5 \times \text{Limit}$) are eligible as candidate minimums.
2. **Current Smoothing ($RTT_{\text{sample}}$):**
   An Exponentially Weighted Moving Average (EWMA) of observed round-trip times:
   $$RTT_{\text{sample}} = \alpha \times RTT_{\text{observed}} + (1 - \alpha) \times RTT_{\text{sample}}$$
   Where $\alpha = 0.2$.
3. **Gradient Calculation:**
   $$\text{rawGradient} = \left(\frac{RTT_{\text{min}}}{RTT_{\text{sample}}}\right) \times \text{Tolerance}$$
   $$\text{clampedGradient} = \max\left(0.5, \min\left(1.0, \text{rawGradient}\right)\right)$$
4. **App-Limited Probing & Limit Adjustment:**
   $$\text{Limit}_{\text{new}} = \text{Limit}_{\text{current}} \times \text{clampedGradient} + \text{Probe}$$
   Where $\text{Probe} = \sqrt{\text{Limit}_{\text{current}}}$ **only if** $\text{InFlight} \ge 0.5 \times \text{Limit}_{\text{current}}$ and $\text{clampedGradient} \ge 1.0$.

### Why These Guards Matter

* **Anti-Ratchet Filter (Mitigating Sustained Queuing):** 
  If a backend sits at its limit and queues for an entire 30s window, raw samples will all be inflated. If a naive candidate window adopted these inflated samples, $RTT_{\text{min}}$ would rise, resetting the gradient to 1.0 and causing runaway queueing. FlowGate filters out samples taken when the system is operating near its ceiling ($\text{InFlight} > 0.5 \times \text{Limit}$).
* **App-Limited Guard (Preventing Idle Inflation):** 
  If incoming traffic is low (e.g. 2 req/s on a limit of 100), the system is app-limited, not saturated. Adding $+\sqrt{\text{Limit}}$ during idle periods would cause the limit to drift to infinity. Probing is only active when traffic is actively testing capacity.
* **`Tolerance` (default: 1.25):** 
  Because $RTT_{\text{sample}} \ge RTT_{\text{min}}$, the raw ratio is always $\le 1.0$. Multiplying by `1.25` allows normal network jitter up to 25% above baseline without dropping capacity.

---

## 3. Threat Model: Priority Spoofing (`X-Priority`)

### Vulnerability
FlowGate supports priority-tiered load shedding:
* `Critical`: 100% capacity reserved (payments, checkout).
* `Standard`: 85% capacity reserved.
* `Best-Effort`: 70% capacity reserved (background jobs, webhooks).

If client requests are exposed directly to the public internet, malicious clients could send `X-Priority: critical` to bypass shedding and starve legitimate users.

### Mitigation Architecture
* **`TrustClientPriority` Flag (default: `false`):** 
  By default, FlowGate ignores client-supplied `X-Priority` headers.
* **Server-Side Path Classification:**
  FlowGate includes a default server-side classifier that maps path prefixes internally:
  * `/checkout`, `/payments`, `/auth`, `/orders` $\rightarrow$ `PriorityCritical`
  * `/analytics`, `/metrics`, `/logs`, `/webhooks` $\rightarrow$ `PriorityBestEffort`
  * All other routes $\rightarrow$ `PriorityStandard`
* **Custom Classifier Hook:** Developers can supply a `ClassifierFunc` to extract priority from authenticated JWT claims or API keys.

---

## 4. Failure Isolation: Gateway Sheds vs. Circuit Breaker

FlowGate strictly separates gateway-level load shedding from upstream infrastructure failure:
* When FlowGate rejects an incoming request with `429 Too Many Requests`, the rejection is generated entirely by FlowGate in **$< 0.25\mu\text{s}$**.
* It **never dials the upstream service** and **never increments the circuit breaker failure counter**.
* The Circuit Breaker only trips upon actual upstream dial failures, socket drops, or upstream $5\text{xx}$ responses originating from the backend.
* Enforced by unit test: `TestResilientGateway_Shed429DoesNotTripBreaker`.

---

## 5. Streaming, SSE, and WebSocket Protocols

**Important limitation:** Long-lived streaming protocols (Server-Sent Events, WebSockets, gRPC bidirectional streams) hold connections open for minutes or hours. 
* Measuring request-completion RTT on streaming endpoints produces artificial durations equal to the stream lifetime, which would instantly crush the adaptive limiter's capacity to minimum.
* **Production Recommendation:** Streaming endpoints must either:
  1. Bypass the adaptive concurrency limiter entirely.
  2. Measure **Time to First Byte (TTFB)** upon handshake completion rather than connection close.

---

## 6. HTTP Status Code Choices: 429 vs 503

* **`429 Too Many Requests` (RFC 6585):** Used for adaptive concurrency shedding. Indicates the client should back off and includes a `Retry-After: 1` header.
* **`503 Service Unavailable` (RFC 9110):** Reserved for when the Circuit Breaker trips `OPEN` due to hard upstream service outages.

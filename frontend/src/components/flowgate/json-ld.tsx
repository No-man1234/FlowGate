export function FlowGateJsonLd() {
  const schema = {
    "@context": "https://schema.org",
    "@graph": [
      {
        "@type": "SoftwareApplication",
        "@id": "https://flowgate-fire.vercel.app/#software",
        "name": "FlowGate",
        "url": "https://flowgate-fire.vercel.app",
        "applicationCategory": "DeveloperApplication",
        "operatingSystem": "Linux, macOS, Windows",
        "softwareVersion": "1.0.0",
        "description":
          "High-concurrency resilient API gateway with Netflix-style adaptive gradient concurrency limiting, 3-state circuit breaking, and priority-tiered backpressure in Go.",
        "offers": {
          "@type": "Offer",
          "price": "0",
          "priceCurrency": "USD",
        },
        "codeRepository": "https://github.com/No-man1234/FlowGate",
        "programmingLanguage": "Go",
        "license": "https://github.com/No-man1234/FlowGate/blob/main/LICENSE",
        "author": {
          "@type": "Person",
          "name": "Abdullah Al Noman",
          "url": "https://github.com/No-man1234",
        },
      },
      {
        "@type": "WebSite",
        "@id": "https://flowgate-fire.vercel.app/#website",
        "name": "FlowGate — Adaptive Concurrency Gateway",
        "url": "https://flowgate-fire.vercel.app",
        "description":
          "Resilience when upstream databases slow down. Stop database brownouts from becoming blackouts with adaptive gradient concurrency limiting in Go.",
        "publisher": {
          "@type": "Organization",
          "name": "FlowGate",
          "url": "https://flowgate-fire.vercel.app",
          "logo": "https://flowgate-fire.vercel.app/icons/icon-512x512.png",
        },
      },
      {
        "@type": "FAQPage",
        "@id": "https://flowgate-fire.vercel.app/#faq",
        "mainEntity": [
          {
            "@type": "Question",
            "name": "Why do traditional rate limiters fail during upstream database brownouts?",
            "acceptedAnswer": {
              "@type": "Answer",
              "text":
                "Traditional rate limiters count requests per second (e.g., 500 req/sec) using algorithms like token bucket or leaky bucket. They assume average response latency is constant (e.g., 20ms). When an upstream database stalls or locks and response times climb to 400ms+, requests stay in-flight 20x longer. Even though request rates remain within the allowed 500 req/sec, concurrent in-flight connections multiply exponentially, exhausting goroutines, socket file descriptors, and thread pools until the entire cluster experiences cascading collapse.",
            },
          },
          {
            "@type": "Question",
            "name": "How does FlowGate's adaptive gradient concurrency limiting work?",
            "acceptedAnswer": {
              "@type": "Answer",
              "text":
                "FlowGate measures latency instead of counting requests. Inspired by Netflix concurrency limits and Vegas congestion control, FlowGate tracks minRTT (the minimum observed round-trip latency when upstream is healthy) and measuredRTT (the smoothed latency of active requests). The gradient formula gradient = minRTT / measuredRTT calculates system pressure. When measuredRTT rises above baseline, the gradient drops below 1.0, immediately contracting the allowable concurrency window limit = limit * gradient + sqrt(limit) to shed load in sub-milliseconds before queues form.",
            },
          },
          {
            "@type": "Question",
            "name": "What is the overhead and performance cost of FlowGate on the hot path?",
            "acceptedAnswer": {
              "@type": "Answer",
              "text":
                "FlowGate was engineered for high-throughput zero-allocation performance in Go. In standard Go microbenchmarks (`go test -bench`), FlowGate's admission engine evaluates and admits requests in 147.3 ns/op with 0 B/op heap allocation. In empirical chaos testing with 100 concurrent workers under a simulated database brownout, FlowGate held p50 latency at 22.5ms (+61% goodput improvement over unprotected backends) and shed excess load via the fast-fail 429 path in under 747 microseconds.",
            },
          },
          {
            "@type": "Question",
            "name": "How does FlowGate prioritize critical requests over background traffic?",
            "acceptedAnswer": {
              "@type": "Answer",
              "text":
                "FlowGate supports priority-tiered backpressure. When concurrency approaches capacity, the gateway sheds requests deterministically based on tier headers: Best-effort traffic (/analytics, webhooks) is shed first at 75% capacity, standard user traffic (/api/v1) is protected up to 95% capacity, while critical requests (/checkout, payments) are granted a 1.2x queue allowance to ensure vital business transactions always succeed.",
            },
          },
          {
            "@type": "Question",
            "name": "How does the built-in 3-state circuit breaker protect failing dependencies?",
            "acceptedAnswer": {
              "@type": "Answer",
              "text":
                "FlowGate includes an automatic state machine (CLOSED, OPEN, HALF-OPEN). If an upstream service begins returning 5xx errors or connection timeouts exceeding the configured threshold (e.g., 4 consecutive failures), the circuit breaker transitions to OPEN. In the OPEN state, all requests fail fast with 503 Service Unavailable in <0.1ms without touching upstream sockets, allowing recovering databases time to heal. After a cool-off window, the breaker enters HALF-OPEN to safely probe upstream recovery with limited traffic.",
            },
          },
          {
            "@type": "Question",
            "name": "How can I deploy FlowGate in production or test it locally?",
            "acceptedAnswer": {
              "@type": "Answer",
              "text":
                "FlowGate is distributed as a single statically linked binary for Linux (amd64/arm64), macOS (Intel/Apple Silicon), and Windows. You can install it directly with `go install github.com/No-man1234/FlowGate/cmd/flowgate@latest` or run the complete multi-tier test environment locally using `docker compose up --build` which launches the FlowGate reverse proxy, simulated downstream services, and the real-time visualizer dashboard.",
            },
          },
        ],
      },
    ],
  };

  return (
    <script
      type="application/ld+json"
      dangerouslySetInnerHTML={{ __html: JSON.stringify(schema) }}
    />
  );
}

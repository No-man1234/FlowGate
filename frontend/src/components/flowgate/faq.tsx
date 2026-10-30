"use client";

import { useState } from "react";
import { ChevronDown, HelpCircle } from "lucide-react";

interface FAQItem {
  question: string;
  answer: string;
}

const faqs: FAQItem[] = [
  {
    question: "Why do traditional rate limiters fail during upstream database brownouts?",
    answer:
      "Traditional rate limiters count requests per second (e.g., 500 req/sec) using algorithms like token bucket or leaky bucket. They assume average response latency is constant (e.g., 20ms). When an upstream database stalls or locks and response times climb to 400ms+, requests stay in-flight 20x longer. Even though request rates remain within the allowed 500 req/sec, concurrent in-flight connections multiply exponentially, exhausting goroutines, socket file descriptors, and thread pools until the entire cluster experiences cascading collapse.",
  },
  {
    question: "How does FlowGate's adaptive gradient concurrency limiting work?",
    answer:
      "FlowGate measures latency instead of counting requests. Inspired by Netflix concurrency limits and Vegas congestion control, FlowGate tracks minRTT (the minimum observed round-trip latency when upstream is healthy) and measuredRTT (the smoothed latency of active requests). The gradient formula gradient = minRTT / measuredRTT calculates system pressure. When measuredRTT rises above baseline, the gradient drops below 1.0, immediately contracting the allowable concurrency window limit = limit * gradient + sqrt(limit) to shed load in sub-milliseconds before queues form.",
  },
  {
    question: "What is the overhead and performance cost of FlowGate on the hot path?",
    answer:
      "FlowGate was engineered for high-throughput zero-allocation performance in Go. In standard Go microbenchmarks (`go test -bench`), FlowGate's admission engine evaluates and admits requests in 147.3 ns/op with 0 B/op heap allocation. In empirical chaos testing with 100 concurrent workers under a simulated database brownout, FlowGate held p50 latency at 22.5ms (+61% goodput improvement over unprotected backends) and shed excess load via the fast-fail 429 path in under 747 microseconds.",
  },
  {
    question: "How does FlowGate prioritize critical requests over background traffic?",
    answer:
      "FlowGate supports priority-tiered backpressure. When concurrency approaches capacity, the gateway sheds requests deterministically based on tier headers: Best-effort traffic (/analytics, webhooks) is shed first at 75% capacity, standard user traffic (/api/v1) is protected up to 95% capacity, while critical requests (/checkout, payments) are granted a 1.2x queue allowance to ensure vital business transactions always succeed.",
  },
  {
    question: "How does the built-in 3-state circuit breaker protect failing dependencies?",
    answer:
      "FlowGate includes an automatic state machine (CLOSED, OPEN, HALF-OPEN). If an upstream service begins returning 5xx errors or connection timeouts exceeding the configured threshold (e.g., 4 consecutive failures), the circuit breaker transitions to OPEN. In the OPEN state, all requests fail fast with 503 Service Unavailable in <0.1ms without touching upstream sockets, allowing recovering databases time to heal. After a cool-off window, the breaker enters HALF-OPEN to safely probe upstream recovery with limited traffic.",
  },
  {
    question: "How can I deploy FlowGate in production or test it locally?",
    answer:
      "FlowGate is distributed as a single statically linked binary for Linux (amd64/arm64), macOS (Intel/Apple Silicon), and Windows. You can install it directly with `go install github.com/No-man1234/FlowGate/cmd/flowgate@latest` or run the complete multi-tier test environment locally using `docker compose up --build` which launches the FlowGate reverse proxy, simulated downstream services, and the real-time visualizer dashboard.",
  },
];

export function FlowGateFAQ() {
  const [openIndex, setOpenIndex] = useState<number | null>(0);

  const toggle = (index: number) => {
    setOpenIndex(openIndex === index ? null : index);
  };

  return (
    <section id="faq" className="flex flex-col gap-8 py-12 border-b border-[var(--line)] scroll-mt-8">
      <div className="flex flex-col gap-2 max-w-2xl">
        <span className="text-xs uppercase tracking-wider font-semibold text-emerald-600 dark:text-emerald-400 font-mono flex items-center gap-1.5">
          <HelpCircle className="w-3.5 h-3.5" />
          Frequently Asked Questions
        </span>
        <h2 className="text-2xl md:text-3xl font-bold tracking-tight text-[var(--fg)]">
          Everything you need to know about adaptive concurrency.
        </h2>
        <p className="text-sm md:text-base text-[var(--muted)] leading-relaxed">
          Common architectural questions regarding rate limiting alternatives, brownout resilience, and Go microservice performance.
        </p>
      </div>

      <div className="flex flex-col divide-y divide-[var(--line)] border border-[var(--line)] rounded-2xl bg-[var(--bg)] overflow-hidden">
        {faqs.map((faq, index) => {
          const isOpen = openIndex === index;
          return (
            <div key={index} className="transition-colors hover:bg-[var(--chip)]">
              <button
                type="button"
                onClick={() => toggle(index)}
                aria-expanded={isOpen}
                className="w-full text-left py-4 px-5 sm:px-6 flex items-center justify-between gap-4 cursor-pointer"
              >
                <span className="font-semibold text-sm sm:text-base text-[var(--fg)]">
                  {faq.question}
                </span>
                <ChevronDown
                  className={`w-4 h-4 text-[var(--muted)] shrink-0 transition-transform duration-200 ${
                    isOpen ? "rotate-180 text-[var(--fg)]" : ""
                  }`}
                />
              </button>
              {isOpen && (
                <div className="px-5 sm:px-6 pb-5 pt-1 text-xs sm:text-sm text-[var(--muted)] leading-relaxed border-t border-[var(--line)]/50 bg-[var(--chip)]/30">
                  {faq.answer}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </section>
  );
}

"use client";

import { Copy, Terminal } from "lucide-react";
import { useState } from "react";

export function FlowGateQuickstart() {
  const [copiedIndex, setCopiedIndex] = useState<number | null>(null);

  const copyCode = (text: string, index: number) => {
    navigator.clipboard.writeText(text);
    setCopiedIndex(index);
    setTimeout(() => setCopiedIndex(null), 2000);
  };

  const cliSnippet = `# Clone repository
git clone https://github.com/No-man1234/FlowGate.git
cd FlowGate

# Start the Mock Upstream Server (injects latency & chaos)
go run cmd/upstream/main.go -port 9000

# Start FlowGate Gateway with adaptive concurrency protection
go run cmd/flowgate/main.go -upstream http://127.0.0.1:9000 -port 8080`;

  const goSnippet = `package main

import (
    "net/http"
    "net/url"
    "github.com/No-man1234/FlowGate/pkg/breaker"
    "github.com/No-man1234/FlowGate/pkg/limiter"
    "github.com/No-man1234/FlowGate/pkg/metrics"
    "github.com/No-man1234/FlowGate/pkg/proxy"
)

func main() {
    target, _ := url.Parse("http://internal-backend:8080")

    gateway := proxy.NewGateway(
        proxy.GatewayConfig{UpstreamURL: target},
        limiter.NewGradientLimiter(limiter.DefaultConfig()),
        breaker.New(breaker.DefaultConfig()),
        metrics.NewCollector(),
    )

    http.ListenAndServe(":8080", gateway)
}`;

  return (
    <section id="quickstart" className="flex flex-col gap-8 py-12 border-b border-[var(--line)] scroll-mt-8">
      <div className="flex flex-col gap-2 max-w-2xl">
        <span className="text-xs uppercase tracking-wider font-semibold text-emerald-600 dark:text-emerald-400 font-mono">
          Developer Quickstart
        </span>
        <h2 className="text-2xl md:text-3xl font-bold tracking-tight text-[var(--fg)]">
          Get running in 30 seconds.
        </h2>
        <p className="text-sm md:text-base text-[var(--muted)] leading-relaxed">
          Run FlowGate as a standalone reverse proxy container or embed it directly into your Go microservices as an <code className="font-mono text-xs px-1.5 py-0.5 rounded bg-[var(--chip)]">http.Handler</code>.
        </p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* CLI Snippet */}
        <div className="rounded-2xl border border-[var(--line)] bg-[var(--chip)] p-5 flex flex-col gap-3">
          <div className="flex items-center justify-between text-xs text-[var(--muted)]">
            <span className="font-semibold text-[var(--fg)] flex items-center gap-1.5 font-mono">
              <Terminal className="w-4 h-4 text-emerald-500" />
              1. Standalone Gateway Proxy
            </span>
            <button
              type="button"
              onClick={() => copyCode(cliSnippet, 1)}
              className="text-[11px] hover:text-[var(--fg)] flex items-center gap-1 text-[var(--muted)] cursor-pointer"
            >
              <Copy className="w-3 h-3" />
              {copiedIndex === 1 ? "Copied!" : "Copy"}
            </button>
          </div>

          <div className="p-4 rounded-xl bg-[var(--bg)] border border-[var(--line)] text-xs font-mono overflow-x-auto leading-relaxed shadow-xs flex flex-col gap-1 text-[var(--fg)]">
            <div className="text-[var(--muted)]"># 1. Clone repository</div>
            <div>
              <span className="text-emerald-700 dark:text-emerald-400 font-semibold">git clone</span>{" "}
              https://github.com/No-man1234/FlowGate.git
            </div>
            <div>
              <span className="text-emerald-700 dark:text-emerald-400 font-semibold">cd</span> FlowGate
            </div>

            <div className="mt-2 text-[var(--muted)]"># 2. Start mock upstream service (simulates latency &amp; chaos)</div>
            <div>
              <span className="text-emerald-700 dark:text-emerald-400 font-semibold">go run</span>{" "}
              cmd/upstream/main.go -port 9000
            </div>

            <div className="mt-2 text-[var(--muted)]"># 3. Start FlowGate Gateway with adaptive concurrency</div>
            <div>
              <span className="text-emerald-700 dark:text-emerald-400 font-semibold">go run</span>{" "}
              cmd/flowgate/main.go -upstream http://127.0.0.1:9000 -port 8080
            </div>
          </div>
        </div>

        {/* Go Code Snippet */}
        <div className="rounded-2xl border border-[var(--line)] bg-[var(--chip)] p-5 flex flex-col gap-3">
          <div className="flex items-center justify-between text-xs text-[var(--muted)]">
            <span className="font-semibold text-[var(--fg)] flex items-center gap-1.5 font-mono">
              <Terminal className="w-4 h-4 text-cyan-500" />
              2. Embed as Go Middleware
            </span>
            <button
              type="button"
              onClick={() => copyCode(goSnippet, 2)}
              className="text-[11px] hover:text-[var(--fg)] flex items-center gap-1 text-[var(--muted)] cursor-pointer"
            >
              <Copy className="w-3 h-3" />
              {copiedIndex === 2 ? "Copied!" : "Copy"}
            </button>
          </div>

          <div className="p-4 rounded-xl bg-[var(--bg)] border border-[var(--line)] text-xs font-mono overflow-x-auto leading-relaxed shadow-xs flex flex-col gap-0.5 text-[var(--fg)]">
            <div>
              <span className="text-purple-700 dark:text-purple-400 font-semibold">package</span> main
            </div>
            <div className="mt-1">
              <span className="text-purple-700 dark:text-purple-400 font-semibold">import</span> (
            </div>
            <div className="pl-4 text-emerald-700 dark:text-emerald-400">&quot;net/http&quot;</div>
            <div className="pl-4 text-emerald-700 dark:text-emerald-400">&quot;net/url&quot;</div>
            <div className="pl-4 text-emerald-700 dark:text-emerald-400">
              &quot;github.com/No-man1234/FlowGate/pkg/breaker&quot;
            </div>
            <div className="pl-4 text-emerald-700 dark:text-emerald-400">
              &quot;github.com/No-man1234/FlowGate/pkg/limiter&quot;
            </div>
            <div className="pl-4 text-emerald-700 dark:text-emerald-400">
              &quot;github.com/No-man1234/FlowGate/pkg/metrics&quot;
            </div>
            <div className="pl-4 text-emerald-700 dark:text-emerald-400">
              &quot;github.com/No-man1234/FlowGate/pkg/proxy&quot;
            </div>
            <div>)</div>

            <div className="mt-1">
              <span className="text-purple-700 dark:text-purple-400 font-semibold">func</span>{" "}
              <span className="text-blue-700 dark:text-blue-400 font-semibold">main</span>() &#123;
            </div>
            <div className="pl-4">
              target, _ := url.Parse(
              <span className="text-emerald-700 dark:text-emerald-400">
                &quot;http://internal-backend:8080&quot;
              </span>
              )
            </div>

            <div className="pl-4 mt-1">gateway := proxy.NewGateway(</div>
            <div className="pl-8">proxy.GatewayConfig&#123;UpstreamURL: target&#125;,</div>
            <div className="pl-8">limiter.NewGradientLimiter(limiter.DefaultConfig()),</div>
            <div className="pl-8">breaker.New(breaker.DefaultConfig()),</div>
            <div className="pl-8">metrics.NewCollector(),</div>
            <div className="pl-4">)</div>

            <div className="pl-4 mt-1">
              http.ListenAndServe(
              <span className="text-emerald-700 dark:text-emerald-400">&quot;:8080&quot;</span>, gateway)
            </div>
            <div>&#125;</div>
          </div>
        </div>
      </div>
    </section>
  );
}

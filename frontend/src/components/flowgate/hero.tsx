import { ArrowDown, Zap } from "lucide-react";
import Link from "next/link";
import { GithubIcon } from "./icons";

export function FlowGateHero() {
  return (
    <section className="flex flex-col gap-8 py-8 md:py-12 border-b border-[var(--line)]">
      {/* Badge */}
      <div className="flex items-center gap-2">
        <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium bg-[var(--chip)] text-[var(--fg2)] border border-[var(--line)]">
          <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse" />
          Production-Grade Resilient Gateway in Go
        </span>
      </div>

      {/* Main Headline */}
      <div className="flex flex-col gap-4 max-w-3xl">
        <h1 className="text-3xl md:text-5xl font-bold tracking-tight text-[var(--fg)] leading-[1.12]">
          Resilience when upstream databases slow down.
        </h1>
        <p className="text-base md:text-lg text-[var(--muted)] leading-relaxed">
          Traditional rate limiters count requests per second; FlowGate measures round-trip latency.
          When downstream services queue or degrade, FlowGate&apos;s gradient controller dynamically
          shrinks concurrency limits in real time—shedding non-critical requests with 429 backpressure
          before thread pools exhaust and servers crash.
        </p>
      </div>

      {/* Action Buttons */}
      <div className="flex flex-wrap items-center gap-3 pt-2">
        <a
          href="#testbench"
          className="btn-primary inline-flex items-center gap-2 px-5 py-2.5 rounded-full font-medium text-sm no-underline hover:no-underline hover:opacity-90 transition-opacity shadow-xs text-[var(--bg)] bg-[var(--fg)]"
          style={{ color: "var(--bg)", backgroundColor: "var(--fg)", textDecoration: "none" }}
        >
          <Zap className="w-4 h-4 fill-current" />
          Launch Interactive Testbench
          <ArrowDown className="w-3.5 h-3.5" />
        </a>

        <Link
          href="https://github.com/No-man1234/FlowGate"
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-2 px-5 py-2.5 rounded-full bg-[var(--chip)] text-[var(--fg)] font-medium text-sm border border-[var(--line)] hover:border-[var(--fg)] transition-all hover:no-underline"
        >
          <GithubIcon className="w-4 h-4" />
          View on GitHub
          <span className="text-xs text-[var(--faint)]">↗</span>
        </Link>
      </div>

      {/* Metric Cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 pt-4">
        <div className="p-4 rounded-xl border border-[var(--line)] bg-[var(--bg)] flex flex-col gap-1">
          <span className="text-[11px] uppercase tracking-wider text-[var(--muted)] font-mono">
            Limiter Hot Path
          </span>
          <span className="text-2xl font-bold font-mono text-emerald-600 dark:text-emerald-400">
            147 ns
          </span>
          <span className="text-xs text-[var(--faint)]">
            7.6M ops/sec on AMD Ryzen
          </span>
        </div>

        <div className="p-4 rounded-xl border border-[var(--line)] bg-[var(--bg)] flex flex-col gap-1">
          <span className="text-[11px] uppercase tracking-wider text-[var(--muted)] font-mono">
            Shed Path Overhead
          </span>
          <span className="text-2xl font-bold font-mono text-[var(--fg)]">
            0 B / op
          </span>
          <span className="text-xs text-[var(--faint)]">
            Zero-allocation fast rejection
          </span>
        </div>

        <div className="p-4 rounded-xl border border-[var(--line)] bg-[var(--bg)] flex flex-col gap-1">
          <span className="text-[11px] uppercase tracking-wider text-[var(--muted)] font-mono">
            Circuit Breaker
          </span>
          <span className="text-2xl font-bold font-mono text-blue-600 dark:text-blue-400">
            &lt;0.1 ms
          </span>
          <span className="text-xs text-[var(--faint)]">
            Fast-fails 503 during outages
          </span>
        </div>

        <div className="p-4 rounded-xl border border-[var(--line)] bg-[var(--bg)] flex flex-col gap-1">
          <span className="text-[11px] uppercase tracking-wider text-[var(--muted)] font-mono">
            Concurrency Safety
          </span>
          <span className="text-2xl font-bold font-mono text-purple-600 dark:text-purple-400">
            100%
          </span>
          <span className="text-xs text-[var(--faint)]">
            Race-clean under Go -race
          </span>
        </div>
      </div>
    </section>
  );
}

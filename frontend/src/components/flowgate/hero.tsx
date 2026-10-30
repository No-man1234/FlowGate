"use client";

import { ArrowRight, Cpu, Flame, Gauge, HelpCircle, Layers, Terminal, Zap } from "lucide-react";
import Link from "next/link";
import { GithubIcon } from "./icons";
import { useNavigation } from "./navigation-context";

export function FlowGateHero() {
  const { setActivePage } = useNavigation();

  return (
    <section className="flex flex-col gap-10 py-8 md:py-12">
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
      <div className="flex flex-wrap items-center gap-3 pt-1">
        <button
          type="button"
          onClick={() => setActivePage("testbench")}
          className="btn-primary inline-flex items-center gap-2 px-5 py-2.5 rounded-full font-medium text-sm no-underline hover:no-underline hover:opacity-90 transition-opacity shadow-xs text-[var(--bg)] bg-[var(--fg)] cursor-pointer"
          style={{ color: "var(--bg)", backgroundColor: "var(--fg)" }}
        >
          <Zap className="w-4 h-4 fill-current text-emerald-400" />
          Launch Interactive Testbench
          <ArrowRight className="w-3.5 h-3.5" />
        </button>

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
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3 pt-2">
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

      {/* Explore FlowGate Chapters / Navigation Hub */}
      <div className="flex flex-col gap-4 pt-4">
        <div className="flex items-center justify-between">
          <span className="text-xs uppercase tracking-wider font-semibold text-[var(--muted)] font-mono">
            Explore Chapters &amp; Architecture
          </span>
          <span className="text-xs text-[var(--faint)]">
            Select any chapter or use top tabs
          </span>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {/* Card 1: Interactive Testbench */}
          <button
            type="button"
            onClick={() => setActivePage("testbench")}
            className="group p-5 rounded-2xl border border-[var(--line)] bg-[var(--chip)] hover:border-emerald-500/50 hover:bg-emerald-500/5 text-left flex flex-col justify-between gap-3 transition-all cursor-pointer hover:no-underline"
          >
            <div className="flex items-center justify-between">
              <div className="w-9 h-9 rounded-xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-600 dark:text-emerald-400">
                <Gauge className="w-4 h-4" />
              </div>
              <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 font-semibold border border-emerald-500/20">
                Interactive
              </span>
            </div>
            <div>
              <h3 className="font-semibold text-sm text-[var(--fg)] group-hover:text-emerald-600 dark:group-hover:text-emerald-400 transition-colors flex items-center gap-1.5">
                Interactive Testbench
                <ArrowRight className="w-3.5 h-3.5 opacity-0 group-hover:opacity-100 group-hover:translate-x-1 transition-all" />
              </h3>
              <p className="text-xs text-[var(--muted)] mt-1 leading-relaxed">
                Run live brownout chaos experiments, adjust concurrency sliders, and inspect real-time request logs.
              </p>
            </div>
          </button>

          {/* Card 2: The Problem */}
          <button
            type="button"
            onClick={() => setActivePage("problem")}
            className="group p-5 rounded-2xl border border-[var(--line)] bg-[var(--chip)] hover:border-rose-500/50 hover:bg-rose-500/5 text-left flex flex-col justify-between gap-3 transition-all cursor-pointer hover:no-underline"
          >
            <div className="flex items-center justify-between">
              <div className="w-9 h-9 rounded-xl bg-rose-500/10 border border-rose-500/20 flex items-center justify-center text-rose-600 dark:text-rose-400">
                <Flame className="w-4 h-4" />
              </div>
              <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/20">
                Brownouts
              </span>
            </div>
            <div>
              <h3 className="font-semibold text-sm text-[var(--fg)] group-hover:text-rose-600 dark:group-hover:text-rose-400 transition-colors flex items-center gap-1.5">
                The Problem
                <ArrowRight className="w-3.5 h-3.5 opacity-0 group-hover:opacity-100 group-hover:translate-x-1 transition-all" />
              </h3>
              <p className="text-xs text-[var(--muted)] mt-1 leading-relaxed">
                Why fixed request/sec rate limits fail when downstream databases stall, causing complete cascading blackouts.
              </p>
            </div>
          </button>

          {/* Card 3: How It Works */}
          <button
            type="button"
            onClick={() => setActivePage("how-it-works")}
            className="group p-5 rounded-2xl border border-[var(--line)] bg-[var(--chip)] hover:border-blue-500/50 hover:bg-blue-500/5 text-left flex flex-col justify-between gap-3 transition-all cursor-pointer hover:no-underline"
          >
            <div className="flex items-center justify-between">
              <div className="w-9 h-9 rounded-xl bg-blue-500/10 border border-blue-500/20 flex items-center justify-center text-blue-600 dark:text-blue-400">
                <Cpu className="w-4 h-4" />
              </div>
              <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-blue-500/10 text-blue-600 dark:text-blue-400 border border-blue-500/20">
                Gradient2
              </span>
            </div>
            <div>
              <h3 className="font-semibold text-sm text-[var(--fg)] group-hover:text-blue-600 dark:group-hover:text-blue-400 transition-colors flex items-center gap-1.5">
                How It Works
                <ArrowRight className="w-3.5 h-3.5 opacity-0 group-hover:opacity-100 group-hover:translate-x-1 transition-all" />
              </h3>
              <p className="text-xs text-[var(--muted)] mt-1 leading-relaxed">
                The mathematical foundation: Gradient concurrency formula, 3-state Circuit Breaker, and priority-tier queues.
              </p>
            </div>
          </button>

          {/* Card 4: Benchmarks */}
          <button
            type="button"
            onClick={() => setActivePage("benchmarks")}
            className="group p-5 rounded-2xl border border-[var(--line)] bg-[var(--chip)] hover:border-cyan-500/50 hover:bg-cyan-500/5 text-left flex flex-col justify-between gap-3 transition-all cursor-pointer hover:no-underline"
          >
            <div className="flex items-center justify-between">
              <div className="w-9 h-9 rounded-xl bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center text-cyan-600 dark:text-cyan-400">
                <Layers className="w-4 h-4" />
              </div>
              <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-cyan-500/10 text-cyan-600 dark:text-cyan-400 border border-cyan-500/20">
                147 ns / op
              </span>
            </div>
            <div>
              <h3 className="font-semibold text-sm text-[var(--fg)] group-hover:text-cyan-600 dark:group-hover:text-cyan-400 transition-colors flex items-center gap-1.5">
                Go Microbenchmarks
                <ArrowRight className="w-3.5 h-3.5 opacity-0 group-hover:opacity-100 group-hover:translate-x-1 transition-all" />
              </h3>
              <p className="text-xs text-[var(--muted)] mt-1 leading-relaxed">
                Empirical latency numbers, memory profile analysis, and head-to-head brownout throughput graphs.
              </p>
            </div>
          </button>

          {/* Card 5: FAQ */}
          <button
            type="button"
            onClick={() => setActivePage("faq")}
            className="group p-5 rounded-2xl border border-[var(--line)] bg-[var(--chip)] hover:border-purple-500/50 hover:bg-purple-500/5 text-left flex flex-col justify-between gap-3 transition-all cursor-pointer hover:no-underline"
          >
            <div className="flex items-center justify-between">
              <div className="w-9 h-9 rounded-xl bg-purple-500/10 border border-purple-500/20 flex items-center justify-center text-purple-600 dark:text-purple-400">
                <HelpCircle className="w-4 h-4" />
              </div>
              <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-purple-500/10 text-purple-600 dark:text-purple-400 border border-purple-500/20">
                Q&amp;A
              </span>
            </div>
            <div>
              <h3 className="font-semibold text-sm text-[var(--fg)] group-hover:text-purple-600 dark:group-hover:text-purple-400 transition-colors flex items-center gap-1.5">
                Architecture FAQ
                <ArrowRight className="w-3.5 h-3.5 opacity-0 group-hover:opacity-100 group-hover:translate-x-1 transition-all" />
              </h3>
              <p className="text-xs text-[var(--muted)] mt-1 leading-relaxed">
                Deep architectural explanations comparing Vegas gradient limits against token buckets, Envoy, and Envoy ratelimit.
              </p>
            </div>
          </button>

          {/* Card 6: Quickstart */}
          <button
            type="button"
            onClick={() => setActivePage("quickstart")}
            className="group p-5 rounded-2xl border border-[var(--line)] bg-[var(--chip)] hover:border-emerald-500/50 hover:bg-emerald-500/5 text-left flex flex-col justify-between gap-3 transition-all cursor-pointer hover:no-underline"
          >
            <div className="flex items-center justify-between">
              <div className="w-9 h-9 rounded-xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-600 dark:text-emerald-400">
                <Terminal className="w-4 h-4" />
              </div>
              <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
                Quick Setup
              </span>
            </div>
            <div>
              <h3 className="font-semibold text-sm text-[var(--fg)] group-hover:text-emerald-600 dark:group-hover:text-emerald-400 transition-colors flex items-center gap-1.5">
                Developer Quickstart
                <ArrowRight className="w-3.5 h-3.5 opacity-0 group-hover:opacity-100 group-hover:translate-x-1 transition-all" />
              </h3>
              <p className="text-xs text-[var(--muted)] mt-1 leading-relaxed">
                Single-command Go install, Docker compose multi-service test stack, and standard Go HTTP middleware integration.
              </p>
            </div>
          </button>
        </div>
      </div>
    </section>
  );
}

import { Cpu, Shield, Layers } from "lucide-react";

export function HowItWorks() {
  return (
    <section id="how-it-works" className="flex flex-col gap-8 py-12 border-b border-[var(--line)] scroll-mt-8">
      <div className="flex flex-col gap-2 max-w-2xl">
        <span className="text-xs uppercase tracking-wider font-semibold text-emerald-600 dark:text-emerald-400 font-mono">
          Architecture &amp; Algorithms
        </span>
        <h2 className="text-2xl md:text-3xl font-bold tracking-tight text-[var(--fg)]">
          The three pillars of FlowGate resilience.
        </h2>
        <p className="text-sm md:text-base text-[var(--muted)] leading-relaxed">
          FlowGate combines adaptive queueing theory with defensive socket engineering to provide autonomous backpressure.
        </p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {/* Pillar 1: Gradient Concurrency Controller */}
        <div className="p-6 rounded-2xl border border-[var(--line)] bg-[var(--bg)] flex flex-col gap-4">
          <div className="w-10 h-10 rounded-xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-600 dark:text-emerald-400">
            <Cpu className="w-5 h-5" />
          </div>

          <h3 className="text-lg font-semibold text-[var(--fg)]">
            1. Gradient2 Limiter
          </h3>

          <p className="text-xs text-[var(--muted)] leading-relaxed">
            Inspired by TCP Vegas and Netflix Concurrency Limits. Continuously tracks minimum baseline latency (minRTT).
          </p>

          <div className="p-3 rounded-xl bg-[var(--chip)] font-mono text-[11px] text-[var(--fg2)] flex flex-col gap-1.5 border border-[var(--line)]">
            <div className="text-emerald-600 dark:text-emerald-400 font-semibold">
              Gradient Formula
            </div>
            <div>g = min(Tolerance, minRTT / RTT)</div>
            <div>limit = limit * g + √limit</div>
            <div className="text-[var(--faint)] text-[10px] mt-1">
              • App-limited guard skips idle inflation<br />
              • Exponential smoothing eliminates jitter
            </div>
          </div>
        </div>

        {/* Pillar 2: 3-State Circuit Breaker */}
        <div className="p-6 rounded-2xl border border-[var(--line)] bg-[var(--bg)] flex flex-col gap-4">
          <div className="w-10 h-10 rounded-xl bg-blue-500/10 border border-blue-500/20 flex items-center justify-center text-blue-600 dark:text-blue-400">
            <Shield className="w-5 h-5" />
          </div>

          <h3 className="text-lg font-semibold text-[var(--fg)]">
            2. Three-State Circuit Breaker
          </h3>

          <p className="text-xs text-[var(--muted)] leading-relaxed">
            Protects downstream from hammering a completely broken upstream with cascading 5xx retries.
          </p>

          <div className="p-3 rounded-xl bg-[var(--chip)] font-mono text-[11px] text-[var(--fg2)] flex flex-col gap-2 border border-[var(--line)]">
            <div className="flex items-center justify-between">
              <span className="text-emerald-600 dark:text-emerald-400 font-semibold">CLOSED</span>
              <span className="text-[10px] text-[var(--muted)]">Healthy pass</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-rose-600 dark:text-rose-400 font-semibold">OPEN</span>
              <span className="text-[10px] text-[var(--muted)]">Fast-fail 503</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-amber-600 dark:text-amber-400 font-semibold">HALF-OPEN</span>
              <span className="text-[10px] text-[var(--muted)]">Cautious probe</span>
            </div>
          </div>
        </div>

        {/* Pillar 3: Priority-Tiered Shedding */}
        <div className="p-6 rounded-2xl border border-[var(--line)] bg-[var(--bg)] flex flex-col gap-4">
          <div className="w-10 h-10 rounded-xl bg-purple-500/10 border border-purple-500/20 flex items-center justify-center text-purple-600 dark:text-purple-400">
            <Layers className="w-5 h-5" />
          </div>

          <h3 className="text-lg font-semibold text-[var(--fg)]">
            3. Priority Classification
          </h3>

          <p className="text-xs text-[var(--muted)] leading-relaxed">
            Not all requests are created equal. FlowGate prioritizes business-critical traffic over background telemetry.
          </p>

          <div className="p-3 rounded-xl bg-[var(--chip)] font-mono text-[11px] text-[var(--fg2)] flex flex-col gap-2 border border-[var(--line)]">
            <div className="flex items-center justify-between">
              <span className="text-emerald-600 dark:text-emerald-400 font-semibold">Critical (/checkout)</span>
              <span className="text-[10px]">100% capacity</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-blue-600 dark:text-blue-400 font-semibold">Standard (/api)</span>
              <span className="text-[10px]">95% capacity</span>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-amber-600 dark:text-amber-400 font-semibold">Best-Effort (/logs)</span>
              <span className="text-[10px]">Shed first</span>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

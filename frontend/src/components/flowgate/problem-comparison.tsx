import { AlertOctagon, CheckCircle2, TrendingDown, TrendingUp, XCircle } from "lucide-react";

export function ProblemComparison() {
  return (
    <section id="problem" className="flex flex-col gap-8 py-12 border-b border-[var(--line)] scroll-mt-8">
      <div className="flex flex-col gap-2 max-w-2xl">
        <span className="text-xs uppercase tracking-wider font-semibold text-emerald-600 dark:text-emerald-400 font-mono">
          Real-Life Context
        </span>
        <h2 className="text-2xl md:text-3xl font-bold tracking-tight text-[var(--fg)]">
          Why traditional rate limiters crash under slow upstreams.
        </h2>
        <p className="text-sm md:text-base text-[var(--muted)] leading-relaxed">
          Counting requests per second works fine when servers are fast. But when an upstream database locks or slows down, static rate limits become completely blind to system queueing.
        </p>
      </div>

      {/* Side by side comparison */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* Left: Traditional Gateway */}
        <div className="p-6 rounded-2xl border border-rose-500/20 bg-rose-500/5 flex flex-col gap-4">
          <div className="flex items-center justify-between">
            <span className="font-semibold text-rose-600 dark:text-rose-400 flex items-center gap-2 text-base">
              <XCircle className="w-5 h-5" />
              Traditional Gateway (Fixed Rate Limit)
            </span>
            <span className="text-xs font-mono px-2 py-0.5 rounded bg-rose-500/10 text-rose-600 dark:text-rose-400">
              e.g. 500 req / sec
            </span>
          </div>

          <div className="text-xs text-[var(--fg2)] flex flex-col gap-3 leading-relaxed">
            <div className="p-3 rounded-xl bg-[var(--bg)] border border-[var(--line)] flex flex-col gap-1">
              <span className="font-semibold text-[var(--fg)] flex items-center gap-1.5">
                <AlertOctagon className="w-3.5 h-3.5 text-rose-500" />
                1. Database Latency Spikes (20ms → 250ms)
              </span>
              <p className="text-[var(--muted)] m-0">
                A slow database query or lock contention delays response times.
              </p>
            </div>

            <div className="p-3 rounded-xl bg-[var(--bg)] border border-[var(--line)] flex flex-col gap-1">
              <span className="font-semibold text-[var(--fg)] flex items-center gap-1.5">
                <TrendingUp className="w-3.5 h-3.5 text-rose-500" />
                2. In-Flight Requests Pile Up Blindly
              </span>
              <p className="text-[var(--muted)] m-0">
                Because rate limit only checks req/sec, it admits all 500 requests. Since requests take 12x longer to finish, concurrent connections multiply from 10 to 125+.
              </p>
            </div>

            <div className="p-3 rounded-xl bg-[var(--bg)] border border-[var(--line)] flex flex-col gap-1">
              <span className="font-semibold text-rose-600 dark:text-rose-400 flex items-center gap-1.5">
                <XCircle className="w-3.5 h-3.5" />
                3. Total Cascading Outage (504 Timeouts)
              </span>
              <p className="text-[var(--muted)] m-0">
                Goroutines and thread pools exhaust, memory balloons, and upstream crashes. All users get gateway timeouts.
              </p>
            </div>
          </div>
        </div>

        {/* Right: FlowGate Gateway */}
        <div className="p-6 rounded-2xl border border-emerald-500/20 bg-emerald-500/5 flex flex-col gap-4">
          <div className="flex items-center justify-between">
            <span className="font-semibold text-emerald-600 dark:text-emerald-400 flex items-center gap-2 text-base">
              <CheckCircle2 className="w-5 h-5" />
              FlowGate (Adaptive Concurrency)
            </span>
            <span className="text-xs font-mono px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-600 dark:text-emerald-400">
              Netflix Gradient2 + Circuit Breaker
            </span>
          </div>

          <div className="text-xs text-[var(--fg2)] flex flex-col gap-3 leading-relaxed">
            <div className="p-3 rounded-xl bg-[var(--bg)] border border-[var(--line)] flex flex-col gap-1">
              <span className="font-semibold text-[var(--fg)] flex items-center gap-1.5">
                <TrendingDown className="w-3.5 h-3.5 text-emerald-500" />
                1. Gateway Detects Latency Shift via TTFB
              </span>
              <p className="text-[var(--muted)] m-0">
                FlowGate tracks baseline latency (minRTT). When measured RTT rises, the gradient immediately drops below 1.0.
              </p>
            </div>

            <div className="p-3 rounded-xl bg-[var(--bg)] border border-[var(--line)] flex flex-col gap-1">
              <span className="font-semibold text-[var(--fg)] flex items-center gap-1.5">
                <CheckCircle2 className="w-3.5 h-3.5 text-emerald-500" />
                2. Dynamic Concurrency Contraction
              </span>
              <p className="text-[var(--muted)] m-0">
                Formula contracts limit from 50 down to 12. Non-critical requests are shed in &lt;150ns with 429 Too Many Requests.
              </p>
            </div>

            <div className="p-3 rounded-xl bg-[var(--bg)] border border-[var(--line)] flex flex-col gap-1">
              <span className="font-semibold text-emerald-600 dark:text-emerald-400 flex items-center gap-1.5">
                <CheckCircle2 className="w-3.5 h-3.5" />
                3. Upstream Stays Pristine
              </span>
              <p className="text-[var(--muted)] m-0">
                Queueing stays near zero. Critical checkout routes complete with 100% reserved capacity, and downstream heals naturally.
              </p>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

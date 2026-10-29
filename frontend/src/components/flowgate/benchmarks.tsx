import { CheckCircle2, Terminal } from "lucide-react";

export function FlowGateBenchmarks() {
  return (
    <section id="benchmarks" className="flex flex-col gap-8 py-12 border-b border-[var(--line)]">
      <div className="flex flex-col gap-2 max-w-2xl">
        <span className="text-xs uppercase tracking-wider font-semibold text-emerald-600 dark:text-emerald-400 font-mono">
          Empirical Validation
        </span>
        <h2 className="text-2xl md:text-3xl font-bold tracking-tight text-[var(--fg)]">
          Real Go microbenchmark numbers.
        </h2>
        <p className="text-sm md:text-base text-[var(--muted)] leading-relaxed">
          FlowGate is engineered for the hottest request paths. All measurements captured with <code className="font-mono text-xs px-1.5 py-0.5 rounded bg-[var(--chip)]">go test -bench -benchmem</code>.
        </p>
      </div>

      {/* Benchmark Terminal Table */}
      <div className="rounded-2xl border border-[var(--line)] bg-[var(--chip)] p-5 font-mono text-xs overflow-x-auto flex flex-col gap-4">
        <div className="flex items-center justify-between border-b border-[var(--line)] pb-3 text-[var(--muted)]">
          <span className="flex items-center gap-2 font-semibold text-[var(--fg)]">
            <Terminal className="w-4 h-4 text-emerald-500" />
            pkg/limiter Benchmark Results (AMD Ryzen 7 5800U, 16 Threads)
          </span>
          <span className="flex items-center gap-1.5 text-emerald-600 dark:text-emerald-400">
            <CheckCircle2 className="w-3.5 h-3.5" />
            100% Race Clean
          </span>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-4 gap-4 py-2">
          <div className="flex flex-col gap-1 p-3 rounded-xl bg-[var(--bg)] border border-[var(--line)]">
            <span className="text-[var(--muted)] text-[11px]">Acquire &amp; Done Path</span>
            <span className="text-xl font-bold text-emerald-600 dark:text-emerald-400">147.3 ns / op</span>
            <span className="text-[10px] text-[var(--faint)]">7,625,438 iterations</span>
          </div>

          <div className="flex flex-col gap-1 p-3 rounded-xl bg-[var(--bg)] border border-[var(--line)]">
            <span className="text-[var(--muted)] text-[11px]">Shed Rejection Path</span>
            <span className="text-xl font-bold text-cyan-600 dark:text-cyan-400">96.7 ns / op</span>
            <span className="text-[10px] text-[var(--faint)]">12,693,133 iterations</span>
          </div>

          <div className="flex flex-col gap-1 p-3 rounded-xl bg-[var(--bg)] border border-[var(--line)]">
            <span className="text-[var(--muted)] text-[11px]">Shed Memory Alloc</span>
            <span className="text-xl font-bold text-purple-600 dark:text-purple-400">0 B / op</span>
            <span className="text-[10px] text-[var(--faint)]">0 allocations per shed</span>
          </div>

          <div className="flex flex-col gap-1 p-3 rounded-xl bg-[var(--bg)] border border-[var(--line)]">
            <span className="text-[var(--muted)] text-[11px]">Proxy Buffer Pool</span>
            <span className="text-xl font-bold text-amber-600 dark:text-amber-400">32 KB Pooled</span>
            <span className="text-[10px] text-[var(--faint)]">Zero GC pressure in reverse proxy</span>
          </div>
        </div>

        {/* Raw Go Benchmark Output */}
        <div className="p-4 rounded-xl bg-[var(--bg)] border border-[var(--line)] text-xs font-mono leading-relaxed overflow-x-auto shadow-xs text-[var(--fg)]">
          <div className="text-[var(--muted)] font-medium">$ go test -bench=. -benchmem ./pkg/limiter</div>
          <div className="text-[var(--faint)] text-[11px] mb-1">goos: windows | goarch: amd64 | cpu: AMD Ryzen 7 5800U</div>
          <div className="text-emerald-700 dark:text-emerald-400 font-semibold">
            BenchmarkGradientLimiter_AcquireDone-16 &nbsp;&nbsp;7625438 &nbsp;&nbsp;147.3 ns/op &nbsp;&nbsp;24 B/op &nbsp;&nbsp;1 allocs/op
          </div>
          <div className="text-cyan-700 dark:text-cyan-400 font-semibold">
            BenchmarkGradientLimiter_ShedPath-16 &nbsp;&nbsp;&nbsp;&nbsp;&nbsp;12693133 &nbsp;&nbsp;&nbsp;96.7 ns/op &nbsp;&nbsp;&nbsp;0 B/op &nbsp;&nbsp;0 allocs/op
          </div>
          <div className="text-emerald-600 dark:text-emerald-400 font-bold mt-1.5 flex items-center gap-1.5">
            <CheckCircle2 className="w-3.5 h-3.5 inline" />
            PASS: ok github.com/No-man1234/FlowGate/pkg/limiter 5.284s
          </div>
        </div>
      </div>
    </section>
  );
}

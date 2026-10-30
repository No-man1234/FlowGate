"use client";

import { useEffect, useState, useRef } from "react";
import {
  Activity,
  AlertTriangle,
  CheckCircle2,
  Flame,
  Gauge,
  Play,
  RotateCcw,
  Shield,
  Square,
  Zap,
} from "lucide-react";

interface TelemetryPoint {
  time: number;
  limit: number;
  inFlight: number;
  rtt: number;
}

interface RequestLog {
  id: string;
  time: string;
  method: string;
  path: string;
  priority: "critical" | "standard" | "best_effort";
  status: number;
  latencyMs: number;
  reason: string;
}

type BreakerState = "CLOSED" | "OPEN" | "HALF_OPEN";

export function FlowGateVisualizer() {
  // Mode selection
  const [mode, setMode] = useState<"simulator" | "live">("simulator");
  const [isRunning, setIsRunning] = useState<boolean>(false);

  // Knobs & Settings
  const [concurrency, setConcurrency] = useState<number>(35); // Workers
  const [upstreamBaseRtt, setUpstreamBaseRtt] = useState<number>(30); // ms
  const [upstreamErrorRate, setUpstreamErrorRate] = useState<number>(0); // % 5xx
  const [criticalPct, setCriticalPct] = useState<number>(20);
  const [bestEffortPct, setBestEffortPct] = useState<number>(25);

  // Gateway Internal State (Simulated FlowGate Engine)
  const [limit, setLimit] = useState<number>(30);
  const [inFlight, setInFlight] = useState<number>(0);
  const [minRtt, setMinRtt] = useState<number>(30);
  const [breakerState, setBreakerState] = useState<BreakerState>("CLOSED");
  const [breakerFailures, setBreakerFailures] = useState<number>(0);
  const [breakerCooldown, setBreakerCooldown] = useState<number>(0);

  // Counters
  const [stats, setStats] = useState({
    total: 0,
    ok200: 0,
    shed429: 0,
    breaker503: 0,
    error502: 0,
    criticalAdmitted: 0,
    criticalShed: 0,
    standardAdmitted: 0,
    standardShed: 0,
    bestEffortAdmitted: 0,
    bestEffortShed: 0,
  });

  // History for charts
  const [history, setHistory] = useState<TelemetryPoint[]>([]);
  const [logs, setLogs] = useState<RequestLog[]>([]);
  const [isStreamPaused, setIsStreamPaused] = useState<boolean>(false);
  const isStreamPausedRef = useRef<boolean>(false);
  const logContainerRef = useRef<HTMLDivElement>(null);
  const touchStartY = useRef<number | null>(null);

  useEffect(() => {
    isStreamPausedRef.current = isStreamPaused;
  }, [isStreamPaused]);

  // Simulation animation frame ref
  const animRef = useRef<number | null>(null);
  const breakerTripTimeRef = useRef<number>(0);

  // State refs to avoid stale closures in the loop
  const stateRef = useRef({
    limit: 30,
    inFlight: 0,
    minRtt: 30,
    breakerState: "CLOSED" as BreakerState,
    breakerFailures: 0,
    stats: { ...stats },
    concurrency,
    upstreamBaseRtt,
    upstreamErrorRate,
    criticalPct,
    bestEffortPct,
    isRunning,
  });

  useEffect(() => {
    stateRef.current.concurrency = concurrency;
    stateRef.current.upstreamBaseRtt = upstreamBaseRtt;
    stateRef.current.upstreamErrorRate = upstreamErrorRate;
    stateRef.current.criticalPct = criticalPct;
    stateRef.current.bestEffortPct = bestEffortPct;
    stateRef.current.isRunning = isRunning;
  }, [concurrency, upstreamBaseRtt, upstreamErrorRate, criticalPct, bestEffortPct, isRunning]);

  // Apply scenario presets
  const applyPreset = (preset: "normal" | "burst" | "latency" | "outage") => {
    if (preset === "normal") {
      setConcurrency(25);
      setUpstreamBaseRtt(25);
      setUpstreamErrorRate(0);
      setLimit(30);
      setMinRtt(25);
      setBreakerState("CLOSED");
      setBreakerFailures(0);
      stateRef.current.limit = 30;
      stateRef.current.minRtt = 25;
      stateRef.current.breakerState = "CLOSED";
      stateRef.current.breakerFailures = 0;
    } else if (preset === "burst") {
      setConcurrency(120);
      setUpstreamBaseRtt(25);
      setUpstreamErrorRate(0);
    } else if (preset === "latency") {
      setUpstreamBaseRtt(240);
      setConcurrency(45);
      setUpstreamErrorRate(0);
    } else if (preset === "outage") {
      setUpstreamErrorRate(100);
      setConcurrency(20);
    }
  };

  const resetAll = () => {
    setIsRunning(false);
    setLimit(30);
    setInFlight(0);
    setMinRtt(25);
    setBreakerState("CLOSED");
    setBreakerFailures(0);
    setBreakerCooldown(0);
    setHistory([]);
    setLogs([]);
    const freshStats = {
      total: 0,
      ok200: 0,
      shed429: 0,
      breaker503: 0,
      error502: 0,
      criticalAdmitted: 0,
      criticalShed: 0,
      standardAdmitted: 0,
      standardShed: 0,
      bestEffortAdmitted: 0,
      bestEffortShed: 0,
    };
    setStats(freshStats);
    stateRef.current.limit = 30;
    stateRef.current.inFlight = 0;
    stateRef.current.minRtt = 25;
    stateRef.current.breakerState = "CLOSED";
    stateRef.current.breakerFailures = 0;
    stateRef.current.stats = freshStats;
  };

  useEffect(() => {
    if (!isRunning) {
      if (animRef.current) cancelAnimationFrame(animRef.current);
      return;
    }

    let lastTime = Date.now();

    const tick = () => {
      const now = Date.now();
      const dt = now - lastTime;
      lastTime = now;

      const s = stateRef.current;

      // 1. Circuit breaker cool-off progression
      if (s.breakerState === "OPEN") {
        const elapsed = (now - breakerTripTimeRef.current) / 1000;
        const remaining = Math.max(0, 5 - elapsed);
        setBreakerCooldown(remaining);
        if (remaining <= 0) {
          s.breakerState = "HALF_OPEN";
          setBreakerState("HALF_OPEN");
        }
      }

      // 2. Dispatch simulated incoming requests according to concurrency target
      const newArrivals = Math.max(1, Math.floor((s.concurrency / 10) * (dt / 50)));
      const batchLogs: RequestLog[] = [];

      for (let i = 0; i < newArrivals; i++) {
        s.stats.total++;

        // Pick priority
        const randPrio = Math.random() * 100;
        let prio: "critical" | "standard" | "best_effort" = "standard";
        let path = "/api/v1/orders";

        if (randPrio < s.criticalPct) {
          prio = "critical";
          path = "/checkout";
        } else if (randPrio > 100 - s.bestEffortPct) {
          prio = "best_effort";
          path = "/analytics/track";
        }

        // Step 1: Circuit breaker check
        if (s.breakerState === "OPEN") {
          s.stats.breaker503++;
          batchLogs.push({
            id: Math.random().toString(36).substring(2, 8),
            time: new Date().toLocaleTimeString(),
            method: "GET",
            path,
            priority: prio,
            status: 503,
            latencyMs: 0.05,
            reason: "Circuit Breaker OPEN (Upstream failure protection)",
          });
          continue;
        }

        // Step 2: Adaptive Concurrency limiter check (Gradient formula)
        let admit = false;
        const capacityRatio = s.inFlight / Math.max(1, s.limit);

        if (prio === "critical") {
          admit = s.inFlight < s.limit * 1.2;
        } else if (prio === "standard") {
          admit = capacityRatio < 0.95;
        } else {
          admit = capacityRatio < 0.75;
        }

        if (!admit) {
          s.stats.shed429++;
          if (prio === "critical") s.stats.criticalShed++;
          else if (prio === "standard") s.stats.standardShed++;
          else s.stats.bestEffortShed++;

          batchLogs.push({
            id: Math.random().toString(36).substring(2, 8),
            time: new Date().toLocaleTimeString(),
            method: "POST",
            path,
            priority: prio,
            status: 429,
            latencyMs: 0.1,
            reason: `Adaptive load shed (${Math.round(capacityRatio * 100)}% capacity)`,
          });
          continue;
        }

        // Request admitted!
        if (prio === "critical") s.stats.criticalAdmitted++;
        else if (prio === "standard") s.stats.standardAdmitted++;
        else s.stats.bestEffortAdmitted++;

        s.inFlight++;

        // Upstream completion simulation
        const isUpstreamError = Math.random() * 100 < s.upstreamErrorRate;
        const jitter = (Math.random() - 0.5) * 15;
        const queuePenalty = Math.max(0, (s.inFlight - s.limit * 0.7) * 4);
        const measuredRtt = Math.max(5, s.upstreamBaseRtt + jitter + queuePenalty);

        setTimeout(() => {
          stateRef.current.inFlight = Math.max(0, stateRef.current.inFlight - 1);
          setInFlight(stateRef.current.inFlight);

          if (isUpstreamError) {
            stateRef.current.stats.error502++;
            stateRef.current.breakerFailures++;
            setBreakerFailures(stateRef.current.breakerFailures);

            stateRef.current.limit = Math.max(5, stateRef.current.limit * 0.85);
            setLimit(stateRef.current.limit);

            if (stateRef.current.breakerFailures >= 4) {
              stateRef.current.breakerState = "OPEN";
              setBreakerState("OPEN");
              breakerTripTimeRef.current = Date.now();
              setBreakerCooldown(5);
            }
          } else {
            stateRef.current.stats.ok200++;

            if (stateRef.current.breakerState === "HALF_OPEN") {
              stateRef.current.breakerState = "CLOSED";
              stateRef.current.breakerFailures = 0;
              setBreakerState("CLOSED");
              setBreakerFailures(0);
            }

            if (s.minRtt === 0 || measuredRtt < s.minRtt) {
              stateRef.current.minRtt = measuredRtt;
              setMinRtt(measuredRtt);
            }

            const rawGradient = (stateRef.current.minRtt * 1.25) / measuredRtt;
            const gradient = Math.max(0.5, Math.min(1.0, rawGradient));
            const oldLimit = stateRef.current.limit;
            const queueAllowance = Math.sqrt(oldLimit);

            const calculatedLimit = oldLimit * gradient + queueAllowance;
            const smoothedLimit = oldLimit * 0.8 + calculatedLimit * 0.2;
            const clampedLimit = Math.max(5, Math.min(250, smoothedLimit));

            stateRef.current.limit = clampedLimit;
            setLimit(clampedLimit);
          }
        }, Math.min(600, measuredRtt * 4));

        batchLogs.push({
          id: Math.random().toString(36).substring(2, 8),
          time: new Date().toLocaleTimeString(),
          method: "POST",
          path,
          priority: prio,
          status: isUpstreamError ? 502 : 200,
          latencyMs: Math.round(measuredRtt),
          reason: isUpstreamError
            ? "Upstream 502 failure"
            : "Admitted by FlowGate limiter",
        });
      }

      setStats({ ...s.stats });
      setInFlight(s.inFlight);

      setHistory((prev) => {
        const point: TelemetryPoint = {
          time: now,
          limit: Math.round(s.limit * 10) / 10,
          inFlight: s.inFlight,
          rtt: Math.round(s.upstreamBaseRtt),
        };
        const next = [...prev, point];
        return next.slice(-40);
      });

      if (batchLogs.length > 0 && !isStreamPausedRef.current) {
        setLogs((prev) => [...batchLogs, ...prev].slice(0, 50));
      }

      animRef.current = requestAnimationFrame(tick);
    };

    animRef.current = requestAnimationFrame(tick);
    return () => {
      if (animRef.current) cancelAnimationFrame(animRef.current);
    };
  }, [isRunning]);

  const maxChartLimit = Math.max(50, ...history.map((h) => Math.max(h.limit, h.inFlight)));
  const chartWidth = 560;
  const chartHeight = 160;

  const pointsLimit = history
    .map((h, i) => {
      const x = (i / Math.max(1, history.length - 1)) * chartWidth;
      const y = chartHeight - (h.limit / maxChartLimit) * (chartHeight - 20) - 10;
      return `${x},${y}`;
    })
    .join(" ");

  const pointsInFlight = history
    .map((h, i) => {
      const x = (i / Math.max(1, history.length - 1)) * chartWidth;
      const y = chartHeight - (h.inFlight / maxChartLimit) * (chartHeight - 20) - 10;
      return `${x},${y}`;
    })
    .join(" ");

  return (
    <section id="testbench" className="flex flex-col gap-8 py-12 border-b border-[var(--line)] scroll-mt-8">
      {/* Header of Testbench */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex flex-col gap-1">
          <span className="text-xs uppercase tracking-wider font-semibold text-emerald-600 dark:text-emerald-400 font-mono flex items-center gap-2">
            <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse" />
            Live Gateway Workbench
          </span>
          <h2 className="text-2xl md:text-3xl font-bold tracking-tight text-[var(--fg)]">
            FlowGate Interactive Testbench
          </h2>
        </div>

        {/* Engine switcher */}
        <div className="flex items-center gap-2 bg-[var(--chip)] p-1 rounded-full border border-[var(--line)] text-xs">
          <button
            type="button"
            onClick={() => setMode("simulator")}
            className={`px-3 py-1 rounded-full transition-all ${
              mode === "simulator"
                ? "bg-[var(--bg)] text-[var(--fg)] font-medium shadow-xs"
                : "text-[var(--muted)] hover:text-[var(--fg)]"
            }`}
          >
            Virtual Simulator
          </button>
          <button
            type="button"
            onClick={() => setMode("live")}
            className={`px-3 py-1 rounded-full transition-all ${
              mode === "live"
                ? "bg-[var(--bg)] text-[var(--fg)] font-medium shadow-xs"
                : "text-[var(--muted)] hover:text-[var(--fg)]"
            }`}
          >
            Live Gateway (localhost:8080)
          </button>
        </div>
      </div>

      {/* Main Grid: Knobs + Telemetry */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        {/* Left Column: Interactive Knobs & Presets */}
        <div className="lg:col-span-5 flex flex-col gap-6">
          {/* Quick Scenario Presets */}
          <div className="p-5 rounded-2xl border border-[var(--line)] bg-[var(--bg)] flex flex-col gap-3">
            <h3 className="text-xs uppercase tracking-wider font-semibold text-[var(--muted)]">
              Quick Test Scenarios
            </h3>
            <div className="grid grid-cols-2 gap-2 text-xs">
              <button
                type="button"
                onClick={() => applyPreset("normal")}
                className="p-2.5 rounded-xl border border-[var(--line)] bg-[var(--chip)] hover:border-emerald-500/50 hover:bg-emerald-500/5 text-left flex flex-col gap-1 transition-all"
              >
                <span className="font-semibold text-emerald-600 dark:text-emerald-400 flex items-center gap-1.5">
                  <CheckCircle2 className="w-3.5 h-3.5" /> 1. Benign Traffic
                </span>
                <span className="text-[11px] text-[var(--muted)]">
                  Low RTT, capacity stays healthy
                </span>
              </button>

              <button
                type="button"
                onClick={() => applyPreset("burst")}
                className="p-2.5 rounded-xl border border-[var(--line)] bg-[var(--chip)] hover:border-amber-500/50 hover:bg-amber-500/5 text-left flex flex-col gap-1 transition-all"
              >
                <span className="font-semibold text-amber-600 dark:text-amber-400 flex items-center gap-1.5">
                  <Flame className="w-3.5 h-3.5" /> 2. Traffic Surge
                </span>
                <span className="text-[11px] text-[var(--muted)]">
                  120 workers, tests 429 shedding
                </span>
              </button>

              <button
                type="button"
                onClick={() => applyPreset("latency")}
                className="p-2.5 rounded-xl border border-[var(--line)] bg-[var(--chip)] hover:border-blue-500/50 hover:bg-blue-500/5 text-left flex flex-col gap-1 transition-all"
              >
                <span className="font-semibold text-blue-600 dark:text-blue-400 flex items-center gap-1.5">
                  <Gauge className="w-3.5 h-3.5" /> 3. Latency Creep
                </span>
                <span className="text-[11px] text-[var(--muted)]">
                  Upstream queues, limit contracts
                </span>
              </button>

              <button
                type="button"
                onClick={() => applyPreset("outage")}
                className="p-2.5 rounded-xl border border-[var(--line)] bg-[var(--chip)] hover:border-rose-500/50 hover:bg-rose-500/5 text-left flex flex-col gap-1 transition-all"
              >
                <span className="font-semibold text-rose-600 dark:text-rose-400 flex items-center gap-1.5">
                  <AlertTriangle className="w-3.5 h-3.5" /> 4. Upstream Outage
                </span>
                <span className="text-[11px] text-[var(--muted)]">
                  5xx errors trip Circuit Breaker
                </span>
              </button>
            </div>
          </div>

          {/* Knobs */}
          <div className="p-5 rounded-2xl border border-[var(--line)] bg-[var(--bg)] flex flex-col gap-4">
            <div className="flex items-center justify-between">
              <h3 className="text-xs uppercase tracking-wider font-semibold text-[var(--muted)]">
                Traffic Knobs
              </h3>
              <span className="text-xs text-[var(--faint)] font-mono">
                {mode === "simulator" ? "Virtual Mode" : "Target: 127.0.0.1:8080"}
              </span>
            </div>

            {/* Slider: Concurrency */}
            <div className="flex flex-col gap-1.5">
              <div className="flex justify-between text-xs font-medium">
                <span className="flex items-center gap-1.5 text-[var(--fg2)]">
                  <Zap className="w-3.5 h-3.5 text-amber-500" />
                  Concurrent Load (Virtual Workers)
                </span>
                <span className="font-mono text-emerald-600 dark:text-emerald-400 font-bold">
                  {concurrency} workers
                </span>
              </div>
              <input
                type="range"
                min="5"
                max="150"
                value={concurrency}
                onChange={(e) => setConcurrency(Number(e.target.value))}
                className="w-full accent-emerald-500 cursor-pointer"
              />
            </div>

            {/* Slider: Base RTT */}
            <div className="flex flex-col gap-1.5">
              <div className="flex justify-between text-xs font-medium">
                <span className="flex items-center gap-1.5 text-[var(--fg2)]">
                  <Activity className="w-3.5 h-3.5 text-blue-500" />
                  Upstream Base RTT (Latency)
                </span>
                <span className="font-mono text-blue-600 dark:text-blue-400 font-bold">
                  {upstreamBaseRtt} ms
                </span>
              </div>
              <input
                type="range"
                min="10"
                max="400"
                step="5"
                value={upstreamBaseRtt}
                onChange={(e) => setUpstreamBaseRtt(Number(e.target.value))}
                className="w-full accent-blue-500 cursor-pointer"
              />
            </div>

            {/* Slider: 5xx Error Injection */}
            <div className="flex flex-col gap-1.5">
              <div className="flex justify-between text-xs font-medium">
                <span className="flex items-center gap-1.5 text-[var(--fg2)]">
                  <AlertTriangle className="w-3.5 h-3.5 text-rose-500" />
                  Chaos Error Injection (5xx)
                </span>
                <span className="font-mono text-rose-600 dark:text-rose-400 font-bold">
                  {upstreamErrorRate}%
                </span>
              </div>
              <input
                type="range"
                min="0"
                max="100"
                step="5"
                value={upstreamErrorRate}
                onChange={(e) => setUpstreamErrorRate(Number(e.target.value))}
                className="w-full accent-rose-500 cursor-pointer"
              />
            </div>

            {/* Priority Mix */}
            <div className="pt-2 border-t border-[var(--line)] flex flex-col gap-2">
              <div className="flex justify-between text-xs text-[var(--muted)] font-medium">
                <span>Priority Mix (% Critical):</span>
                <span className="font-mono text-emerald-600 dark:text-emerald-400">{criticalPct}%</span>
              </div>
              <input
                type="range"
                min="5"
                max="50"
                value={criticalPct}
                onChange={(e) => setCriticalPct(Number(e.target.value))}
                className="w-full accent-emerald-500 cursor-pointer"
              />

              <div className="flex justify-between text-xs text-[var(--muted)] font-medium">
                <span>Priority Mix (% Best-Effort):</span>
                <span className="font-mono text-amber-600 dark:text-amber-400">{bestEffortPct}%</span>
              </div>
              <input
                type="range"
                min="5"
                max="50"
                value={bestEffortPct}
                onChange={(e) => setBestEffortPct(Number(e.target.value))}
                className="w-full accent-amber-500 cursor-pointer"
              />
            </div>

            {/* Run / Pause Controls */}
            <div className="flex items-center gap-3 pt-2">
              <button
                type="button"
                onClick={() => setIsRunning((prev) => !prev)}
                className={`flex-1 flex items-center justify-center gap-2 py-2.5 px-4 rounded-xl font-medium text-sm transition-all shadow-xs ${
                  isRunning
                    ? "bg-amber-600 hover:bg-amber-700 text-white"
                    : "bg-emerald-600 hover:bg-emerald-700 text-white"
                }`}
              >
                {isRunning ? (
                  <>
                    <Square className="w-4 h-4 fill-current" /> Pause Traffic
                  </>
                ) : (
                  <>
                    <Play className="w-4 h-4 fill-current" /> Start Traffic
                  </>
                )}
              </button>

              <button
                type="button"
                onClick={resetAll}
                className="p-2.5 rounded-xl border border-[var(--line)] text-[var(--muted)] hover:text-[var(--fg)] hover:bg-[var(--chip)] transition-colors"
                title="Reset counters"
              >
                <RotateCcw className="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>

        {/* Right Column: Multi-Line Chart, Metrics, Circuit Breaker */}
        <div className="lg:col-span-7 flex flex-col gap-6">
          {/* Metrics Scorecard */}
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
            <div className="p-4 rounded-2xl border border-[var(--line)] bg-[var(--bg)] flex flex-col gap-1">
              <span className="text-[11px] uppercase tracking-wider text-[var(--muted)]">
                Adaptive Limit
              </span>
              <span className="text-2xl font-bold font-mono text-emerald-600 dark:text-emerald-400">
                {Math.round(limit * 10) / 10}
              </span>
              <span className="text-[10px] text-[var(--faint)]">
                limit*g + √limit
              </span>
            </div>

            <div className="p-4 rounded-2xl border border-[var(--line)] bg-[var(--bg)] flex flex-col gap-1">
              <span className="text-[11px] uppercase tracking-wider text-[var(--muted)]">
                In-Flight Slots
              </span>
              <span className="text-2xl font-bold font-mono text-[var(--fg)]">
                {inFlight}{" "}
                <span className="text-sm font-normal text-[var(--muted)]">
                  / {Math.round(limit)}
                </span>
              </span>
              <div className="w-full bg-[var(--line)] h-1.5 rounded-full overflow-hidden mt-1">
                <div
                  className="bg-emerald-500 h-full transition-all duration-150"
                  style={{
                    width: `${Math.min(100, (inFlight / Math.max(1, limit)) * 100)}%`,
                  }}
                />
              </div>
            </div>

            <div className="p-4 rounded-2xl border border-[var(--line)] bg-[var(--bg)] flex flex-col gap-1">
              <span className="text-[11px] uppercase tracking-wider text-[var(--muted)]">
                Load Shed (429)
              </span>
              <span className="text-2xl font-bold font-mono text-amber-600 dark:text-amber-400">
                {stats.total > 0
                  ? `${Math.round((stats.shed429 / stats.total) * 100)}%`
                  : "0%"}
              </span>
              <span className="text-[10px] text-[var(--faint)] tabular-nums">
                {stats.shed429} shed
              </span>
            </div>

            <div className="p-4 rounded-2xl border border-[var(--line)] bg-[var(--bg)] flex flex-col gap-1">
              <span className="text-[11px] uppercase tracking-wider text-[var(--muted)]">
                Min RTT Baseline
              </span>
              <span className="text-2xl font-bold font-mono text-blue-600 dark:text-blue-400">
                {Math.round(minRtt)} ms
              </span>
              <span className="text-[10px] text-[var(--faint)]">
                Uncongested baseline
              </span>
            </div>
          </div>

          {/* SVG Multi-Line Chart */}
          <div className="p-5 rounded-2xl border border-[var(--line)] bg-[var(--bg)] flex flex-col gap-3">
            <div className="flex items-center justify-between">
              <h3 className="text-xs uppercase tracking-wider font-semibold text-[var(--muted)] flex items-center gap-2">
                <Activity className="w-3.5 h-3.5 text-emerald-500" />
                Adaptive Concurrency Dynamics (Live Chart)
              </h3>
              <div className="flex items-center gap-4 text-xs font-mono">
                <span className="flex items-center gap-1.5 text-emerald-600 dark:text-emerald-400">
                  <span className="w-2.5 h-2.5 rounded-full bg-emerald-500" />
                  Estimated Limit
                </span>
                <span className="flex items-center gap-1.5 text-cyan-600 dark:text-cyan-400">
                  <span className="w-2.5 h-2.5 rounded-full bg-cyan-500" />
                  In-Flight Requests
                </span>
              </div>
            </div>

            <div className="relative w-full h-40 bg-[var(--chip)] rounded-xl overflow-hidden p-2 flex items-center justify-center">
              {history.length > 2 ? (
                <svg
                  viewBox={`0 0 ${chartWidth} ${chartHeight}`}
                  className="w-full h-full overflow-visible"
                  preserveAspectRatio="none"
                >
                  <line x1="0" y1="40" x2={chartWidth} y2="40" stroke="var(--line)" strokeDasharray="4 4" />
                  <line x1="0" y1="80" x2={chartWidth} y2="80" stroke="var(--line)" strokeDasharray="4 4" />
                  <line x1="0" y1="120" x2={chartWidth} y2="120" stroke="var(--line)" strokeDasharray="4 4" />

                  <polyline
                    fill="none"
                    stroke="#10b981"
                    strokeWidth="2.5"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    points={pointsLimit}
                  />

                  <polyline
                    fill="none"
                    stroke="#06b6d4"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    points={pointsInFlight}
                  />
                </svg>
              ) : (
                <div className="text-xs text-[var(--muted)] flex items-center gap-2">
                  <Activity className="w-4 h-4 animate-spin" />
                  Awaiting traffic telemetry... Click &quot;Start Traffic&quot; to begin.
                </div>
              )}
            </div>
          </div>

          {/* Circuit Breaker & Priority Status */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            {/* Circuit Breaker */}
            <div className="p-4 rounded-2xl border border-[var(--line)] bg-[var(--bg)] flex flex-col gap-3">
              <div className="flex items-center justify-between">
                <span className="text-xs uppercase tracking-wider font-semibold text-[var(--muted)] flex items-center gap-1.5">
                  <Shield className="w-3.5 h-3.5 text-blue-500" />
                  Circuit Breaker
                </span>
                <span
                  className={`px-2.5 py-0.5 rounded-full text-xs font-bold uppercase tracking-wider ${
                    breakerState === "CLOSED"
                      ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20"
                      : breakerState === "OPEN"
                      ? "bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/20 animate-pulse"
                      : "bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20"
                  }`}
                >
                  {breakerState}
                </span>
              </div>

              <div className="text-xs text-[var(--fg2)] flex flex-col gap-1.5">
                <div className="flex justify-between">
                  <span className="text-[var(--muted)]">Consecutive Failures:</span>
                  <span className="font-mono">{breakerFailures} / 4</span>
                </div>
                {breakerState === "OPEN" && (
                  <div className="flex justify-between text-rose-600 dark:text-rose-400 font-medium">
                    <span>Cool-off Timer:</span>
                    <span className="font-mono">{breakerCooldown.toFixed(1)}s</span>
                  </div>
                )}
                <div className="flex justify-between">
                  <span className="text-[var(--muted)]">Fast-Fail Response:</span>
                  <span className="font-mono">503 in &lt;0.1ms</span>
                </div>
              </div>

              <div className="flex gap-2 pt-1 border-t border-[var(--line)]">
                <button
                  type="button"
                  onClick={() => {
                    setBreakerState("OPEN");
                    stateRef.current.breakerState = "OPEN";
                    breakerTripTimeRef.current = Date.now();
                    setBreakerCooldown(5);
                  }}
                  className="flex-1 text-[11px] py-1.5 px-2 rounded-lg bg-rose-500/10 text-rose-600 dark:text-rose-400 hover:bg-rose-500/20 transition-colors font-medium text-center"
                >
                  Manual Trip
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setBreakerState("CLOSED");
                    setBreakerFailures(0);
                    stateRef.current.breakerState = "CLOSED";
                    stateRef.current.breakerFailures = 0;
                  }}
                  className="flex-1 text-[11px] py-1.5 px-2 rounded-lg bg-[var(--chip)] text-[var(--fg)] hover:bg-[var(--chip-hover)] transition-colors font-medium text-center"
                >
                  Reset
                </button>
              </div>
            </div>

            {/* Priority Admission Bars */}
            <div className="p-4 rounded-2xl border border-[var(--line)] bg-[var(--bg)] flex flex-col gap-2.5">
              <span className="text-xs uppercase tracking-wider font-semibold text-[var(--muted)]">
                Priority Tier Pass-Through
              </span>

              <div className="flex flex-col gap-2 text-xs">
                <div>
                  <div className="flex justify-between mb-1">
                    <span className="font-medium text-emerald-600 dark:text-emerald-400">
                      Critical (/checkout)
                    </span>
                    <span className="font-mono text-[var(--muted)]">
                      {stats.criticalAdmitted} ok / {stats.criticalShed} shed
                    </span>
                  </div>
                  <div className="w-full bg-[var(--chip)] h-1.5 rounded-full overflow-hidden">
                    <div
                      className="bg-emerald-500 h-full"
                      style={{
                        width: `${
                          stats.criticalAdmitted + stats.criticalShed > 0
                            ? (stats.criticalAdmitted /
                                (stats.criticalAdmitted + stats.criticalShed)) *
                              100
                            : 100
                        }%`,
                      }}
                    />
                  </div>
                </div>

                <div>
                  <div className="flex justify-between mb-1">
                    <span className="font-medium text-blue-600 dark:text-blue-400">
                      Standard (/api)
                    </span>
                    <span className="font-mono text-[var(--muted)]">
                      {stats.standardAdmitted} ok / {stats.standardShed} shed
                    </span>
                  </div>
                  <div className="w-full bg-[var(--chip)] h-1.5 rounded-full overflow-hidden">
                    <div
                      className="bg-blue-500 h-full"
                      style={{
                        width: `${
                          stats.standardAdmitted + stats.standardShed > 0
                            ? (stats.standardAdmitted /
                                (stats.standardAdmitted + stats.standardShed)) *
                              100
                            : 100
                        }%`,
                      }}
                    />
                  </div>
                </div>

                <div>
                  <div className="flex justify-between mb-1">
                    <span className="font-medium text-amber-600 dark:text-amber-400">
                      Best-Effort (/analytics)
                    </span>
                    <span className="font-mono text-[var(--muted)]">
                      {stats.bestEffortAdmitted} ok / {stats.bestEffortShed} shed
                    </span>
                  </div>
                  <div className="w-full bg-[var(--chip)] h-1.5 rounded-full overflow-hidden">
                    <div
                      className="bg-amber-500 h-full"
                      style={{
                        width: `${
                          stats.bestEffortAdmitted + stats.bestEffortShed > 0
                            ? (stats.bestEffortAdmitted /
                                (stats.bestEffortAdmitted + stats.bestEffortShed)) *
                              100
                            : 100
                        }%`,
                      }}
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Real-Time Request Inspector Log */}
      <div className="p-4 sm:p-5 rounded-2xl border border-[var(--line)] bg-[var(--bg)] flex flex-col gap-3">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2.5 border-b border-[var(--line)] pb-3">
          <div className="flex flex-wrap items-center gap-2.5">
            <h3 className="text-xs uppercase tracking-wider font-semibold text-[var(--muted)] flex items-center gap-2">
              <span
                className={`w-2 h-2 rounded-full ${
                  isStreamPaused ? "bg-amber-500" : "bg-emerald-500 animate-pulse"
                }`}
              />
              Live Request Stream (Gateway Decisions)
            </h3>
            <button
              type="button"
              onClick={() => {
                if (isStreamPaused) {
                  setIsStreamPaused(false);
                  if (logContainerRef.current) {
                    logContainerRef.current.scrollTop = 0;
                  }
                } else {
                  setIsStreamPaused(true);
                }
              }}
              className="text-[11px] font-mono px-2.5 py-0.5 rounded border border-[var(--line)] bg-[var(--chip)] text-[var(--fg2)] hover:bg-[var(--chip-hover)] transition-colors flex items-center gap-1.5 shadow-sm"
              title={isStreamPaused ? "Resume live streaming" : "Pause live streaming to inspect logs"}
            >
              {isStreamPaused ? (
                <>
                  <Play className="w-2.5 h-2.5 text-emerald-500 fill-emerald-500" />
                  <span className="font-semibold text-emerald-600 dark:text-emerald-400">Resume Stream</span>
                </>
              ) : (
                <>
                  <Square className="w-2.5 h-2.5 text-amber-500 fill-amber-500" />
                  <span>Pause Stream</span>
                </>
              )}
            </button>
            {isStreamPaused && (
              <span className="text-[11px] text-amber-600 dark:text-amber-400 font-mono hidden sm:inline-block">
                (Paused for inspection)
              </span>
            )}
            {logs.length > 0 && (
              <button
                type="button"
                onClick={() => setLogs([])}
                className="text-[11px] font-mono px-2 py-0.5 rounded border border-[var(--line)] bg-[var(--chip)] text-[var(--muted)] hover:text-rose-500 hover:bg-[var(--chip-hover)] transition-colors"
              >
                Clear
              </button>
            )}
          </div>

          <div className="flex flex-wrap items-center gap-1.5 text-[11px] font-mono">
            <span className="px-2 py-0.5 rounded bg-[var(--chip)] text-[var(--fg2)] border border-[var(--line)]">
              Total: <strong className="text-[var(--fg)]">{stats.total}</strong>
            </span>
            <span className="px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
              200 OK: <strong>{stats.ok200}</strong>
            </span>
            <span className="px-2 py-0.5 rounded bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20">
              429 Shed: <strong>{stats.shed429}</strong>
            </span>
            <span className="px-2 py-0.5 rounded bg-rose-500/10 text-rose-600 dark:text-rose-400 border border-rose-500/20">
              503 Breaker: <strong>{stats.breaker503}</strong>
            </span>
          </div>
        </div>

        <div
          ref={logContainerRef}
          style={{ overflowAnchor: "none" }}
          onScroll={() => {
            const el = logContainerRef.current;
            if (!el) return;
            // If the user manually scrolls down the scrollbar to inspect history, auto-pause the stream!
            if (el.scrollTop > 15 && !isStreamPausedRef.current) {
              setIsStreamPaused(true);
            }
          }}
          onWheel={(e) => {
            const el = logContainerRef.current;
            if (!el) return;
            if (!isStreamPaused) {
              // When live streaming, keep stream pinned to latest and let user scroll webpage freely!
              window.scrollBy({ top: e.deltaY, behavior: "auto" });
            } else {
              // When paused, allow internal inspecting of log history with boundary pass-through
              if (e.deltaY < 0 && el.scrollTop <= 2) {
                window.scrollBy({ top: e.deltaY, behavior: "auto" });
              } else if (e.deltaY > 0 && el.scrollTop + el.clientHeight >= el.scrollHeight - 2) {
                window.scrollBy({ top: e.deltaY, behavior: "auto" });
              }
            }
          }}
          onTouchStart={(e) => {
            touchStartY.current = e.touches[0].clientY;
          }}
          onTouchMove={(e) => {
            const el = logContainerRef.current;
            if (!el || touchStartY.current === null) return;
            const currentY = e.touches[0].clientY;
            const deltaY = touchStartY.current - currentY; // positive = scroll down, negative = scroll up

            if (!isStreamPaused) {
              window.scrollBy({ top: deltaY, behavior: "auto" });
            } else {
              if (deltaY < 0 && el.scrollTop <= 2) {
                window.scrollBy({ top: deltaY, behavior: "auto" });
              } else if (deltaY > 0 && el.scrollTop + el.clientHeight >= el.scrollHeight - 2) {
                window.scrollBy({ top: deltaY, behavior: "auto" });
              }
            }
            touchStartY.current = currentY;
          }}
          onTouchEnd={() => {
            touchStartY.current = null;
          }}
          className="font-mono text-xs max-h-60 overflow-y-auto overflow-x-hidden divide-y divide-[var(--line)] border border-[var(--line)] rounded-xl bg-[var(--chip)] overscroll-y-auto"
        >
          {logs.length === 0 ? (
            <div className="p-4 text-center text-[var(--muted)]">
              No requests recorded yet. Click &quot;Start Traffic&quot; above to watch real-time gateway decisions.
            </div>
          ) : (
            logs.map((log) => (
              <div
                key={log.id}
                className="flex flex-col sm:flex-row sm:items-center justify-between gap-1.5 sm:gap-4 p-2.5 hover:bg-[var(--chip-hover)] transition-colors min-w-0"
              >
                {/* Line 1 on mobile, left block on desktop */}
                <div className="flex items-center justify-between sm:justify-start gap-2.5 min-w-0">
                  <div className="flex items-center gap-2 min-w-0">
                    <span
                      className={`px-2 py-0.5 rounded text-[10px] font-bold shrink-0 ${
                        log.status === 200
                          ? "bg-emerald-500/20 text-emerald-600 dark:text-emerald-400"
                          : log.status === 429
                          ? "bg-amber-500/20 text-amber-600 dark:text-amber-400"
                          : log.status === 503
                          ? "bg-rose-500/20 text-rose-600 dark:text-rose-400"
                          : "bg-purple-500/20 text-purple-600 dark:text-purple-400"
                      }`}
                    >
                      {log.status}
                    </span>
                    <span className="font-bold text-[var(--fg)] shrink-0">{log.method}</span>
                    <span className="text-[var(--fg2)] truncate max-w-[130px] sm:max-w-[220px]">{log.path}</span>
                    <span className="hidden sm:inline-block text-[10px] px-1.5 py-0.2 rounded bg-[var(--bg)] text-[var(--muted)] border border-[var(--line)] uppercase shrink-0">
                      {log.priority}
                    </span>
                  </div>
                  {/* Latency right-aligned on mobile line 1 */}
                  <span className="sm:hidden font-bold text-[var(--fg)] shrink-0 text-right">{log.latencyMs}ms</span>
                </div>

                {/* Line 2 on mobile, right block on desktop */}
                <div className="flex items-center justify-between sm:justify-end gap-2 sm:gap-4 text-right text-[var(--muted)] min-w-0">
                  {/* Priority badge on mobile line 2 */}
                  <span className="sm:hidden text-[10px] px-1.5 py-0.2 rounded bg-[var(--bg)] text-[var(--muted)] border border-[var(--line)] uppercase shrink-0">
                    {log.priority}
                  </span>
                  <span className="truncate sm:whitespace-nowrap text-[11px] sm:text-xs text-[var(--muted)]">{log.reason}</span>
                  {/* Latency right-aligned on desktop */}
                  <span className="hidden sm:inline-block font-bold text-[var(--fg)] shrink-0 min-w-[45px] text-right">{log.latencyMs}ms</span>
                </div>
              </div>
            ))
          )}
        </div>
      </div>
    </section>
  );
}

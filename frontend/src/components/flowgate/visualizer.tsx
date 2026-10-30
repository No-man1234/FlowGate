"use client";

import { useEffect, useState, useRef, useCallback } from "react";
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
  Terminal,
  Wifi,
  WifiOff,
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
  // Mode selection: virtual simulation vs live gateway
  const [mode, setMode] = useState<"simulator" | "live">("simulator");
  const [liveConnected, setLiveConnected] = useState<boolean>(false);
  const [liveError, setLiveError] = useState<string | null>(null);

  const [isRunning, setIsRunning] = useState<boolean>(false);
  const [activePreset, setActivePreset] = useState<string | null>(null);

  // Knobs & Settings
  const [concurrency, setConcurrency] = useState<number>(35); // Workers
  const [upstreamBaseRtt, setUpstreamBaseRtt] = useState<number>(30); // ms
  const [upstreamErrorRate, setUpstreamErrorRate] = useState<number>(0); // % 5xx
  const [criticalPct, setCriticalPct] = useState<number>(20);
  const [bestEffortPct, setBestEffortPct] = useState<number>(25);

  // Gateway Internal State (Simulated / Live)
  const [limit, setLimit] = useState<number>(30);
  const [inFlight, setInFlight] = useState<number>(0);
  const [minRtt, setMinRtt] = useState<number>(30);
  const [sampleRtt, setSampleRtt] = useState<number>(30);
  const [breakerState, setBreakerState] = useState<BreakerState>("CLOSED");
  const [breakerFailures, setBreakerFailures] = useState<number>(0);
  const [breakerCooldown, setBreakerCooldown] = useState<number>(0);

  // Stats Counters
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

  // Refs for animation loop and timeouts
  const animRef = useRef<number | null>(null);
  const breakerTripTimeRef = useRef<number>(0);
  const pendingTimeoutsRef = useRef<Set<NodeJS.Timeout>>(new Set());

  // Mutable state ref to prevent stale closures in requestAnimationFrame
  const stateRef = useRef({
    limit: 30,
    inFlight: 0,
    minRtt: 30,
    sampleRtt: 30,
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

  // Keep stateRef synced with latest component state
  useEffect(() => {
    stateRef.current.concurrency = concurrency;
    stateRef.current.upstreamBaseRtt = upstreamBaseRtt;
    stateRef.current.upstreamErrorRate = upstreamErrorRate;
    stateRef.current.criticalPct = criticalPct;
    stateRef.current.bestEffortPct = bestEffortPct;
    stateRef.current.isRunning = isRunning;
  }, [concurrency, upstreamBaseRtt, upstreamErrorRate, criticalPct, bestEffortPct, isRunning]);

  // Apply scenario presets and auto-start
  const applyPreset = (preset: "normal" | "burst" | "latency" | "outage") => {
    setActivePreset(preset);
    if (preset === "normal") {
      setConcurrency(25);
      setUpstreamBaseRtt(25);
      setUpstreamErrorRate(0);
      setLimit(30);
      setMinRtt(25);
      setSampleRtt(25);
      setBreakerState("CLOSED");
      setBreakerFailures(0);
      setBreakerCooldown(0);
      stateRef.current.limit = 30;
      stateRef.current.minRtt = 25;
      stateRef.current.sampleRtt = 25;
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
      setConcurrency(25);
    }

    // Auto-start simulation if in simulator mode
    if (mode === "simulator") {
      setIsRunning(true);
    }
  };

  // Full reset of state and pending requests
  const resetAll = useCallback(() => {
    setIsRunning(false);
    setActivePreset(null);

    // Cancel all in-flight request completions
    pendingTimeoutsRef.current.forEach((t) => clearTimeout(t));
    pendingTimeoutsRef.current.clear();

    setLimit(30);
    setInFlight(0);
    setMinRtt(25);
    setSampleRtt(25);
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
    stateRef.current.sampleRtt = 25;
    stateRef.current.breakerState = "CLOSED";
    stateRef.current.breakerFailures = 0;
    stateRef.current.stats = freshStats;
  }, []);

  // Live Gateway polling effect (localhost:8080/metrics)
  useEffect(() => {
    if (mode !== "live") return;

    let isMounted = true;
    const fetchLiveMetrics = async () => {
      try {
        const res = await fetch("http://127.0.0.1:8080/metrics", {
          method: "GET",
          headers: { Accept: "application/json" },
          cache: "no-store",
        });

        if (!res.ok) {
          throw new Error(`HTTP ${res.status}`);
        }

        const data = await res.json();
        if (!isMounted) return;

        setLiveConnected(true);
        setLiveError(null);

        // Map live Go gateway metrics to visualizer state
        const currentLimitVal = data.current_limit ?? 50;
        const inFlightVal = data.in_flight ?? 0;
        const minRttVal = data.min_rtt_ms ?? 15;
        const sampleRttVal = data.sample_rtt_ms ?? 20;
        const circuitStateVal = (data.circuit_state?.toUpperCase() ?? "CLOSED") as BreakerState;

        setLimit(currentLimitVal);
        setInFlight(inFlightVal);
        setMinRtt(minRttVal);
        setSampleRtt(sampleRttVal);
        setBreakerState(circuitStateVal);

        setStats((prev) => ({
          ...prev,
          total: data.total_requests ?? prev.total,
          ok200: data.accepted_requests ?? prev.ok200,
          shed429: data.shed_requests ?? prev.shed429,
          breaker503: data.circuit_breaker_trips ?? prev.breaker503,
          error502: data.upstream_errors ?? prev.error502,
        }));

        setHistory((prev) => {
          const point: TelemetryPoint = {
            time: Date.now(),
            limit: Math.round(currentLimitVal * 10) / 10,
            inFlight: inFlightVal,
            rtt: Math.round(sampleRttVal),
          };
          return [...prev, point].slice(-40);
        });
      } catch (err) {
        if (!isMounted) return;
        setLiveConnected(false);
        setLiveError("Cannot connect to FlowGate on 127.0.0.1:8080. Ensure the binary is running.");
      }
    };

    fetchLiveMetrics();
    const interval = setInterval(fetchLiveMetrics, 1000);

    return () => {
      isMounted = false;
      clearInterval(interval);
    };
  }, [mode]);

  // Virtual Simulator animation loop
  useEffect(() => {
    if (mode !== "simulator" || !isRunning) {
      if (animRef.current) cancelAnimationFrame(animRef.current);
      return;
    }

    let lastTime = Date.now();
    let accumulator = 0;

    const tick = () => {
      const now = Date.now();
      const dt = Math.min(100, now - lastTime);
      lastTime = now;
      accumulator += dt;

      const s = stateRef.current;

      // 1. Circuit breaker cool-off progression in OPEN state
      if (s.breakerState === "OPEN") {
        const elapsed = (now - breakerTripTimeRef.current) / 1000;
        const remaining = Math.max(0, 5 - elapsed);
        setBreakerCooldown(remaining);
        if (remaining <= 0) {
          s.breakerState = "HALF_OPEN";
          s.breakerFailures = 0;
          setBreakerState("HALF_OPEN");
          setBreakerFailures(0);
        }
      }

      // Generate request pulses based on concurrency workers (target: ~2 requests per worker per second)
      const intervalMs = Math.max(25, 1000 / Math.max(1, s.concurrency * 1.5));
      if (accumulator >= intervalMs) {
        accumulator = 0;

        const batchLogs: RequestLog[] = [];
        const arrivals = Math.max(1, Math.round(s.concurrency / 25));

        for (let i = 0; i < arrivals; i++) {
          s.stats.total++;

          // Select priority tier
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
            admit = capacityRatio < 0.85;
          } else {
            admit = capacityRatio < 0.70;
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
          const jitter = (Math.random() - 0.5) * 8;
          const queuePenalty = Math.max(0, (s.inFlight - s.limit * 0.7) * 3);
          const measuredRtt = Math.max(8, s.upstreamBaseRtt + jitter + queuePenalty);

          // Simulated response delay (scaled for smooth browser visualization)
          const timeoutDuration = Math.min(500, Math.max(40, measuredRtt * 2.5));

          const timeoutId = setTimeout(() => {
            pendingTimeoutsRef.current.delete(timeoutId);

            stateRef.current.inFlight = Math.max(0, stateRef.current.inFlight - 1);
            setInFlight(stateRef.current.inFlight);

            if (isUpstreamError) {
              stateRef.current.stats.error502++;
              stateRef.current.breakerFailures++;
              setBreakerFailures(stateRef.current.breakerFailures);

              // Multiplicative limit backoff on upstream error (0.8x)
              stateRef.current.limit = Math.max(5, stateRef.current.limit * 0.8);
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

              // Update minRTT baseline
              if (s.minRtt === 0 || measuredRtt < s.minRtt) {
                stateRef.current.minRtt = measuredRtt;
                setMinRtt(measuredRtt);
              }

              // EWMA smoothed sample RTT
              const alpha = 0.2;
              const smoothedSample = stateRef.current.sampleRtt * (1.0 - alpha) + measuredRtt * alpha;
              stateRef.current.sampleRtt = smoothedSample;
              setSampleRtt(smoothedSample);

              // APP-LIMITED GUARD (Matches Go backend pkg/limiter/gradient.go):
              // Only adjust limit upward when system is sufficiently loaded (inFlight >= 45% limit).
              // When idle or uncongested, prevent false limit inflation.
              const currentLoad = stateRef.current.inFlight;
              const oldLimit = stateRef.current.limit;

              if (currentLoad >= oldLimit * 0.45) {
                const rawGradient = (stateRef.current.minRtt / smoothedSample) * 1.25;
                const gradient = Math.max(0.5, Math.min(1.0, rawGradient));

                // Incremental probe factor: scaled per completion to avoid runaway inflation
                const headroomProbe = gradient >= 1.0 ? Math.sqrt(oldLimit) / oldLimit : 0;
                const targetLimit = oldLimit * gradient + headroomProbe;

                const smoothedLimit = oldLimit * 0.9 + targetLimit * 0.1;
                const clampedLimit = Math.max(5, Math.min(150, smoothedLimit));

                stateRef.current.limit = clampedLimit;
                setLimit(clampedLimit);
              }
            }
          }, timeoutDuration);

          pendingTimeoutsRef.current.add(timeoutId);

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
            rtt: Math.round(s.sampleRtt),
          };
          return [...prev, point].slice(-40);
        });

        if (batchLogs.length > 0) {
          setLogs((prev) => [...batchLogs, ...prev].slice(0, 50));
        }
      }

      animRef.current = requestAnimationFrame(tick);
    };

    animRef.current = requestAnimationFrame(tick);
    return () => {
      if (animRef.current) cancelAnimationFrame(animRef.current);
    };
  }, [isRunning, mode]);

  // Clean up timeouts on unmount
  useEffect(() => {
    return () => {
      pendingTimeoutsRef.current.forEach((t) => clearTimeout(t));
      pendingTimeoutsRef.current.clear();
    };
  }, []);

  // Chart coordinate calculations
  const maxChartLimit = Math.max(50, ...history.map((h) => Math.max(h.limit, h.inFlight)));
  const chartWidth = 560;
  const chartHeight = 160;

  const pointsLimit = history
    .map((h, i) => {
      const x = (i / Math.max(1, history.length - 1)) * chartWidth;
      const y = chartHeight - (h.limit / maxChartLimit) * (chartHeight - 30) - 15;
      return `${x},${y}`;
    })
    .join(" ");

  const pointsInFlight = history
    .map((h, i) => {
      const x = (i / Math.max(1, history.length - 1)) * chartWidth;
      const y = chartHeight - (h.inFlight / maxChartLimit) * (chartHeight - 30) - 15;
      return `${x},${y}`;
    })
    .join(" ");

  return (
    <section id="testbench" className="flex flex-col gap-8 py-8 md:py-12 scroll-mt-8">
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
            onClick={() => {
              setMode("simulator");
              setIsRunning(false);
            }}
            className={`px-3 py-1 rounded-full transition-all cursor-pointer ${
              mode === "simulator"
                ? "bg-[var(--bg)] text-[var(--fg)] font-medium shadow-xs"
                : "text-[var(--muted)] hover:text-[var(--fg)]"
            }`}
          >
            Virtual Simulator
          </button>
          <button
            type="button"
            onClick={() => {
              setMode("live");
              setIsRunning(false);
            }}
            className={`px-3 py-1 rounded-full transition-all cursor-pointer flex items-center gap-1.5 ${
              mode === "live"
                ? "bg-[var(--bg)] text-[var(--fg)] font-medium shadow-xs"
                : "text-[var(--muted)] hover:text-[var(--fg)]"
            }`}
          >
            <span>Live Gateway (localhost:8080)</span>
            <span
              className={`w-2 h-2 rounded-full ${
                liveConnected ? "bg-emerald-500" : "bg-rose-500"
              }`}
            />
          </button>
        </div>
      </div>

      {/* Live Gateway Mode Banner when disconnected */}
      {mode === "live" && (
        <div
          className={`p-4 rounded-2xl border text-xs flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 ${
            liveConnected
              ? "bg-emerald-500/10 border-emerald-500/20 text-emerald-700 dark:text-emerald-300"
              : "bg-rose-500/10 border-rose-500/20 text-rose-700 dark:text-rose-300"
          }`}
        >
          <div className="flex items-center gap-2.5">
            {liveConnected ? (
              <Wifi className="w-4 h-4 text-emerald-500 shrink-0" />
            ) : (
              <WifiOff className="w-4 h-4 text-rose-500 shrink-0" />
            )}
            <div>
              <span className="font-semibold">
                {liveConnected
                  ? "Connected to Local FlowGate Gateway (127.0.0.1:8080)"
                  : "Local Gateway Offline on 127.0.0.1:8080"}
              </span>
              <p className="text-[11px] opacity-80 mt-0.5">
                {liveConnected
                  ? "Streaming real Go atomic counters, Vegas gradient limits, and circuit breaker metrics directly from localhost."
                  : "Start the gateway with: go run cmd/flowgate/main.go -upstream http://127.0.0.1:9090 -port 8080"}
              </p>
            </div>
          </div>

          {!liveConnected && (
            <button
              type="button"
              onClick={() => {
                setMode("simulator");
              }}
              className="px-3 py-1.5 rounded-lg bg-[var(--bg)] border border-[var(--line)] text-[var(--fg)] font-mono text-xs hover:border-[var(--fg)] shrink-0 cursor-pointer"
            >
              Switch to Virtual Mode
            </button>
          )}
        </div>
      )}

      {/* Main Grid: Knobs + Telemetry */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        {/* Left Column: Interactive Knobs & Presets */}
        <div className="lg:col-span-5 flex flex-col gap-6">
          {/* Quick Scenario Presets */}
          <div className="p-5 rounded-2xl border border-[var(--line)] bg-[var(--bg)] flex flex-col gap-3">
            <div className="flex items-center justify-between">
              <h3 className="text-xs uppercase tracking-wider font-semibold text-[var(--muted)]">
                Quick Test Scenarios
              </h3>
              {activePreset && (
                <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20 uppercase">
                  Active: {activePreset}
                </span>
              )}
            </div>

            <div className="grid grid-cols-2 gap-2 text-xs">
              <button
                type="button"
                onClick={() => applyPreset("normal")}
                className={`p-2.5 rounded-xl border text-left flex flex-col gap-1 transition-all cursor-pointer ${
                  activePreset === "normal"
                    ? "border-emerald-500 bg-emerald-500/10"
                    : "border-[var(--line)] bg-[var(--chip)] hover:border-emerald-500/50 hover:bg-emerald-500/5"
                }`}
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
                className={`p-2.5 rounded-xl border text-left flex flex-col gap-1 transition-all cursor-pointer ${
                  activePreset === "burst"
                    ? "border-amber-500 bg-amber-500/10"
                    : "border-[var(--line)] bg-[var(--chip)] hover:border-amber-500/50 hover:bg-amber-500/5"
                }`}
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
                className={`p-2.5 rounded-xl border text-left flex flex-col gap-1 transition-all cursor-pointer ${
                  activePreset === "latency"
                    ? "border-blue-500 bg-blue-500/10"
                    : "border-[var(--line)] bg-[var(--chip)] hover:border-blue-500/50 hover:bg-blue-500/5"
                }`}
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
                className={`p-2.5 rounded-xl border text-left flex flex-col gap-1 transition-all cursor-pointer ${
                  activePreset === "outage"
                    ? "border-rose-500 bg-rose-500/10"
                    : "border-[var(--line)] bg-[var(--chip)] hover:border-rose-500/50 hover:bg-rose-500/5"
                }`}
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
                {mode === "simulator" ? "Virtual Simulator Mode" : "Target: 127.0.0.1:8080"}
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
                disabled={mode === "live"}
                onChange={(e) => {
                  setConcurrency(Number(e.target.value));
                  setActivePreset(null);
                }}
                className="w-full accent-emerald-500 cursor-pointer disabled:opacity-50"
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
                disabled={mode === "live"}
                onChange={(e) => {
                  setUpstreamBaseRtt(Number(e.target.value));
                  setActivePreset(null);
                }}
                className="w-full accent-blue-500 cursor-pointer disabled:opacity-50"
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
                disabled={mode === "live"}
                onChange={(e) => {
                  setUpstreamErrorRate(Number(e.target.value));
                  setActivePreset(null);
                }}
                className="w-full accent-rose-500 cursor-pointer disabled:opacity-50"
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
                disabled={mode === "live"}
                onChange={(e) => setCriticalPct(Number(e.target.value))}
                className="w-full accent-emerald-500 cursor-pointer disabled:opacity-50"
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
                disabled={mode === "live"}
                onChange={(e) => setBestEffortPct(Number(e.target.value))}
                className="w-full accent-amber-500 cursor-pointer disabled:opacity-50"
              />
            </div>

            {/* Run / Pause Controls (Simulator mode) */}
            {mode === "simulator" && (
              <div className="flex items-center gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setIsRunning((prev) => !prev)}
                  className={`flex-1 flex items-center justify-center gap-2 py-2.5 px-4 rounded-xl font-medium text-sm transition-all shadow-xs cursor-pointer ${
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
                  className="p-2.5 rounded-xl border border-[var(--line)] text-[var(--muted)] hover:text-[var(--fg)] hover:bg-[var(--chip)] transition-colors cursor-pointer"
                  title="Reset counters"
                >
                  <RotateCcw className="w-4 h-4" />
                </button>
              </div>
            )}
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

          {/* SVG Multi-Line Chart with Y-Axis Indicators */}
          <div className="p-5 rounded-2xl border border-[var(--line)] bg-[var(--bg)] flex flex-col gap-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h3 className="text-xs uppercase tracking-wider font-semibold text-[var(--muted)] flex items-center gap-2">
                <Activity className="w-3.5 h-3.5 text-emerald-500" />
                Adaptive Concurrency Dynamics (Live Chart)
              </h3>
              <div className="flex items-center gap-4 text-xs font-mono">
                <span className="flex items-center gap-1.5 text-emerald-600 dark:text-emerald-400">
                  <span className="w-2.5 h-2.5 rounded-full bg-emerald-500" />
                  Limit: <strong>{Math.round(limit * 10) / 10}</strong>
                </span>
                <span className="flex items-center gap-1.5 text-cyan-600 dark:text-cyan-400">
                  <span className="w-2.5 h-2.5 rounded-full bg-cyan-500" />
                  In-Flight: <strong>{inFlight}</strong>
                </span>
              </div>
            </div>

            <div className="relative w-full h-44 bg-[var(--chip)] rounded-xl overflow-hidden p-3 flex items-center justify-center">
              {history.length > 2 ? (
                <div className="relative w-full h-full">
                  {/* Y-Axis Scale Guides */}
                  <div className="absolute left-1 top-1 text-[9px] font-mono text-[var(--faint)] select-none">
                    {Math.round(maxChartLimit)}
                  </div>
                  <div className="absolute left-1 top-1/2 -translate-y-1/2 text-[9px] font-mono text-[var(--faint)] select-none">
                    {Math.round(maxChartLimit / 2)}
                  </div>
                  <div className="absolute left-1 bottom-1 text-[9px] font-mono text-[var(--faint)] select-none">
                    0
                  </div>

                  <svg
                    viewBox={`0 0 ${chartWidth} ${chartHeight}`}
                    className="w-full h-full pl-6 overflow-visible"
                    preserveAspectRatio="none"
                  >
                    <line x1="0" y1="20" x2={chartWidth} y2="20" stroke="var(--line)" strokeDasharray="3 3" opacity="0.6" />
                    <line x1="0" y1="80" x2={chartWidth} y2="80" stroke="var(--line)" strokeDasharray="3 3" opacity="0.6" />
                    <line x1="0" y1="140" x2={chartWidth} y2="140" stroke="var(--line)" strokeDasharray="3 3" opacity="0.6" />

                    {/* Limit Curve */}
                    <polyline
                      fill="none"
                      stroke="#10b981"
                      strokeWidth="2.5"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      points={pointsLimit}
                    />

                    {/* In-Flight Curve */}
                    <polyline
                      fill="none"
                      stroke="#06b6d4"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      points={pointsInFlight}
                    />
                  </svg>
                </div>
              ) : (
                <div className="text-xs text-[var(--muted)] flex items-center gap-2">
                  <Activity className="w-4 h-4 animate-spin text-emerald-500" />
                  {mode === "simulator"
                    ? 'Awaiting telemetry... Click "Start Traffic" above to begin.'
                    : "Connecting to 127.0.0.1:8080 metrics endpoint..."}
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
                  className="flex-1 text-[11px] py-1.5 px-2 rounded-lg bg-rose-500/10 text-rose-600 dark:text-rose-400 hover:bg-rose-500/20 transition-colors font-medium text-center cursor-pointer"
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
                  className="flex-1 text-[11px] py-1.5 px-2 rounded-lg bg-[var(--chip)] text-[var(--fg)] hover:bg-[var(--chip-hover)] transition-colors font-medium text-center cursor-pointer"
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
                      className="bg-emerald-500 h-full transition-all duration-200"
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
                      className="bg-blue-500 h-full transition-all duration-200"
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
                      className="bg-amber-500 h-full transition-all duration-200"
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
          <div className="flex items-center gap-2.5">
            <h3 className="text-xs uppercase tracking-wider font-semibold text-[var(--muted)] flex items-center gap-2">
              <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse" />
              Live Request Stream (Gateway Decisions)
            </h3>
            {logs.length > 0 && (
              <button
                type="button"
                onClick={() => setLogs([])}
                className="text-[11px] font-mono px-2 py-0.5 rounded border border-[var(--line)] bg-[var(--chip)] text-[var(--muted)] hover:text-rose-500 hover:bg-[var(--chip-hover)] transition-colors cursor-pointer"
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

        {/* Scrollable logs with standard overscroll containment */}
        <div className="font-mono text-xs max-h-64 overflow-y-auto overflow-x-hidden divide-y divide-[var(--line)] border border-[var(--line)] rounded-xl bg-[var(--chip)] overscroll-contain">
          {logs.length === 0 ? (
            <div className="p-6 text-center text-[var(--muted)] flex flex-col items-center gap-2">
              <Terminal className="w-5 h-5 opacity-40" />
              <span>
                {mode === "simulator"
                  ? 'No requests recorded yet. Click "Start Traffic" above or choose a scenario preset to begin.'
                  : "Waiting for requests on http://127.0.0.1:8080..."}
              </span>
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

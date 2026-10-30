import type { Metadata } from "next";
import { FlowGateShell } from "@/components/flowgate/page-shell";

export const metadata: Metadata = {
  title: "Interactive Adaptive Concurrency Testbench",
  description:
    "Simulate real-time database brownouts, latency spikes, priority-tier shedding, and circuit breaker fast-fails in Go with FlowGate.",
  alternates: {
    canonical: "https://flowgate-fire.vercel.app/flowgate",
  },
};

export default function FlowGatePage() {
  return <FlowGateShell defaultPage="testbench" />;
}

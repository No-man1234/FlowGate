import type { Metadata } from "next";
import { FlowGateHeader } from "@/components/flowgate/header";
import { FlowGateVisualizer } from "@/components/flowgate/visualizer";
import { FlowGateFooter } from "@/components/flowgate/footer";

export const metadata: Metadata = {
  title: "Interactive Adaptive Concurrency Testbench",
  description:
    "Simulate real-time database brownouts, latency spikes, priority-tier shedding, and circuit breaker fast-fails in Go with FlowGate.",
  alternates: {
    canonical: "https://flowgate-fire.vercel.app/flowgate",
  },
};

export default function FlowGatePage() {
  return (
    <div className="page min-h-screen px-4 md:px-8 max-w-6xl mx-auto flex flex-col box-border">
      <FlowGateHeader />
      <main className="flex flex-col py-6">
        <FlowGateVisualizer />
      </main>
      <FlowGateFooter />
    </div>
  );
}

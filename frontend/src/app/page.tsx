import { FlowGateHeader } from "@/components/flowgate/header";
import { FlowGateHero } from "@/components/flowgate/hero";
import { FlowGateVisualizer } from "@/components/flowgate/visualizer";
import { ProblemComparison } from "@/components/flowgate/problem-comparison";
import { HowItWorks } from "@/components/flowgate/how-it-works";
import { FlowGateBenchmarks } from "@/components/flowgate/benchmarks";
import { FlowGateQuickstart } from "@/components/flowgate/quickstart";
import { FlowGateFooter } from "@/components/flowgate/footer";

export default function Home() {
  return (
    <div className="page min-h-screen px-4 md:px-8 max-w-6xl mx-auto flex flex-col box-border">
      <FlowGateHeader />
      <main className="flex flex-col">
        <FlowGateHero />
        <FlowGateVisualizer />
        <ProblemComparison />
        <HowItWorks />
        <FlowGateBenchmarks />
        <FlowGateQuickstart />
      </main>
      <FlowGateFooter />
    </div>
  );
}

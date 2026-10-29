import { FlowGateHeader } from "@/components/flowgate/header";
import { FlowGateVisualizer } from "@/components/flowgate/visualizer";
import { FlowGateFooter } from "@/components/flowgate/footer";

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

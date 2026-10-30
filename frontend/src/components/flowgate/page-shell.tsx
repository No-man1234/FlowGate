"use client";

import { FlowGateHeader } from "./header";
import { FlowGateHero } from "./hero";
import { FlowGateVisualizer } from "./visualizer";
import { ProblemComparison } from "./problem-comparison";
import { HowItWorks } from "./how-it-works";
import { FlowGateBenchmarks } from "./benchmarks";
import { FlowGateQuickstart } from "./quickstart";
import { FlowGateFAQ } from "./faq";
import { FlowGateFooter } from "./footer";
import { FlowGatePager } from "./page-pager";
import { NavigationProvider, useNavigation, PageId } from "./navigation-context";

function FlowGateContent() {
  const { activePage } = useNavigation();

  return (
    <div className="page min-h-screen px-4 md:px-8 max-w-6xl mx-auto flex flex-col box-border">
      <FlowGateHeader />
      <main className="flex flex-col flex-1">
        {/* Page 1: Overview */}
        <div className={activePage === "overview" ? "block animate-page-in" : "hidden"}>
          <FlowGateHero />
        </div>

        {/* Page 2: Interactive Testbench */}
        <div className={activePage === "testbench" ? "block animate-page-in" : "hidden"}>
          <FlowGateVisualizer />
        </div>

        {/* Page 3: The Problem */}
        <div className={activePage === "problem" ? "block animate-page-in" : "hidden"}>
          <ProblemComparison />
        </div>

        {/* Page 4: How It Works */}
        <div className={activePage === "how-it-works" ? "block animate-page-in" : "hidden"}>
          <HowItWorks />
        </div>

        {/* Page 5: Benchmarks */}
        <div className={activePage === "benchmarks" ? "block animate-page-in" : "hidden"}>
          <FlowGateBenchmarks />
        </div>

        {/* Page 6: FAQ */}
        <div className={activePage === "faq" ? "block animate-page-in" : "hidden"}>
          <FlowGateFAQ />
        </div>

        {/* Page 7: Quickstart */}
        <div className={activePage === "quickstart" ? "block animate-page-in" : "hidden"}>
          <FlowGateQuickstart />
        </div>

        {/* Bottom Pager on every page */}
        <FlowGatePager />
      </main>
      <FlowGateFooter />
    </div>
  );
}

export function FlowGateShell({ defaultPage = "overview" }: { defaultPage?: PageId }) {
  return (
    <NavigationProvider defaultPage={defaultPage}>
      <FlowGateContent />
    </NavigationProvider>
  );
}

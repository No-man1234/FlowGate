"use client";

import React, { createContext, useContext, useState, useEffect, useCallback } from "react";

export type PageId =
  | "overview"
  | "testbench"
  | "problem"
  | "how-it-works"
  | "benchmarks"
  | "faq"
  | "quickstart";

export interface PageItem {
  id: PageId;
  label: string;
  shortLabel: string;
  description: string;
  badge?: string;
}

export const PAGES: PageItem[] = [
  {
    id: "overview",
    label: "Overview",
    shortLabel: "Overview",
    description: "Core architecture premise, metrics, and high-concurrency gateway overview.",
  },
  {
    id: "testbench",
    label: "Interactive Testbench",
    shortLabel: "Testbench",
    description: "Simulate brownouts, latency creep, and 429 shedding live in your browser.",
    badge: "Live",
  },
  {
    id: "problem",
    label: "The Problem",
    shortLabel: "The Problem",
    description: "Why traditional RPS rate limits crash upstream databases during latency spikes.",
  },
  {
    id: "how-it-works",
    label: "How It Works",
    shortLabel: "How It Works",
    description: "Gradient2 formula, 3-state Circuit Breaker, and Priority Tier shedding.",
  },
  {
    id: "benchmarks",
    label: "Benchmarks",
    shortLabel: "Benchmarks",
    description: "Real Go microbenchmark numbers: 147ns/op, zero heap allocs, 100% race-clean.",
    badge: "147ns",
  },
  {
    id: "faq",
    label: "FAQ",
    shortLabel: "FAQ",
    description: "Frequently asked questions about adaptive concurrency and Go resilience.",
  },
  {
    id: "quickstart",
    label: "Quickstart",
    shortLabel: "Quickstart",
    description: "Go install commands, Docker Compose, and HTTP middleware code examples.",
  },
];

interface NavigationContextValue {
  activePage: PageId;
  setActivePage: (page: PageId) => void;
  prevPage: PageItem | null;
  nextPage: PageItem | null;
  currentPageIndex: number;
}

const NavigationContext = createContext<NavigationContextValue | null>(null);

function getInitialPage(): PageId {
  if (typeof window === "undefined") return "overview";
  const hash = window.location.hash.replace(/^#/, "");
  const found = PAGES.find((p) => p.id === hash);
  return found ? found.id : "overview";
}

export function NavigationProvider({
  children,
  defaultPage = "overview",
}: {
  children: React.ReactNode;
  defaultPage?: PageId;
}) {
  const [activePage, setActivePageState] = useState<PageId>(defaultPage);

  // Sync state from initial window hash
  useEffect(() => {
    const hash = window.location.hash.replace(/^#/, "");
    const match = PAGES.find((p) => p.id === hash);
    if (match) {
      setActivePageState(match.id);
    }
  }, []);

  // Listen to browser forward/back buttons
  useEffect(() => {
    const handleHashChange = () => {
      const hash = window.location.hash.replace(/^#/, "");
      const match = PAGES.find((p) => p.id === hash);
      if (match) {
        setActivePageState(match.id);
      } else if (!hash) {
        setActivePageState("overview");
      }
    };

    window.addEventListener("hashchange", handleHashChange);
    return () => window.removeEventListener("hashchange", handleHashChange);
  }, []);

  const setActivePage = useCallback((page: PageId) => {
    setActivePageState(page);
    if (typeof window !== "undefined") {
      history.pushState(null, "", `#${page}`);
      window.scrollTo({ top: 0, behavior: "smooth" });
    }
  }, []);

  const currentIndex = PAGES.findIndex((p) => p.id === activePage);
  const prevPage = currentIndex > 0 ? PAGES[currentIndex - 1] : null;
  const nextPage = currentIndex < PAGES.length - 1 ? PAGES[currentIndex + 1] : null;

  return (
    <NavigationContext.Provider
      value={{
        activePage,
        setActivePage,
        prevPage,
        nextPage,
        currentPageIndex: currentIndex >= 0 ? currentIndex : 0,
      }}
    >
      {children}
    </NavigationContext.Provider>
  );
}

export function useNavigation() {
  const context = useContext(NavigationContext);
  if (!context) {
    throw new Error("useNavigation must be used within a NavigationProvider");
  }
  return context;
}

"use client";

import Link from "next/link";
import { Moon, Sun, ShieldCheck } from "lucide-react";
import { GithubIcon } from "./icons";
import { useThemeTransition } from "@/hooks/use-theme-transition";

export function FlowGateHeader() {
  const { isDark, toggleTheme } = useThemeTransition();

  const handleNavClick = (id: string) => (e: React.MouseEvent<HTMLAnchorElement>) => {
    e.preventDefault();
    const el = document.getElementById(id);
    if (el) {
      el.scrollIntoView({ behavior: "smooth", block: "start" });
      history.pushState(null, "", `#${id}`);
    }
  };

  return (
    <header className="site-header flex items-center justify-between gap-4 py-4 border-b border-[var(--line)]">
      {/* Brand */}
      <div className="flex items-center gap-3">
        <Link href="/" className="flex items-center gap-2.5 font-semibold text-[17px] tracking-tight hover:no-underline">
          <span className="w-8 h-8 rounded-lg bg-[var(--chip)] border border-[var(--line)] flex items-center justify-center text-emerald-600 dark:text-emerald-400">
            <ShieldCheck className="w-4 h-4" />
          </span>
          <span className="text-[var(--fg)]">FlowGate</span>
          <span className="text-[11px] font-mono px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
            v1.0.0
          </span>
        </Link>
      </div>

      {/* Navigation */}
      <nav className="hidden md:flex items-center gap-6 text-[14px] text-[var(--muted)]">
        <a href="#testbench" onClick={handleNavClick("testbench")} className="hover:text-[var(--fg)] hover:no-underline transition-colors cursor-pointer">
          Interactive Testbench
        </a>
        <a href="#problem" onClick={handleNavClick("problem")} className="hover:text-[var(--fg)] hover:no-underline transition-colors cursor-pointer">
          The Problem
        </a>
        <a href="#how-it-works" onClick={handleNavClick("how-it-works")} className="hover:text-[var(--fg)] hover:no-underline transition-colors cursor-pointer">
          How It Works
        </a>
        <a href="#benchmarks" onClick={handleNavClick("benchmarks")} className="hover:text-[var(--fg)] hover:no-underline transition-colors cursor-pointer">
          Benchmarks
        </a>
        <a href="#faq" onClick={handleNavClick("faq")} className="hover:text-[var(--fg)] hover:no-underline transition-colors cursor-pointer">
          FAQ
        </a>
        <a href="#quickstart" onClick={handleNavClick("quickstart")} className="hover:text-[var(--fg)] hover:no-underline transition-colors cursor-pointer">
          Quickstart
        </a>
      </nav>

      {/* Action / GitHub / Theme */}
      <div className="flex items-center gap-3">
        <Link
          href="https://github.com/No-man1234/FlowGate"
          target="_blank"
          rel="noopener noreferrer"
          className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium bg-[var(--chip)] text-[var(--fg)] border border-[var(--line)] hover:border-[var(--fg)] hover:no-underline transition-all"
        >
          <GithubIcon className="w-3.5 h-3.5" />
          <span>GitHub</span>
          <span className="text-[10px] text-[var(--faint)]">↗</span>
        </Link>

        <button
          type="button"
          aria-label={isDark ? "Switch to light theme" : "Switch to dark theme"}
          onClick={toggleTheme}
          className="theme-toggle"
        >
          {isDark ? (
            <Sun className="w-3.5 h-3.5" />
          ) : (
            <Moon className="w-3.5 h-3.5" />
          )}
        </button>
      </div>
    </header>
  );
}

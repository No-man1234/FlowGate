"use client";

import Link from "next/link";
import { Moon, Sun } from "lucide-react";
import { GithubIcon, FlowGateLogo } from "./icons";
import { useThemeTransition } from "@/hooks/use-theme-transition";

export function FlowGateFooter() {
  const { isDark, toggleTheme } = useThemeTransition();

  return (
    <footer className="flex flex-wrap items-center justify-between gap-4 py-8 border-t border-[var(--line)] text-xs text-[var(--muted)]">
      <div className="flex items-center gap-2">
        <FlowGateLogo className="w-4 h-4 text-emerald-600 dark:text-emerald-400" />
        <span className="font-semibold text-[var(--fg)]">FlowGate</span>
        <span>— Released under the MIT License</span>
      </div>

      <div className="flex items-center gap-6">
        <Link
          href="https://github.com/No-man1234/FlowGate"
          target="_blank"
          rel="noopener noreferrer"
          className="hover:text-[var(--fg)] flex items-center gap-1.5 hover:no-underline"
        >
          <GithubIcon className="w-3.5 h-3.5" />
          GitHub Repository ↗
        </Link>
        <Link
          href="https://github.com/No-man1234/FlowGate/blob/main/LICENSE"
          target="_blank"
          rel="noopener noreferrer"
          className="hover:text-[var(--fg)] hover:no-underline"
        >
          License (MIT)
        </Link>
        <button
          type="button"
          onClick={() => window.scrollTo({ top: 0, behavior: "smooth" })}
          className="hover:text-[var(--fg)] hover:no-underline cursor-pointer bg-transparent border-0 p-0 text-xs text-[var(--muted)] hover:text-[var(--fg)] transition-colors"
        >
          Back to Top ↑
        </button>

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
    </footer>
  );
}

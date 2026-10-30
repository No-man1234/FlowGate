"use client";

import Link from "next/link";
import { Moon, Sun, ShieldCheck } from "lucide-react";
import { GithubIcon } from "./icons";
import { useThemeTransition } from "@/hooks/use-theme-transition";
import { useNavigation, PAGES } from "./navigation-context";

export function FlowGateHeader() {
  const { isDark, toggleTheme } = useThemeTransition();
  const { activePage, setActivePage } = useNavigation();

  const handleBrandClick = (e: React.MouseEvent) => {
    e.preventDefault();
    setActivePage("overview");
  };

  return (
    <header className="site-header flex flex-col border-b border-[var(--line)]">
      {/* Top Bar: Brand, Desktop Nav, Actions */}
      <div className="flex items-center justify-between gap-4 py-4">
        {/* Brand */}
        <div className="flex items-center gap-3 shrink-0">
          <button
            type="button"
            onClick={handleBrandClick}
            className="flex items-center gap-2.5 font-semibold text-[17px] tracking-tight hover:no-underline cursor-pointer bg-transparent border-0 p-0 text-left"
          >
            <span className="w-8 h-8 rounded-lg bg-[var(--chip)] border border-[var(--line)] flex items-center justify-center text-emerald-600 dark:text-emerald-400 shrink-0">
              <ShieldCheck className="w-4 h-4" />
            </span>
            <span className="text-[var(--fg)]">FlowGate</span>
            <span className="text-[11px] font-mono px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
              v1.0.0
            </span>
          </button>
        </div>

        {/* Desktop Navigation */}
        <nav
          aria-label="Main Navigation"
          className="hidden lg:flex items-center gap-1 text-[13px] text-[var(--muted)]"
        >
          {PAGES.map((page) => {
            const isActive = activePage === page.id;
            return (
              <button
                key={page.id}
                type="button"
                onClick={() => setActivePage(page.id)}
                className={`relative px-3 py-1.5 rounded-lg transition-all cursor-pointer flex items-center gap-1.5 ${
                  isActive
                    ? "bg-[var(--chip)] text-[var(--fg)] font-semibold border border-[var(--line)] shadow-xs"
                    : "text-[var(--muted)] hover:text-[var(--fg)] hover:bg-[var(--chip)]/60"
                }`}
              >
                {isActive && (
                  <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 shrink-0" />
                )}
                <span>{page.label}</span>
                {page.badge && (
                  <span
                    className={`text-[9px] font-mono px-1 py-0.2 rounded-full border ${
                      isActive
                        ? "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border-emerald-500/30"
                        : "bg-[var(--line)] text-[var(--faint)] border-[var(--line)]"
                    }`}
                  >
                    {page.badge}
                  </span>
                )}
              </button>
            );
          })}
        </nav>

        {/* Action / GitHub / Theme */}
        <div className="flex items-center gap-2.5 shrink-0">
          <Link
            href="https://github.com/No-man1234/FlowGate"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium bg-[var(--chip)] text-[var(--fg)] border border-[var(--line)] hover:border-[var(--fg)] hover:no-underline transition-all"
          >
            <GithubIcon className="w-3.5 h-3.5" />
            <span className="hidden sm:inline">GitHub</span>
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
      </div>

      {/* Sub Navigation Bar for tablet and mobile (< lg) */}
      <nav
        aria-label="Mobile Navigation"
        className="flex lg:hidden items-center gap-1.5 overflow-x-auto py-2.5 no-scrollbar -mx-4 px-4 border-t border-[var(--line)]"
      >
        {PAGES.map((page) => {
          const isActive = activePage === page.id;
          return (
            <button
              key={page.id}
              type="button"
              onClick={() => setActivePage(page.id)}
              className={`whitespace-nowrap px-3 py-1 rounded-full text-xs font-medium transition-all shrink-0 cursor-pointer flex items-center gap-1.5 ${
                isActive
                  ? "bg-[var(--fg)] text-[var(--bg)] font-semibold shadow-xs"
                  : "bg-[var(--chip)] text-[var(--muted)] hover:text-[var(--fg)] border border-[var(--line)]"
              }`}
            >
              {isActive && (
                <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 shrink-0" />
              )}
              <span>{page.shortLabel}</span>
              {page.badge && (
                <span className="text-[9px] opacity-80">({page.badge})</span>
              )}
            </button>
          );
        })}
      </nav>
    </header>
  );
}

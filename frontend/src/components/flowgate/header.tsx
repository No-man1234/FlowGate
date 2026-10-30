"use client";

import Link from "next/link";
import { useState, useEffect } from "react";
import { Moon, Sun, ShieldCheck, Menu, X, ChevronRight } from "lucide-react";
import { GithubIcon } from "./icons";
import { useThemeTransition } from "@/hooks/use-theme-transition";
import { useNavigation, PAGES } from "./navigation-context";

export function FlowGateHeader() {
  const { isDark, toggleTheme } = useThemeTransition();
  const { activePage, setActivePage } = useNavigation();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  // Close mobile menu on Escape key
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape" && mobileMenuOpen) {
        setMobileMenuOpen(false);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [mobileMenuOpen]);

  const handleBrandClick = (e: React.MouseEvent) => {
    e.preventDefault();
    setActivePage("overview");
    setMobileMenuOpen(false);
  };

  return (
    <header className="site-header flex flex-col border-b border-[var(--line)]">
      {/* Top Bar: Brand, Desktop Nav, Actions */}
      <div className="flex items-center justify-between gap-3 py-3.5 md:py-4">
        {/* Brand without version tag */}
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
          </button>
        </div>

        {/* Desktop Navigation - strictly single-line, zero double-line buttons */}
        <nav
          aria-label="Main Navigation"
          className="hidden lg:flex items-center gap-1.5 text-[13px] text-[var(--muted)]"
        >
          {PAGES.map((page) => {
            const isActive = activePage === page.id;
            return (
              <button
                key={page.id}
                type="button"
                onClick={() => setActivePage(page.id)}
                className={`whitespace-nowrap shrink-0 px-3 py-1.5 rounded-lg transition-all cursor-pointer flex items-center gap-1.5 ${
                  isActive
                    ? "bg-[var(--chip)] text-[var(--fg)] font-semibold border border-[var(--line)] shadow-xs"
                    : "text-[var(--muted)] hover:text-[var(--fg)] hover:bg-[var(--chip)]/60"
                }`}
              >
                {isActive && (
                  <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 shrink-0" />
                )}
                <span className="whitespace-nowrap">{page.shortLabel}</span>
                {page.badge && (
                  <span
                    className={`text-[9px] font-mono px-1.5 py-0.2 rounded-full border whitespace-nowrap ${
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

        {/* Actions & Mobile Menu Toggle */}
        <div className="flex items-center gap-2 shrink-0">
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

          {/* Smartphone / Mobile Menu Toggle Button */}
          <button
            type="button"
            aria-label={mobileMenuOpen ? "Close navigation menu" : "Open navigation menu"}
            aria-expanded={mobileMenuOpen}
            onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
            className="lg:hidden p-2 rounded-lg bg-[var(--chip)] text-[var(--fg)] border border-[var(--line)] hover:border-[var(--fg)] transition-all cursor-pointer flex items-center justify-center"
          >
            {mobileMenuOpen ? (
              <X className="w-4 h-4 text-[var(--fg)]" />
            ) : (
              <Menu className="w-4 h-4 text-[var(--fg)]" />
            )}
          </button>
        </div>
      </div>

      {/* Smartphone / Tablet Responsive Drawer Menu (< lg) */}
      {mobileMenuOpen && (
        <nav
          aria-label="Mobile Navigation"
          className="lg:hidden flex flex-col gap-1.5 py-3 border-t border-[var(--line)] animate-page-in"
        >
          <div className="flex items-center justify-between px-2 pb-1 text-[11px] font-mono uppercase tracking-wider text-[var(--faint)]">
            <span>Navigation Chapters</span>
            <span>Tap to jump</span>
          </div>

          <div className="flex flex-col gap-1">
            {PAGES.map((page, index) => {
              const isActive = activePage === page.id;
              return (
                <button
                  key={page.id}
                  type="button"
                  onClick={() => {
                    setActivePage(page.id);
                    setMobileMenuOpen(false);
                  }}
                  className={`flex items-center justify-between p-3 rounded-xl text-left transition-all cursor-pointer ${
                    isActive
                      ? "bg-[var(--chip)] text-[var(--fg)] font-semibold border border-[var(--line)] shadow-xs"
                      : "text-[var(--muted)] hover:text-[var(--fg)] hover:bg-[var(--chip)]/60"
                  }`}
                >
                  <div className="flex items-center gap-3 min-w-0">
                    <span
                      className={`w-6 h-6 rounded-lg text-xs font-mono flex items-center justify-center shrink-0 ${
                        isActive
                          ? "bg-emerald-500 text-white font-bold"
                          : "bg-[var(--bg)] border border-[var(--line)] text-[var(--faint)]"
                      }`}
                    >
                      {index + 1}
                    </span>
                    <div className="flex flex-col min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-medium text-[var(--fg)] truncate">
                          {page.label}
                        </span>
                        {page.badge && (
                          <span
                            className={`text-[9px] font-mono px-1.5 py-0.2 rounded-full border ${
                              isActive
                                ? "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border-emerald-500/30"
                                : "bg-[var(--line)] text-[var(--faint)] border-[var(--line)]"
                            }`}
                          >
                            {page.badge}
                          </span>
                        )}
                      </div>
                      <span className="text-[11px] text-[var(--muted)] truncate">
                        {page.description}
                      </span>
                    </div>
                  </div>

                  <ChevronRight
                    className={`w-4 h-4 shrink-0 transition-transform ${
                      isActive ? "text-emerald-500 translate-x-0.5" : "text-[var(--faint)]"
                    }`}
                  />
                </button>
              );
            })}
          </div>
        </nav>
      )}
    </header>
  );
}

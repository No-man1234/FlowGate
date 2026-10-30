"use client";

import { ArrowLeft, ArrowRight, BookOpen, Layers } from "lucide-react";
import { useNavigation, PAGES } from "./navigation-context";

export function FlowGatePager() {
  const { activePage, setActivePage, prevPage, nextPage, currentPageIndex } = useNavigation();

  return (
    <nav
      aria-label="Page navigation"
      className="flex flex-col gap-4 pt-10 pb-8 mt-6 border-t border-[var(--line)]"
    >
      {/* Upper row: Progress dots & indicator */}
      <div className="flex flex-wrap items-center justify-between gap-3 text-xs text-[var(--muted)]">
        <div className="flex items-center gap-2 font-mono">
          <BookOpen className="w-3.5 h-3.5 text-emerald-500" />
          <span>
            Chapter {currentPageIndex + 1} of {PAGES.length}:{" "}
            <strong className="text-[var(--fg)] font-semibold">
              {PAGES[currentPageIndex]?.label}
            </strong>
          </span>
        </div>

        {/* Step indicator pills */}
        <div className="flex items-center gap-1.5" role="tablist" aria-label="Chapters">
          {PAGES.map((page, index) => (
            <button
              key={page.id}
              type="button"
              role="tab"
              aria-selected={activePage === page.id}
              aria-label={`Jump to ${page.label}`}
              onClick={() => setActivePage(page.id)}
              className={`h-2 rounded-full transition-all cursor-pointer ${
                activePage === page.id
                  ? "w-7 bg-emerald-500"
                  : "w-2 bg-[var(--line)] hover:bg-[var(--muted)]"
              }`}
              title={`${index + 1}. ${page.label}`}
            />
          ))}
        </div>
      </div>

      {/* Main navigation buttons */}
      <div className="grid grid-cols-2 gap-3 sm:gap-4 pt-2">
        {/* Previous page */}
        {prevPage ? (
          <button
            type="button"
            onClick={() => setActivePage(prevPage.id)}
            className="group flex items-center gap-3 p-3.5 sm:p-4 rounded-2xl border border-[var(--line)] bg-[var(--chip)] hover:border-[var(--fg)] text-left transition-all cursor-pointer hover:no-underline"
          >
            <div className="w-8 h-8 rounded-xl bg-[var(--bg)] border border-[var(--line)] flex items-center justify-center text-[var(--muted)] group-hover:text-[var(--fg)] group-hover:-translate-x-0.5 transition-transform shrink-0">
              <ArrowLeft className="w-4 h-4" />
            </div>
            <div className="flex flex-col min-w-0">
              <span className="text-[10px] uppercase font-mono tracking-wider text-[var(--faint)]">
                Previous Chapter
              </span>
              <span className="text-sm font-semibold text-[var(--fg)] truncate">
                {prevPage.label}
              </span>
            </div>
          </button>
        ) : (
          <div className="hidden sm:block p-3.5 rounded-2xl border border-dashed border-[var(--line)]/50 text-[var(--faint)] text-xs flex items-center justify-center">
            Beginning of documentation
          </div>
        )}

        {/* Next page */}
        {nextPage ? (
          <button
            type="button"
            onClick={() => setActivePage(nextPage.id)}
            className="group flex items-center justify-between p-3.5 sm:p-4 rounded-2xl border border-[var(--line)] bg-[var(--chip)] hover:border-[var(--fg)] text-right transition-all cursor-pointer hover:no-underline col-start-2"
          >
            <div className="flex flex-col items-end min-w-0 flex-1 pr-3">
              <span className="text-[10px] uppercase font-mono tracking-wider text-[var(--faint)]">
                Next Chapter
              </span>
              <span className="text-sm font-semibold text-[var(--fg)] truncate">
                {nextPage.label}
              </span>
            </div>
            <div className="w-8 h-8 rounded-xl bg-[var(--bg)] border border-[var(--line)] flex items-center justify-center text-[var(--muted)] group-hover:text-[var(--fg)] group-hover:translate-x-0.5 transition-transform shrink-0">
              <ArrowRight className="w-4 h-4" />
            </div>
          </button>
        ) : (
          <button
            type="button"
            onClick={() => setActivePage("overview")}
            className="group flex items-center justify-between p-3.5 sm:p-4 rounded-2xl border border-emerald-500/30 bg-emerald-500/5 hover:border-emerald-500 text-right transition-all cursor-pointer hover:no-underline col-start-2"
          >
            <div className="flex flex-col items-end min-w-0 flex-1 pr-3">
              <span className="text-[10px] uppercase font-mono tracking-wider text-emerald-600 dark:text-emerald-400">
                Documentation Complete
              </span>
              <span className="text-sm font-semibold text-[var(--fg)]">
                Back to Overview ↺
              </span>
            </div>
            <div className="w-8 h-8 rounded-xl bg-[var(--bg)] border border-emerald-500/30 flex items-center justify-center text-emerald-600 dark:text-emerald-400 group-hover:rotate-180 transition-transform shrink-0">
              <Layers className="w-4 h-4" />
            </div>
          </button>
        )}
      </div>
    </nav>
  );
}

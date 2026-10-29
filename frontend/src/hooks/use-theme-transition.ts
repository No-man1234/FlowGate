"use client";

import { useEffect, useState, useCallback } from "react";

export function useThemeTransition() {
  const [isDark, setIsDark] = useState(false);

  useEffect(() => {
    const frame = requestAnimationFrame(() => {
      setIsDark(document.documentElement.classList.contains("dark"));
    });
    return () => cancelAnimationFrame(frame);
  }, []);

  const toggleTheme = useCallback(async (e?: React.MouseEvent<HTMLElement>) => {
    const willBeDark = !document.documentElement.classList.contains("dark");
    setIsDark(willBeDark);

    const applyTheme = () => {
      if (willBeDark) {
        document.documentElement.classList.add("dark");
        try {
          localStorage.setItem("theme", "dark");
        } catch {}
      } else {
        document.documentElement.classList.remove("dark");
        try {
          localStorage.setItem("theme", "light");
        } catch {}
      }
    };

    // Check if View Transitions API is supported
    if (!document.startViewTransition) {
      applyTheme();
      return;
    }

    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      document.documentElement.classList.add("vt", "vt-fade");
      document.startViewTransition(applyTheme).finished.finally(() => {
        document.documentElement.classList.remove("vt", "vt-fade");
      });
      return;
    }

    // Compute center of clicked button or center of viewport
    let x = window.innerWidth / 2;
    let y = window.innerHeight / 2;

    if (e && e.currentTarget) {
      const rect = e.currentTarget.getBoundingClientRect();
      x = rect.left + rect.width / 2;
      y = rect.top + rect.height / 2;
    }

    const maxRadius = Math.hypot(
      Math.max(x, window.innerWidth - x),
      Math.max(y, window.innerHeight - y)
    );

    document.documentElement.classList.add("vt");

    const transition = document.startViewTransition(applyTheme);

    transition.finished
      .catch(() => {})
      .finally(() => {
        document.documentElement.classList.remove("vt");
      });

    try {
      await transition.ready;
      const animation = document.documentElement.animate(
        {
          clipPath: [
            `circle(0px at ${x}px ${y}px)`,
            `circle(${maxRadius}px at ${x}px ${y}px)`,
          ],
        },
        {
          duration: 650,
          easing: "cubic-bezier(0.65, 0, 0.35, 1)",
          pseudoElement: "::view-transition-new(root)",
        }
      );
      await animation.finished.catch(() => {});
    } catch {
      // In case animate or ready fails
    }
  }, []);

  return { isDark, toggleTheme };
}

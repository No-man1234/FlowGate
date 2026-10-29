import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "FlowGate — Adaptive Concurrency API Gateway in Go",
  description:
    "High-concurrency resilient API gateway with Netflix-style adaptive gradient concurrency limiting, 3-state circuit breaking, and priority-tiered backpressure in Go.",
  icons: {
    icon: "/favicon.ico",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="theme" suppressHydrationWarning>
      <head>
        <script
          dangerouslySetInnerHTML={{
            __html: `
              (function() {
                try {
                  const stored = localStorage.getItem('theme');
                  const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
                  if (stored === 'dark' || (!stored && prefersDark)) {
                    document.documentElement.classList.add('dark');
                  } else {
                    document.documentElement.classList.remove('dark');
                  }
                } catch (e) {}
              })();
            `,
          }}
        />
      </head>
      <body className="min-h-screen antialiased bg-[var(--bg)] text-[var(--fg)]">{children}</body>
    </html>
  );
}

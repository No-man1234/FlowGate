import type { Metadata, Viewport } from "next";
import "./globals.css";
import { FlowGateJsonLd } from "@/components/flowgate/json-ld";

export const viewport: Viewport = {
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#fdfdfc" },
    { media: "(prefers-color-scheme: dark)", color: "#0e0e0d" },
  ],
  width: "device-width",
  initialScale: 1,
  maximumScale: 5,
};

export const metadata: Metadata = {
  metadataBase: new URL("https://flowgate-fire.vercel.app"),
  title: {
    default: "FlowGate — Adaptive Concurrency API Gateway in Go",
    template: "%s | FlowGate",
  },
  description:
    "High-concurrency resilient API gateway with Netflix-style adaptive gradient concurrency limiting, 3-state circuit breaking, and priority-tiered backpressure in Go. Prevents cascading failures and database brownouts.",
  applicationName: "FlowGate",
  authors: [{ name: "Abdullah Al Noman", url: "https://github.com/No-man1234" }],
  generator: "Next.js",
  keywords: [
    "flowgate",
    "api gateway",
    "adaptive concurrency",
    "concurrency limiter",
    "golang",
    "go api gateway",
    "circuit breaker",
    "load shedding",
    "rate limiting alternative",
    "gradient concurrency",
    "database brownout",
    "sre",
    "resilience engineering",
    "reverse proxy",
    "backpressure",
    "distributed systems",
    "microservices gateway",
    "netflix concurrency limits",
    "high throughput proxy",
    "zero allocs",
    "goroutine leak prevention",
    "p50 latency",
    "p99 latency",
    "vegas congestion control",
  ],
  creator: "FlowGate",
  publisher: "FlowGate",
  formatDetection: {
    email: false,
    address: false,
    telephone: false,
  },
  alternates: {
    canonical: "https://flowgate-fire.vercel.app",
  },
  openGraph: {
    type: "website",
    locale: "en_US",
    url: "https://flowgate-fire.vercel.app",
    siteName: "FlowGate",
    title: "FlowGate — Adaptive Concurrency API Gateway in Go",
    description:
      "Stop database brownouts from becoming blackouts. FlowGate replaces static rate limits with sub-microsecond adaptive gradient concurrency limiting and 3-state circuit breaking in Go.",
    images: [
      {
        url: "/seo/og-image.png",
        width: 1200,
        height: 630,
        alt: "FlowGate — Adaptive Concurrency API Gateway in Go",
        type: "image/png",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: "FlowGate — Adaptive Concurrency API Gateway in Go",
    description:
      "Stop database brownouts from becoming blackouts. Adaptive gradient concurrency limiting in Go with zero heap allocations on the hot path.",
    images: ["/seo/og-image.png"],
    creator: "@No_man1234",
  },
  robots: {
    index: true,
    follow: true,
    nocache: false,
    googleBot: {
      index: true,
      follow: true,
      noimageindex: false,
      "max-video-preview": -1,
      "max-image-preview": "large",
      "max-snippet": -1,
    },
  },
  icons: {
    icon: [
      { url: "/favicon.ico" },
      { url: "/icons/icon-32x32.png", sizes: "32x32", type: "image/png" },
      { url: "/icons/icon-192x192.png", sizes: "192x192", type: "image/png" },
    ],
    apple: [
      { url: "/icons/apple-touch-icon.png", sizes: "180x180", type: "image/png" },
    ],
  },
  manifest: "/manifest.webmanifest",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="theme" suppressHydrationWarning>
      <head>
        <FlowGateJsonLd />
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

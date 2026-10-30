import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "FlowGate — Adaptive Concurrency Gateway",
    short_name: "FlowGate",
    description:
      "High-concurrency resilient API gateway with adaptive gradient concurrency limiting, 3-state circuit breaking, and priority-tiered backpressure in Go.",
    start_url: "/",
    display: "standalone",
    background_color: "#0e0e0d",
    theme_color: "#0e0e0d",
    icons: [
      {
        src: "/icons/icon-192x192.png",
        sizes: "192x192",
        type: "image/png",
      },
      {
        src: "/icons/icon-512x512.png",
        sizes: "512x512",
        type: "image/png",
      },
    ],
  };
}

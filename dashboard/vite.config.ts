import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { VitePWA } from "vite-plugin-pwa";

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    VitePWA({
      registerType: "autoUpdate",
      manifest: false,
      workbox: {
        cleanupOutdatedCaches: true,
        clientsClaim: true,
        skipWaiting: true,
        navigateFallback: "/index.html",
        globPatterns: ["**/*.{js,css,html,svg,png,webmanifest,woff,woff2}"],
        runtimeCaching: [
          {
            urlPattern: ({ url, request }) =>
              request.method === "GET" &&
              /^\/api\/v1\/photos\/\d+\/(thumbnail|image)$/.test(url.pathname),
            handler: "StaleWhileRevalidate",
            options: {
              cacheName: "ok-folio-piece-images",
              expiration: {
                maxEntries: 500,
                maxAgeSeconds: 60 * 60 * 24 * 30,
              },
              cacheableResponse: {
                statuses: [0, 200],
              },
            },
          },
          {
            urlPattern: ({ url, request }) =>
              request.method === "GET" &&
              url.pathname.startsWith("/api/v1/") &&
              !url.pathname.startsWith("/api/v1/stream/") &&
              request.headers.get("accept") !== "text/event-stream" &&
              !/^\/api\/v1\/photos\/\d+\/(thumbnail|image)$/.test(url.pathname),
            handler: "NetworkFirst",
            options: {
              cacheName: "ok-folio-api-get",
              expiration: {
                maxEntries: 120,
                maxAgeSeconds: 60 * 60 * 6,
              },
              cacheableResponse: {
                statuses: [0, 200],
              },
            },
          },
        ],
      },
    }),
  ],
  server: {
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
      "/health": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes("node_modules")) {
            return;
          }

          const dependencyPath = id.split("node_modules/").pop() ?? id;

          if (
            dependencyPath.startsWith("recharts/") ||
            dependencyPath.startsWith("d3-")
          ) {
            return "charts";
          }

          if (dependencyPath.startsWith("@tanstack/react-query/")) {
            return "query";
          }

          if (
            dependencyPath.startsWith("react/") ||
            dependencyPath.startsWith("react-dom/") ||
            dependencyPath.startsWith("react-router/")
          ) {
            return "react";
          }
        },
      },
    },
  },
});

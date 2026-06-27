import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  define: {
    "import.meta.env.VITE_PHOTOPRISM_PORT": JSON.stringify(
      process.env.VITE_PHOTOPRISM_PORT || "1111",
    ),
  },
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

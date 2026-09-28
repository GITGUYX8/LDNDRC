import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Student SPA is served by the Go binary itself (embed.FS on `/`).
// Keep base "/" so asset URLs work behind control.ros-platform.local.
export default defineConfig({
  plugins: [react()],
  base: "/",
  build: {
    outDir: "dist",
    emptyOutDir: true,
    sourcemap: false,
  },
  server: {
    port: 5173,
    proxy: {
      "/api": "http://127.0.0.1:8082",
      "/healthz": "http://127.0.0.1:8082",
    },
  },
});

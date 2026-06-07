import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

declare const process: { env: Record<string, string | undefined> };

const backendPort = process.env.BACKEND_PORT ?? "3000";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/ws": {
        target: `ws://localhost:${backendPort}`,
        ws: true
      },
      "/health": {
        target: `http://localhost:${backendPort}`
      },
      "/auth": {
        target: `http://localhost:${backendPort}`
      },
      "/games": {
        target: `http://localhost:${backendPort}`
      }
    }
  }
});

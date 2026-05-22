import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/ws": {
        target: "ws://localhost:3000",
        ws: true
      },
      "/health": {
        target: "http://localhost:3000"
      },
      "/auth": {
        target: "http://localhost:3000"
      },
      "/games": {
        target: "http://localhost:3000"
      }
    }
  }
});

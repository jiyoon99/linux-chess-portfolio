import { defineConfig, devices } from "@playwright/test";

declare const process: { env: Record<string, string | undefined> };

const backendPort = process.env.BACKEND_PORT ?? "3000";
const frontendPort = process.env.FRONTEND_PORT ?? "5173";

export default defineConfig({
  testDir: "./tests/smoke",
  timeout: 30_000,
  expect: {
    timeout: 10_000
  },
  use: {
    baseURL: `http://127.0.0.1:${frontendPort}`,
    trace: "retain-on-failure"
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] }
    }
  ],
  webServer: process.env.SKIP_WEBSERVER
    ? []
    : [
        {
          command: `PORT=${backendPort} npm run dev:backend`,
          url: `http://127.0.0.1:${backendPort}/health`,
          reuseExistingServer: !process.env.CI,
          stdout: "pipe",
          stderr: "pipe"
        },
        {
          command: `BACKEND_PORT=${backendPort} npm --workspace frontend run dev -- --host 127.0.0.1 --port ${frontendPort}`,
          url: `http://127.0.0.1:${frontendPort}`,
          reuseExistingServer: !process.env.CI,
          stdout: "pipe",
          stderr: "pipe"
        }
      ]
});

import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./tests/smoke",
  timeout: 30_000,
  expect: {
    timeout: 10_000
  },
  use: {
    baseURL: "http://127.0.0.1:5173",
    trace: "retain-on-failure"
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] }
    }
  ],
  webServer: [
    {
      command: "npm run dev:backend",
      url: "http://127.0.0.1:3000/health",
      reuseExistingServer: !process.env.CI,
      stdout: "pipe",
      stderr: "pipe"
    },
    {
      command: "npm --workspace frontend run dev -- --host 127.0.0.1 --port 5173",
      url: "http://127.0.0.1:5173",
      reuseExistingServer: !process.env.CI,
      stdout: "pipe",
      stderr: "pipe"
    }
  ]
});

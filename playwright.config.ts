import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests/browser",
  testMatch: "**/*.pw.ts",
  fullyParallel: true,
  workers: 2,
  reporter: "list",
  outputDir: "output/playwright/test-results",
  use: {
    ...(process.env.PLAYWRIGHT_CHANNEL
      ? { channel: process.env.PLAYWRIGHT_CHANNEL }
      : {}),
    baseURL: "http://127.0.0.1:4322",
    viewport: { width: 360, height: 800 },
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
  },
  webServer: [
    {
      command: "bun tests/support/public-api.mjs",
      url: "http://127.0.0.1:8089/api/v1/categories",
      reuseExistingServer: false,
    },
    {
      env: { CATALOG_API_ORIGIN: "http://127.0.0.1:8089" },
      command: "bun run dev -- --host 127.0.0.1 --port 4322",
      url: "http://127.0.0.1:4322",
      reuseExistingServer: false,
      timeout: 30000,
    },
  ],
});

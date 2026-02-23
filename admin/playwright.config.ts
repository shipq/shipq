import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "tests/e2e",
  timeout: 30_000,
  use: {
    baseURL: "http://localhost:3000",
    headless: true,
  },
  webServer: {
    command: "cd .. && npm run admin:build:dev && tsx admin/mock-server.ts",
    url: "http://localhost:3000/admin",
    reuseExistingServer: !process.env.CI,
    timeout: 10_000,
  },
});

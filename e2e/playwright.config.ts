import { defineConfig } from '@playwright/test';

/**
 * Playwright config for the marketplace API e2e suite. The marketplace is a
 * headless Go HTTP service, so the tests use Playwright's `request` (API) mode
 * against the public REST surface — no browser project is needed.
 *
 * The webServer block boots the Go service from the repo root and indexes the
 * committed catalog. `reuseExistingServer` lets a locally running instance be
 * reused (start it yourself with `HTTP_PORT=6060 CATALOG_DIR=./catalog go run ./src/main.go`).
 */
const PORT = process.env.MARKETPLACE_PORT ?? '6060';
const BASE_URL = `http://127.0.0.1:${PORT}`;

export default defineConfig({
  testDir: '.',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  ...(process.env.CI ? { workers: 1 } : {}),
  reporter: 'html',

  use: {
    baseURL: BASE_URL,
    extraHTTPHeaders: { Accept: 'application/json' },
    trace: 'on-first-retry',
  },

  webServer: {
    command: `cd .. && CATALOG_DIR=./catalog HTTP_PORT=${PORT} GO_ENV=dev go run ./src/main.go`,
    url: `${BASE_URL}/api/v1/devices?perPage=1`,
    reuseExistingServer: true,
    timeout: 120 * 1000,
  },
});

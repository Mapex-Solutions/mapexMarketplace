import { test as base } from '@playwright/test';
import { DevicesCatalogResource } from '../devices/devices-catalog.resource';
import { PluginsCatalogResource } from '../workflow_plugins/plugins-catalog.resource';

/**
 * Shared test fixture for the marketplace API suites. It provides one API
 * resource per catalog type, each bound to Playwright's request context. The
 * marketplace is a public, read-only service, so no authentication is wired
 * (the API-only analog of the mapexOS `authenticatedPage` fixture).
 */
export const test = base.extend<{
  devices: DevicesCatalogResource;
  plugins: PluginsCatalogResource;
}>({
  devices: async ({ request }, use) => {
    await use(new DevicesCatalogResource(request));
  },
  plugins: async ({ request }, use) => {
    await use(new PluginsCatalogResource(request));
  },
});

export { expect } from '@playwright/test';

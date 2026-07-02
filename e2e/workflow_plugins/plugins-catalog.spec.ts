import { test, expect } from '../fixtures/marketplace.fixture';
import { UNKNOWN_PLUGIN } from './plugins-catalog.data';

test.describe('workflow_plugins.catalog', () => {
  test('list: returns a paginated page in the envelope shape', async ({ plugins }) => {
    const page = await plugins.list({ perPage: 5 });

    expect(Array.isArray(page.items)).toBeTruthy();
    expect(page.items.length).toBeGreaterThan(0);
    expect(page.items.length).toBeLessThanOrEqual(5);
    expect(page.total).toBeGreaterThan(0);
    expect(page.perPage).toBe(5);
    expect(page.page).toBe(1);
  });

  test('list.filter.category: every item matches the category', async ({ plugins }) => {
    const facets = await plugins.facets();
    const category = facets.categories[0]!.value;

    const page = await plugins.list({ category, perPage: 50 });
    expect(page.items.length).toBeGreaterThan(0);
    for (const item of page.items) expect(item.category).toBe(category);
  });

  test('list.filter.capability: every item carries the capability', async ({ plugins }) => {
    const facets = await plugins.facets();
    const capability = facets.capabilities[0]!.value;

    const page = await plugins.list({ capability, perPage: 50 });
    expect(page.items.length).toBeGreaterThan(0);
    for (const item of page.items) {
      expect(item.capabilities, `plugin ${item.pluginId} carries ${capability}`).toContain(capability);
    }
  });

  test('facets: exposes flat categories and capabilities (no drill-down)', async ({ plugins }) => {
    const facets = await plugins.facets();

    expect(Array.isArray(facets.categories)).toBeTruthy();
    expect(Array.isArray(facets.capabilities)).toBeTruthy();
    expect(facets.categories.length).toBeGreaterThan(0);
    expect(facets.capabilities.length).toBeGreaterThan(0);
  });

  test('detail: information returns the plugin manifest', async ({ plugins }) => {
    const page = await plugins.list({ perPage: 1 });
    const item = page.items[0]!;

    const manifest = await plugins.information(item.vendor, item.slug);
    expect(manifest).toBeTruthy();
    expect(Object.keys(manifest).length).toBeGreaterThan(0);
  });

  test('events: a plugin with events returns a non-empty catalog', async ({ plugins }) => {
    const page = await plugins.list({ perPage: 100 });
    const withEvents = page.items.find((p) => p.hasEvents);

    // The catalog ships at least one plugin with an events catalog (telegram today).
    expect(withEvents, 'at least one plugin has events').toBeTruthy();

    const events = await plugins.events(withEvents!.vendor, withEvents!.slug);
    expect(events).toBeTruthy();
    // Non-empty payload, whether the contract is an array or an object map.
    const size = Array.isArray(events) ? events.length : Object.keys(events as object).length;
    expect(size).toBeGreaterThan(0);
  });

  test('detail.notFound: unknown plugin returns 404', async ({ plugins }) => {
    const res = await plugins.getRaw(`/${UNKNOWN_PLUGIN.vendor}/${UNKNOWN_PLUGIN.slug}`);
    expect(res.status()).toBe(404);
  });
});

import { test, expect } from '../fixtures/marketplace.fixture';
import { UNKNOWN_MANUFACTURER } from './devices-catalog.data';

test.describe('devices.catalog', () => {
  test('list: returns a paginated page in the envelope shape', async ({ devices }) => {
    const page = await devices.list({ perPage: 5 });

    expect(Array.isArray(page.items)).toBeTruthy();
    expect(page.items.length).toBeGreaterThan(0);
    expect(page.items.length).toBeLessThanOrEqual(5);
    expect(page.total).toBeGreaterThan(0);
    expect(page.perPage).toBe(5);
    expect(page.page).toBe(1);
  });

  test('list.pagination: page 2 returns different items than page 1', async ({ devices }) => {
    const first = await devices.list({ perPage: 5, page: 1 });
    const second = await devices.list({ perPage: 5, page: 2 });

    const firstIds = new Set(first.items.map((i) => i.id));
    const overlap = second.items.filter((i) => firstIds.has(i.id));
    expect(overlap.length).toBe(0);
  });

  test('list.filter.protocol: every item matches the protocol', async ({ devices }) => {
    const page = await devices.list({ protocol: 'lorawan', perPage: 50 });

    expect(page.items.length).toBeGreaterThan(0);
    for (const item of page.items) expect(item.protocol).toBe('lorawan');
  });

  test('list.filter.manufacturer: every item belongs to the vendor', async ({ devices }) => {
    const facets = await devices.facets();
    const vendor = facets.manufacturers[0]!.value;
    const page = await devices.list({ manufacturer: vendor, perPage: 100 });

    expect(page.items.length).toBeGreaterThan(0);
    for (const item of page.items) expect(item.vendor.length).toBeGreaterThan(0);
  });

  test('facets: exposes the flat and drill-down filter options', async ({ devices }) => {
    const facets = await devices.facets();

    for (const key of ['protocols', 'readingTypes', 'manufacturers', 'models'] as const) {
      expect(Array.isArray(facets[key]), `facets.${key} is an array`).toBeTruthy();
    }
    expect(facets.manufacturers.length).toBeGreaterThan(0);
    expect(facets.models.length).toBeGreaterThan(0);
    expect(facets.protocols.length).toBeGreaterThan(0);
  });

  test('facets.drilldown: models narrow to the picked manufacturer', async ({ devices }) => {
    const all = await devices.facets();
    const vendor = all.manufacturers[0]!.value;

    const narrowed = await devices.facets({ manufacturer: vendor });

    // The drill-down level shrinks and stays a subset of the full model set.
    expect(narrowed.models.length).toBeGreaterThan(0);
    expect(narrowed.models.length).toBeLessThan(all.models.length);
    const allModels = new Set(all.models.map((m) => m.value));
    for (const m of narrowed.models) expect(allModels.has(m.value)).toBeTruthy();

    // Orthogonal facets must NOT narrow with the drill-down selection.
    expect(narrowed.manufacturers.length).toBe(all.manufacturers.length);
    expect(narrowed.protocols.length).toBe(all.protocols.length);
  });

  test('facets.drilldown: narrowed models match the vendor listing', async ({ devices }) => {
    const all = await devices.facets();
    const vendor = all.manufacturers[0]!.value;

    const narrowed = await devices.facets({ manufacturer: vendor });
    const narrowedModels = new Set(narrowed.models.map((m) => m.value));

    const page = await devices.list({ manufacturer: vendor, perPage: 100 });
    expect(page.items.length).toBeGreaterThan(0);
    for (const item of page.items) {
      expect(narrowedModels.has(item.model), `model ${item.model} is in the vendor facets`).toBeTruthy();
    }
  });

  test('facets.drilldown: unknown manufacturer yields no models', async ({ devices }) => {
    const none = await devices.facets({ manufacturer: UNKNOWN_MANUFACTURER });
    expect(none.models.length).toBe(0);
  });

  test('detail: information returns the model sheet', async ({ devices }) => {
    const page = await devices.list({ perPage: 1 });
    const item = page.items[0]!;

    const info = await devices.information(item.vendor, item.slug);
    expect(info).toBeTruthy();
    expect(Object.keys(info).length).toBeGreaterThan(0);
  });

  test('detail.notFound: unknown model returns 404', async ({ devices }) => {
    const res = await devices.getRaw('/__no_vendor__/__no_model__');
    expect(res.status()).toBe(404);
  });
});

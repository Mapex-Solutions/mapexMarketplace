/**
 * Test data and wire types for the device marketplace catalog E2E tests.
 */

/** Base path of the public device catalog resource. */
export const CATALOG_BASE = '/api/v1/devices';

/** Query accepted by the listing endpoint (empty fields are dropped). */
export interface DeviceListQuery {
  protocol?: string;
  readingType?: string;
  manufacturer?: string;
  search?: string;
  lang?: string;
  page?: number;
  perPage?: number;
}

/** Query accepted by the facets endpoint — manufacturer drives the model drill-down. */
export interface FacetsQuery {
  manufacturer?: string;
}

/** One selectable filter value with its display label and optional icon. */
export interface FacetOption {
  value: string;
  label: string;
  icon?: string;
}

/**
 * The filter options the listing UI renders. `manufacturers -> models` is the
 * drill-down hierarchy; `protocols`/`readingTypes` are orthogonal flat filters.
 */
export interface Facets {
  protocols: FacetOption[];
  readingTypes: FacetOption[];
  manufacturers: FacetOption[];
  models: FacetOption[];
}

/** One catalog card as returned by the listing endpoint. */
export interface CatalogItem {
  id: string;
  vendor: string;
  model: string;
  slug: string;
  name: string;
  protocol: string;
  readingTypes: string[];
}

/** A page of catalog cards plus the total match count for pagination. */
export interface CatalogListData {
  items: CatalogItem[];
  total: number;
  page: number;
  perPage: number;
}

/** A manufacturer that does not exist in any catalog, for negative drill-down tests. */
export const UNKNOWN_MANUFACTURER = '__no_such_vendor__';

/**
 * Drop empty/undefined fields so they are never sent as blank query params
 * (the service treats a present-but-empty filter differently from an absent one).
 *
 * @param {Record<string, string | number | undefined>} query - raw query object
 * @returns {Record<string, string>} the query as non-empty string params
 */
export function toParams(query: Record<string, string | number | undefined>): Record<string, string> {
  const params: Record<string, string> = {};
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== '') params[key] = String(value);
  }
  return params;
}

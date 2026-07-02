/**
 * Test data and wire types for the workflow plugins marketplace E2E tests.
 */

/** Base path of the public workflow plugins catalog resource. */
export const PLUGINS_BASE = '/api/v1/workflow_plugins';

/** Query accepted by the listing endpoint (empty fields are dropped). */
export interface PluginListQuery {
  category?: string;
  capability?: string;
  tag?: string;
  search?: string;
  lang?: string;
  page?: number;
  perPage?: number;
}

/** One selectable filter value with its display label and optional icon. */
export interface FacetOption {
  value: string;
  label: string;
  icon?: string;
}

/**
 * The plugin filter options. Unlike devices, these are FLAT (orthogonal): there
 * is no drill-down hierarchy — a plugin can carry several capabilities and one
 * category, so neither contains the other.
 */
export interface PluginFacets {
  categories: FacetOption[];
  capabilities: FacetOption[];
}

/** One plugin card as returned by the listing endpoint. */
export interface PluginItem {
  vendor: string;
  vendorName: string;
  pluginId: string;
  slug: string;
  name: string;
  category: string;
  capabilities: string[];
  nodeCount: number;
  hasEvents: boolean;
}

/** A page of plugin cards plus the total match count for pagination. */
export interface PluginListData {
  items: PluginItem[];
  total: number;
  page: number;
  perPage: number;
}

/** A vendor/slug that does not exist, for negative detail tests. */
export const UNKNOWN_PLUGIN = { vendor: '__no_vendor__', slug: '__no_plugin__' };

/**
 * Drop empty/undefined fields so they are never sent as blank query params.
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

import type { APIRequestContext, APIResponse } from '@playwright/test';
import { expect } from '@playwright/test';
import {
  CATALOG_BASE,
  toParams,
  type CatalogListData,
  type DeviceListQuery,
  type Facets,
  type FacetsQuery,
} from './devices-catalog.data';

/**
 * API resource for the device marketplace catalog — the API-mode analog of a
 * page object. It wraps the public REST surface (`/api/v1/devices`...) and
 * unwraps the Mapex `{ status, errors, data }` envelope so specs read `.data`
 * directly.
 */
export class DevicesCatalogResource {
  constructor(private readonly request: APIRequestContext) {}

  /**
   * List a page of catalog cards filtered by the query.
   *
   * @param {DeviceListQuery} query - filters plus page/perPage
   * @returns {Promise<CatalogListData>} the page payload
   */
  async list(query: DeviceListQuery = {}): Promise<CatalogListData> {
    const res = await this.request.get(CATALOG_BASE, { params: toParams({ ...query }) });
    expect(res.ok(), `GET ${CATALOG_BASE} -> ${res.status()}`).toBeTruthy();
    return (await res.json()).data as CatalogListData;
  }

  /**
   * Fetch the filter options. A manufacturer narrows the model drill-down.
   *
   * @param {FacetsQuery} query - the current drill-down selection
   * @returns {Promise<Facets>} the facet set
   */
  async facets(query: FacetsQuery = {}): Promise<Facets> {
    const res = await this.request.get(`${CATALOG_BASE}/facets`, { params: toParams({ ...query }) });
    expect(res.ok(), `GET ${CATALOG_BASE}/facets -> ${res.status()}`).toBeTruthy();
    return (await res.json()).data as Facets;
  }

  /**
   * Fetch one model's raw information sheet.
   *
   * @param {string} vendor - vendor slug
   * @param {string} slug - model slug
   * @returns {Promise<Record<string, unknown>>} the information JSON
   */
  async information(vendor: string, slug: string): Promise<Record<string, unknown>> {
    const res = await this.request.get(`${CATALOG_BASE}/${vendor}/${slug}`);
    expect(res.ok(), `GET ${CATALOG_BASE}/${vendor}/${slug} -> ${res.status()}`).toBeTruthy();
    return (await res.json()).data as Record<string, unknown>;
  }

  /**
   * Raw GET against a catalog sub-path, for negative/status assertions.
   *
   * @param {string} path - path appended to the catalog base (e.g. `/x/y`)
   * @returns {Promise<APIResponse>} the unparsed response
   */
  async getRaw(path: string): Promise<APIResponse> {
    return this.request.get(`${CATALOG_BASE}${path}`);
  }
}

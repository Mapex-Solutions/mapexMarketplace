import type { APIRequestContext, APIResponse } from '@playwright/test';
import { expect } from '@playwright/test';
import {
  PLUGINS_BASE,
  toParams,
  type PluginFacets,
  type PluginListData,
  type PluginListQuery,
} from './plugins-catalog.data';

/**
 * API resource for the workflow plugins marketplace — the API-mode analog of a
 * page object. It wraps the public REST surface (`/api/v1/workflow_plugins`...)
 * and unwraps the Mapex `{ status, errors, data }` envelope.
 */
export class PluginsCatalogResource {
  constructor(private readonly request: APIRequestContext) {}

  /**
   * List a page of plugin cards filtered by the query.
   *
   * @param {PluginListQuery} query - filters plus page/perPage
   * @returns {Promise<PluginListData>} the page payload
   */
  async list(query: PluginListQuery = {}): Promise<PluginListData> {
    const res = await this.request.get(PLUGINS_BASE, { params: toParams({ ...query }) });
    expect(res.ok(), `GET ${PLUGINS_BASE} -> ${res.status()}`).toBeTruthy();
    return (await res.json()).data as PluginListData;
  }

  /**
   * Fetch the flat filter options (categories, capabilities).
   *
   * @returns {Promise<PluginFacets>} the facet set
   */
  async facets(): Promise<PluginFacets> {
    const res = await this.request.get(`${PLUGINS_BASE}/facets`);
    expect(res.ok(), `GET ${PLUGINS_BASE}/facets -> ${res.status()}`).toBeTruthy();
    return (await res.json()).data as PluginFacets;
  }

  /**
   * Fetch one plugin's raw manifest (information).
   *
   * @param {string} vendor - vendor slug
   * @param {string} slug - plugin slug
   * @returns {Promise<Record<string, unknown>>} the manifest JSON
   */
  async information(vendor: string, slug: string): Promise<Record<string, unknown>> {
    const res = await this.request.get(`${PLUGINS_BASE}/${vendor}/${slug}`);
    expect(res.ok(), `GET ${PLUGINS_BASE}/${vendor}/${slug} -> ${res.status()}`).toBeTruthy();
    return (await res.json()).data as Record<string, unknown>;
  }

  /**
   * Fetch one plugin's trigger-events catalog.
   *
   * @param {string} vendor - vendor slug
   * @param {string} slug - plugin slug
   * @returns {Promise<unknown>} the events JSON
   */
  async events(vendor: string, slug: string): Promise<unknown> {
    const res = await this.request.get(`${PLUGINS_BASE}/${vendor}/${slug}/events`);
    expect(res.ok(), `GET ${PLUGINS_BASE}/${vendor}/${slug}/events -> ${res.status()}`).toBeTruthy();
    return (await res.json()).data;
  }

  /**
   * Raw GET against a catalog sub-path, for negative/status assertions.
   *
   * @param {string} path - path appended to the plugins base (e.g. `/x/y`)
   * @returns {Promise<APIResponse>} the unparsed response
   */
  async getRaw(path: string): Promise<APIResponse> {
    return this.request.get(`${PLUGINS_BASE}${path}`);
  }
}

package dtos

import contracts "mapexmarketplace/packages/contracts/devices"

// The module speaks the canonical catalog contracts; it never redefines the wire
// shapes. Aliases keep handlers and the service on the contract types.
type CatalogItem = contracts.CatalogItem

type CatalogQuery = contracts.CatalogQuery

type CatalogListResponse = contracts.CatalogListResponse

type FacetOption = contracts.FacetOption

type Facets = contracts.Facets

type Codec = contracts.Codec

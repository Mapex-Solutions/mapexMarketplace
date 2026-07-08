package bundle

import web "github.com/Mapex-Solutions/mapexGoKit/microservices/http/web"

// Response headers for a verifiable raw bundle. The body is the exact on-disk
// bytes the published sha256 is computed over; identity and integrity ride as
// headers so an installer verifies the hash without a re-encoded envelope.
const (
	headerContentType       = "Content-Type"
	headerMarketplaceGuid   = "X-Marketplace-Guid"
	headerMarketplaceSha256 = "X-Marketplace-Sha256"
)

// ServeVerifiableBundle writes body verbatim with its identity and integrity
// metadata as headers, so a consumer can hard-verify the published sha256 against
// exactly these bytes.
//
// It marks the response "private" on purpose: a shared cache or CDN that stores
// the body may drop the non-safelisted X-Marketplace-* headers, and a client
// receiving a header-stripped body would be unable to verify the sha256 and would
// reject a perfectly valid bundle. Private keeps the response out of shared caches
// while still letting a browser cache it (browsers preserve their own headers).
//
// This is the one place marketplace bundles become verifiable-serveable; new
// catalog modules serve their installable bundles through here rather than
// re-implementing the header + cache handling.
func ServeVerifiableBundle(c *web.Ctx, body []byte, marketplaceGuid, sha256 string) error {
	c.Set(headerContentType, "application/json")
	c.Set(headerMarketplaceGuid, marketplaceGuid)
	c.Set(headerMarketplaceSha256, sha256)
	c.Set("Cache-Control", "private, max-age=300")
	return c.Send(body)
}

package bootstrap

import (
	"embed"
	_ "embed"

	web "github.com/Mapex-Solutions/mapexGoKit/microservices/http/web"
)

// homePage is the MapexOS ecosystem portal served at the service root, embedded at
// build time so the binary stays self-contained (no asset directory to ship). It
// opens with a living view of the platform — an animated ingest → transform → route
// → automate flow — and its Marketplace section renders the device grid and
// headline counters in the browser from this service's own live catalog API
// (/api/v1/devices), so a newly indexed vendor or model shows up with no change here.
//
//go:embed home.html
var homePage []byte

// logoMark is the MapexOS icon (the node-network hexagon) and logoFull is the full
// wordmark, both embedded so the portal carries the real brand without a separate
// asset server.
//
//go:embed only-logo.png
var logoMark []byte

//go:embed logo-mapex.png
var logoFull []byte

// shots holds the embedded product screenshots rendered by the portal's gallery
// sections (the MapexOS platform under shots/mapexos/, the Devices Simulator at the
// top level), served under /assets/shots/.
//
//go:embed shots
var shots embed.FS

// InitHome registers GET / (the MapexOS ecosystem portal) and the two brand assets
// it references. These sit alongside the health probe as infrastructure routes
// rather than a domain module: they serve self-contained documents and own no
// business logic. Without them the root path returned the framework's 404.
func InitHome(app *web.App) {
	app.Get("/", func(c *web.Ctx) error {
		c.Type("html", "utf-8")
		return c.Send(homePage)
	})
	app.Get("/assets/only-logo.png", servePNG(logoMark))
	app.Get("/assets/logo-mapex.png", servePNG(logoFull))
	app.Get("/assets/shots/*", func(c *web.Ctx) error {
		data, err := shots.ReadFile("shots/" + c.Params("*"))
		if err != nil {
			return web.NewError(web.StatusNotFound, "screenshot not found")
		}
		c.Type("png")
		c.Set("Cache-Control", "public, max-age=86400")
		return c.Send(data)
	})
}

// servePNG returns a handler that streams an embedded PNG with a long cache TTL,
// since the brand assets are immutable for the life of the binary.
func servePNG(data []byte) web.Handler {
	return func(c *web.Ctx) error {
		c.Type("png")
		c.Set("Cache-Control", "public, max-age=86400")
		return c.Send(data)
	}
}

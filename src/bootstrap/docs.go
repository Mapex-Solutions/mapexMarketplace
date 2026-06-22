package bootstrap

import (
	"bytes"
	"embed"
	"encoding/json"
	"html/template"

	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
	web "github.com/Mapex-Solutions/mapexGoKit/microservices/http/web"
)

// docsFS holds the generated documentation: one HTML fragment per page under
// docs/<section>/<page>.html plus docs/manifest.json describing the navigation.
// The fragments are produced offline by INTERNALS/docsgen from the MapexOS docs
// markdown, so the marketplace binary serves the documentation with no markdown
// dependency at runtime.
//
//go:embed docs
var docsFS embed.FS

//go:embed docs_layout.html
var docsLayoutRaw string

// docPage / docSection / docManifest mirror docs/manifest.json.
type docPage struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
}
type docSection struct {
	Slug  string    `json:"slug"`
	Title string    `json:"title"`
	Pages []docPage `json:"pages"`
}
type docManifest struct {
	Sections []docSection `json:"sections"`
}

// docRef is a flattened pointer into the manifest, used to resolve prev/next and
// to look up a page's section title for the breadcrumb.
type docRef struct {
	Section docSection
	Page    docPage
}

var (
	docManifestData docManifest
	docFlat         []docRef
	docIndex        = map[string]int{} // "section/page" -> position in docFlat
	docTemplate     *template.Template
)

// docView is the data passed to the layout template per request.
type docView struct {
	Title       string
	Description string
	Crumb       string
	Sidebar     template.HTML
	Content     template.HTML
	PrevHref    string
	PrevTitle   string
	NextHref    string
	NextTitle   string
}

// InitDocs parses the embedded manifest, prepares the layout template, and
// registers the documentation routes (GET /docs and /docs/:section/:page). The
// pages live entirely inside this app — no external redirects.
func InitDocs(app *web.App) {
	raw, err := docsFS.ReadFile("docs/manifest.json")
	if err != nil {
		logger.Panic("[DOCS] read manifest: " + err.Error())
	}
	if err := json.Unmarshal(raw, &docManifestData); err != nil {
		logger.Panic("[DOCS] parse manifest: " + err.Error())
	}
	for _, s := range docManifestData.Sections {
		for _, p := range s.Pages {
			docIndex[s.Slug+"/"+p.Slug] = len(docFlat)
			docFlat = append(docFlat, docRef{Section: s, Page: p})
		}
	}
	docTemplate, err = template.New("docs").Parse(docsLayoutRaw)
	if err != nil {
		logger.Panic("[DOCS] parse layout: " + err.Error())
	}

	app.Get("/docs", func(c *web.Ctx) error {
		if len(docFlat) == 0 {
			return web.NewError(web.StatusNotFound, "no documentation")
		}
		return renderDoc(c, docFlat[0].Section.Slug, docFlat[0].Page.Slug)
	})
	app.Get("/docs/:section/:page", func(c *web.Ctx) error {
		return renderDoc(c, c.Params("section"), c.Params("page"))
	})

	logger.Info("[DOCS] routes registered, pages=" + itoa(len(docFlat)))
}

// renderDoc wraps a page's embedded HTML fragment in the themed layout, with the
// sidebar marked for the current page and prev/next resolved from the flat order.
func renderDoc(c *web.Ctx, section, page string) error {
	pos, ok := docIndex[section+"/"+page]
	if !ok {
		return web.NewError(web.StatusNotFound, "doc not found")
	}
	frag, err := docsFS.ReadFile("docs/" + section + "/" + page + ".html")
	if err != nil {
		return web.NewError(web.StatusNotFound, "doc not found")
	}
	ref := docFlat[pos]

	view := docView{
		Title:       ref.Page.Title,
		Description: ref.Page.Description,
		Crumb:       ref.Section.Title + " / " + ref.Page.Title,
		Sidebar:     template.HTML(buildSidebar(section, page)), //nolint:gosec // trusted, generated markup
		Content:     template.HTML(frag),                        //nolint:gosec // generated from our own docs
	}
	if pos > 0 {
		p := docFlat[pos-1]
		view.PrevHref = "/docs/" + p.Section.Slug + "/" + p.Page.Slug
		view.PrevTitle = p.Page.Title
	}
	if pos < len(docFlat)-1 {
		n := docFlat[pos+1]
		view.NextHref = "/docs/" + n.Section.Slug + "/" + n.Page.Slug
		view.NextTitle = n.Page.Title
	}

	var buf bytes.Buffer
	if err := docTemplate.Execute(&buf, view); err != nil {
		return err
	}
	c.Type("html", "utf-8")
	return c.Send(buf.Bytes())
}

// buildSidebar renders the section/page tree, highlighting the active page.
func buildSidebar(curSection, curPage string) string {
	var b bytes.Buffer
	for _, s := range docManifestData.Sections {
		b.WriteString(`<div class="sec"><p class="sec-t">`)
		b.WriteString(template.HTMLEscapeString(s.Title))
		b.WriteString(`</p>`)
		for _, p := range s.Pages {
			cls := "pg"
			if s.Slug == curSection && p.Slug == curPage {
				cls = "pg active"
			}
			b.WriteString(`<a class="` + cls + `" href="/docs/` + s.Slug + `/` + p.Slug + `">`)
			b.WriteString(template.HTMLEscapeString(p.Title))
			b.WriteString(`</a>`)
		}
		b.WriteString(`</div>`)
	}
	return b.String()
}

// itoa is a tiny dependency-free int formatter for the boot log line.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

package bootstrap

import (
	"bytes"
	"embed"
	"encoding/json"
	"html/template"
	"strconv"
	"strings"

	web "github.com/Mapex-Solutions/mapexGoKit/microservices/http/web"
	logger "github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// docsFS holds the generated documentation, versioned and localized:
//
//	docs/versions.json                       — version index (+ latest, langs)
//	docs/<version>/<lang>/manifest.json       — nav tree
//	docs/<version>/<lang>/<section>/<page>.html
//
// The fragments are produced offline by INTERNALS/docsgen, so the marketplace
// binary serves the documentation with no markdown dependency at runtime.
//
//go:embed docs
var docsFS embed.FS

//go:embed docs_layout.html
var docsLayoutRaw string

// docPage / docSection / docManifest mirror a per-version-lang manifest.json.
type docPage struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Group       string `json:"group,omitempty"`
}
type docSection struct {
	Slug  string    `json:"slug"`
	Title string    `json:"title"`
	Pages []docPage `json:"pages"`
}
type docManifest struct {
	Sections []docSection `json:"sections"`
}

// docVersionsIndex mirrors docs/versions.json.
type docVersionsIndex struct {
	Versions    []string `json:"versions"` // newest first
	Latest      string   `json:"latest"`
	Langs       []string `json:"langs"`
	DefaultLang string   `json:"defaultLang"`
}

// docRef is a flattened pointer into a bundle, for prev/next and breadcrumbs.
type docRef struct {
	Section docSection
	Page    docPage
}

// docBundle is one (version, lang) navigation tree, indexed for O(1) lookups.
type docBundle struct {
	flat      []docRef
	index     map[string]int // "section/page" -> position in flat
	sections  []docSection
	firstSec  string
	firstPage string
}

var (
	docVersions docVersionsIndex
	docBundles  = map[string]*docBundle{} // key: "version/lang"
	docTemplate *template.Template
)

func bundleKey(ver, lang string) string { return ver + "/" + lang }

// docView is the data passed to the layout template per request.
type docView struct {
	Title       string
	Description string
	Crumb       string
	VerBar      template.HTML
	Sidebar     template.HTML
	Content     template.HTML
	PrevHref    string
	PrevTitle   string
	NextHref    string
	NextTitle   string
}

// InitDocs parses the embedded version index + per-version manifests, prepares
// the layout template, and registers the versioned documentation routes. The
// pages live entirely inside this app — no external redirects; /docs always
// lands on the latest version.
func InitDocs(app *web.App) {
	raw, err := docsFS.ReadFile("docs/versions.json")
	if err != nil {
		logger.Panic("[DOCS] read versions.json: " + err.Error())
	}
	if err := json.Unmarshal(raw, &docVersions); err != nil {
		logger.Panic("[DOCS] parse versions.json: " + err.Error())
	}

	for _, ver := range docVersions.Versions {
		for _, lang := range docVersions.Langs {
			mraw, err := docsFS.ReadFile("docs/" + ver + "/" + lang + "/manifest.json")
			if err != nil {
				continue // not every version need carry every language
			}
			var mf docManifest
			if json.Unmarshal(mraw, &mf) != nil {
				continue
			}
			b := &docBundle{index: map[string]int{}, sections: mf.Sections}
			for _, s := range mf.Sections {
				for _, p := range s.Pages {
					if b.firstSec == "" {
						b.firstSec, b.firstPage = s.Slug, p.Slug
					}
					b.index[s.Slug+"/"+p.Slug] = len(b.flat)
					b.flat = append(b.flat, docRef{Section: s, Page: p})
				}
			}
			docBundles[bundleKey(ver, lang)] = b
		}
	}

	docTemplate, err = template.New("docs").Parse(docsLayoutRaw)
	if err != nil {
		logger.Panic("[DOCS] parse layout: " + err.Error())
	}

	// /docs and partial paths redirect to a concrete page on the latest version.
	app.Get("/docs", func(c *web.Ctx) error {
		return redirectHome(c, docVersions.Latest, docVersions.DefaultLang)
	})
	app.Get("/docs/:version", func(c *web.Ctx) error {
		return redirectHome(c, c.Params("version"), docVersions.DefaultLang)
	})
	app.Get("/docs/:version/:lang", func(c *web.Ctx) error {
		return redirectHome(c, c.Params("version"), c.Params("lang"))
	})
	app.Get("/docs/:version/:lang/:section/:page", func(c *web.Ctx) error {
		return renderDoc(c, c.Params("version"), c.Params("lang"), c.Params("section"), c.Params("page"))
	})

	logger.Info("[DOCS] routes registered, versions=" + strconv.Itoa(len(docVersions.Versions)) +
		" latest=" + docVersions.Latest + " bundles=" + strconv.Itoa(len(docBundles)))
}

// redirectHome sends the caller to the first page of the requested version/lang,
// falling back to the latest+default bundle when the request is unknown.
func redirectHome(c *web.Ctx, ver, lang string) error {
	b := docBundles[bundleKey(ver, lang)]
	if b == nil {
		ver, lang = docVersions.Latest, docVersions.DefaultLang
		b = docBundles[bundleKey(ver, lang)]
	}
	if b == nil {
		return web.NewError(web.StatusNotFound, "no documentation")
	}
	return c.Redirect("/docs/"+ver+"/"+lang+"/"+b.firstSec+"/"+b.firstPage, 302)
}

// renderDoc wraps a page's embedded HTML fragment in the themed layout, with the
// version/language selectors, the sidebar marked for the current page, and
// prev/next resolved from the version's flat order.
func renderDoc(c *web.Ctx, ver, lang, section, page string) error {
	b := docBundles[bundleKey(ver, lang)]
	if b == nil {
		return web.NewError(web.StatusNotFound, "doc not found")
	}
	pos, ok := b.index[section+"/"+page]
	if !ok {
		return web.NewError(web.StatusNotFound, "doc not found")
	}
	frag, err := docsFS.ReadFile("docs/" + ver + "/" + lang + "/" + section + "/" + page + ".html")
	if err != nil {
		return web.NewError(web.StatusNotFound, "doc not found")
	}
	ref := b.flat[pos]
	base := "/docs/" + ver + "/" + lang + "/"

	view := docView{
		Title:       ref.Page.Title,
		Description: ref.Page.Description,
		Crumb:       ref.Section.Title + " / " + ref.Page.Title,
		VerBar:      template.HTML(buildVerBar(ver, lang, section, page)), //nolint:gosec // generated markup
		Sidebar:     template.HTML(buildSidebar(b, base, section, page)),  //nolint:gosec // generated markup
		Content:     template.HTML(frag),                                  //nolint:gosec // generated from our own docs
	}
	if pos > 0 {
		p := b.flat[pos-1]
		view.PrevHref = base + p.Section.Slug + "/" + p.Page.Slug
		view.PrevTitle = p.Page.Title
	}
	if pos < len(b.flat)-1 {
		n := b.flat[pos+1]
		view.NextHref = base + n.Section.Slug + "/" + n.Page.Slug
		view.NextTitle = n.Page.Title
	}

	var buf bytes.Buffer
	if err := docTemplate.Execute(&buf, view); err != nil {
		return err
	}
	c.Type("html", "utf-8")
	return c.Send(buf.Bytes())
}

// bestHref resolves the equivalent page in another (version, lang): the same
// section/page if it exists there, else that bundle's first page. Empty when the
// bundle does not exist.
func bestHref(ver, lang, section, page string) string {
	b := docBundles[bundleKey(ver, lang)]
	if b == nil {
		return ""
	}
	prefix := "/docs/" + ver + "/" + lang + "/"
	if _, ok := b.index[section+"/"+page]; ok {
		return prefix + section + "/" + page
	}
	return prefix + b.firstSec + "/" + b.firstPage
}

// buildVerBar renders the version + language selectors at the top of the sidebar.
func buildVerBar(ver, lang, section, page string) string {
	var b bytes.Buffer
	b.WriteString(`<div class="verbar">`)

	// Version selector — switching keeps the same page when it exists in the target.
	b.WriteString(`<select class="vsel" aria-label="Version" onchange="location.href=this.value">`)
	for _, v := range docVersions.Versions {
		href := bestHref(v, lang, section, page)
		if href == "" {
			continue
		}
		label := "v" + v
		if v == docVersions.Latest {
			label += " · latest"
		}
		sel := ""
		if v == ver {
			sel = " selected"
		}
		b.WriteString(`<option value="` + href + `"` + sel + `>` + template.HTMLEscapeString(label) + `</option>`)
	}
	b.WriteString(`</select>`)

	// Language selector — only the languages this version actually ships.
	b.WriteString(`<select class="lsel" aria-label="Language" onchange="location.href=this.value">`)
	for _, l := range docVersions.Langs {
		href := bestHref(ver, l, section, page)
		if href == "" {
			continue
		}
		sel := ""
		if l == lang {
			sel = " selected"
		}
		b.WriteString(`<option value="` + href + `"` + sel + `>` + template.HTMLEscapeString(strings.ToUpper(l)) + `</option>`)
	}
	b.WriteString(`</select>`)

	b.WriteString(`</div>`)
	return b.String()
}

// buildSidebar renders the section/page tree (with sub-group labels), highlighting
// the active page. base is "/docs/<version>/<lang>/".
func buildSidebar(b *docBundle, base, curSection, curPage string) string {
	var out bytes.Buffer
	for _, s := range b.sections {
		out.WriteString(`<div class="sec"><p class="sec-t">`)
		out.WriteString(template.HTMLEscapeString(s.Title))
		out.WriteString(`</p>`)
		curGroup := ""
		for _, p := range s.Pages {
			if p.Group != curGroup {
				curGroup = p.Group
				if curGroup != "" {
					out.WriteString(`<p class="sec-g">` + template.HTMLEscapeString(curGroup) + `</p>`)
				}
			}
			cls := "pg"
			if s.Slug == curSection && p.Slug == curPage {
				cls = "pg active"
			}
			out.WriteString(`<a class="` + cls + `" href="` + base + s.Slug + `/` + p.Slug + `">`)
			out.WriteString(template.HTMLEscapeString(p.Title))
			out.WriteString(`</a>`)
		}
		out.WriteString(`</div>`)
	}
	return out.String()
}

package main

// The sitemap with the landing page of each language: Zola's lists the
// site's pages, which the languages' pages, made after it, are not; each
// page of the landing page names the others as its alternates, as search
// engines read them.

import (
	"encoding/xml"
	"fmt"
	"strings"
)

type sitemapURL struct {
	Loc     string `xml:"loc"`
	Lastmod string `xml:"lastmod,omitempty"`
}

type urlset struct {
	URLs []sitemapURL `xml:"url"`
}

// sitemap returns Zola's sitemap src with the landing page in each
// language, at canonical, and the alternates of the landing pages.
func sitemap(src []byte, canonical string) ([]byte, error) {
	var set urlset
	if err := xml.Unmarshal(src, &set); err != nil {
		return nil, fmt.Errorf("sitemap.xml: %w", err)
	}
	canonical = strings.TrimRight(canonical, "/") + "/"
	home := map[string]bool{}
	for _, l := range languages {
		home[canonical+l.path()] = true
	}
	var b strings.Builder
	b.WriteString(xml.Header)
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9" xmlns:xhtml="http://www.w3.org/1999/xhtml">` + "\n")
	write := func(u sitemapURL) {
		b.WriteString("  <url>\n    <loc>")
		xml.EscapeText(&b, []byte(u.Loc))
		b.WriteString("</loc>\n")
		if u.Lastmod != "" {
			b.WriteString("    <lastmod>")
			xml.EscapeText(&b, []byte(u.Lastmod))
			b.WriteString("</lastmod>\n")
		}
		if home[u.Loc] {
			for _, l := range languages {
				fmt.Fprintf(&b, "    <xhtml:link rel=\"alternate\" hreflang=\"%s\" href=\"%s\" />\n", l.Tag, canonical+l.path())
			}
			fmt.Fprintf(&b, "    <xhtml:link rel=\"alternate\" hreflang=\"x-default\" href=\"%s\" />\n", canonical)
		}
		b.WriteString("  </url>\n")
	}
	listed := map[string]bool{}
	for _, u := range set.URLs {
		u.Loc = strings.TrimSpace(u.Loc)
		if listed[u.Loc] {
			continue
		}
		listed[u.Loc] = true
		write(u)
	}
	for _, l := range languages {
		if loc := canonical + l.path(); !listed[loc] {
			listed[loc] = true
			write(sitemapURL{Loc: loc})
		}
	}
	b.WriteString("</urlset>\n")
	return []byte(b.String()), nil
}

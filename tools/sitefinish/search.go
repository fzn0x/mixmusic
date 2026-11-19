package main

// The docs' search index, docs/search.json, which the docs' search field
// reads (static/docs-search.js): each docs page as built, cut into its
// sections at its headings, each with the heading's anchor as the page
// has it and the words under it. It is made from the built pages, so a
// result can't name a heading the page lacks or an anchor it spells
// another way. The pages come in the order of the docs' drawer.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// searchIndex is docs/search.json. Its keys are short: a visitor's
// browser downloads it once they search.
type searchIndex struct {
	Pages    []searchPage    `json:"pages"`
	Sections []searchSection `json:"sections"`
}

// searchPage is a docs page: its title, the group of the drawer it is in,
// and its address under /docs/, as "playing-music/".
type searchPage struct {
	Title string `json:"t"`
	Group string `json:"g,omitempty"`
	Path  string `json:"u"`
}

// searchSection is the words of a page under one heading, until the next:
// the page's index in Pages, the heading and its anchor, and the words.
// The words before the first heading have neither.
type searchSection struct {
	Page    int    `json:"p"`
	Heading string `json:"h,omitempty"`
	Anchor  string `json:"a,omitempty"`
	Text    string `json:"x"`
}

// writeSearch writes docs/search.json under public, from the docs Zola
// built there, and returns how many pages and sections it holds.
func writeSearch(public string) (pages, sections int, err error) {
	idx, err := searchDocs(filepath.Join(public, "docs"))
	if err != nil {
		return 0, 0, err
	}
	out, err := json.Marshal(idx)
	if err != nil {
		return 0, 0, err
	}
	if err := os.WriteFile(filepath.Join(public, "docs", "search.json"), out, 0o644); err != nil {
		return 0, 0, err
	}
	return len(idx.Pages), len(idx.Sections), nil
}

// searchDocs reads the docs under dir, site/public/docs: the drawer of its
// overview, index.html, lists the pages, and each page's index.html
// gives its sections.
func searchDocs(dir string) (searchIndex, error) {
	var idx searchIndex
	src, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		return idx, fmt.Errorf("the docs' overview: %w", err)
	}
	doc, err := parse(src)
	if err != nil {
		return idx, err
	}
	drawer := find(doc, func(n *html.Node) bool { return hasClass(n, "drawer") })
	if drawer == nil {
		return idx, fmt.Errorf("%s: no drawer of the docs", filepath.Join(dir, "index.html"))
	}
	group := ""
	seen := map[string]bool{}
	each(drawer, func(n *html.Node) {
		switch {
		case hasClass(n, "drawer-label"):
			group = norm(text(n))
		case hasClass(n, "drawer-item"):
			path, ok := docsPath(attr(n, "href"))
			if !ok || path == "" || seen[path] {
				return // the overview, or a page listed twice
			}
			seen[path] = true
			idx.Pages = append(idx.Pages, searchPage{Title: norm(text(n)), Group: group, Path: path})
		}
	})
	if len(idx.Pages) == 0 {
		return idx, fmt.Errorf("%s: the drawer lists no docs", filepath.Join(dir, "index.html"))
	}
	for i := range idx.Pages {
		p := &idx.Pages[i]
		src, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p.Path), "index.html"))
		if err != nil {
			return idx, fmt.Errorf("the docs' page %s: %w", p.Path, err)
		}
		doc, err := parse(src)
		if err != nil {
			return idx, err
		}
		article := find(doc, func(n *html.Node) bool { return n.DataAtom == atom.Article && hasClass(n, "prose") })
		if article == nil {
			return idx, fmt.Errorf("the docs' page %s: no article", p.Path)
		}
		if h1 := find(article, func(n *html.Node) bool { return n.DataAtom == atom.H1 }); h1 != nil {
			p.Title = norm(text(h1))
		}
		idx.Sections = append(idx.Sections, sections(article, i)...)
	}
	return idx, nil
}

// docsPath is the address under /docs/ of a link of the drawer, which
// Zola writes in full: "playing-music/" of
// https://example.test/mixmusic/docs/playing-music/, "" for the overview.
func docsPath(href string) (string, bool) {
	_, path, ok := strings.Cut(href, "/docs/")
	if !ok {
		return "", false
	}
	path, _, _ = strings.Cut(path, "#")
	return path, true
}

// sections cuts a page's article into its sections, at each heading below
// the title that has an anchor.
func sections(article *html.Node, page int) []searchSection {
	var out []searchSection
	cur := searchSection{Page: page}
	var words strings.Builder
	flush := func() {
		cur.Text = norm(words.String())
		if cur.Text != "" || cur.Heading != "" {
			out = append(out, cur)
		}
		words.Reset()
	}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		switch {
		case n.Type == html.TextNode:
			words.WriteString(n.Data)
			return
		case n.Type != html.ElementNode:
		case n.DataAtom == atom.H1:
			return // the page's title, which Pages holds
		case n.DataAtom == atom.H2 || n.DataAtom == atom.H3 || n.DataAtom == atom.H4:
			if id := attr(n, "id"); id != "" {
				flush()
				cur = searchSection{Page: page, Heading: norm(text(n)), Anchor: id}
				return
			}
		case n.DataAtom == atom.Script || n.DataAtom == atom.Style || n.DataAtom == atom.Svg || n.DataAtom == atom.Img:
			return
		}
		// Blocks, and a line's break, part their words from the next.
		block := n.Type == html.ElementNode && (!phrasing[n.DataAtom] || n.DataAtom == atom.Br)
		if block {
			words.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if block {
			words.WriteByte(' ')
		}
	}
	for c := article.FirstChild; c != nil; c = c.NextSibling {
		walk(c)
	}
	flush()
	return out
}

// text is the words of n and its children: a heading's are in the link
// to itself that Zola puts around them (templates/anchor-link.html).
func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			return
		}
		if n.Type == html.ElementNode && n.DataAtom == atom.Svg {
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// hasClass reports whether the element n has class c.
func hasClass(n *html.Node, c string) bool {
	if n.Type != html.ElementNode {
		return false
	}
	for _, f := range strings.Fields(attr(n, "class")) {
		if f == c {
			return true
		}
	}
	return false
}

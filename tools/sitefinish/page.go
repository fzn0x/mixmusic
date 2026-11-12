package main

// Each language's landing page, made from the English one Zola built: its
// words translated, its lang (and dir, read right to left), its relative
// addresses a folder up, its canonical address and Open Graph locale, the
// alternates search engines read (hreflang), the language menus filled,
// and its structured data in its language.

import (
	"bytes"
	"encoding/json"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// parse parses a page, with the elements of its noscript as elements, so
// that their words translate too.
func parse(src []byte) (*html.Node, error) {
	return html.ParseWithOptions(bytes.NewReader(src), html.ParseOptionEnableScripting(false))
}

func render(doc *html.Node) ([]byte, error) {
	var b bytes.Buffer
	if err := html.Render(&b, doc); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// find returns the first element under n for which ok holds.
func find(n *html.Node, ok func(*html.Node) bool) *html.Node {
	if n.Type == html.ElementNode && ok(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if f := find(c, ok); f != nil {
			return f
		}
	}
	return nil
}

// each calls f with each element under n.
func each(n *html.Node, f func(*html.Node)) {
	if n.Type == html.ElementNode {
		f(n)
	}
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		each(c, f)
		c = next
	}
}

func element(a atom.Atom, attrs ...string) *html.Node {
	n := &html.Node{Type: html.ElementNode, DataAtom: a, Data: a.String()}
	for i := 0; i+1 < len(attrs); i += 2 {
		n.Attr = append(n.Attr, html.Attribute{Key: attrs[i], Val: attrs[i+1]})
	}
	return n
}

// relative reports whether the address u is relative to the page: not a
// fragment, a path from the root, another site's, or data.
func relative(u string) bool {
	u = strings.TrimSpace(u)
	return u != "" && !strings.HasPrefix(u, "#") && !strings.HasPrefix(u, "/") &&
		!strings.Contains(u, ":")
}

// up moves the relative addresses of n a folder up, as the pages of the
// languages are a folder down: "assets/icon.png" is "../assets/icon.png".
func up(n *html.Node) {
	for i, a := range n.Attr {
		switch a.Key {
		case "src", "href", "poster":
			if relative(a.Val) {
				n.Attr[i].Val = "../" + strings.TrimSpace(a.Val)
			}
		case "srcset":
			parts := strings.Split(a.Val, ",")
			for j, p := range parts {
				p = strings.TrimSpace(p)
				if relative(p) {
					parts[j] = "../" + p
				} else {
					parts[j] = p
				}
			}
			n.Attr[i].Val = strings.Join(parts, ", ")
		}
	}
}

// link is the address of the page of to from the page of from.
func link(from, to language) string {
	if from.Dir == "" {
		if to.Dir == "" {
			return "./"
		}
		return to.path()
	}
	return "../" + to.path()
}

// finish makes doc the page of l, at canonical (the site's address for
// search engines) + l.path(): see the file's comment. A page finished once
// finishes the same again.
func finish(doc *html.Node, l language, canonical string) {
	canonical = strings.TrimRight(canonical, "/") + "/"
	url := canonical + l.path()
	root := find(doc, func(n *html.Node) bool { return n.DataAtom == atom.Html })
	head := find(doc, func(n *html.Node) bool { return n.DataAtom == atom.Head })
	if root == nil || head == nil {
		return
	}
	setAttr(root, "lang", l.Tag)
	if l.RTL {
		setAttr(root, "dir", "rtl")
	}
	if l.Dir != "" {
		setAttr(root, "data-root", "../")
		each(doc, up)
	}
	// What a page finished before has: its alternates and locales.
	each(head, func(n *html.Node) {
		if (n.DataAtom == atom.Link && attr(n, "hreflang") != "") ||
			(n.DataAtom == atom.Meta && attr(n, "property") == "og:locale:alternate") {
			n.Parent.RemoveChild(n)
		}
	})
	var canon *html.Node
	each(head, func(n *html.Node) {
		switch {
		case n.DataAtom == atom.Link && attr(n, "rel") == "canonical":
			setAttr(n, "href", url)
			canon = n
		case n.DataAtom == atom.Meta && attr(n, "property") == "og:url":
			setAttr(n, "content", url)
		case n.DataAtom == atom.Meta && attr(n, "property") == "og:locale":
			setAttr(n, "content", l.Locale)
			for i := len(languages) - 1; i >= 0; i-- {
				if o := languages[i]; o.Tag != l.Tag {
					n.Parent.InsertBefore(element(atom.Meta, "property", "og:locale:alternate", "content", o.Locale), n.NextSibling)
				}
			}
		}
	})
	// The alternates: each language's page, and English's as the one for
	// any other language.
	var alts []*html.Node
	for _, o := range languages {
		alts = append(alts, element(atom.Link, "rel", "alternate", "hreflang", o.Tag, "href", canonical+o.path()))
	}
	alts = append(alts, element(atom.Link, "rel", "alternate", "hreflang", "x-default", "href", canonical))
	for i := len(alts) - 1; i >= 0; i-- {
		if canon != nil {
			head.InsertBefore(alts[i], canon.NextSibling)
		} else {
			head.AppendChild(alts[i])
		}
	}
	// The language menus, and the name of the one shown.
	each(doc, func(n *html.Node) {
		if _, ok := attrOK(n, "data-lang-name"); ok {
			replaceText(n, l.Name)
		}
		if _, ok := attrOK(n, "data-languages"); !ok {
			return
		}
		for c := n.FirstChild; c != nil; {
			next := c.NextSibling
			n.RemoveChild(c)
			c = next
		}
		for _, o := range languages {
			a := element(atom.A, "href", link(l, o), "hreflang", o.Tag, "lang", o.Tag)
			if o.RTL {
				setAttr(a, "dir", "rtl")
			}
			if o.Tag == l.Tag {
				setAttr(a, "aria-current", "page")
			}
			a.AppendChild(&html.Node{Type: html.TextNode, Data: o.Name})
			li := element(atom.Li)
			li.AppendChild(a)
			n.AppendChild(li)
		}
	})
	// The structured data: the page's language, and its address.
	each(head, func(n *html.Node) {
		if n.DataAtom != atom.Script || attr(n, "type") != "application/ld+json" || n.FirstChild == nil {
			return
		}
		var data any
		if json.Unmarshal([]byte(n.FirstChild.Data), &data) != nil {
			return
		}
		var visit func(x any)
		visit = func(x any) {
			switch x := x.(type) {
			case map[string]any:
				if _, ok := x["inLanguage"]; ok {
					x["inLanguage"] = l.Tag
					if t, _ := x["@type"].(string); t == "WebSite" || t == "FAQPage" {
						x["url"] = url
					}
				}
				for _, v := range x {
					visit(v)
				}
			case []any:
				for _, v := range x {
					visit(v)
				}
			}
		}
		visit(data)
		var b bytes.Buffer
		enc := json.NewEncoder(&b)
		enc.SetEscapeHTML(true)
		if enc.Encode(data) == nil {
			n.FirstChild.Data = strings.TrimSpace(b.String())
		}
	})
}

func attrOK(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

// replaceText makes text the only child of n.
func replaceText(n *html.Node, text string) {
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		n.RemoveChild(c)
		c = next
	}
	n.AppendChild(&html.Node{Type: html.TextNode, Data: text})
}

// page makes the page of l from the English page src, with table, the
// translations of its units, at canonical: the page, and the English of
// the units it has no translation for.
func page(src []byte, l language, table map[string]string, canonical string) ([]byte, []string, error) {
	doc, err := parse(src)
	if err != nil {
		return nil, nil, err
	}
	var missing []string
	if l.Dir != "" {
		missing = translate(doc, table)
	}
	finish(doc, l, canonical)
	out, err := render(doc)
	return out, missing, err
}

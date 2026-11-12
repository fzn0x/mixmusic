package main

// The pieces of the landing page that a translation replaces whole, keyed
// by their English: the inner HTML of an element of words (a heading, a
// paragraph, a list's item), with the inline elements in it, which a
// translation keeps and may move; the attributes a visitor reads or hears
// (alt, title, aria-label…); the meta tags of search engines and link
// previews; the strings of the structured data; and the words of the
// page's scripts, #site-text.

import (
	"bytes"
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// unit is a piece of the page a translation replaces whole: its English,
// what it is, and where it is, for the translator. Kind "html" is the
// inner HTML of an element, whose tags and entities a translation keeps;
// "text" is words alone: a text, an attribute, a meta's content, a
// string of JSON.
type unit struct {
	Text  string `json:"text"`
	Kind  string `json:"kind"`
	Where string `json:"where"`
}

// wordAttrs are the attributes whose words the page shows or reads out:
// data-alt, data-desc and data-copy the page's scripts show.
var wordAttrs = map[string]bool{
	"alt": true, "title": true, "aria-label": true, "placeholder": true,
	"data-alt": true, "data-desc": true, "data-copy": true,
}

// wordMetas are the meta tags whose content is words, by their name or
// property.
var wordMetas = map[string]bool{
	"description": true, "og:title": true, "og:description": true, "og:image:alt": true,
	"twitter:title": true, "twitter:description": true, "twitter:image:alt": true,
}

// phrasing are the elements that sit within a line of words, which a
// translation keeps as they are around the words it moves.
var phrasing = map[atom.Atom]bool{
	atom.A: true, atom.Abbr: true, atom.B: true, atom.Bdi: true, atom.Bdo: true,
	atom.Br: true, atom.Cite: true, atom.Code: true, atom.Data: true, atom.Dfn: true,
	atom.Em: true, atom.I: true, atom.Img: true, atom.Kbd: true, atom.Mark: true,
	atom.Q: true, atom.S: true, atom.Samp: true, atom.Small: true, atom.Span: true,
	atom.Strong: true, atom.Sub: true, atom.Sup: true, atom.Time: true, atom.U: true,
	atom.Var: true, atom.Wbr: true,
}

// space is a run of white space, which a unit's key reads as one space.
var space = regexp.MustCompile(`\s+`)

// norm is the key of words: their white space as single spaces, trimmed.
func norm(s string) string { return strings.TrimSpace(space.ReplaceAllString(s, " ")) }

// hasLetters reports whether s holds a letter: "·" or "0:00" is no unit.
func hasLetters(s string) bool { return strings.IndexFunc(s, unicode.IsLetter) >= 0 }

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func setAttr(n *html.Node, key, val string) {
	for i, a := range n.Attr {
		if a.Key == key {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: val})
}

// hidden reports whether n is no part of the page's words: decorative
// (aria-hidden), or kept as it is (translate="no"), as the working copy
// of MixMusic's player is, whose labels are the app's, in English.
func hidden(n *html.Node) bool {
	return attr(n, "aria-hidden") == "true" || attr(n, "translate") == "no"
}

// skipped reports whether the words of n, and those within it, are no
// unit: code, scripts and styles, and SVG's drawings.
func skipped(n *html.Node) bool {
	if n.Namespace != "" {
		return true
	}
	switch n.DataAtom {
	case atom.Script, atom.Style, atom.Template, atom.Pre, atom.Textarea, atom.Kbd:
		// A key's name, as Ctrl, stays as it is; within words, it moves
		// with them.
		return true
	}
	return false
}

// words reports whether n holds words a visitor reads: text with a letter,
// outside the parts that aren't translated.
func words(n *html.Node) bool {
	if n.Type == html.TextNode {
		return hasLetters(n.Data)
	}
	if n.Type != html.ElementNode || skipped(n) || hidden(n) {
		return false
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if words(c) {
			return true
		}
	}
	return false
}

// inline reports whether every element within n sits within a line of
// words: SVG's drawings count as one, whatever is in them.
func inline(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || c.Namespace != "" {
			continue
		}
		if !phrasing[c.DataAtom] || !inline(c) {
			return false
		}
	}
	return true
}

// isUnit reports whether n is translated whole: its words are on one line,
// with inline elements alone, and some are its own. Without words of its
// own, as a nav of links or a link of a title and a line, each element
// within it is a unit of its own.
func isUnit(n *html.Node) bool {
	if !words(n) || !inline(n) {
		return false
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode && hasLetters(c.Data) {
			return true
		}
	}
	return false
}

// loneText returns the one text of n's own that has words, when every
// element within n has none, as an icon beside a button's label: that text
// is the unit then, and the icon stays where it is.
func loneText(n *html.Node) *html.Node {
	var lone *html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch {
		case c.Type == html.TextNode && hasLetters(c.Data):
			if lone != nil {
				return nil
			}
			lone = c
		case words(c):
			return nil
		}
	}
	return lone
}

// inner is the inner HTML of n, as its key reads it: Render writes an
// apostrophe as &#39;, which reads as one in text and in the double-quoted
// attributes it writes.
func inner(n *html.Node) string {
	var b bytes.Buffer
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		html.Render(&b, c)
	}
	return norm(strings.ReplaceAll(b.String(), "&#39;", "'"))
}

// where tells where n is, for a translator: its tag, and the nearest id
// around it.
func where(n *html.Node) string {
	w := n.Data
	for p := n; p != nil; p = p.Parent {
		if id := attr(p, "id"); id != "" && p.Type == html.ElementNode {
			if p == n {
				return w + "#" + id
			}
			return "#" + id + " " + w
		}
	}
	return w
}

// visitor walks the page's units, giving each to its functions: unit to
// an element translated whole, text to a text node, value to an
// attribute's or a meta's value. Each returns the replacement, or "".
type visitor struct {
	unit  func(n *html.Node, key string) string
	text  func(n *html.Node, key string) string
	value func(n *html.Node, key, where string) string
	// json is given each string of the page's JSON (structured data and
	// #site-text) that is words, and returns its replacement, or "".
	json func(key, where string) string
}

// walk visits the units of n and within it.
func (v visitor) walk(n *html.Node) {
	switch n.Type {
	case html.DocumentNode:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			v.walk(c)
		}
		return
	case html.ElementNode:
	default:
		return
	}
	if n.DataAtom == atom.Script {
		v.script(n)
		return
	}
	if attr(n, "aria-hidden") == "true" || skipped(n) {
		return
	}
	v.attrs(n)
	if attr(n, "translate") == "no" {
		return
	}
	if n.DataAtom == atom.Title {
		if c := n.FirstChild; c != nil && c.Type == html.TextNode {
			if r := v.value(n, norm(c.Data), "title"); r != "" {
				c.Data = r
			}
		}
		return
	}
	if isUnit(n) {
		if lone := loneText(n); lone != nil {
			v.textNode(lone)
			return
		}
		if r := v.unit(n, inner(n)); r != "" {
			replaceInner(n, r)
		}
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			v.textNode(c)
			continue
		}
		v.walk(c)
	}
}

// textNode visits a text with words, keeping the white space around it.
func (v visitor) textNode(c *html.Node) {
	key := norm(c.Data)
	if !hasLetters(key) {
		return
	}
	if r := v.text(c, key); r != "" {
		lead := c.Data[:len(c.Data)-len(strings.TrimLeftFunc(c.Data, unicode.IsSpace))]
		trail := c.Data[len(strings.TrimRightFunc(c.Data, unicode.IsSpace)):]
		c.Data = lead + r + trail
	}
}

// attrs visits the attributes of n that are words, and a meta's content.
func (v visitor) attrs(n *html.Node) {
	for i, a := range n.Attr {
		if !wordAttrs[a.Key] || !hasLetters(a.Val) {
			continue
		}
		if r := v.value(n, norm(a.Val), where(n)+" @"+a.Key); r != "" {
			n.Attr[i].Val = r
		}
	}
	if n.DataAtom != atom.Meta {
		return
	}
	name := attr(n, "name")
	if name == "" {
		name = attr(n, "property")
	}
	if wordMetas[name] {
		if r := v.value(n, norm(attr(n, "content")), "meta "+name); r != "" {
			setAttr(n, "content", r)
		}
	}
}

// script visits the words of a script that holds data: the structured
// data's strings, and the words of the page's scripts, #site-text.
func (v visitor) script(n *html.Node) {
	typ := attr(n, "type")
	if (typ != "application/ld+json" && typ != "application/json") || n.FirstChild == nil {
		return
	}
	var data any
	if err := json.Unmarshal([]byte(n.FirstChild.Data), &data); err != nil {
		return
	}
	scripts := attr(n, "id") == "site-text"
	changed := false
	var visit func(x any, typ, key string) any
	visit = func(x any, typ, key string) any {
		switch x := x.(type) {
		case map[string]any:
			t, _ := x["@type"].(string)
			// In the keys' order, so that the pieces keep theirs.
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			for _, k := range keys {
				x[k] = visit(x[k], t, k)
			}
			return x
		case []any:
			for i, val := range x {
				x[i] = visit(val, typ, key)
			}
			return x
		case string:
			isWords := scripts || key == "description" || key == "text" || (key == "name" && typ == "Question")
			if !isWords || !hasLetters(x) {
				return x
			}
			w := "structured data " + typ + "." + key
			if scripts {
				w = "#site-text " + key
			}
			if r := v.json(norm(x), w); r != "" {
				changed = true
				return r
			}
		}
		return x
	}
	data = visit(data, "", "")
	if !changed {
		return
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(data); err == nil {
		n.FirstChild.Data = strings.TrimSpace(b.String())
	}
}

// replaceInner makes the inner HTML of n the fragment s.
func replaceInner(n *html.Node, s string) {
	nodes, err := html.ParseFragment(strings.NewReader(s), n)
	if err != nil {
		return
	}
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		n.RemoveChild(c)
		c = next
	}
	for _, c := range nodes {
		n.AppendChild(c)
	}
}

// extract returns the units of the page doc, each once, in the page's
// order.
func extract(doc *html.Node) []unit {
	var out []unit
	seen := map[string]bool{}
	add := func(key, kind, w string) string {
		if !seen[key] {
			seen[key] = true
			out = append(out, unit{Text: key, Kind: kind, Where: w})
		}
		return ""
	}
	visitor{
		unit:  func(n *html.Node, key string) string { return add(key, "html", where(n)) },
		text:  func(n *html.Node, key string) string { return add(key, "text", where(n.Parent)) },
		value: func(n *html.Node, key, w string) string { return add(key, "text", w) },
		json:  func(key, w string) string { return add(key, "text", w) },
	}.walk(doc)
	return out
}

// heading reports whether n is a heading, whose English a translation
// keeps in data-en: scripts find a card by its heading (tiles.js).
func heading(n *html.Node) bool {
	switch n.DataAtom {
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		return true
	}
	return false
}

// translate translates the page doc with table, the translations keyed by
// their English, and returns the English of the units it has none for,
// each once.
func translate(doc *html.Node, table map[string]string) (missing []string) {
	seen := map[string]bool{}
	look := func(key string) string {
		if r, ok := table[key]; ok && strings.TrimSpace(r) != "" {
			return r
		}
		if !seen[key] {
			seen[key] = true
			missing = append(missing, key)
		}
		return ""
	}
	keep := func(n *html.Node, key, r string) string {
		if r != "" && n != nil && heading(n) {
			setAttr(n, "data-en", key)
		}
		return r
	}
	visitor{
		unit:  func(n *html.Node, key string) string { return keep(n, key, look(key)) },
		text:  func(n *html.Node, key string) string { return keep(n.Parent, key, look(key)) },
		value: func(_ *html.Node, key, _ string) string { return look(key) },
		json:  func(key, _ string) string { return look(key) },
	}.walk(doc)
	return missing
}

// markPattern finds a piece's tags and placeholders ({os}), which its
// translations keep.
var markPattern = regexp.MustCompile(`</?[a-zA-Z][^>]*>|\{[a-z_]+\}`)

// Command sitedocs makes the website's docs pages from docs/*.md, which stay
// their only source: the README and GitHub link to them, and the site is
// built from them. Each docs/<name>.md becomes a page of Zola's,
// site/content/docs/<name>/index.md, shown at /docs/<name>/, with the
// doc's title, the first sentences of its first paragraph as its
// description, and its place in the docs' order. The pages are not
// committed: build the site after running this.
//
// Links are rewritten for the site. A link to another doc goes to its
// page, an image of the repository is copied beside the page that shows
// it, and a link to any other file or folder of the repository goes to it
// on GitHub. A link to something the repository lacks fails, naming the
// doc and the line.
//
//	go run ./tools/sitedocs && zola --root site build
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// order is the docs' order on the site: what users set up, what they make,
// then how MixMusic is made. Docs it does not list follow, by name.
var order = []string{"spotify", "themes", "plugins", "troubleshooting", "architecture"}

// marker starts the second line of every page the command writes: a page
// with it is the command's to replace, and any other is left alone.
const marker = "# Made by tools/sitedocs from "

// options tell the command where things are.
type options struct {
	root   string // the repository's root
	out    string // the docs section of the site's content, under root: a folder of content/
	repo   string // the repository on GitHub, as https://github.com/owner/name
	branch string // the branch GitHub links point at
}

// section is the docs section's path in the site's content, which Zola's
// @/ links start from.
func (o options) section() string { return filepath.Base(filepath.Clean(o.out)) }

// doc is one of docs/*.md, on its way to being a page.
type doc struct {
	file        string // as the repository has it: docs/plugins.md
	slug        string // the page's folder in the docs section: plugins
	title       string
	description string
	weight      int
	body        string            // the Markdown after the title, its links rewritten
	offset      int               // the lines of the file before the body
	assets      map[string]string // files to copy beside the page: their path there, to the repository's
}

func main() {
	var o options
	flag.StringVar(&o.root, "root", ".", "the repository's root")
	flag.StringVar(&o.out, "out", filepath.Join("site", "content", "docs"), "the folder of the site's docs section, under the root")
	flag.StringVar(&o.repo, "repo", "https://github.com/fzn0x/mixmusic", "the repository on GitHub")
	flag.StringVar(&o.branch, "branch", "main", "the branch links to GitHub point at")
	flag.Parse()
	docs, err := run(o)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("made %d docs pages in %s\n", len(docs), filepath.ToSlash(o.out))
}

// run makes the pages of every doc, and returns the docs.
func run(o options) ([]*doc, error) {
	docs, err := read(o.root)
	if err != nil {
		return nil, err
	}
	bySlug := map[string]*doc{}
	for _, d := range docs {
		bySlug[d.slug] = d
	}
	var errs []error
	for _, d := range docs {
		errs = append(errs, d.rewrite(o, bySlug))
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	out := filepath.Join(o.root, o.out)
	if err := clean(out); err != nil {
		return nil, err
	}
	for _, d := range docs {
		if err := d.write(o.root, out); err != nil {
			return nil, err
		}
	}
	return docs, nil
}

// read reads docs/*.md, in the docs' order.
func read(root string) ([]*doc, error) {
	files, err := filepath.Glob(filepath.Join(root, "docs", "*.md"))
	if err != nil {
		return nil, err
	}
	var docs []*doc
	var errs []error
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		name := filepath.Base(f)
		d := &doc{file: "docs/" + name, slug: strings.ToLower(strings.TrimSuffix(name, ".md"))}
		if err := d.parse(string(data)); err != nil {
			errs = append(errs, err)
			continue
		}
		docs = append(docs, d)
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("no docs in %s", filepath.Join(root, "docs"))
	}
	rank := func(d *doc) int {
		if i := slices.Index(order, d.slug); i >= 0 {
			return i
		}
		return len(order)
	}
	slices.SortStableFunc(docs, func(a, b *doc) int {
		if ra, rb := rank(a), rank(b); ra != rb {
			return ra - rb
		}
		return strings.Compare(a.slug, b.slug)
	})
	for i, d := range docs {
		d.weight = (i + 1) * 10
	}
	return docs, nil
}

var heading1 = regexp.MustCompile(`^#[ \t]+(.+?)(?:[ \t]+#+)?[ \t]*$`)

// parse takes the doc's title from the heading of its first line, as the
// page shows the title itself, and its description from its first
// paragraph.
func (d *doc) parse(text string) error {
	text = strings.TrimPrefix(text, string(rune(0xFEFF))) // a byte order mark
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	blank := func(n int) int {
		for n < len(lines) && strings.TrimSpace(lines[n]) == "" {
			n++
		}
		return n
	}
	n := blank(0)
	var m []string
	if n < len(lines) {
		m = heading1.FindStringSubmatch(lines[n])
	}
	if m == nil {
		return fmt.Errorf("%s:%d: a doc starts with its title, as # Title", d.file, n+1)
	}
	d.title = plain(m[1])
	d.offset = blank(n + 1)
	d.body = strings.Join(lines[d.offset:], "\n")
	d.description = summary(d.body)
	return nil
}

// summary is the first sentences of the first paragraph of prose, up to
// some sixty characters or more, as plain text.
func summary(body string) string {
	for _, par := range strings.Split(body, "\n\n") {
		par = strings.TrimSpace(par)
		if par == "" || strings.ContainsAny(par[:1], "#`~|<>-*+=") || startsList(par) {
			continue
		}
		text := plain(par)
		if text == "" {
			continue
		}
		var words []string
		for _, w := range strings.Fields(text) {
			words = append(words, w)
			if n := len(strings.Join(words, " ")); n >= 60 && endsSentence(w) {
				break
			}
		}
		s := strings.Join(words, " ")
		if strings.HasSuffix(s, ":") {
			s = strings.TrimSuffix(s, ":") + "."
		}
		return s
	}
	return ""
}

// startsList tells whether a paragraph is an ordered list: 1. or 1).
func startsList(par string) bool {
	digits := strings.TrimLeftFunc(par, unicode.IsDigit)
	return len(digits) < len(par) && (strings.HasPrefix(digits, ". ") || strings.HasPrefix(digits, ") "))
}

func endsSentence(word string) bool {
	return strings.HasSuffix(word, ".") || strings.HasSuffix(word, "!") || strings.HasSuffix(word, "?")
}

var (
	plainImage = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	plainLink  = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	plainTag   = regexp.MustCompile(`<[^>]+>`)
)

// plain strips Markdown's marks from a line of inline text.
func plain(s string) string {
	s = plainImage.ReplaceAllString(s, "")
	s = plainLink.ReplaceAllString(s, "$1")
	s = plainTag.ReplaceAllString(s, "")
	s = strings.NewReplacer("`", "", "**", "", "__", "", `\`, "").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

var (
	// inlineLink is a link or an image: ![text](target "title").
	inlineLink = regexp.MustCompile(`(!?)\[(?:[^\[\]]|\[[^\[\]]*\])*\]\([ \t]*(<[^<>\n]*>|[^\s()<>]+)(?:[ \t]+(?:"[^"]*"|'[^']*'|\([^()]*\)))?[ \t]*\)`)
	// refDef defines a link by reference: [name]: target "title".
	refDef = regexp.MustCompile(`^ {0,3}\[[^\]]+\]:[ \t]*(<[^<>]*>|\S+)`)
	// htmlAttr is an attribute of HTML that holds an address.
	htmlAttr = regexp.MustCompile(`(?i)\s(src|href|poster)[ \t]*=[ \t]*("[^"]*"|'[^']*')`)
	scheme   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
)

// edit replaces a target in a line: line[start:end] becomes text.
type edit struct {
	start, end int
	text       string
}

// rewrite rewrites the links of the doc's Markdown for the site, outside
// code.
func (d *doc) rewrite(o options, docs map[string]*doc) error {
	d.assets = map[string]string{}
	lines := strings.Split(d.body, "\n")
	var errs []error
	fence := ""
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, fence[:1]+" \t") == "" {
				fence = ""
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fence = trimmed[:len(trimmed)-len(strings.TrimLeft(trimmed, trimmed[:1]))]
			continue
		}
		masked := maskCode(line)
		var edits []edit
		target := func(start, end int, image, html bool) {
			t := line[start:end]
			angled := strings.HasPrefix(t, "<")
			t = strings.Trim(t, "<>")
			to, err := d.resolve(t, image, html, o, docs)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s:%d: %s: %w", d.file, d.offset+i+1, t, err))
				return
			}
			if angled {
				to = "<" + to + ">"
			}
			edits = append(edits, edit{start, end, to})
		}
		if m := refDef.FindStringSubmatchIndex(masked); m != nil {
			target(m[2], m[3], false, false)
		} else {
			for _, m := range inlineLink.FindAllStringSubmatchIndex(masked, -1) {
				target(m[4], m[5], m[3] > m[2], false)
			}
		}
		for _, m := range htmlAttr.FindAllStringSubmatchIndex(masked, -1) {
			attr := strings.ToLower(masked[m[2]:m[3]])
			target(m[4]+1, m[5]-1, attr != "href", true)
		}
		slices.SortFunc(edits, func(a, b edit) int { return a.start - b.start })
		var b strings.Builder
		at := 0
		for _, e := range edits {
			if e.start < at {
				continue
			}
			b.WriteString(line[at:e.start])
			b.WriteString(e.text)
			at = e.end
		}
		b.WriteString(line[at:])
		lines[i] = b.String()
	}
	d.body = strings.Join(lines, "\n")
	return errors.Join(errs...)
}

// maskCode hides the code spans of a line, keeping its length, so that what
// looks like a link in code is left as it is.
func maskCode(line string) string {
	b := []byte(line)
	for i := 0; i < len(b); {
		if b[i] != '`' {
			i++
			continue
		}
		n := 0
		for i+n < len(b) && b[i+n] == '`' {
			n++
		}
		end := -1
		for j := i + n; j < len(b); {
			if b[j] != '`' {
				j++
				continue
			}
			m := 0
			for j+m < len(b) && b[j+m] == '`' {
				m++
			}
			if m == n {
				end = j + m
				break
			}
			j += m
		}
		if end < 0 {
			i += n
			continue
		}
		for k := i; k < end; k++ {
			b[k] = 0
		}
		i = end
	}
	return string(b)
}

// resolve rewrites one link's target for the site. Images, and links to
// files of docs/ other than docs, are copied beside the page. Markdown's
// links go through Zola's @/ paths, which it checks; Zola leaves HTML's
// as they are, so those are relative to the page, at /docs/<slug>/.
func (d *doc) resolve(target string, image, html bool, o options, docs map[string]*doc) (string, error) {
	if target == "" || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "//") || scheme.MatchString(target) {
		return target, nil
	}
	p, suffix := target, ""
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p, suffix = p[:i], p[i:]
	}
	p, err := url.PathUnescape(p)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(p, "/") {
		p = path.Clean(strings.TrimPrefix(p, "/"))
	} else {
		p = path.Join(path.Dir(d.file), p)
	}
	if p == ".." || strings.HasPrefix(p, "../") {
		return "", errors.New("outside the repository")
	}
	info, err := os.Stat(filepath.Join(o.root, filepath.FromSlash(p)))
	if err != nil {
		return "", errors.New("not in the repository")
	}
	inDocs := path.Dir(p) == "docs" || strings.HasPrefix(p, "docs/")
	switch {
	case info.IsDir():
		return github(o, "tree", p) + suffix, nil
	case path.Dir(p) == "docs" && path.Ext(p) == ".md":
		slug := strings.ToLower(strings.TrimSuffix(path.Base(p), ".md"))
		if _, ok := docs[slug]; ok {
			if html {
				return "../" + escape(slug) + "/" + suffix, nil
			}
			return "@/" + path.Join(o.section(), slug, "index.md") + suffix, nil
		}
	case image || inDocs && path.Ext(p) != ".md":
		local := strings.TrimPrefix(p, "docs/")
		if !inDocs {
			local = path.Join("repo", p)
		}
		d.assets[local] = p
		if html {
			return escape(local) + suffix, nil
		}
		return "@/" + path.Join(o.section(), d.slug, escape(local)) + suffix, nil
	}
	return github(o, "blob", p) + suffix, nil
}

// github is the address of a file (blob) or a folder (tree) on GitHub.
func github(o options, kind, p string) string {
	return strings.TrimSuffix(o.repo, "/") + "/" + kind + "/" + o.branch + "/" + escape(p)
}

// escape escapes each part of a slash-separated path.
func escape(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}

// write writes the doc's page, and copies its assets beside it.
func (d *doc) write(root, out string) error {
	dir := filepath.Join(out, d.slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(d.page()), 0o644); err != nil {
		return err
	}
	for local, src := range d.assets {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(src)))
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, filepath.FromSlash(local))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// page is the page's Markdown, with its front matter.
func (d *doc) page() string {
	var b strings.Builder
	b.WriteString("+++\n")
	b.WriteString(marker + d.file + ": edit that file, not this one.\n")
	fmt.Fprintf(&b, "title = %s\n", tomlString(d.title))
	if d.description != "" {
		fmt.Fprintf(&b, "description = %s\n", tomlString(d.description))
	}
	fmt.Fprintf(&b, "weight = %d\n\n[extra]\nsource = %s\n+++\n\n", d.weight, tomlString(d.file))
	b.WriteString(strings.TrimRight(d.body, "\n"))
	b.WriteString("\n")
	return b.String()
}

// tomlString quotes a string as TOML's basic strings are.
func tomlString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// clean deletes the pages made before, so that a doc deleted or renamed
// leaves no page behind, and keeps everything else of the section.
func clean(out string) error {
	entries, err := os.ReadDir(out)
	if errors.Is(err, fs.ErrNotExist) {
		return os.MkdirAll(out, 0o755)
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(out, e.Name())
		data, err := os.ReadFile(filepath.Join(dir, "index.md"))
		if err != nil {
			continue
		}
		_, rest, _ := strings.Cut(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
		if strings.HasPrefix(rest, marker) {
			if err := os.RemoveAll(dir); err != nil {
				return err
			}
		}
	}
	return nil
}

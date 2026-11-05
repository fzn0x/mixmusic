// Command sitedocs makes the website's docs pages from docs/*.md, which stay
// their only source: the README and GitHub link to them, and the site is
// built from them. Each docs/<name>.md becomes a page of Zola's,
// site/content/docs/<name>/index.md, shown at /docs/<name>/, with the
// doc's title, the first sentences of its first paragraph as its
// description, and its place in the docs. A folder of docs with a
// README.md, docs/<name>/, becomes one page too, /docs/<name>/: its README,
// then each of its other files under a heading of its own, in the order
// the README links them, and those it doesn't by name. A Markdown file at
// the repository's root that names its place, as CONTRIBUTING.md does,
// becomes a page too, /docs/contributing/, and an underscore of its name a
// dash: CODE_OF_CONDUCT.md is /docs/code-of-conduct/. The pages are not
// committed: build the site after running this.
//
// Each doc names its place on the line after its title, in a comment that
// GitHub doesn't show: its group, and its order in the group, smaller
// first. The groups are those of groups, below.
//
//	# Lyrics
//	<!-- docs: group=using order=30 -->
//
// Links are rewritten for the site. A link to another doc goes to its
// page, or to its heading on its folder's page, an image of the repository
// is copied beside the page that shows it, and a link to any other file or
// folder of the repository goes to it on GitHub. A link to something the
// repository lacks fails, naming the doc and the line. The page's front
// matter gives the size of each image beside it, so the site keeps its
// room before it loads.
//
//	go run ./tools/sitedocs && zola --root site build
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/gif"  // the sizes of the images beside the pages
	_ "image/jpeg" // ...
	_ "image/png"  // ...
	"io/fs"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	_ "golang.org/x/image/webp" // ...
)

// group is a group of the docs: the id a doc names it by, and its title on
// the site.
type group struct{ id, title string }

// groups are the docs' groups, in the site's order: what a user starts
// with, what MixMusic does, how it does it, what to make for it, and the
// project.
var groups = []group{
	{"start", "Getting started"},
	{"using", "Using MixMusic"},
	{"inside", "How it works"},
	{"make", "Make your own"},
	{"project", "Project"},
}

// groupIndex is the place of the group id among the groups, or -1.
func groupIndex(id string) int {
	return slices.IndexFunc(groups, func(g group) bool { return g.id == id })
}

// groupIDs are the groups' ids, as an error lists them.
func groupIDs() string {
	var ids []string
	for _, g := range groups {
		ids = append(ids, g.id)
	}
	return strings.Join(ids, ", ")
}

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

// doc is one of docs/*.md, a file of a folder of docs, or a file of the
// root, on its way to being a page or a part of one.
type doc struct {
	file        string // as the repository has it: docs/plugins.md, a folder's docs/architecture/README.md, or CONTRIBUTING.md
	folder      string // of a folder's README: the folder, as docs/architecture/
	slug        string // the page's folder in the docs section: plugins
	anchor      string // of a folder's other file: the id of its heading on the folder's page
	heading     string // the title, as the Markdown has it
	title       string
	description string
	group       string // the id of the doc's group, "" if it names none
	order       int    // its order in the group, smaller first
	ordered     bool   // whether it names its order: those that don't follow those that do, by slug
	first       bool   // whether its page is the first of its group
	line        int    // the line of its title
	weight      int
	body        string            // the Markdown after the title and its place, its links rewritten
	offset      int               // the lines of the file before the body
	assets      map[string]string // files to copy beside the page: their path there, to the repository's
	parts       []*doc            // of a folder's README: the folder's other files, which the page shows after it, in order
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
	// What a link can go to: each doc, by its file, and each folder of
	// docs, by the folder. Their paths are in lower case.
	targets := map[string]*doc{}
	for _, d := range docs {
		targets[strings.ToLower(d.file)] = d
		if d.folder != "" {
			targets[strings.ToLower(strings.TrimSuffix(d.folder, "/"))] = d
		}
		for _, p := range d.parts {
			targets[strings.ToLower(p.file)] = p
		}
	}
	var errs []error
	for _, d := range docs {
		errs = append(errs, d.rewrite(o, targets))
		for _, p := range d.parts {
			errs = append(errs, p.rewrite(o, targets))
		}
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

// read reads docs/*.md, the folders of docs/ with a README.md, and the
// Markdown files of the root that name their place, in the docs' order.
func read(root string) ([]*doc, error) {
	files, err := filepath.Glob(filepath.Join(root, "docs", "*.md"))
	if err != nil {
		return nil, err
	}
	readmes, err := filepath.Glob(filepath.Join(root, "docs", "*", "README.md"))
	if err != nil {
		return nil, err
	}
	rooted, err := filepath.Glob(filepath.Join(root, "*.md"))
	if err != nil {
		return nil, err
	}
	var docs []*doc
	for _, f := range files {
		name := filepath.Base(f)
		docs = append(docs, &doc{file: "docs/" + name, slug: strings.ToLower(strings.TrimSuffix(name, ".md"))})
	}
	for _, f := range readmes {
		name := filepath.Base(filepath.Dir(f))
		d := &doc{file: "docs/" + name + "/README.md", folder: "docs/" + name + "/", slug: strings.ToLower(name)}
		parts, err := filepath.Glob(filepath.Join(filepath.Dir(f), "*.md"))
		if err != nil {
			return nil, err
		}
		for _, p := range parts {
			if base := filepath.Base(p); base != "README.md" {
				d.parts = append(d.parts, &doc{file: d.folder + base, slug: d.slug, anchor: anchor(strings.TrimSuffix(base, ".md"))})
			}
		}
		docs = append(docs, d)
	}
	// The root's files are pages only if they name their place: the
	// README and the changelog don't.
	for _, f := range rooted {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		if !namesPlace(string(data)) {
			continue
		}
		name := filepath.Base(f)
		slug := strings.ToLower(strings.ReplaceAll(strings.TrimSuffix(name, ".md"), "_", "-"))
		docs = append(docs, &doc{file: name, slug: slug})
	}
	var errs []error
	bySlug := map[string]*doc{}
	for _, d := range docs {
		if other := bySlug[d.slug]; other != nil {
			errs = append(errs, fmt.Errorf("%s and %s would both be the page /docs/%s/", other.file, d.file, d.slug))
		}
		bySlug[d.slug] = d
		if err := d.load(root); err != nil {
			errs = append(errs, err)
		} else if d.group == "" {
			errs = append(errs, fmt.Errorf("%s:%d: a doc names its place on the line after its title, as <!-- docs: group=using order=30 -->, in one of the groups %s", d.file, d.line+1, groupIDs()))
		}
		for _, p := range d.parts {
			errs = append(errs, p.load(root))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	for _, d := range docs {
		d.arrange()
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("no docs in %s", filepath.Join(root, "docs"))
	}
	slices.SortStableFunc(docs, func(a, b *doc) int {
		if ga, gb := groupIndex(a.group), groupIndex(b.group); ga != gb {
			return ga - gb
		}
		switch {
		case a.ordered != b.ordered:
			if a.ordered {
				return -1
			}
			return 1
		case a.order != b.order:
			return a.order - b.order
		}
		return strings.Compare(a.slug, b.slug)
	})
	for i, d := range docs {
		d.weight = (i + 1) * 10
		d.first = i == 0 || docs[i-1].group != d.group
	}
	return docs, nil
}

// load reads and parses the doc's file.
func (d *doc) load(root string) error {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(d.file)))
	if err != nil {
		return err
	}
	return d.parse(string(data))
}

// arrange puts a folder's other files in the order its README links them,
// and those it doesn't link after them, by name.
func (d *doc) arrange() {
	rank := map[string]int{}
	lines := strings.Split(d.body, "\n")
	links(lines, func(i, start, end int, image, html bool) {
		p, _, err := inRepo(d.file, strings.Trim(lines[i][start:end], "<>"))
		if p = strings.ToLower(p); err == nil && p != "" && !image {
			if _, ok := rank[p]; !ok {
				rank[p] = len(rank)
			}
		}
	})
	at := func(p *doc) int {
		if r, ok := rank[strings.ToLower(p.file)]; ok {
			return r
		}
		return len(rank)
	}
	slices.SortStableFunc(d.parts, func(a, b *doc) int { return at(a) - at(b) })
}

// anchor is the id of a heading named after a file: the letters, digits and
// underscores of its name, in lower case, with a dash for what's between
// them.
func anchor(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			dash = true
			continue
		}
		if dash && b.Len() > 0 {
			b.WriteByte('-')
		}
		dash = false
		b.WriteRune(r)
	}
	return b.String()
}

var heading1 = regexp.MustCompile(`^#[ \t]+(.+?)(?:[ \t]+#+)?[ \t]*$`)

// place is the comment that names a doc's place, on the line after its
// title: <!-- docs: group=using order=30 -->.
var place = regexp.MustCompile(`^[ \t]*<!--[ \t]*docs:(.*?)-->[ \t]*$`)

// lines are a file's lines, without a byte order mark or carriage returns.
func lines(text string) []string {
	text = strings.TrimPrefix(text, string(rune(0xFEFF)))
	return strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
}

// namesPlace tells whether a file's first lines are its title and the
// comment that names its place.
func namesPlace(text string) bool {
	n := 0
	for _, line := range lines(text) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if n++; n == 2 {
			return place.MatchString(line)
		}
		if !heading1.MatchString(line) {
			return false
		}
	}
	return false
}

// parse takes the doc's title from the heading of its first line, as the
// page shows the title itself, its place from the comment after it, and
// its description from its first paragraph.
func (d *doc) parse(text string) error {
	lines := lines(text)
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
	d.line = n + 1
	d.heading = m[1]
	d.title = plain(m[1])
	d.offset = blank(n + 1)
	if d.offset < len(lines) {
		if m := place.FindStringSubmatch(lines[d.offset]); m != nil {
			if err := d.placeOf(m[1]); err != nil {
				return fmt.Errorf("%s:%d: %w", d.file, d.offset+1, err)
			}
			d.offset = blank(d.offset + 1)
		}
	}
	d.body = strings.Join(lines[d.offset:], "\n")
	d.description = summary(d.body)
	return nil
}

// placeOf reads the fields of the comment that names the doc's place:
// group=<id>, and order=<number>, which may be left out.
func (d *doc) placeOf(fields string) error {
	for _, f := range strings.Fields(fields) {
		key, value, ok := strings.Cut(f, "=")
		switch {
		case !ok:
			return fmt.Errorf("%q isn't key=value in the doc's place", f)
		case key == "group":
			if groupIndex(value) < 0 {
				return fmt.Errorf("the group %q isn't one of %s", value, groupIDs())
			}
			d.group = value
		case key == "order":
			n, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("the order %q isn't a whole number", value)
			}
			d.order, d.ordered = n, true
		default:
			return fmt.Errorf("the doc's place has group and order, not %q", key)
		}
	}
	if d.group == "" {
		return errors.New("the doc's place names no group")
	}
	return nil
}

// description is the most a description says: search engines show some
// 160 characters of it.
const description = 160

// summary is the first sentences of the first paragraph of prose, up to
// some sixty characters or more, as plain text, and shortened at a word
// to keep it under 160.
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
		for utf8.RuneCountInString(s) > description && len(words) > 1 {
			words = words[:len(words)-1]
			s = strings.TrimRight(strings.Join(words, " "), ",;:") + "…"
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
func (d *doc) rewrite(o options, targets map[string]*doc) error {
	d.assets = map[string]string{}
	lines := strings.Split(d.body, "\n")
	edits := make([][]edit, len(lines))
	var errs []error
	links(lines, func(i, start, end int, image, html bool) {
		t := lines[i][start:end]
		angled := strings.HasPrefix(t, "<")
		t = strings.Trim(t, "<>")
		to, err := d.resolve(t, image, html, o, targets)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s:%d: %s: %w", d.file, d.offset+i+1, t, err))
			return
		}
		if angled {
			to = "<" + to + ">"
		}
		edits[i] = append(edits[i], edit{start, end, to})
	})
	for i, line := range lines {
		if len(edits[i]) == 0 {
			continue
		}
		slices.SortFunc(edits[i], func(a, b edit) int { return a.start - b.start })
		var b strings.Builder
		at := 0
		for _, e := range edits[i] {
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

// links calls f with the target of each link of the Markdown's lines,
// outside code: of a link or an image, of a link defined by reference, and
// of an attribute of HTML that holds an address. It names the line, and
// where the target is in it.
func links(lines []string, f func(i, start, end int, image, html bool)) {
	code := inCode(lines)
	for i, line := range lines {
		if code[i] {
			continue
		}
		masked := maskCode(line)
		if m := refDef.FindStringSubmatchIndex(masked); m != nil {
			f(i, m[2], m[3], false, false)
		} else {
			for _, m := range inlineLink.FindAllStringSubmatchIndex(masked, -1) {
				f(i, m[4], m[5], m[3] > m[2], false)
			}
		}
		for _, m := range htmlAttr.FindAllStringSubmatchIndex(masked, -1) {
			attr := strings.ToLower(masked[m[2]:m[3]])
			f(i, m[4]+1, m[5]-1, attr != "href", true)
		}
	}
}

// inCode tells which of the Markdown's lines are fenced code, with their
// fences.
func inCode(lines []string) []bool {
	code := make([]bool, len(lines))
	fence := ""
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if fence != "" {
			code[i] = true
			if strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, fence[:1]+" \t") == "" {
				fence = ""
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			code[i] = true
			fence = trimmed[:len(trimmed)-len(strings.TrimLeft(trimmed, trimmed[:1]))]
		}
	}
	return code
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

// resolve rewrites one link's target for the site. A link to a doc, or to
// a folder of docs, goes to its page, and to a folder's other file, to its
// heading there, unless the link names a heading of its own. Images, and
// links to files of docs/ other than docs, are copied beside the page.
// Markdown's links go through Zola's @/ paths, which it checks; Zola
// leaves HTML's as they are, so those are relative to the page, at
// /docs/<slug>/.
func (d *doc) resolve(target string, image, html bool, o options, targets map[string]*doc) (string, error) {
	p, suffix, err := inRepo(d.file, target)
	if err != nil {
		return "", err
	}
	if p == "" {
		return target, nil
	}
	info, err := os.Stat(filepath.Join(o.root, filepath.FromSlash(p)))
	if err != nil {
		return "", errors.New("not in the repository")
	}
	if t := targets[strings.ToLower(p)]; t != nil {
		if t.anchor != "" && !strings.Contains(suffix, "#") {
			suffix += "#" + t.anchor
		}
		if html {
			return "../" + escape(t.slug) + "/" + suffix, nil
		}
		return "@/" + path.Join(o.section(), t.slug, "index.md") + suffix, nil
	}
	inDocs := path.Dir(p) == "docs" || strings.HasPrefix(p, "docs/")
	switch {
	case info.IsDir():
		return github(o, "tree", p) + suffix, nil
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

// inRepo is what a link of the file from points at in the repository: its
// path from the root, slash-separated, and the query or fragment after it.
// The path is empty for a link elsewhere, or within the page.
func inRepo(from, target string) (p, suffix string, err error) {
	if target == "" || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "//") || scheme.MatchString(target) {
		return "", "", nil
	}
	p = target
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p, suffix = p[:i], p[i:]
	}
	p, err = url.PathUnescape(p)
	if err != nil {
		return "", "", err
	}
	if strings.HasPrefix(p, "/") {
		p = path.Clean(strings.TrimPrefix(p, "/"))
	} else {
		p = path.Join(path.Dir(from), p)
	}
	if p == ".." || strings.HasPrefix(p, "../") {
		return "", "", errors.New("outside the repository")
	}
	return p, suffix, nil
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

// asset is a file copied beside a page: its data, and its size if it's an
// image.
type asset struct {
	data []byte
	size image.Point
}

// write writes the doc's page, and copies its assets, and its parts', beside
// it.
func (d *doc) write(root, out string) error {
	dir := filepath.Join(out, d.slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	assets := map[string]asset{}
	for _, p := range append([]*doc{d}, d.parts...) {
		for local, src := range p.assets {
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(src)))
			if err != nil {
				return err
			}
			a := asset{data: data}
			if c, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil && c.Width > 0 && c.Height > 0 {
				a.size = image.Pt(c.Width, c.Height)
			}
			assets[local] = a
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte(d.page(assets)), 0o644); err != nil {
		return err
	}
	for local, a := range assets {
		dst := filepath.Join(dir, filepath.FromSlash(local))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, a.data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// page is the page's Markdown, with its front matter: its title,
// description and weight, and as extra its file, its group, and the size
// of each image beside it, by its path as the page's links have it. A
// folder's page follows its README with its other files, each under a
// heading of its title, with the anchor of its name, and its own headings
// a level down.
func (d *doc) page(assets map[string]asset) string {
	source, edit := d.file, "edit that file, not this one."
	if d.folder != "" {
		source, edit = d.folder, "edit its files, not this one."
	}
	var b strings.Builder
	b.WriteString("+++\n")
	b.WriteString(marker + source + ": " + edit + "\n")
	fmt.Fprintf(&b, "title = %s\n", tomlString(d.title))
	if d.description != "" {
		fmt.Fprintf(&b, "description = %s\n", tomlString(d.description))
	}
	fmt.Fprintf(&b, "weight = %d\n\n[extra]\nsource = %s\n", d.weight, tomlString(source))
	if g := groupIndex(d.group); g >= 0 {
		fmt.Fprintf(&b, "group = %s\ngroup_title = %s\n", tomlString(groups[g].id), tomlString(groups[g].title))
		if d.first {
			b.WriteString("group_first = true\n")
		}
	}
	var images []string
	for local, a := range assets {
		if a.size != (image.Point{}) {
			images = append(images, local)
		}
	}
	if len(images) > 0 {
		slices.Sort(images)
		b.WriteString("\n[extra.images]\n")
		for _, local := range images {
			s := assets[local].size
			fmt.Fprintf(&b, "%s = [%d, %d]\n", tomlString(escape(local)), s.X, s.Y)
		}
	}
	b.WriteString("+++\n\n")
	b.WriteString(strings.TrimRight(d.body, "\n"))
	for _, p := range d.parts {
		fmt.Fprintf(&b, "\n\n## %s {#%s}", p.heading, p.anchor)
		if body := strings.TrimRight(demote(p.body), "\n"); body != "" {
			b.WriteString("\n\n" + body)
		}
	}
	b.WriteString("\n")
	return b.String()
}

// heading is a heading of Markdown's that a # more leaves one: # to #####.
var heading = regexp.MustCompile(`^( {0,3})(#{1,5})([ \t]|$)`)

// demote takes the headings of Markdown down a level, outside code.
func demote(body string) string {
	lines := strings.Split(body, "\n")
	code := inCode(lines)
	for i, line := range lines {
		if !code[i] {
			lines[i] = heading.ReplaceAllString(line, "$1#$2$3")
		}
	}
	return strings.Join(lines, "\n")
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

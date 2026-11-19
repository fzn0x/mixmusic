package main

// A link to a heading of another doc names it as GitHub makes its anchor,
// so that it works where GitHub shows the docs: "An artist's songs" is
// #an-artists-songs there, and "Last.fm" #lastfm. Zola makes its own
// anchor of the same heading, #an-artist-s-songs and #last-fm, and checks
// a page's links against those. siteAnchor gives a link the anchor Zola
// made, so that it reaches the heading on the site too.

import (
	"bufio"
	"os"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var (
	atxHeading = regexp.MustCompile(`^ {0,3}(#{1,6})[ \t]+(.*?)[ \t]*#*[ \t]*$`)
	ownID      = regexp.MustCompile(`[ \t]*\{#([^}\s]+)\}$`)
	mdLink     = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)
)

// headingAnchors reads the headings of a doc, outside its code, and maps
// the anchor GitHub makes of each to the one Zola makes. A heading with an
// id of its own, as {#id}, has that one in both.
func headingAnchors(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]string{}
	seenGitHub, seenZola := map[string]int{}, map[string]int{}
	fence := ""
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		trimmed := strings.TrimLeft(line, " ")
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fence = trimmed[:3]
			continue
		}
		m := atxHeading.FindStringSubmatch(line)
		if m == nil || len(m[1]) == 1 {
			continue // not a heading, or the doc's title
		}
		text := m[2]
		if id := ownID.FindStringSubmatch(text); id != nil {
			out[id[1]] = id[1]
			continue
		}
		text = mdLink.ReplaceAllString(text, "$1")
		text = strings.ReplaceAll(text, "`", "")
		out[unique(githubAnchor(text), seenGitHub)] = unique(zolaAnchor(text), seenZola)
	}
	return out, sc.Err()
}

// unique gives a heading's anchor that another heading of the page has
// already a number, as both GitHub and Zola do: intro, intro-1, intro-2.
func unique(a string, seen map[string]int) string {
	n := seen[a]
	seen[a] = n + 1
	if n == 0 {
		return a
	}
	return a + "-" + itoa(n)
}

func itoa(n int) string {
	var b []byte
	for ; n > 0 || len(b) == 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

// githubAnchor is a heading's anchor as GitHub makes it: in lower case,
// its punctuation dropped, but hyphens and underscores, and each space a
// hyphen.
func githubAnchor(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// zolaAnchor is a heading's anchor as Zola makes it: in Latin letters
// without their accents, in lower case, and a hyphen for whatever is
// between letters and digits.
func zolaAnchor(text string) string {
	var b strings.Builder
	dash := false
	for _, r := range norm.NFD.String(strings.ToLower(text)) {
		if unicode.IsMark(r) {
			continue
		}
		if r < 0x80 && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
			continue
		}
		dash = true
	}
	return b.String()
}

// siteAnchor is the anchor on the site of a link's fragment to the doc
// file: Zola's for a heading the fragment names as GitHub does, else the
// fragment as it is, which Zola checks.
func (o options) siteAnchor(file, fragment string) string {
	if o.anchors == nil {
		return fragment
	}
	m, ok := o.anchors[file]
	if !ok {
		m, _ = headingAnchors(file)
		o.anchors[file] = m
	}
	if a, ok := m[fragment]; ok {
		return a
	}
	return fragment
}

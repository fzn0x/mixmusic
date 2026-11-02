// Command metainfo writes the Flatpak's AppStream metadata, which software
// centers show: packaging/flatpak/io.github.fzn0x.mixmusic.metainfo.xml,
// with the releases of CHANGELOG.md in place of its <releases/>, each with
// its date, its summary (the first paragraph of its section) and a link to
// its page. The Flatpak builds of ci.yml and release.yml run it before
// flatpak-builder, which reads the file it writes.
//
//	go run ./tools/metainfo -o packaging/flatpak/dist/io.github.fzn0x.mixmusic.metainfo.xml
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// template is the metadata with <releases/> in place of the releases.
const template = "packaging/flatpak/io.github.fzn0x.mixmusic.metainfo.xml"

// releasePage is the page of a version's release, by its tag.
const releasePage = "https://github.com/fzn0x/mixmusic/releases/tag/v"

func main() {
	root := flag.String("root", ".", "the repository's root")
	out := flag.String("o", "", "the file to write; standard output when empty")
	flag.Parse()
	tmpl, err := os.ReadFile(filepath.Join(*root, template))
	if err != nil {
		log.Fatal(err)
	}
	changelog, err := os.ReadFile(filepath.Join(*root, "CHANGELOG.md"))
	if err != nil {
		log.Fatal(err)
	}
	meta, err := metainfo(string(tmpl), string(changelog))
	if err != nil {
		log.Fatal(err)
	}
	if *out == "" {
		fmt.Print(meta)
		return
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, []byte(meta), 0o644); err != nil {
		log.Fatal(err)
	}
}

// release is a version CHANGELOG.md has a section of.
type release struct {
	version, date string
	// summary is the first paragraph of the section, in plain text, or ""
	// when a list or a heading comes first.
	summary string
}

// heading starts the section of a released version; ## [Unreleased]
// doesn't match.
var heading = regexp.MustCompile(`^## \[(\d+\.\d+\.\d+)\] - (\d{4}-\d{2}-\d{2})\s*$`)

// releases returns the released versions of a changelog, newest first, as
// it lists them.
func releases(changelog string) []release {
	lines := strings.Split(strings.ReplaceAll(changelog, "\r\n", "\n"), "\n")
	var rs []release
	for i, l := range lines {
		m := heading.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		r := release{version: m[1], date: m[2]}
		j := i + 1
		for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
			j++
		}
		var para []string
		for ; j < len(lines); j++ {
			t := strings.TrimSpace(lines[j])
			if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") || strings.HasPrefix(t, "[") && strings.Contains(t, "]: ") {
				break
			}
			para = append(para, t)
		}
		r.summary = plain(strings.Join(para, " "))
		rs = append(rs, r)
	}
	return rs
}

var (
	markdownLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	htmlTag      = regexp.MustCompile(`</?[a-zA-Z][^>]*>`)
	// offered is the sentence that tells which copies offer the version,
	// as "MixMusic 0.3.0 and later offer this version in Settings ›
	// About.": a Flatpak is offered its next bundle.
	offered = regexp.MustCompile(`MixMusic [0-9.]+( and ([0-9.]+|later))? offers? this version in Settings › About\.`)
)

// plain returns a summary in Markdown as plain text: links as their text,
// without emphasis, code marks or HTML tags, nor the sentence that tells
// which copies offer the version.
func plain(md string) string {
	md = markdownLink.ReplaceAllString(md, "$1")
	md = htmlTag.ReplaceAllString(md, "")
	md = offered.ReplaceAllString(md, "")
	md = strings.NewReplacer("**", "", "__", "", "`", "").Replace(md)
	return strings.Join(strings.Fields(md), " ")
}

// metainfo returns the metadata of tmpl with the releases of changelog in
// place of its <releases/>, indented as it is.
func metainfo(tmpl, changelog string) (string, error) {
	rs := releases(changelog)
	if len(rs) == 0 {
		return "", errors.New("CHANGELOG.md has no released version: no \"## [x.y.z] - YYYY-MM-DD\" heading")
	}
	const placeholder = "<releases/>"
	if n := strings.Count(tmpl, placeholder); n != 1 {
		return "", fmt.Errorf("%s has %d %s, want one", template, n, placeholder)
	}
	at := strings.Index(tmpl, placeholder)
	indent := tmpl[strings.LastIndex(tmpl[:at], "\n")+1 : at]
	if strings.TrimSpace(indent) != "" {
		return "", fmt.Errorf("%s: %s must be alone on its line", template, placeholder)
	}
	var b strings.Builder
	b.WriteString("<releases>\n")
	for _, r := range rs {
		fmt.Fprintf(&b, "%s  <release version=%q date=%q>\n", indent, r.version, r.date)
		fmt.Fprintf(&b, "%s    <url type=\"details\">%s%s</url>\n", indent, releasePage, r.version)
		if r.summary != "" {
			fmt.Fprintf(&b, "%s    <description>\n%s      <p>%s</p>\n%s    </description>\n", indent, indent, escape(r.summary), indent)
		}
		fmt.Fprintf(&b, "%s  </release>\n", indent)
	}
	b.WriteString(indent + "</releases>")
	return tmpl[:at] + b.String() + tmpl[at+len(placeholder):], nil
}

// escape returns s as XML text.
var escape = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace

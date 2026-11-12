package main

// llms.txt and llms-full.txt, for AI agents and the tools that read a
// site for them, as llmstxt.org proposes: what MixMusic is, where to get
// it, and its docs, each a link with its description in llms.txt, and all
// of them in Markdown in llms-full.txt. The docs are the pages
// tools/sitedocs made in site/content/docs, from docs/*.md.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// docPage is a page of the docs, as sitedocs wrote it.
type docPage struct {
	title, description, group, path, body string
	weight                                int
}

var frontLine = regexp.MustCompile(`(?m)^(title|description|weight|group_title) = (.*)$`)

// tomlValue reads a TOML string or number of sitedocs's front matter.
func tomlValue(s string) string {
	s = strings.TrimSpace(s)
	if v, err := strconv.Unquote(s); err == nil {
		return v
	}
	return strings.Trim(s, `"'`)
}

// readDocs reads the pages of the docs under dir, site/content/docs, in
// their order.
func readDocs(dir string) ([]docPage, error) {
	var pages []docPage
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "index.md" {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := strings.ReplaceAll(string(src), "\r\n", "\n")
		parts := strings.SplitN(text, "+++\n", 3)
		if len(parts) != 3 {
			return nil
		}
		var p docPage
		for _, m := range frontLine.FindAllStringSubmatch(parts[1], -1) {
			switch m[1] {
			case "title":
				p.title = tomlValue(m[2])
			case "description":
				p.description = tomlValue(m[2])
			case "weight":
				p.weight, _ = strconv.Atoi(tomlValue(m[2]))
			case "group_title":
				p.group = tomlValue(m[2])
			}
		}
		rel, _ := filepath.Rel(dir, filepath.Dir(path))
		p.path = "docs/" + filepath.ToSlash(rel) + "/"
		p.body = strings.TrimSpace(parts[2])
		if p.title != "" {
			pages = append(pages, p)
		}
		return nil
	})
	slices.SortStableFunc(pages, func(a, b docPage) int {
		if a.weight != b.weight {
			return a.weight - b.weight
		}
		return strings.Compare(a.path, b.path)
	})
	return pages, err
}

// llms returns llms.txt and llms-full.txt for the site at canonical,
// described as description, with the docs pages.
func llms(pages []docPage, canonical, description string) (index, full string) {
	canonical = strings.TrimRight(canonical, "/") + "/"
	var b strings.Builder
	b.WriteString("# MixMusic\n\n")
	fmt.Fprintf(&b, "> %s\n\n", description)
	b.WriteString("MixMusic is one desktop player for YouTube Music and Spotify: one search, one queue and one library across both, with synced lyrics, podcasts, downloads, themes and plugins. YouTube Music works with no account; Spotify needs Premium. It is free and open source, under the MIT license or the GNU GPL version 3, for macOS 12 or later, Windows 10 and 11, and Linux.\n\n")
	b.WriteString("## Get it\n\n")
	b.WriteString("- [Latest release](https://github.com/fzn0x/mixmusic/releases/latest): installers and archives for macOS, Windows and Linux\n")
	b.WriteString("- [Source code](https://github.com/fzn0x/mixmusic): the repository, its README and its issues\n")
	fmt.Fprintf(&b, "- [Website](%s): the landing page\n", canonical)
	fmt.Fprintf(&b, "- [Docs](%sdocs/): every page below\n\n", canonical)
	for i, p := range pages {
		g := p.group
		if g == "" {
			g = "Docs"
		}
		if i == 0 || pages[i-1].group != p.group {
			if i > 0 {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "## %s\n\n", g)
		}
		fmt.Fprintf(&b, "- [%s](%s%s)", p.title, canonical, p.path)
		if p.description != "" {
			fmt.Fprintf(&b, ": %s", p.description)
		}
		b.WriteString("\n")
	}
	if len(pages) > 0 {
		b.WriteString("\n")
	}
	b.WriteString("## Languages\n\n")
	b.WriteString("The landing page is in English and in the languages of the most populous countries; the docs are in English.\n\n")
	for _, l := range languages {
		fmt.Fprintf(&b, "- [%s](%s%s)\n", l.Name, canonical, l.path())
	}
	b.WriteString("\n## Optional\n\n")
	fmt.Fprintf(&b, "- [All the docs in one file](%sllms-full.txt): every page of the docs, in Markdown\n", canonical)
	index = b.String()

	var f strings.Builder
	f.WriteString("# MixMusic docs\n\n")
	fmt.Fprintf(&f, "> %s\n\n", description)
	fmt.Fprintf(&f, "Every page of MixMusic's docs, in Markdown, from %sdocs/. Each starts with its title and its address.\n\n", canonical)
	for _, p := range pages {
		fmt.Fprintf(&f, "---\n\n# %s\n\nSource: %s%s\n\n%s\n\n", p.title, canonical, p.path, p.body)
	}
	return index, f.String()
}

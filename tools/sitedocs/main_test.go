package main

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repo writes the files of a repository into a temporary folder.
func repo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, text := range files {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func opts(root string) options {
	return options{root: root, out: filepath.Join("site", "content", "docs"), repo: "https://github.com/owner/name", branch: "main"}
}

func readFile(t *testing.T, file string) string {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// slugs are the docs' slugs, in their order.
func slugs(docs []*doc) string {
	var s []string
	for _, d := range docs {
		s = append(s, d.slug)
	}
	return strings.Join(s, " ")
}

const plugins = "# Plugins\r\n" +
	"<!-- docs: group=make order=20 -->\r\n" +
	"\r\n" +
	"A plugin adds a page. It is a folder:\r\n" +
	"\r\n" +
	"See [themes](themes.md#colors), [the API](../sdk/index.d.ts), [a theme](../community/themes/mint),\r\n" +
	"[the README](../README.md#develop), [home](/README.md), [the web](https://example.com/a.md) and [below](#the-module).\r\n" +
	"![The banner](banner.webp \"A banner\") and <img src=\"shots/home.webp\" alt=\"\"> and ![logo](../resources/logo.png).\r\n" +
	"<a href='themes.md#colors'>Colors</a>, <a href=\"../sdk/index.d.ts\">types</a>, <video poster=\"/docs/shots/home.webp\"></video>\r\n" +
	"[`index.d.ts`](../sdk/index.d.ts) and `[not a link](missing.md)` and ``a `[x](gone.md)` b``.\r\n" +
	"\r\n" +
	"```js\r\n" +
	"[not a link](missing.md)\r\n" +
	"```\r\n" +
	"\r\n" +
	"1. A step:\r\n" +
	"\r\n" +
	"   ````\r\n" +
	"   ```\r\n" +
	"   [still code](missing.md)\r\n" +
	"   ````\r\n" +
	"\r\n" +
	"[sdk]: ../sdk/index.d.ts\r\n"

func TestRun(t *testing.T) {
	root := repo(t, map[string]string{
		"docs/plugins.md":                  plugins,
		"docs/themes.md":                   "# Themes `json`\n<!--docs: group=make  order=10-->\nA theme sets the colors, the typefaces and the corners of MixMusic, which draws everything with them. A theme is a folder.\n\n## Colors\n",
		"docs/zebra.md":                    "# Zebra\n\n<!-- docs: group=make -->\n\n- a list first\n\nThen prose: it ends in a colon:\n",
		"docs/banner.webp":                 "banner",
		"docs/shots/home.webp":             "home",
		"resources/logo.png":               "logo",
		"sdk/index.d.ts":                   "types",
		"README.md":                        "readme",
		"community/themes/mint/theme.json": "{}",
		// A page made before, of a doc since deleted, and a folder of
		// someone else's.
		"site/content/docs/old/index.md":  "+++\n" + marker + "docs/old.md: edit that file, not this one.\n+++\n",
		"site/content/docs/mine/index.md": "+++\ntitle = \"Mine\"\n+++\n",
		"site/content/docs/_index.md":     "+++\ntitle = \"Docs\"\n+++\n",
	})
	docs, err := run(opts(root))
	if err != nil {
		t.Fatal(err)
	}
	if got := slugs(docs); got != "themes plugins zebra" {
		t.Errorf("order: %s", got)
	}

	out := filepath.Join(root, "site", "content", "docs")
	page := readFile(t, filepath.Join(out, "plugins", "index.md"))
	want := "+++\n" +
		marker + "docs/plugins.md: edit that file, not this one.\n" +
		"title = \"Plugins\"\n" +
		"description = \"A plugin adds a page. It is a folder.\"\n" +
		"weight = 20\n" +
		"\n" +
		"[extra]\n" +
		"source = \"docs/plugins.md\"\n" +
		"group = \"make\"\n" +
		"group_title = \"Make your own\"\n" +
		"+++\n" +
		"\n" +
		"A plugin adds a page. It is a folder:\n" +
		"\n" +
		"See [themes](@/docs/themes/index.md#colors), [the API](https://github.com/owner/name/blob/main/sdk/index.d.ts), [a theme](https://github.com/owner/name/tree/main/community/themes/mint),\n" +
		"[the README](https://github.com/owner/name/blob/main/README.md#develop), [home](https://github.com/owner/name/blob/main/README.md), [the web](https://example.com/a.md) and [below](#the-module).\n" +
		"![The banner](@/docs/plugins/banner.webp \"A banner\") and <img src=\"shots/home.webp\" alt=\"\"> and ![logo](@/docs/plugins/repo/resources/logo.png).\n" +
		"<a href='../themes/#colors'>Colors</a>, <a href=\"https://github.com/owner/name/blob/main/sdk/index.d.ts\">types</a>, <video poster=\"shots/home.webp\"></video>\n" +
		"[`index.d.ts`](https://github.com/owner/name/blob/main/sdk/index.d.ts) and `[not a link](missing.md)` and ``a `[x](gone.md)` b``.\n" +
		"\n" +
		"```js\n" +
		"[not a link](missing.md)\n" +
		"```\n" +
		"\n" +
		"1. A step:\n" +
		"\n" +
		"   ````\n" +
		"   ```\n" +
		"   [still code](missing.md)\n" +
		"   ````\n" +
		"\n" +
		"[sdk]: https://github.com/owner/name/blob/main/sdk/index.d.ts\n"
	if page != want {
		t.Errorf("plugins page:\n%s\nwant:\n%s", page, want)
	}
	for file, text := range map[string]string{"plugins/banner.webp": "banner", "plugins/shots/home.webp": "home", "plugins/repo/resources/logo.png": "logo"} {
		if got := readFile(t, filepath.Join(out, filepath.FromSlash(file))); got != text {
			t.Errorf("%s: %q", file, got)
		}
	}

	themes := readFile(t, filepath.Join(out, "themes", "index.md"))
	for _, s := range []string{"title = \"Themes json\"\n", "description = \"A theme sets the colors, the typefaces and the corners of MixMusic, which draws everything with them.\"\n", "weight = 10\n", "group = \"make\"\n", "group_first = true\n", "+++\n\nA theme sets"} {
		if !strings.Contains(themes, s) {
			t.Errorf("themes page lacks %q:\n%s", s, themes)
		}
	}
	zebra := readFile(t, filepath.Join(out, "zebra", "index.md"))
	for _, s := range []string{"description = \"Then prose: it ends in a colon.\"\n", "weight = 30\n", "+++\n\n- a list first"} {
		if !strings.Contains(zebra, s) {
			t.Errorf("zebra page lacks %q:\n%s", s, zebra)
		}
	}
	if strings.Contains(zebra, "group_first") || strings.Contains(zebra, "<!--") {
		t.Errorf("zebra page:\n%s", zebra)
	}

	if _, err := os.Stat(filepath.Join(out, "old")); !os.IsNotExist(err) {
		t.Errorf("the page of a deleted doc stays: %v", err)
	}
	for _, file := range []string{"mine/index.md", "_index.md"} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(file))); err != nil {
			t.Errorf("%s is gone: %v", file, err)
		}
	}
}

func TestFolder(t *testing.T) {
	root := repo(t, map[string]string{
		"docs/arch/README.md": "# Arch\n<!-- docs: group=project order=60 -->\n\nHow it is made, in parts that each have a file.\n\n" +
			"1. [Two](two.md): the second.\n2. [One](one.md#deep): the first.\n\n" +
			"See [the guide](../guide.md), [the folder](./) and ![a picture](pic.png).\n",
		"docs/arch/one.md": "# One `pkg`\n\nOne links to [two](two.md), [its README](README.md) and <a href=\"two.md\">two</a>.\n\n" +
			"## Deep\n\n```\n# not a heading\n```\n\n![pic](pic.png)\n",
		"docs/arch/two.md":   "# Two\n\nTwo links to [the guide](../guide.md#top) and [code](../../src/main.go).\n",
		"docs/arch/three.md": "# Three\n",
		"docs/arch/pic.png":  "pic",
		"docs/guide.md":      "# Guide\n<!-- docs: group=start order=10 -->\n\nSee [the parts](arch/README.md), [one](arch/one.md), [two's end](arch/two.md#end) and [the folder](arch/).\n",
		"docs/shots/a.png":   "a folder without a README, which is no page",
		"src/main.go":        "package main",
	})
	docs, err := run(opts(root))
	if err != nil {
		t.Fatal(err)
	}
	if got := slugs(docs); got != "guide arch" {
		t.Errorf("pages: %s", got)
	}

	out := filepath.Join(root, "site", "content", "docs")
	page := readFile(t, filepath.Join(out, "arch", "index.md"))
	want := "+++\n" +
		marker + "docs/arch/: edit its files, not this one.\n" +
		"title = \"Arch\"\n" +
		"description = \"How it is made, in parts that each have a file.\"\n" +
		"weight = 20\n" +
		"\n" +
		"[extra]\n" +
		"source = \"docs/arch/\"\n" +
		"group = \"project\"\n" +
		"group_title = \"Project\"\n" +
		"group_first = true\n" +
		"+++\n" +
		"\n" +
		"How it is made, in parts that each have a file.\n" +
		"\n" +
		"1. [Two](@/docs/arch/index.md#two): the second.\n" +
		"2. [One](@/docs/arch/index.md#deep): the first.\n" +
		"\n" +
		"See [the guide](@/docs/guide/index.md), [the folder](@/docs/arch/index.md) and ![a picture](@/docs/arch/arch/pic.png).\n" +
		"\n" +
		"## Two {#two}\n" +
		"\n" +
		"Two links to [the guide](@/docs/guide/index.md#top) and [code](https://github.com/owner/name/blob/main/src/main.go).\n" +
		"\n" +
		"## One `pkg` {#one}\n" +
		"\n" +
		"One links to [two](@/docs/arch/index.md#two), [its README](@/docs/arch/index.md) and <a href=\"../arch/#two\">two</a>.\n" +
		"\n" +
		"### Deep\n" +
		"\n" +
		"```\n" +
		"# not a heading\n" +
		"```\n" +
		"\n" +
		"![pic](@/docs/arch/arch/pic.png)\n" +
		"\n" +
		"## Three {#three}\n"
	if page != want {
		t.Errorf("arch page:\n%s\nwant:\n%s", page, want)
	}
	if got := readFile(t, filepath.Join(out, "arch", "arch", "pic.png")); got != "pic" {
		t.Errorf("arch/pic.png: %q", got)
	}

	guide := readFile(t, filepath.Join(out, "guide", "index.md"))
	if s := "See [the parts](@/docs/arch/index.md), [one](@/docs/arch/index.md#one), [two's end](@/docs/arch/index.md#end) and [the folder](@/docs/arch/index.md).\n"; !strings.Contains(guide, s) {
		t.Errorf("guide page lacks %q:\n%s", s, guide)
	}
	if _, err := os.Stat(filepath.Join(out, "shots")); !os.IsNotExist(err) {
		t.Errorf("a folder without a README has a page: %v", err)
	}
}

func TestFolderAndFile(t *testing.T) {
	root := repo(t, map[string]string{
		"docs/arch.md":        "# Arch\n",
		"docs/arch/README.md": "# Arch\n",
	})
	if _, err := run(opts(root)); err == nil || !strings.Contains(err.Error(), "docs/arch.md and docs/arch/README.md would both be the page /docs/arch/") {
		t.Errorf("error: %v", err)
	}
}

func TestFolderBrokenLink(t *testing.T) {
	root := repo(t, map[string]string{
		"docs/arch/README.md": "# Arch\n<!-- docs: group=inside -->\n\n[One](one.md)\n",
		"docs/arch/one.md":    "# One\n\nText.\n\n[gone](gone.md)\n",
	})
	if _, err := run(opts(root)); err == nil || !strings.Contains(err.Error(), "docs/arch/one.md:5: gone.md: not in the repository") {
		t.Errorf("error: %v", err)
	}
}

// TestGroups puts each doc in its group, the groups in their order, and a
// doc's order within its group, smaller first, then those of none.
func TestGroups(t *testing.T) {
	root := repo(t, map[string]string{
		"docs/lyrics.md":       "# Lyrics\n<!-- docs: group=using order=30 -->\n\nLyrics.\n",
		"docs/playing.md":      "# Playing\n<!-- docs: group=using order=10 -->\n\nPlaying.\n",
		"docs/zz.md":           "# Zz\n<!-- docs: group=using -->\n\nZz.\n",
		"docs/aa.md":           "# Aa\n<!-- docs: group=using -->\n\nAa.\n",
		"docs/same.md":         "# Same\n<!-- docs: group=using order=30 -->\n\nSame.\n",
		"docs/start.md":        "# Start\n<!-- docs: group=start order=90 -->\n\nStart.\n",
		"docs/plugins.md":      "# Plugins\n<!-- docs: order=20 group=make -->\n\nPlugins.\n",
		"docs/inside.md":       "# Inside\n\n<!-- docs: group=inside order=10 -->\n\nInside.\n",
		"docs/notes/README.md": "# Notes\n<!-- docs: group=project order=60 -->\n\n[A](a.md)\n",
		"docs/notes/a.md":      "# A\n<!-- docs: group=start order=1 -->\n\nA part names no place of its own.\n",
	})
	docs, err := run(opts(root))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := slugs(docs), "start playing lyrics same aa zz inside plugins notes"; got != want {
		t.Errorf("order: %s, want %s", got, want)
	}
	out := filepath.Join(root, "site", "content", "docs")
	for slug, want := range map[string][]string{
		"start":   {"weight = 10\n", "group = \"start\"\ngroup_title = \"Getting started\"\ngroup_first = true\n"},
		"playing": {"weight = 20\n", "group = \"using\"\ngroup_title = \"Using MixMusic\"\ngroup_first = true\n"},
		"lyrics":  {"weight = 30\n", "group = \"using\"\ngroup_title = \"Using MixMusic\"\n+++\n\nLyrics.\n"},
		"inside":  {"group = \"inside\"\ngroup_title = \"How it works\"\ngroup_first = true\n", "+++\n\nInside.\n"},
		"notes":   {"group = \"project\"\n", "## A {#a}\n\nA part names no place of its own.\n"},
	} {
		page := readFile(t, filepath.Join(out, slug, "index.md"))
		for _, s := range want {
			if !strings.Contains(page, s) {
				t.Errorf("%s page lacks %q:\n%s", slug, s, page)
			}
		}
		if strings.Contains(page, "<!--") {
			t.Errorf("%s page keeps the comment of its place:\n%s", slug, page)
		}
	}
	if page := readFile(t, filepath.Join(out, "lyrics", "index.md")); strings.Contains(page, "group_first") {
		t.Errorf("lyrics page is the first of its group:\n%s", page)
	}
}

func TestPlaceErrors(t *testing.T) {
	root := repo(t, map[string]string{
		"docs/none.md":    "\n# None\n\nNo place.\n",
		"docs/group.md":   "# Group\n<!-- docs: group=elsewhere order=10 -->\n",
		"docs/order.md":   "# Order\n<!-- docs: group=using order=first -->\n",
		"docs/key.md":     "# Key\n<!-- docs: group=using weight=10 -->\n",
		"docs/nogroup.md": "# No group\n<!-- docs: order=10 -->\n",
	})
	_, err := run(opts(root))
	if err == nil {
		t.Fatal("no error")
	}
	for _, s := range []string{
		"docs/none.md:3: a doc names its place on the line after its title, as <!-- docs: group=using order=30 -->, in one of the groups start, using, inside, make, project",
		"docs/group.md:2: the group \"elsewhere\" isn't one of start, using, inside, make, project",
		"docs/order.md:2: the order \"first\" isn't a whole number",
		"docs/key.md:2: the doc's place has group and order, not \"weight\"",
		"docs/nogroup.md:2: the doc's place names no group",
	} {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("the error lacks %q:\n%v", s, err)
		}
	}
}

// TestRoot makes pages of the root's Markdown files that name their place,
// and only of those, and links docs and those pages both ways.
func TestRoot(t *testing.T) {
	root := repo(t, map[string]string{
		"CONTRIBUTING.md":    "# Contributing\n<!-- docs: group=project order=10 -->\n\nThank you for helping. Read [themes](docs/themes.md), [the config](site/zola.toml), [the conduct](CODE_OF_CONDUCT.md) and [below](#commits).\n\n## Commits\n",
		"CODE_OF_CONDUCT.md": "# Code of conduct\n<!-- docs: group=project order=40 -->\n\nBe kind.\n",
		"README.md":          "# MixMusic\n\nThe README is no page.\n",
		"CHANGELOG.md":       "<!-- docs: group=project -->\n",
		"NOTES.md":           "# Notes\n\nText, then <!-- docs: group=project --> too late.\n",
		"docs/themes.md":     "# Themes\n<!-- docs: group=make order=10 -->\n\nSee [contributing](../CONTRIBUTING.md#commits) and <a href=\"../CODE_OF_CONDUCT.md\">the code of conduct</a>.\n",
		"site/zola.toml":     "",
	})
	docs, err := run(opts(root))
	if err != nil {
		t.Fatal(err)
	}
	if got := slugs(docs); got != "themes contributing code-of-conduct" {
		t.Errorf("pages: %s", got)
	}
	out := filepath.Join(root, "site", "content", "docs")
	contributing := readFile(t, filepath.Join(out, "contributing", "index.md"))
	for _, s := range []string{
		marker + "CONTRIBUTING.md: edit that file, not this one.\n",
		"source = \"CONTRIBUTING.md\"\n",
		"description = \"Thank you for helping. Read themes, the config, the conduct and below.\"\n",
		"Read [themes](@/docs/themes/index.md), [the config](https://github.com/owner/name/blob/main/site/zola.toml), [the conduct](@/docs/code-of-conduct/index.md) and [below](#commits).\n",
	} {
		if !strings.Contains(contributing, s) {
			t.Errorf("contributing page lacks %q:\n%s", s, contributing)
		}
	}
	themes := readFile(t, filepath.Join(out, "themes", "index.md"))
	if s := "See [contributing](@/docs/contributing/index.md#commits) and <a href=\"../code-of-conduct/\">the code of conduct</a>.\n"; !strings.Contains(themes, s) {
		t.Errorf("themes page lacks %q:\n%s", s, themes)
	}
	for _, slug := range []string{"readme", "changelog", "notes"} {
		if _, err := os.Stat(filepath.Join(out, slug)); !os.IsNotExist(err) {
			t.Errorf("%s has a page: %v", slug, err)
		}
	}
}

// TestImages gives the size of each image beside a page, by its path as
// the page's links have it.
func TestImages(t *testing.T) {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatal(err)
	}
	root := repo(t, map[string]string{
		"docs/look.md":             "# Look\n<!-- docs: group=using order=50 -->\n\n![The home page](shots/home page.png) and <img src=\"shots/home page.png\" alt=\"\"> and [notes](notes.txt).\n",
		"docs/shots/home page.png": b.String(),
		"docs/notes.txt":           "not an image",
	})
	if _, err := run(opts(root)); err != nil {
		t.Fatal(err)
	}
	page := readFile(t, filepath.Join(root, "site", "content", "docs", "look", "index.md"))
	if s := "group = \"using\"\ngroup_title = \"Using MixMusic\"\ngroup_first = true\n\n[extra.images]\n\"shots/home%20page.png\" = [3, 2]\n+++\n"; !strings.Contains(page, s) {
		t.Errorf("look page lacks %q:\n%s", s, page)
	}
	if strings.Contains(page, "notes.txt\" =") {
		t.Errorf("look page gives a size of a file that isn't an image:\n%s", page)
	}
}

func TestAnchor(t *testing.T) {
	for name, want := range map[string]string{"player": "player", "Audio engine": "audio-engine", "a--b_c.d": "a-b_c-d", "-x-": "x"} {
		if got := anchor(name); got != want {
			t.Errorf("anchor(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestBrokenLinks(t *testing.T) {
	root := repo(t, map[string]string{
		"docs/a.md": "# A\n<!-- docs: group=start -->\n\nText.\n\n[gone](gone.md) and ![gone](shots/gone.png)\n\n[out](../../elsewhere.md)\n",
	})
	_, err := run(opts(root))
	if err == nil {
		t.Fatal("no error")
	}
	for _, s := range []string{"docs/a.md:6: gone.md: not in the repository", "docs/a.md:6: shots/gone.png: not in the repository", "docs/a.md:8: ../../elsewhere.md: outside the repository"} {
		if !strings.Contains(err.Error(), s) {
			t.Errorf("the error lacks %q:\n%v", s, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "site")); !os.IsNotExist(err) {
		t.Errorf("pages were written: %v", err)
	}
}

func TestNoTitle(t *testing.T) {
	root := repo(t, map[string]string{"docs/a.md": "\nText first.\n"})
	if _, err := run(opts(root)); err == nil || !strings.Contains(err.Error(), "docs/a.md:2: a doc starts with its title") {
		t.Errorf("error: %v", err)
	}
}

func TestSummary(t *testing.T) {
	long := strings.Repeat("word ", 40) + "end."
	for body, want := range map[string]string{
		"Short. Then a longer sentence that takes the summary past sixty characters. And more.": "Short. Then a longer sentence that takes the summary past sixty characters.",
		"```\ncode\n```\n\n![x](x.png)\n\n| a | b |\n\nA [linked](x.md) **bold** `code` word.":  "A linked bold code word.",
		"1. a step\n\n> a quote\n\n": "",
		// Search engines show some 160 characters: a longer first sentence
		// is cut at a word.
		long: strings.TrimSpace(strings.Repeat("word ", 32)) + "…",
	} {
		got := summary(body)
		if got != want {
			t.Errorf("summary(%q) = %q, want %q", body, got, want)
		}
		if n := len([]rune(got)); n > description {
			t.Errorf("summary(%q) has %d characters", body, n)
		}
	}
}

func TestTOMLString(t *testing.T) {
	if got, want := tomlString("a \"b\" \\ c\td"), `"a \"b\" \\ c\u0009d"`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

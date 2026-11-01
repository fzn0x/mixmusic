package main

import (
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

const plugins = "# Plugins\r\n" +
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
		"docs/themes.md":                   "# Themes `json`\n\nA theme sets the colors, the typefaces and the corners of MixMusic, which draws everything with them. A theme is a folder.\n\n## Colors\n",
		"docs/zebra.md":                    "# Zebra\n\n- a list first\n\nThen prose: it ends in a colon:\n",
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
	var slugs []string
	for _, d := range docs {
		slugs = append(slugs, d.slug)
	}
	if got := strings.Join(slugs, " "); got != "themes plugins zebra" {
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
	for _, s := range []string{"title = \"Themes json\"\n", "description = \"A theme sets the colors, the typefaces and the corners of MixMusic, which draws everything with them.\"\n", "weight = 10\n"} {
		if !strings.Contains(themes, s) {
			t.Errorf("themes page lacks %q:\n%s", s, themes)
		}
	}
	if zebra := readFile(t, filepath.Join(out, "zebra", "index.md")); !strings.Contains(zebra, "description = \"Then prose: it ends in a colon.\"\n") {
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

func TestBrokenLinks(t *testing.T) {
	root := repo(t, map[string]string{
		"docs/a.md": "# A\n\nText.\n\n[gone](gone.md) and ![gone](shots/gone.png)\n\n[out](../../elsewhere.md)\n",
	})
	_, err := run(opts(root))
	if err == nil {
		t.Fatal("no error")
	}
	for _, s := range []string{"docs/a.md:5: gone.md: not in the repository", "docs/a.md:5: shots/gone.png: not in the repository", "docs/a.md:7: ../../elsewhere.md: outside the repository"} {
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
	for body, want := range map[string]string{
		"Short. Then a longer sentence that takes the summary past sixty characters. And more.": "Short. Then a longer sentence that takes the summary past sixty characters.",
		"```\ncode\n```\n\n![x](x.png)\n\n| a | b |\n\nA [linked](x.md) **bold** `code` word.":  "A linked bold code word.",
		"1. a step\n\n> a quote\n\n": "",
	} {
		if got := summary(body); got != want {
			t.Errorf("summary(%q) = %q, want %q", body, got, want)
		}
	}
}

func TestTOMLString(t *testing.T) {
	if got, want := tomlString("a \"b\" \\ c\td"), `"a \"b\" \\ c\u0009d"`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

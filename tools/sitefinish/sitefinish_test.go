package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// testPage is a landing page in small: each kind of piece, the parts that
// aren't translated, and what finish changes.
const testPage = `<!doctype html>
<html lang="en" class="no-js">
<head>
<meta charset="utf-8" />
<title>MixMusic: one player</title>
<meta name="description" content="A free desktop player." />
<meta property="og:locale" content="en_US" />
<meta property="og:url" content="https://example.test/" />
<link rel="canonical" href="https://example.test/" />
<link rel="icon" href="assets/icon.png" />
<link rel="stylesheet" href="styles.css" />
<script type="application/json" id="site-text">{"download_for": "Download for {os}", "light": "light"}</script>
<script type="application/ld+json">{"@context": "https://schema.org", "@graph": [
 {"@type": "SoftwareApplication", "name": "MixMusic", "description": "A free player."},
 {"@type": "WebSite", "name": "MixMusic", "url": "https://example.test/", "inLanguage": "en"}]}</script>
<script type="application/ld+json">{"@type": "FAQPage", "url": "https://example.test/", "inLanguage": "en", "mainEntity": [
 {"@type": "Question", "name": "Is it free?", "acceptedAnswer": {"@type": "Answer", "text": "Yes, <code>always</code>."}}]}</script>
<script>const notWords = "Not a unit";</script>
</head>
<body>
<a class="skip" href="#features">Skip to the features</a>
<nav aria-label="Sections"> <a href="#features">Features</a> <a href="/docs/">Docs</a> </nav>
<details class="lang-menu"><summary aria-label="Language"></summary><ul translate="no" data-languages></ul></details>
<h1 id="hero">One <strong>player</strong> for both.</h1>
<h3><svg class="icon"><use href="#i-star"></use></svg>Podcasts</h3>
<a class="btn" href="https://example.test/release"><svg><use href="#i-download"></use></svg><span>Download</span></a>
<p>Press <kbd>Space</kbd> to play.</p>
<dl><dt><kbd>Ctrl</kbd><kbd>K</kbd></dt><dd>Command palette</dd></dl>
<div aria-hidden="true"><p>Decoration</p></div>
<div class="app" aria-label="A working copy" translate="no"><p>Up next</p><button aria-label="Play"></button></div>
<img src="assets/shot.webp" srcset="assets/shot-960.webp 960w, https://cdn.test/x.webp 2x" alt="A screenshot" />
<noscript><img src="assets/still.webp" alt="A still" /></noscript>
<p>0:00 · 3:12</p>
<footer><nav aria-label="Languages"><ul translate="no" data-languages></ul></nav></footer>
</body>
</html>`

// TestExtract finds every piece of words, each once, as html or text, and
// none of code, decoration, the parts kept as they are or a key's name.
func TestExtract(t *testing.T) {
	doc, err := parse([]byte(testPage))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, u := range extract(doc) {
		if _, dup := got[u.Text]; dup {
			t.Errorf("%q twice", u.Text)
		}
		got[u.Text] = u.Kind
	}
	want := map[string]string{
		"MixMusic: one player":                  "text",
		"A free desktop player.":                "text",
		"Download for {os}":                     "text",
		"light":                                 "text",
		"A free player.":                        "text",
		"Is it free?":                           "text",
		"Yes, <code>always</code>.":             "text",
		"Skip to the features":                  "text",
		"Sections":                              "text",
		"Features":                              "text",
		"Docs":                                  "text",
		"Language":                              "text",
		"One <strong>player</strong> for both.": "html",
		"Podcasts":                              "text",
		"Download":                              "text",
		"Press <kbd>Space</kbd> to play.":       "html",
		"Command palette":                       "text",
		"A working copy":                        "text",
		"A screenshot":                          "text",
		"A still":                               "text",
		"Languages":                             "text",
	}
	for text, kind := range want {
		if got[text] != kind {
			t.Errorf("%q: kind %q, want %q", text, got[text], kind)
		}
	}
	for text := range got {
		if _, ok := want[text]; !ok {
			t.Errorf("a piece that isn't one: %q", text)
		}
	}
}

// testTable translates testPage into a made-up language, all but "Docs".
var testTable = map[string]string{
	"MixMusic: one player":                  "MixMusic: ein Spieler",
	"A free desktop player.":                "Ein freier Spieler.",
	"Download for {os}":                     "Laden für {os}",
	"light":                                 "hell",
	"A free player.":                        "Ein freier.",
	"Is it free?":                           "Ist es frei?",
	"Yes, <code>always</code>.":             "Ja, <code>immer</code>.",
	"Skip to the features":                  "Zu den Funktionen",
	"Sections":                              "Teile",
	"Features":                              "Funktionen",
	"Language":                              "Sprache",
	"One <strong>player</strong> for both.": "Ein <strong>Spieler</strong> für beide.",
	"Podcasts":                              "Sendungen",
	"Download":                              "Laden",
	"Press <kbd>Space</kbd> to play.":       "Drücke <kbd>Space</kbd>.",
	"Command palette":                       "Befehle",
	"A working copy":                        "Eine Kopie",
	"A screenshot":                          "Ein Bild",
	"A still":                               "Ein Standbild",
	"Languages":                             "Sprachen",
}

// TestPage makes a language's page: its words translated, its lang and
// dir, its relative addresses a folder up, its canonical address and
// locale, the alternates, the menus, the structured data, and the English
// of what it has no translation for.
func TestPage(t *testing.T) {
	ur := language{Dir: "ur", Tag: "ur", Name: "اردو", Locale: "ur_PK", RTL: true}
	out, missing, err := page([]byte(testPage), ur, testTable, "https://example.test")
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{
		`<html lang="ur" class="no-js" dir="rtl" data-root="../">`,
		`<title>MixMusic: ein Spieler</title>`,
		`content="Ein freier Spieler."`,
		`<link rel="canonical" href="https://example.test/ur/"/>`,
		`<meta property="og:url" content="https://example.test/ur/"/>`,
		`<meta property="og:locale" content="ur_PK"/>`,
		`<meta property="og:locale:alternate" content="en_US"/>`,
		`<link rel="alternate" hreflang="hi" href="https://example.test/hi/"/>`,
		`<link rel="alternate" hreflang="x-default" href="https://example.test/"/>`,
		`<link rel="icon" href="../assets/icon.png"/>`,
		`href="../styles.css"`,
		`src="../assets/shot.webp" srcset="../assets/shot-960.webp 960w, https://cdn.test/x.webp 2x" alt="Ein Bild"`,
		`<img src="../assets/still.webp" alt="Ein Standbild"/>`,
		`<a class="skip" href="#features">Zu den Funktionen</a>`,
		`<a href="#features">Funktionen</a>`,
		`<a href="/docs/">Docs</a>`,
		`data-en="One &lt;strong&gt;player&lt;/strong&gt; for both.">Ein <strong>Spieler</strong> für beide.</h1>`,
		`<h3 data-en="Podcasts"><svg class="icon"><use href="#i-star"></use></svg>Sendungen</h3>`,
		`<span>Laden</span>`,
		`<p>Drücke <kbd>Space</kbd>.</p>`,
		`<dt><kbd>Ctrl</kbd><kbd>K</kbd></dt><dd>Befehle</dd>`,
		`<p>Decoration</p>`,
		`aria-label="Eine Kopie" translate="no"><p>Up next</p><button aria-label="Play">`,
		`"Laden für {os}"`,
		`"Ist es frei?"`,
		`"Ja, \u003ccode\u003eimmer\u003c/code\u003e."`,
		`"inLanguage":"ur"`,
		`"url":"https://example.test/ur/"`,
		`<li><a href="../" hreflang="en" lang="en">English</a></li>`,
		`<li><a href="../ur/" hreflang="ur" lang="ur" dir="rtl" aria-current="page">اردو</a></li>`,
		`const notWords = "Not a unit";`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("the page lacks %s", want)
		}
	}
	if strings.Count(s, `rel="alternate" hreflang=`) != len(languages)+1 {
		t.Errorf("%d alternates, want %d", strings.Count(s, `rel="alternate" hreflang=`), len(languages)+1)
	}
	if strings.Count(s, `<li><a href=`) != 2*len(languages) {
		t.Errorf("%d links in the menus, want %d", strings.Count(s, `<li><a href=`), 2*len(languages))
	}
	if !slices.Equal(missing, []string{"Docs"}) {
		t.Errorf("missing %q, want Docs", missing)
	}
}

// TestEnglishPage finishes the English page without changing its words or
// addresses, and a page finished twice is the same.
func TestEnglishPage(t *testing.T) {
	en := languages[0]
	once, missing, err := page([]byte(testPage), en, nil, "https://example.test/")
	if err != nil || len(missing) > 0 {
		t.Fatal(err, missing)
	}
	twice, _, err := page(once, en, nil, "https://example.test/")
	if err != nil {
		t.Fatal(err)
	}
	if string(once) != string(twice) {
		t.Errorf("finished twice, the page changed:\n%s\n---\n%s", once, twice)
	}
	s := string(once)
	for _, want := range []string{
		`<html lang="en" class="no-js">`,
		`<link rel="icon" href="assets/icon.png"/>`,
		`<link rel="canonical" href="https://example.test/"/>`,
		`<li><a href="./" hreflang="en" lang="en" aria-current="page">English</a></li>`,
		`<li><a href="hi/" hreflang="hi" lang="hi">हिन्दी</a></li>`,
		`<h3><svg class="icon"><use href="#i-star"></use></svg>Podcasts</h3>`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("the page lacks %s", want)
		}
	}
	if strings.Contains(s, "data-root") || strings.Contains(s, `dir="rtl" data`) {
		t.Error("the English page has a root or a direction")
	}
}

// TestSitemap adds each language's landing page, and the alternates of the
// landing pages, keeping Zola's pages and their dates.
func TestSitemap(t *testing.T) {
	src := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
    <url>
        <loc>https://example.test/</loc>
    </url>
    <url>
        <loc>https://example.test/docs/lyrics/</loc>
        <lastmod>2026-11-01</lastmod>
    </url>
</urlset>`
	out, err := sitemap([]byte(src), "https://example.test")
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if n := strings.Count(s, "<url>"); n != 2+len(languages)-1 {
		t.Errorf("%d pages, want %d", n, 2+len(languages)-1)
	}
	for _, want := range []string{
		`xmlns:xhtml="http://www.w3.org/1999/xhtml"`,
		"<loc>https://example.test/docs/lyrics/</loc>\n    <lastmod>2026-11-01</lastmod>\n  </url>",
		"<loc>https://example.test/zh-cn/</loc>",
		`<xhtml:link rel="alternate" hreflang="zh-Hans" href="https://example.test/zh-cn/" />`,
		`<xhtml:link rel="alternate" hreflang="x-default" href="https://example.test/" />`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("the sitemap lacks %s", want)
		}
	}
	// Each landing page names every language and the default: the docs'
	// page none.
	if n := strings.Count(s, "<xhtml:link"); n != len(languages)*(len(languages)+1) {
		t.Errorf("%d alternates, want %d", n, len(languages)*(len(languages)+1))
	}
}

// TestLLMS lists the docs by group, in their order, with their
// descriptions, and puts each in full in llms-full.txt.
func TestLLMS(t *testing.T) {
	dir := t.TempDir()
	write := func(name, front, body string) {
		p := filepath.Join(dir, name, "index.md")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("+++\n"+front+"+++\n\n"+body+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("install", "title = \"Install\"\ndescription = \"Get MixMusic.\"\nweight = 10\n\n[extra]\ngroup_title = \"Getting started\"\n", "Download it.")
	write("lyrics", "title = \"Lyrics\"\ndescription = \"Synced \\\"lyrics\\\".\"\nweight = 200\n\n[extra]\ngroup_title = \"Using MixMusic\"\n", "Lyrics follow the song.")
	write("accounts", "title = \"Accounts\"\nweight = 20\n\n[extra]\ngroup_title = \"Getting started\"\n", "Sign in.")
	pages, err := readDocs(dir)
	if err != nil {
		t.Fatal(err)
	}
	index, full := llms(pages, "https://example.test", "One player.")
	for _, want := range []string{
		"# MixMusic\n\n> One player.\n",
		"## Getting started\n\n- [Install](https://example.test/docs/install/): Get MixMusic.\n- [Accounts](https://example.test/docs/accounts/)\n\n## Using MixMusic\n\n- [Lyrics](https://example.test/docs/lyrics/): Synced \"lyrics\".\n\n## Languages",
		"- [Русский](https://example.test/ru/)",
		"- [All the docs in one file](https://example.test/llms-full.txt)",
	} {
		if !strings.Contains(index, want) {
			t.Errorf("llms.txt lacks %q:\n%s", want, index)
		}
	}
	for _, want := range []string{"# Install\n\nSource: https://example.test/docs/install/\n\nDownload it.", "# Lyrics\n\nSource: https://example.test/docs/lyrics/\n\nLyrics follow the song."} {
		if !strings.Contains(full, want) {
			t.Errorf("llms-full.txt lacks %q", want)
		}
	}
}

// TestTables checks the translations in site/i18n: each language's file
// is JSON of the English pieces of strings.json, keeps a piece's tags, as
// <strong> or <code>, and its placeholders, as {os}.
func TestTables(t *testing.T) {
	dir := filepath.Join("..", "..", "site", "i18n")
	src, err := os.ReadFile(filepath.Join(dir, "strings.json"))
	if os.IsNotExist(err) {
		t.Skip("no site/i18n/strings.json")
	}
	if err != nil {
		t.Fatal(err)
	}
	var units []unit
	if err := json.Unmarshal(src, &units); err != nil {
		t.Fatal(err)
	}
	english := map[string]bool{}
	for _, u := range units {
		english[u.Text] = true
	}
	for _, l := range languages[1:] {
		table, err := readTable(filepath.Join(dir, l.Dir+".json"))
		if err != nil {
			t.Errorf("%s: %v", l.Tag, err)
			continue
		}
		for en, tr := range table {
			if !english[en] {
				t.Errorf("%s: a translation of a piece the page doesn't have: %q", l.Tag, en)
				continue
			}
			if a, b := marks(en), marks(tr); !slices.Equal(a, b) {
				t.Errorf("%s: %q keeps %q, not %q", l.Tag, tr, b, a)
			}
		}
	}
}

// marks returns the tags and placeholders of a piece, in order of their
// kind, as a translation must keep them.
func marks(s string) []string {
	var out []string
	for _, m := range markPattern.FindAllString(s, -1) {
		if strings.HasPrefix(m, "<") {
			// A tag's name and attributes, not where it moved.
			m = strings.Fields(strings.Trim(m, "</>"))[0]
		}
		out = append(out, m)
	}
	slices.Sort(out)
	return out
}

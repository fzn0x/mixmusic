package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// testOverview is the docs' overview in small: its drawer, with a group's
// label, the overview's own link and a page listed twice.
const testOverview = `<!doctype html><html><body>
<nav class="drawer" id="docs-drawer">
<ul class="drawer-list"><li><a class="drawer-item" href="https://example.test/mixmusic/docs/">Overview</a></li></ul>
<p class="drawer-label">Using MixMusic</p>
<ul class="drawer-list">
<li><a class="drawer-item" href="https://example.test/mixmusic/docs/playing-music/">Playing music</a></li>
<li><a class="drawer-item" href="https://example.test/mixmusic/docs/lyrics/">Lyrics</a></li>
<li><a class="drawer-item" href="https://example.test/mixmusic/docs/lyrics/#romaji">Lyrics again</a></li>
</ul>
</nav>
<main><h1>MixMusic docs</h1></main>
</body></html>`

// testDoc is a docs page as Zola builds it: the title, words before the
// first heading, headings whose words are in their link to themselves, a
// heading without an id, code, a table and what isn't words.
const testDoc = `<!doctype html><html><body>
<nav class="drawer"><a class="drawer-item" href="https://example.test/mixmusic/docs/lyrics/">Not the page</a></nav>
<article class="prose">
<h1>Playing music</h1>
<p>Search both services <em>at once</em>.<br>Then play.</p>
<h2 id="crossfade"><a class="anchor" href="#crossfade">Crossfade</a></h2>
<p>Fades one track into the next.</p>
<ul><li>Settings › Playback</li><li>Up to <code>12</code> seconds</li></ul>
<h3>No anchor here</h3>
<p>Still the crossfade's words.</p>
<h3 id="gapless"><a class="anchor" href="#gapless">Gapless</a></h3>
<table><tr><td>Album</td><td>plays through</td></tr></table>
<img alt="A picture" src="x.webp"><svg><text>drawn</text></svg>
<script>notWords()</script>
</article>
</body></html>`

const testLyrics = `<!doctype html><html><body><article class="prose">
<h1>Lyrics</h1>
<h2 id="romaji"><a class="anchor" href="#romaji">Romaji</a></h2>
<p>Japanese in Latin letters.</p>
</article></body></html>`

// TestSearch indexes the docs as built: the pages in the drawer's order,
// each once, and their sections at the headings with an id.
func TestSearch(t *testing.T) {
	public := t.TempDir()
	write := func(path, src string) {
		t.Helper()
		path = filepath.Join(public, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("docs/index.html", testOverview)
	write("docs/playing-music/index.html", testDoc)
	write("docs/lyrics/index.html", testLyrics)
	pages, sections, err := writeSearch(public)
	if err != nil {
		t.Fatal(err)
	}
	if pages != 2 || sections != 4 {
		t.Errorf("%d pages, %d sections; want 2 and 4", pages, sections)
	}
	got, err := searchDocs(filepath.Join(public, "docs"))
	if err != nil {
		t.Fatal(err)
	}
	want := searchIndex{
		Pages: []searchPage{
			{Title: "Playing music", Group: "Using MixMusic", Path: "playing-music/"},
			{Title: "Lyrics", Group: "Using MixMusic", Path: "lyrics/"},
		},
		// The intro of Lyrics has no words, and isn't a section.
		Sections: []searchSection{
			{Page: 0, Text: "Search both services at once. Then play."},
			{Page: 0, Heading: "Crossfade", Anchor: "crossfade", Text: "Fades one track into the next. Settings › Playback Up to 12 seconds No anchor here Still the crossfade's words."},
			{Page: 0, Heading: "Gapless", Anchor: "gapless", Text: "Album plays through"},
			{Page: 1, Heading: "Romaji", Anchor: "romaji", Text: "Japanese in Latin letters."},
		},
	}
	if !reflect.DeepEqual(got.Pages, want.Pages) {
		t.Errorf("pages:\n got %+v\nwant %+v", got.Pages, want.Pages)
	}
	if !reflect.DeepEqual(got.Sections, want.Sections) {
		t.Errorf("sections:\n got %+v\nwant %+v", got.Sections, want.Sections)
	}
	if _, err := os.Stat(filepath.Join(public, "docs", "search.json")); err != nil {
		t.Error(err)
	}
}

// TestSearchNoDocs fails when the docs weren't built, rather than write an
// empty index.
func TestSearchNoDocs(t *testing.T) {
	if _, _, err := writeSearch(t.TempDir()); err == nil {
		t.Error("no error without the docs")
	}
}

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/i18n"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
)

// pseudoPages are the pages TestPseudoLocale reads under en-XA.
var pseudoPages = []string{"/", "/library", "/library/liked", "/now-playing", "/search?q=midnight", "/explore", "/stats", "/marketplace",
	"/settings/appearance", "/settings/playback", "/settings/accounts", "/settings/plugins", "/settings/desktop", "/settings/shortcuts", "/settings/about"}

// english is a text of two English words or more: the pseudo locale accents
// and brackets every message of the catalogs, so one without brackets is
// written in the code. Names of one word (Spotify, Last.fm, ⌘K) pass.
var english = regexp.MustCompile(`[A-Za-z]{2,} [A-Za-z]{2,}`)

// pseudoWritten are texts that are names in every language: the themes'
// and services'.
var pseudoWritten = map[string]bool{"Walnut Light": true, "Material Dark": true, "YouTube Music": true, "youtube music": true,
	"Better Lyrics": true, "CC BY-NC-SA 4.0": true}

// usePseudoLocale has the app say its messages in en-XA until the test
// ends.
func usePseudoLocale(t *testing.T) {
	t.Helper()
	// MyGo's own function is unexported, and App.Locale waits for a main
	// loop the tests don't run: the tests after say what MYGO_LOCALE says.
	was := i18n.Locale()
	i18n.SetLocaleFunc(func() string { return "en-XA" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return was }) })
}

// checkPseudo fails on each of the texts where shows under en-XA that no
// catalog translates, or that is a sentence of pieces. The names of songs,
// artists and albums are as their services tell them, in every language.
func checkPseudo(t *testing.T, where string, texts []string, names ...string) {
	t.Helper()
	xa := locales.In("en-XA")
	// Zoom's description puts whole sentences side by side: what zoom
	// does, its keys and its bounds, each there or not.
	pieced := []string{xa.MenuZoomDescription()}
	for _, txt := range texts {
		words := txt
		for _, n := range names {
			words = strings.ReplaceAll(words, n, "")
		}
		// A request's error, to the tests' closed port, is Go's.
		if english.MatchString(words) && !strings.ContainsAny(txt, "[]") && !strings.Contains(txt, "://") && !pseudoWritten[txt] {
			t.Errorf("%s shows %q, which no catalog translates", where, txt)
		}
		if groups(txt) > 1 && !startsWithAny(txt, pieced) {
			t.Errorf("%s shows %q, a sentence of pieces: make it one message, with arguments", where, txt)
		}
	}
}

// pseudoShell is a test shell with a history: plays of today from both
// services, which Stats' headline counts, and recapPlays, whose past year
// Your year looks back at. It returns the names of the history's songs,
// artists and albums.
func pseudoShell(t *testing.T) (*shell, []string) {
	t.Helper()
	now := time.Now()
	plays := []library.Play{
		{Track: testTrack("sp:1", "Midnight City", "M83", "Hurry Up, We're Dreaming", media.SourceSpotify), PlayedAt: now},
		{Track: testTrack("yt:1", "Glósóli", "Sigur Rós", "Takk...", media.SourceYouTube), PlayedAt: now.Add(-time.Minute)},
	}
	for i := range plays {
		plays[i].Track.DurationMs = 4 * 60000
	}
	plays = append(plays, recapPlays()...)
	var names []string
	for _, p := range plays {
		names = append(names, p.Track.Title, p.Track.Album.Name)
		for _, a := range p.Track.Artists {
			names = append(names, a.Name)
		}
	}
	return newTestShellOf(t, func(o *app.Options) {
		data, err := json.Marshal(library.Data{History: plays})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(o.DataDir, "library.json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}), names
}

// TestPseudoLocale shows the app's pages in the pseudo locale, en-XA, which
// MyGo makes of English: each message accented, a third longer, and in
// brackets. A text of English words is written in the code, not the
// catalogs, and a text of brackets side by side is a sentence made of
// pieces, whose order a translation can't change. Stats has plays, and
// Your year a year to look back at, so that their headlines show. Words
// cut short don't show in the texts: MIXMUSIC_RENDER gets each page, for a
// look.
func TestPseudoLocale(t *testing.T) {
	usePseudoLocale(t)
	s, names := pseudoShell(t)
	// Settings › Appearance shows the options of the theme worn alone.
	for _, th := range s.inv.Themes {
		for _, o := range th.Options {
			paths := map[string]string{o.ID: o.Label}
			for _, ch := range o.Choices {
				paths[o.ID+"/"+ch.Value] = ch.Label
			}
			for path, label := range paths {
				if got := optionLabel(th, path, label); th.Builtin && !strings.HasPrefix(got, "[") {
					t.Errorf("the option %s of the theme %s says %q, which no catalog translates", path, th.ID, got)
				}
			}
		}
	}
	tt := tester(t, s)
	tt.SetLocale("en-XA")
	dir := os.Getenv("MIXMUSIC_RENDER")
	xa := locales.In("en-XA")
	for _, p := range pseudoPages {
		show(s, tt, p)
		if dir != "" {
			renderTo(t, s, tt, dir, "en-XA"+strings.ReplaceAll(strings.SplitN(p, "?", 2)[0], "/", "-"))
		}
		checkPseudo(t, p, tt.Texts(), names...)
		if p == "/stats" && !hasTextStarting(tt.Texts(), xa.StatsHeadline("", "", "", "")) {
			t.Errorf("Stats shows no headline: %q", tt.Texts())
		}
	}
	// Stats' last tab, Your year, looks back at the past year.
	s.statsRange = len(statsRanges)
	show(s, tt, "/stats")
	if dir != "" {
		renderTo(t, s, tt, dir, "en-XA-stats-your-year")
	}
	checkPseudo(t, "Your year", tt.Texts(), names...)
	if !hasTextStarting(tt.Texts(), xa.RecapHeadline(2025, "", "")) {
		t.Errorf("Your year shows no headline: %q", tt.Texts())
	}
}

// hasTextStarting tells whether one of texts starts as msg, said with
// empty placeholders, does: up to its first placeholder.
func hasTextStarting(texts []string, msg string) bool {
	start, _, _ := strings.Cut(msg, "  ")
	for _, txt := range texts {
		if strings.HasPrefix(txt, start) {
			return true
		}
	}
	return false
}

// groups counts the outermost brackets of txt: a message's arguments nest
// inside it, but the messages of a sentence of pieces stand side by side.
func groups(txt string) (n int) {
	depth := 0
	for _, r := range txt {
		switch r {
		case '[':
			if depth == 0 {
				n++
			}
			depth++
		case ']':
			depth = max(depth-1, 0)
		}
	}
	return n
}

func startsWithAny(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

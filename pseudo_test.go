package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/egoist/mygo/i18n"

	"github.com/fzn0x/mixmusic/locales"
)

// pseudoPages are the pages TestPseudoLocale reads under en-XA.
var pseudoPages = []string{"/", "/library", "/library/liked", "/now-playing", "/search?q=midnight", "/explore", "/stats", "/marketplace",
	"/settings/appearance", "/settings/playback", "/settings/accounts", "/settings/plugins", "/settings/desktop", "/settings/shortcuts", "/settings/about"}

// english is a text of two English words or more: the pseudo locale accents
// and brackets every message of the catalogs, so one without brackets is
// written in the code. Names of one word (Spotify, Last.fm, ⌘K) pass.
var english = regexp.MustCompile(`[A-Za-z]{2,} [A-Za-z]{2,}`)

// TestPseudoLocale shows the app's pages in the pseudo locale, en-XA, which
// MyGo makes of English: each message accented, a third longer, and in
// brackets. A text of English words is written in the code, not the
// catalogs, and a text of brackets side by side is a sentence made of
// pieces, whose order a translation can't change. Words cut short don't
// show in the texts: MIXMUSIC_RENDER gets each page, for a look.
func TestPseudoLocale(t *testing.T) {
	// MyGo's own function is unexported, and App.Locale waits for a main
	// loop the tests don't run: the tests after say what MYGO_LOCALE says.
	was := i18n.Locale()
	i18n.SetLocaleFunc(func() string { return "en-XA" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return was }) })
	xa := locales.In("en-XA")
	// shortcut: these sentences put a link or a shortcut between their
	// pieces, which one message can't hold yet; drop each when it's one.
	pieced := []string{xa.MarketplaceCommunity(), xa.MenuZoomDescription(), xa.PlaybackSignInSpotifyLead(), xa.EqualizerSpotifyLead(),
		xa.AccountSettingsSpotifyStep1(), xa.AccountSettingsSpotifyRedirect(), xa.AccountSettingsLastfmStep1A(), xa.DesktopSettingsDiscordBefore(),
		xa.ListenBrainzSettingsStep1(), xa.DiscordPlaceholdersLabel(), xa.SkipPartsCreditBefore()}
	s := newTestShell(t)
	// The themes' and services' names are names, in every language.
	written := map[string]bool{"Walnut Light": true, "Material Dark": true, "YouTube Music": true, "youtube music": true,
		"Better Lyrics": true, "CC BY-NC-SA 4.0": true}
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
	for _, p := range pseudoPages {
		show(s, tt, p)
		if dir != "" {
			renderTo(t, s, tt, dir, "en-XA"+strings.ReplaceAll(strings.SplitN(p, "?", 2)[0], "/", "-"))
		}
		for _, txt := range tt.Texts() {
			// A request's error, to the tests' closed port, is Go's.
			if english.MatchString(txt) && !strings.ContainsAny(txt, "[]") && !strings.Contains(txt, "://") && !written[txt] {
				t.Errorf("%s shows %q, which no catalog translates", p, txt)
			}
			if groups(txt) > 1 && !startsWithAny(txt, pieced) {
				t.Errorf("%s shows %q, a sentence of pieces: make it one message, with arguments", p, txt)
			}
		}
	}
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

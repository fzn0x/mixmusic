package main

import (
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/locales"
)

// TestAboutNamesTranslators names, in About, the translators of a language
// whose file names them, joined as the language joins names, beside its
// machine's note; English, whose file names none, shows neither.
func TestAboutNamesTranslators(t *testing.T) {
	if err := addFixtureLanguage(); err != nil {
		t.Fatal(err)
	}
	s := newTestShell(t)
	tt := tester(t, s)
	show(s, tt, "/settings/about")
	// The sentence before its names: the data folder's path, which About
	// shows, holds the test's name, and so the row's title.
	if tt.HasText(strings.TrimSuffix(locales.In("en").AboutTranslatorsNames(""), ".")) {
		t.Error("About names English's translators")
	}
	inLanguage(t, tt, "qaa")
	qaa := locales.In("qaa")
	if !tt.HasText(qaa.AboutTranslators()) || !tt.HasText(qaa.AboutTranslatorsNames("Ada Lovelace & Grace Hopper")) {
		t.Fatalf("About in qaa, whose file names Ada Lovelace and Grace Hopper, says %q", tt.Texts())
	}
	if !tt.HasText(qaa.AboutTranslationMachine()) {
		t.Error("naming qaa's translators hid its machine's note")
	}
}

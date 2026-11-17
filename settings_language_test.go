package main

import (
	"testing"

	"github.com/fzn0x/mixmusic/locales"
)

// TestLanguageRow lists the app's languages, those added as it starts too,
// each named in itself: by CLDR, else by its own catalog, else by its tag.
func TestLanguageRow(t *testing.T) {
	if err := addFixtureLanguage(); err != nil {
		t.Fatal(err)
	}
	for tag, want := range map[string]string{"en": "English", "de": "Deutsch", "qaa": locales.In("qaa").LanguageName(), "qtz": "qtz"} {
		if got := languageName(tag); got != want {
			t.Errorf("%s is named %q, want %q", tag, got, want)
		}
	}
	s := newTestShell(t)
	tt := tester(t, s)
	show(s, tt, "/settings/appearance")
	click(t, s, tt, locales.LanguageSystem())
	tt.Frame()
	for _, name := range []string{"English", "qaa: Qaa"} {
		if !tt.HasText(name) {
			t.Errorf("the Language row offers %q, not %s", tt.Texts(), name)
		}
	}
}

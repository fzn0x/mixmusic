package main

import (
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

// addFixtureLanguage adds locales/testdata/qaa.json to the app's
// languages, once: a catalog a machine translated, in a language of
// private use, which no translation will be.
var addFixtureLanguage = sync.OnceValue(func() error {
	return locales.AddTranslations(os.DirFS(filepath.Join("locales", "testdata")))
})

// inLanguage shows the app's messages and tt's window in tag until the
// test ends. MyGo's own function of the app's language is unexported, so
// the tests after say what it said before.
func inLanguage(t *testing.T, tt *ui.Tester, tag string) {
	t.Helper()
	was := i18n.Locale()
	i18n.SetLocaleFunc(func() string { return tag })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return was }) })
	tt.SetLocale(tag)
	tt.Frame()
}

// TestAboutTranslatedByMachine shows, in a language a machine translated,
// About's word that it did, in that language, with the link to how to fix
// it. English shows none.
func TestAboutTranslatedByMachine(t *testing.T) {
	if err := addFixtureLanguage(); err != nil {
		t.Fatal(err)
	}
	s := newTestShell(t)
	tt := tester(t, s)
	show(s, tt, "/settings/about")
	if tt.HasText(locales.In("en").AboutTranslationMachine()) {
		t.Error("About says English was translated by machine")
	}
	inLanguage(t, tt, "qaa")
	qaa := locales.In("qaa")
	if !tt.HasText(qaa.AboutTranslation()) || !tt.HasText(qaa.AboutTranslationMachine()) {
		t.Fatalf("About in qaa, which a machine translated, says %q", tt.Texts())
	}
	click(t, s, tt, qaa.AboutTranslationFix())
	tt.Frame()
	if got := tt.OpenedURLs(); !slices.Contains(got, fixTranslation) {
		t.Errorf("the link opened %q, want %q", got, fixTranslation)
	}
}

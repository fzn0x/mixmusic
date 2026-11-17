package locales

import (
	"slices"
	"testing"
	"testing/fstest"

	"github.com/egoist/mygo/ui"
)

// TestAddedLanguagesSayMyGosWords shows MyGo's own words, the menu of a
// text field, in languages added as the app starts: the words a catalog
// gives as mygo.<key>, and the others in English, or in MyGo's own
// translation for a language MyGo has one of, as Japanese.
func TestAddedLanguagesSayMyGosWords(t *testing.T) {
	if err := addFixture(); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(Catalog.Languages(), "ja") {
		if err := AddTranslations(fstest.MapFS{"ja.json": {Data: []byte(`{"@@locale": "ja"}`)}}); err != nil {
			t.Fatal(err)
		}
	}
	for tag, want := range map[string][]string{"qaa": {"qaa: Copy", "Paste"}, "ja": {"コピー", "ペースト"}} {
		text := "hello"
		tt := ui.NewTester(func(c *ui.Context) { ui.TextInput(c, &text).Label("field") }, 300, 100)
		tt.SetLocale(tag)
		tt.Frame()
		if err := tt.RightClick("field"); err != nil {
			t.Fatal(err)
		}
		for _, w := range want {
			if menu := tt.Menu(); !slices.Contains(menu, w) {
				t.Errorf("in %s, a text field's menu says %q, not %q", tag, menu, w)
			}
		}
	}
}

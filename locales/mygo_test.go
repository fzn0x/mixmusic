package locales

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestAddedLanguagesSayMyGosWords shows MyGo's own words, the menu of a
// text field, in a language added as the app starts: the words its
// catalog gives as mygo.<key>, and the others in English.
func TestAddedLanguagesSayMyGosWords(t *testing.T) {
	if err := addFixture(); err != nil {
		t.Fatal(err)
	}
	text := "hello"
	tt := ui.NewTester(func(c *ui.Context) { ui.TextInput(c, &text).Label("field") }, 300, 100)
	tt.SetLocale("qaa")
	tt.Frame()
	if err := tt.RightClick("field"); err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"qaa: Copy", "Paste"} {
		if menu := tt.Menu(); !slices.Contains(menu, w) {
			t.Errorf("in qaa, a text field's menu says %q, not %q", menu, w)
		}
	}
}

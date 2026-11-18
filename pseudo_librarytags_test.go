package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

// TestPseudoLocaleLibraryTags shows the row of tags above Library ›
// Mixes, Tags…, Manage tags, a tag, its Choose items and Delete tag, and
// Library › Tags, in en-XA: only the names of the mixes and the tags are
// English.
func TestPseudoLocaleLibraryTags(t *testing.T) {
	usePseudoLocale(t)
	s := libraryShell(t)
	ids := makeMixes(t, s, "Night drive", "Rainy day")
	makeTag(t, s, "Evenings", "indigo", mixMark(ids, "Night drive"))
	tt := testerOf(t, s, 1280, 1400)
	tt.SetLocale("en-XA")
	show(s, tt, "/library/mixes")
	names := []string{"Night drive", "Rainy day", "Evenings"}
	checkPseudo(t, "Library › Mixes with a tag", tt.Texts(), names...)
	xa := locales.In("en-XA")
	chooseIn(t, s, tt, "Rainy day", xa.LibraryTagsItem())
	if !tt.HasText(xa.LibraryTagsItemTitle("Rainy day")) {
		t.Fatalf("Tags… doesn't show: %q", tt.Texts())
	}
	checkPseudo(t, "Tags…", tt.Texts(), names...)
	press(s, tt, 0, ui.KeyEscape)

	click(t, s, tt, xa.LibraryTagsManage())
	checkPseudo(t, "Manage tags", tt.Texts(), names...)
	click(t, s, tt, xa.LibraryTagsRow("Evenings", 1))
	checkPseudo(t, "A tag of Manage tags", tt.Texts(), names...)
	click(t, s, tt, xa.LibraryTagsChooseItems())
	if !tt.HasText(xa.LibraryTagsKindOf("mix", "")) {
		t.Errorf("Choose items doesn't say what the mixes are: %q", tt.Texts())
	}
	checkPseudo(t, "Choose items", tt.Texts(), names...)
	press(s, tt, 0, ui.KeyEscape)
	click(t, s, tt, xa.LibraryTagsDelete())
	checkPseudo(t, "Delete tag", tt.Texts(), names...)
	for range 3 {
		press(s, tt, 0, ui.KeyEscape)
	}
	if s.order.tags.managing {
		t.Fatal("Escape didn't close Manage tags")
	}

	show(s, tt, "/library/tags")
	checkPseudo(t, "Library › Tags", tt.Texts(), names...)
}

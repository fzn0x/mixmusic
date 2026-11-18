package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// libraryShell is a test shell whose sidebar is folded: the names of the
// mixes show in the library alone.
func libraryShell(t *testing.T) *shell {
	t.Helper()
	s := newTestShell(t)
	s.set.SidebarCollapsed = true
	s.save()
	return s
}

// makeMixes makes mixes of titles, the first made first, and returns
// their IDs, by title.
func makeMixes(t *testing.T, s *shell, titles ...string) map[string]string {
	t.Helper()
	ids := map[string]string{}
	for _, title := range titles {
		p, err := s.svc.Collection.CreatePlaylist(title, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		ids[title] = p.ID
	}
	return ids
}

// inReadingOrder returns those of labels whose elements show, in the
// order a reader reads them: row by row, from the left.
func inReadingOrder(tt *ui.Tester, labels ...string) []string {
	type at struct {
		label string
		x, y  float32
	}
	var shown []at
	for _, l := range labels {
		if r, ok := tt.Find(l); ok {
			shown = append(shown, at{l, r.X, r.Y})
		}
	}
	slices.SortFunc(shown, func(a, b at) int {
		switch {
		case a.y < b.y-4:
			return -1
		case a.y > b.y+4:
			return 1
		case a.x < b.x:
			return -1
		case a.x > b.x:
			return 1
		}
		return 0
	})
	out := []string{}
	for _, a := range shown {
		out = append(out, a.label)
	}
	return out
}

// mixesKept returns the titles of the mixes in the user's own order, as
// library.json, beside settings.json, keeps it.
func mixesKept(t *testing.T, s *shell) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(filepath.Dir(s.svc.Preferences.File().Path), "library.json"))
	if err != nil {
		t.Fatal(err)
	}
	var lib library.Data
	if err := json.Unmarshal(data, &lib); err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for _, p := range lib.Playlists {
		out = append(out, p.Title)
	}
	return out
}

// TestMixesSort sorts Library › Mixes by title, both ways, which
// settings.json keeps, and back in the user's own order, which a new
// install starts in.
func TestMixesSort(t *testing.T) {
	s := libraryShell(t)
	makeMixes(t, s, "Banjo", "apples", "Cello")
	tt := tester(t, s)
	show(s, tt, "/library/mixes")
	titles := []string{"Banjo", "apples", "Cello"}
	if got, want := inReadingOrder(tt, titles...), []string{"Cello", "apples", "Banjo"}; !slices.Equal(got, want) {
		t.Fatalf("in their own order, the mixes show %q, want %q", got, want)
	}
	click(t, s, tt, "Sort by")
	click(t, s, tt, "Title")
	if got, want := inReadingOrder(tt, titles...), []string{"apples", "Banjo", "Cello"}; !slices.Equal(got, want) {
		t.Errorf("by title, the mixes show %q, want %q", got, want)
	}
	click(t, s, tt, "A to Z")
	if got, want := inReadingOrder(tt, titles...), []string{"Cello", "Banjo", "apples"}; !slices.Equal(got, want) {
		t.Errorf("by title, Z to A, the mixes show %q, want %q", got, want)
	}
	if set := s.svc.Preferences.Get(); set.MixesSort != settings.MixesByTitle || !set.MixesDescending {
		t.Errorf("settings.json keeps %q, descending %v", set.MixesSort, set.MixesDescending)
	}
	// Add to mix lists them so too.
	var items []string
	for _, p := range s.svc.Collection.Playlists() {
		items = append(items, p.Title)
	}
	if !slices.Equal(items, []string{"Cello", "Banjo", "apples"}) {
		t.Errorf("Add to mix lists %q", items)
	}
	click(t, s, tt, "Sort by")
	click(t, s, tt, "Your order")
	if got, want := inReadingOrder(tt, titles...), []string{"Cello", "apples", "Banjo"}; !slices.Equal(got, want) {
		t.Errorf("back in their own order, the mixes show %q, want %q", got, want)
	}
	if tt.HasText("A to Z") || tt.HasText("Z to A") {
		t.Error("their own order shows a way to sort")
	}
}

// dragTile drags the tile labelled from onto the tile labelled onto.
func dragTile(t *testing.T, s *shell, tt *ui.Tester, from, onto string) {
	t.Helper()
	a, ok := tt.Find(from)
	b, ok2 := tt.Find(onto)
	if !ok || !ok2 {
		t.Fatalf("the window doesn't show %q and %q: %q", from, onto, tt.Texts())
	}
	tt.Press(a.X+a.W/2, a.Y+a.H/3)
	for step := float32(1); step <= 8; step++ {
		tt.Move(a.X+a.W/2+(b.X-a.X)*step/8, a.Y+a.H/3+(b.Y-a.Y)*step/8)
		tt.Frame()
	}
	tt.Release(b.X+b.W/2, b.Y+b.H/3)
	s.drain()
	tt.Frame()
}

// TestMixesDragWhileSearching searches Library › Mixes and drags a mix
// onto another of those it shows: those the search hides keep their
// places.
func TestMixesDragWhileSearching(t *testing.T) {
	s := libraryShell(t)
	makeMixes(t, s, "Rain", "Sun", "Rainbow", "Storm")
	tt := tester(t, s)
	show(s, tt, "/library/mixes")
	click(t, s, tt, "Search your mixes")
	tt.Type("rain")
	s.drain()
	tt.Frame()
	if got, want := inReadingOrder(tt, "Rain", "Sun", "Rainbow", "Storm"), []string{"Rainbow", "Rain"}; !slices.Equal(got, want) {
		t.Fatalf("searched, the mixes show %q, want %q", got, want)
	}
	if !tt.HasText("2 of 4") {
		t.Errorf("the search doesn't say how many match: %q", tt.Texts())
	}
	dragTile(t, s, tt, "Rain", "Rainbow")
	if got, want := mixesKept(t, s), []string{"Storm", "Rain", "Sun", "Rainbow"}; !slices.Equal(got, want) {
		t.Errorf("after the drag, the library keeps %q, want %q", got, want)
	}
	click(t, s, tt, "Search your mixes")
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if s.order.mixes != "" {
		t.Errorf("Escape left the search %q", s.order.mixes)
	}
	tt.Type("hail")
	s.drain()
	tt.Frame()
	if !tt.HasText("No mix matches “hail”.") {
		t.Errorf("a search without matches says nothing: %q", tt.Texts())
	}
}

// TestMixesMoveWithKeys moves the mix whose tile has the focus with Alt
// and the arrows, which keep the focus on it, while the mixes show in
// their own order, and not while they're sorted.
func TestMixesMoveWithKeys(t *testing.T) {
	s := libraryShell(t)
	makeMixes(t, s, "C", "B", "A")
	tt := tester(t, s)
	show(s, tt, "/library/mixes")
	focusOn(t, tt, "A")
	press := func(key ui.Key) {
		tt.Key(ui.Alt, key)
		s.drain()
		tt.Frame()
		tt.Frame()
	}
	press(ui.KeyRight)
	press(ui.KeyDown)
	if got, want := mixesKept(t, s), []string{"B", "C", "A"}; !slices.Equal(got, want) {
		t.Errorf("after Alt+Right and Alt+Down, the library keeps %q, want %q", got, want)
	}
	if info, _ := tt.Access("A"); !info.Focused {
		t.Error("the focus didn't follow the mix")
	}
	press(ui.KeyLeft)
	if got, want := mixesKept(t, s), []string{"B", "A", "C"}; !slices.Equal(got, want) {
		t.Errorf("after Alt+Left, the library keeps %q, want %q", got, want)
	}
	s.set.MixesSort = settings.MixesByTitle
	s.save()
	tt.Frame()
	focusOn(t, tt, "A")
	press(ui.KeyRight)
	if got, want := mixesKept(t, s), []string{"B", "A", "C"}; !slices.Equal(got, want) {
		t.Errorf("sorted by title, Alt+Right moved the mixes to %q", got)
	}
}

// TestPinMix pins a mix from its tile's menu, which leads every sort with
// a pin mark, in the library, in Add to mix and on its page, and unpins
// it.
func TestPinMix(t *testing.T) {
	s := libraryShell(t)
	ids := makeMixes(t, s, "Banjo", "apples", "Cello")
	s.set.MixesSort = settings.MixesByTitle
	s.save()
	tt := tester(t, s)
	show(s, tt, "/library/mixes")
	if err := tt.RightClick("Cello"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Pin to top"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	s.drain()
	tt.Frame()
	if got, want := inReadingOrder(tt, "Banjo", "apples", "Cello, pinned"), []string{"Cello, pinned", "apples", "Banjo"}; !slices.Equal(got, want) {
		t.Errorf("pinned, the mixes show %q, want %q", got, want)
	}
	if got := s.svc.Collection.Pins(); len(got) != 1 || got[0].ID != ids["Cello"] || got[0].Kind != library.MarkPlaylist {
		t.Errorf("the library pins %+v", got)
	}
	if got := s.svc.Collection.Playlists(); got[0].Title != "Cello" {
		t.Errorf("Add to mix lists %q first", got[0].Title)
	}
	show(s, tt, playlistPath(ids["Cello"]))
	until(t, s, tt, "the mix's page", func() bool { _, ok := tt.Find("More"); return ok })
	if err := tt.Click("More"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Unpin"); err != nil {
		t.Fatalf("%v: the page's menu holds %q", err, tt.Menu())
	}
	s.drain()
	if s.pinned(ids["Cello"]) {
		t.Error("Unpin in the page's menu left the mix pinned")
	}
}

// TestServicePlaylistsSearchSortHide searches and sorts the playlists of
// Library › YouTube Music, pins one, which leads them, and hides one,
// which waits under Hidden with Show again.
func TestServicePlaylistsSearchSortHide(t *testing.T) {
	s := libraryShell(t)
	short := media.Playlist{ID: "yt:PLshort", Source: media.SourceYouTube, Title: "Short", Owner: "Ada", TrackCount: 3, Saved: true}
	youtubeLibraryOf(s, ownPlaylist, savedPlaylist, short)
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/youtube")
	titles := []string{"Late night", "Chill", "Short"}
	if got := inReadingOrder(tt, titles...); !slices.Equal(got, titles) {
		t.Fatalf("in YouTube Music's order, the playlists show %q", got)
	}
	click(t, s, tt, "Sort by")
	click(t, s, tt, "Title")
	if got, want := inReadingOrder(tt, titles...), []string{"Chill", "Late night", "Short"}; !slices.Equal(got, want) {
		t.Errorf("by title, the playlists show %q, want %q", got, want)
	}
	click(t, s, tt, "Search your playlists")
	tt.Type("night")
	s.drain()
	tt.Frame()
	if got, want := inReadingOrder(tt, titles...), []string{"Late night"}; !slices.Equal(got, want) {
		t.Errorf("searched, the playlists show %q, want %q", got, want)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()

	if err := tt.RightClick("Short"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Pin to top"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	s.drain()
	tt.Frame()
	if got, want := inReadingOrder(tt, "Chill", "Late night", "Short, pinned"), []string{"Short, pinned", "Chill", "Late night"}; !slices.Equal(got, want) {
		t.Errorf("pinned, the playlists show %q, want %q", got, want)
	}

	if err := tt.RightClick("Chill"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Hide from library"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	s.drain()
	tt.Frame()
	if tt.HasText("Lofi Girl") {
		t.Error("the tile of the playlist hidden still shows")
	}
	if !s.svc.Collection.IsHidden(savedPlaylist.ID) || !tt.HasText(s.lk.words("Hidden")) {
		t.Errorf("Hidden doesn't list the playlist: %q", tt.Texts())
	}
	click(t, s, tt, "Show Chill again")
	tt.Frame()
	if s.svc.Collection.IsHidden(savedPlaylist.ID) || !tt.HasText("Lofi Girl") {
		t.Error("Show again leaves the playlist hidden")
	}
}

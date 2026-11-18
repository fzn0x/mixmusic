package main

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// sideShell is a test shell with the mixes of titles, the first made
// first, and the playlist Road trip of YouTube Music pinned, then the mix
// pin, if any: a window shows a page without mixes, beside the sidebar.
func sideShell(t *testing.T, pin string, titles ...string) (*shell, *ui.Tester, map[string]string) {
	t.Helper()
	s := newTestShell(t)
	ids := makeMixes(t, s, titles...)
	col := s.svc.Collection
	if err := col.SetPinned(library.Mark{ID: "yt:PLroad", Kind: library.MarkPlaylist, Title: "Road trip"}, true); err != nil {
		t.Fatal(err)
	}
	if pin != "" {
		if err := col.SetPinned(library.Mark{ID: ids[pin], Kind: library.MarkPlaylist, Title: pin}, true); err != nil {
			t.Fatal(err)
		}
	}
	// Tall enough that the sidebar's list doesn't scroll as a drag nears
	// its foot.
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/settings/about")
	return s, tt, ids
}

// sideOrder returns the entries of the sidebar's Your mixes, of labels,
// from the top down.
func sideOrder(tt *ui.Tester, labels ...string) []string {
	return byTop(tt, labels, func(l string) string { return l })
}

// dragSide drags the entry labelled from onto the entry labelled onto.
func dragSide(t *testing.T, s *shell, tt *ui.Tester, from, onto string) {
	t.Helper()
	a, ok := tt.Find(from)
	b, ok2 := tt.Find(onto)
	if !ok || !ok2 {
		t.Fatalf("the sidebar doesn't show %q and %q: %q", from, onto, tt.Texts())
	}
	x := a.X + a.W/3
	tt.Press(x, a.Y+a.H/2)
	for step := float32(1); step <= 8; step++ {
		tt.Move(x, a.Y+a.H/2+(b.Y-a.Y)*step/8)
		tt.Frame()
	}
	tt.Release(x, b.Y+b.H/2)
	s.drain()
	tt.Frame()
}

// TestSidebarPins lists Liked songs first, then the pins, of the
// services' libraries and of the mixes, with their pin marks, then the
// mixes, and moves the pins among themselves, by drag and by keys.
func TestSidebarPins(t *testing.T) {
	s, tt, _ := sideShell(t, "Alpha", "Charlie", "Bravo", "Alpha")
	liked := s.lk.words("Liked songs")
	labels := []string{liked, "Alpha, pinned", "Road trip, pinned", "Bravo", "Charlie"}
	if got := sideOrder(tt, labels...); !slices.Equal(got, labels) {
		t.Fatalf("the sidebar lists %q, want %q", got, labels)
	}
	dragSide(t, s, tt, "Road trip, pinned", "Alpha, pinned")
	if got, want := sideOrder(tt, labels...), []string{liked, "Road trip, pinned", "Alpha, pinned", "Bravo", "Charlie"}; !slices.Equal(got, want) {
		t.Errorf("a pin dragged up, the sidebar lists %q, want %q", got, want)
	}
	// A pin goes no further than the pins.
	focusOn(t, tt, "Alpha, pinned")
	tt.Key(ui.Alt, ui.KeyDown)
	s.drain()
	tt.Frame()
	if got := s.svc.Collection.Pins(); got[1].Title != "Alpha" {
		t.Errorf("a pin moved among the mixes: %+v", got)
	}
	if info, _ := tt.Access("Alpha, pinned"); !info.Focused {
		t.Error("the focus left the pin")
	}
	tt.Key(ui.Alt, ui.KeyUp)
	s.drain()
	tt.Frame()
	if got := s.svc.Collection.Pins(); got[0].Title != "Alpha" {
		t.Errorf("after Alt+Up, the pins are %+v", got)
	}
	// Unpinned from its menu, the mix goes back among the mixes.
	if err := tt.RightClick("Road trip, pinned"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Unpin"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	s.drain()
	tt.Frame()
	if tt.HasText("Road trip") {
		t.Error("the playlist unpinned still shows in the sidebar")
	}
}

// TestSidebarMixesMove drags a mix onto another in the sidebar, in their
// own order, while searching too, and not while the Library sorts them.
func TestSidebarMixesMove(t *testing.T) {
	s, tt, _ := sideShell(t, "", "Rain", "Sun", "Rainbow", "Storm")
	dragSide(t, s, tt, "Storm", "Sun")
	if got, want := mixesKept(t, s), []string{"Rainbow", "Sun", "Storm", "Rain"}; !slices.Equal(got, want) {
		t.Errorf("dragged down, the library keeps %q, want %q", got, want)
	}
	click(t, s, tt, "Search your mixes")
	tt.Type("rain")
	s.drain()
	tt.Frame()
	if got, want := sideOrder(tt, "Rain", "Sun", "Rainbow", "Storm", "Road trip, pinned"), []string{"Rainbow", "Rain"}; !slices.Equal(got, want) {
		t.Fatalf("searched, the sidebar lists %q, want %q", got, want)
	}
	dragSide(t, s, tt, "Rain", "Rainbow")
	if got, want := mixesKept(t, s), []string{"Rain", "Sun", "Storm", "Rainbow"}; !slices.Equal(got, want) {
		t.Errorf("dragged while searching, the library keeps %q, want %q", got, want)
	}
	s.order.side = ""
	s.set.MixesSort = settings.MixesByTitle
	s.save()
	tt.Frame()
	dragSide(t, s, tt, "Storm", "Rain")
	if got, want := mixesKept(t, s), []string{"Rain", "Sun", "Storm", "Rainbow"}; !slices.Equal(got, want) {
		t.Errorf("sorted by title, a drag made the library %q", got)
	}
}

// TestSidebarPlaces moves Liked songs among the pins and the mixes, by
// drag and by keys, which settings.json keeps, hides it from its menu and
// shows it again from the menu of Your mixes.
func TestSidebarPlaces(t *testing.T) {
	s, tt, ids := sideShell(t, "", "Bravo", "Alpha")
	liked := s.lk.words("Liked songs")
	labels := []string{liked, "Road trip, pinned", "Alpha", "Bravo"}
	dragSide(t, s, tt, liked, "Alpha")
	if got, want := sideOrder(tt, labels...), []string{"Road trip, pinned", "Alpha", liked, "Bravo"}; !slices.Equal(got, want) {
		t.Errorf("dragged, the sidebar lists %q, want %q", got, want)
	}
	if got := s.svc.Preferences.Get().SidebarPlaces; len(got) != 1 || got[0].Key != settings.PlaceLiked || got[0].After != "mix:"+ids["Alpha"] {
		t.Errorf("settings.json keeps %+v", got)
	}
	// Alt+Up on the mix under it takes Liked songs the other way.
	focusOn(t, tt, "Bravo")
	tt.Key(ui.Alt, ui.KeyUp)
	s.drain()
	tt.Frame()
	if got, want := sideOrder(tt, labels...), []string{"Road trip, pinned", "Alpha", "Bravo", liked}; !slices.Equal(got, want) {
		t.Errorf("after Alt+Up on B, the sidebar lists %q, want %q", got, want)
	}
	if got := mixesKept(t, s); !slices.Equal(got, []string{"Alpha", "Bravo"}) {
		t.Errorf("the mixes moved: %q", got)
	}

	if err := tt.RightClick(liked); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Hide from the sidebar"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	s.drain()
	tt.Frame()
	if tt.HasText(liked) {
		t.Error("Liked songs hidden still shows")
	}
	if err := tt.RightClick(s.lk.words("Your mixes")); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Show " + liked + " in the sidebar"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	s.drain()
	tt.Frame()
	if got, want := sideOrder(tt, labels...), []string{"Road trip, pinned", "Alpha", "Bravo", liked}; !slices.Equal(got, want) {
		t.Errorf("shown again, the sidebar lists %q, want %q", got, want)
	}
}

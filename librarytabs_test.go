package main

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

// tabOrder returns those of the Library's tabs of labels that show, from
// the start of the line.
func tabOrder(s *shell, tt *ui.Tester, labels ...string) []string {
	var words []string
	for _, l := range labels {
		words = append(words, s.lk.words(l))
	}
	return inReadingOrder(tt, words...)
}

// tabsKept returns the keys of the Library's tabs settings.json keeps,
// those hidden after a "-".
func tabsKept(s *shell) []string {
	return keysOf(s.svc.Preferences.Get().LibraryTabs)
}

// openEditTabs opens Edit tabs, from the end of the Library's tabs.
func openEditTabs(t *testing.T, s *shell, tt *ui.Tester) {
	t.Helper()
	click(t, s, tt, "Edit tabs")
	until(t, s, tt, "the rows of the tabs", func() bool { _, ok := tt.Find("Mixes"); return ok })
}

// TestEditTabs moves the Library's tabs by drag and by keys, as Edit home
// moves Home's shelves, hides one, which still opens from a link, shows
// it again and resets them.
func TestEditTabs(t *testing.T) {
	s := libraryShell(t)
	tt := tester(t, s)
	show(s, tt, "/library")
	openEditTabs(t, s, tt)
	if tt.HasText(s.lk.words("Podcasts")) {
		t.Error("the tabs show while they're edited")
	}
	dragHomeRowOf(t, s, tt, "Mixes", "Liked")
	if got, want := tabsKept(s)[:2], []string{"mixes", "liked"}; !slices.Equal(got, want) {
		t.Errorf("dragged, settings.json keeps %q, want %q first", tabsKept(s), want)
	}
	click(t, s, tt, "History")
	tt.Key(ui.Alt, ui.KeyUp)
	s.drain()
	tt.Frame()
	tt.Frame()
	if info, _ := tt.Access("History"); !info.Focused {
		t.Error("the focus didn't follow the tab")
	}
	click(t, s, tt, "Hide Podcasts")
	if got := tabsKept(s); !slices.Contains(got, "-podcasts") || slices.Index(got, "history") >= slices.Index(got, "computer") {
		t.Errorf("settings.json keeps %q", got)
	}
	click(t, s, tt, "done")
	var want []string
	for _, l := range []string{"Mixes", "Liked", "Artists", "Downloads", "History", "On this computer"} {
		want = append(want, s.lk.words(l))
	}
	if got := tabOrder(s, tt, "Liked", "Mixes", "Podcasts", "Artists", "Downloads", "On this computer", "History"); !slices.Equal(got, want) {
		t.Errorf("the Library's tabs show %q, want %q", got, want)
	}
	// A link to the tab hidden still opens it, among the tabs.
	show(s, tt, "/library/podcasts")
	if !tt.HasText(s.lk.words("Podcasts")) {
		t.Errorf("a link doesn't open the tab hidden: %q", tt.Texts())
	}

	openEditTabs(t, s, tt)
	click(t, s, tt, "Show Podcasts")
	if got := tabsKept(s); slices.Contains(got, "-podcasts") {
		t.Errorf("shown again, settings.json keeps %q", got)
	}
	click(t, s, tt, "reset tabs")
	click(t, s, tt, "reset")
	if got := s.svc.Preferences.Get().LibraryTabs; len(got) != 0 {
		t.Errorf("reset, settings.json keeps %+v", got)
	}
	click(t, s, tt, "done")
	if got, want := tabOrder(s, tt, "Liked", "Mixes", "Podcasts"), []string{s.lk.words("Liked"), s.lk.words("Mixes"), s.lk.words("Podcasts")}; !slices.Equal(got, want) {
		t.Errorf("reset, the tabs show %q, want %q", got, want)
	}
}

// dragHomeRowOf drags the row labelled from onto the row labelled onto,
// in a list the user arranges.
func dragHomeRowOf(t *testing.T, s *shell, tt *ui.Tester, from, onto string) {
	t.Helper()
	a, ok := tt.Find(from)
	b, ok2 := tt.Find(onto)
	if !ok || !ok2 {
		t.Fatalf("the list doesn't show %q and %q: %q", from, onto, tt.Texts())
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

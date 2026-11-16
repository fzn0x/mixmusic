package main

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/lyrics"
)

// lyricsSourceNames are the sources of lyrics in their own order, as
// their rows name them.
var lyricsSourceNames = []string{"Better Lyrics", "Unison", "LRCLIB", "YouTube Music", "YouTube captions"}

// lyricsSourcesShell shows Settings › Playback with the sources of lyrics
// in view.
func lyricsSourcesShell(t *testing.T) (*shell, *ui.Tester) {
	t.Helper()
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/settings/playback")
	if !reveal(tt, "Better Lyrics, on") || !reveal(tt, "YouTube captions, on") {
		t.Fatalf("Settings › Playback lists no sources of lyrics: %q", tt.Texts())
	}
	return s, tt
}

// sourceRowOrder returns the sources of lyrics as their rows show, from
// the top down, each with ", off" after it when it's off.
func sourceRowOrder(tt *ui.Tester) []string {
	var labels []string
	for _, name := range lyricsSourceNames {
		labels = append(labels, name+", on", name+", off")
	}
	return byTop(tt, labels, func(l string) string { return l })
}

// keptSources returns the keys of the sources of lyrics as settings.json
// keeps them, in their order.
func keptSources(s *shell) []string {
	var keys []string
	for _, src := range s.svc.Preferences.Get().SourceOrder() {
		k := src.Key
		if src.Off {
			k += " off"
		}
		keys = append(keys, k)
	}
	return keys
}

// dragSourceRow drags the row labelled from onto the row labelled onto.
func dragSourceRow(t *testing.T, s *shell, tt *ui.Tester, from, onto string) {
	t.Helper()
	a, ok := tt.Find(from)
	b, ok2 := tt.Find(onto)
	if !ok || !ok2 {
		t.Fatalf("no rows %q and %q: %q", from, onto, tt.Texts())
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

// clickSwitch clicks the switch named name.
func clickSwitch(t *testing.T, s *shell, tt *ui.Tester, name string) {
	t.Helper()
	a, ok := tt.AccessByRole(ui.RoleSwitch, name)
	if !ok {
		t.Fatalf("no switch %q: %q", name, tt.Texts())
	}
	tt.ClickAt(a.Bounds.X+a.Bounds.W/2, a.Bounds.Y+a.Bounds.H/2)
	s.drain()
	tt.Frame()
}

// TestLyricsSourcesRows lists the sources of lyrics in their own order,
// each with what it gets, moves one by dragging it onto another, and
// keeps the order in settings.json; reset to default puts it back.
func TestLyricsSourcesRows(t *testing.T) {
	s, tt := lyricsSourcesShell(t)
	want := []string{"Better Lyrics, on", "Unison, on", "LRCLIB, on", "YouTube Music, on", "YouTube captions, on"}
	if got := sourceRowOrder(tt); !slices.Equal(got, want) {
		t.Fatalf("rows %q, want %q", got, want)
	}
	if !tt.HasText(lyricsSourceText()[settings.LyricsCaptions].sent) || !tt.HasText(lyricsSourcesAbout()) {
		t.Errorf("the rows don't say what each source gets: %q", tt.Texts())
	}
	if info, ok := tt.Access("reset to default"); !ok || !info.Disabled {
		t.Errorf("reset to default, of the sources as they come: %+v, %v", info, ok)
	}
	dragSourceRow(t, s, tt, "LRCLIB, on", "Better Lyrics, on")
	want = []string{"LRCLIB, on", "Better Lyrics, on", "Unison, on", "YouTube Music, on", "YouTube captions, on"}
	if got := sourceRowOrder(tt); !slices.Equal(got, want) {
		t.Errorf("dragged up, rows %q, want %q", got, want)
	}
	if got, want := keptSources(s), []string{"lrclib", "betterlyrics", "unison", "youtube", "captions"}; !slices.Equal(got, want) {
		t.Errorf("settings.json keeps %q, want %q", got, want)
	}
	click(t, s, tt, "reset to default")
	if got := s.svc.Preferences.Get().LyricsSources; len(got) != 0 {
		t.Errorf("reset keeps %+v", got)
	}
}

// TestLyricsSourcesKeys moves the source whose row has the focus with
// Alt+Down and Alt+Up, which keeps the focus on it as it moves.
func TestLyricsSourcesKeys(t *testing.T) {
	s, tt := lyricsSourcesShell(t)
	row := "Unison, on"
	click(t, s, tt, row)
	press := func(key ui.Key) {
		tt.Key(ui.Alt, key)
		s.drain()
		tt.Frame()
		tt.Frame()
	}
	press(ui.KeyDown)
	press(ui.KeyDown)
	if got, want := keptSources(s), []string{"betterlyrics", "lrclib", "youtube", "unison", "captions"}; !slices.Equal(got, want) {
		t.Errorf("after Alt+Down twice, settings.json keeps %q, want %q", got, want)
	}
	if info, _ := tt.Access(row); !info.Focused {
		t.Error("the focus didn't follow the source down")
	}
	press(ui.KeyUp)
	if got, want := keptSources(s), []string{"betterlyrics", "lrclib", "unison", "youtube", "captions"}; !slices.Equal(got, want) {
		t.Errorf("after Alt+Up, settings.json keeps %q, want %q", got, want)
	}
}

// TestLyricsSourcesSwitches switches sources off, and keeps the last one
// on from being switched off.
func TestLyricsSourcesSwitches(t *testing.T) {
	s, tt := lyricsSourcesShell(t)
	for _, name := range lyricsSourceNames[1:] {
		clickSwitch(t, s, tt, name)
	}
	if got, want := keptSources(s), []string{"betterlyrics", "unison off", "lrclib off", "youtube off", "captions off"}; !slices.Equal(got, want) {
		t.Fatalf("settings.json keeps %q, want %q", got, want)
	}
	last := "Better Lyrics, the last source on"
	if info, ok := tt.AccessByRole(ui.RoleSwitch, last); !ok || !info.Disabled || !info.Checked {
		t.Fatalf("the last source's switch: %+v, %v", info, ok)
	}
	clickSwitch(t, s, tt, last)
	if got := keptSources(s); got[0] != "betterlyrics" {
		t.Errorf("the last source on switched off: %q", got)
	}
	if got := sourceRowOrder(tt); len(got) != 5 || got[1] != "Unison, off" {
		t.Errorf("rows %q", got)
	}
}

// TestLyricsPreferWordsRow turns word-by-word lyrics first on, off at
// first, and finds the row by a search for "karaoke".
func TestLyricsPreferWordsRow(t *testing.T) {
	s, tt := lyricsSourcesShell(t)
	if s.svc.Preferences.Get().LyricsPreferWords {
		t.Fatal("on at first")
	}
	clickSwitch(t, s, tt, "Prefer word-by-word lyrics")
	if !s.svc.Preferences.Get().LyricsPreferWords {
		t.Error("the switch didn't save lyricsPreferWords")
	}
	for _, q := range []string{"karaoke", "lyrics sources", "LRCLIB", "caption language"} {
		show(s, tt, settingsPath("", q))
		if !tt.HasText("Prefer word-by-word lyrics") && !tt.HasText("Lyrics sources") && !tt.HasText("Caption language") {
			t.Errorf("searching %q finds none of the rows: %q", q, tt.Texts())
		}
	}
}

// TestLyricsSourcesLookAgain looks the lyrics of the track that plays up
// again once the sources change.
func TestLyricsSourcesLookAgain(t *testing.T) {
	s, tt := lyricsSourcesShell(t)
	key := "lyrics:" + testTracks[0].ID
	kept := &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Source: "Unison", Lines: []lyrics.Line{{Text: "Paper kites"}}}}
	s.loads[key] = kept
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	clickSwitch(t, s, tt, "Unison")
	if l, ok := s.loads[key]; ok && l == any(kept) {
		t.Error("the lyrics kept of the track that plays stay")
	}
}

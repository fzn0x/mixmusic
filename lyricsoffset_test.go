package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
	"github.com/fzn0x/mixmusic/lyrics"
)

// shownLyrics are the lyrics the test shell shows of testTracks[0].
func shownLyrics(s *shell) lyrics.Lyrics {
	return s.loads["lyrics:"+testTracks[0].ID].(*loaded[lyrics.Lyrics]).value
}

// pressNamed clicks the element named name, with the modifier keys mods
// held.
func pressNamed(t *testing.T, s *shell, tt *ui.Tester, mods ui.Modifiers, name string) {
	t.Helper()
	a, ok := tt.AccessByRole(ui.RoleButton, name)
	if !ok {
		t.Fatalf("no button %q: %q", name, tt.Texts())
	}
	tt.ClickAtWith(mods, a.Bounds.X+a.Bounds.W/2, a.Bounds.Y+a.Bounds.H/2)
	s.drain()
	tt.Frame()
	tt.Frame()
}

// lineLit tells whether the line of lyrics text shows in the accent, as
// the line sung now does, rather than in the ink of the others.
func lineLit(t *testing.T, s *shell, tt *ui.Tester, text string) bool {
	t.Helper()
	r, ok := tt.Find(text)
	if !ok {
		t.Fatalf("%q doesn't show: %q", text, tt.Texts())
	}
	img := tt.Image()
	return inkIn(img, r, s.lk.accent) > inkIn(img, r, s.lk.text)+inkIn(img, r, s.lk.muted)
}

// timingShown is what the timing field above the lyrics in the panel
// shows.
func timingShown(s *shell) string { return s.drafts["lyrics-timing-panel"].text }

// TestLyricsTimingMovesTheLines moves the song's lyrics earlier and later
// above them: the line sung now changes at once, the field says how far in
// words, reset shows while they're moved, and the timing holds for the
// track's lyrics of that source alone.
func TestLyricsTimingMovesTheLines(t *testing.T) {
	s, tt := sungIn(t, "en", lanternLines, 59*time.Second)
	t0 := testTracks[0]
	ly := shownLyrics(s)
	showsAll(t, tt, "the lyrics panel", locales.LyricsTimingEarlier(), locales.LyricsTimingLater())
	if got := timingShown(s); got != "as timed" {
		t.Errorf("the timing shows %q at first", got)
	}
	if tt.HasText("reset") {
		t.Error("reset shows for lyrics as timed")
	}
	if !lineLit(t, s, tt, "Lanterns drifting home") || lineLit(t, s, tt, "Quiet harbour") {
		t.Fatal("the first line isn't the one sung at 0:59")
	}
	// With Shift, a second earlier: the line of 1:00 is sung at 0:59.
	pressNamed(t, s, tt, ui.Shift, locales.LyricsTimingEarlier())
	if got := s.svc.Catalog.LyricsOffset(t0, ly); got != time.Second {
		t.Fatalf("Shift and Earlier moved the lyrics by %v", got)
	}
	if lineLit(t, s, tt, "Lanterns drifting home") || !lineLit(t, s, tt, "Quiet harbour") {
		t.Error("the lines don't move earlier at once")
	}
	if got := timingShown(s); got != "1 s earlier" {
		t.Errorf("the timing shows %q", got)
	}
	pressNamed(t, s, tt, 0, locales.LyricsTimingLater())
	if got := s.svc.Catalog.LyricsOffset(t0, ly); got != 950*time.Millisecond {
		t.Errorf("Later moved the lyrics to %v, want 950ms", got)
	}
	if got := timingShown(s); got != "0.95 s earlier" {
		t.Errorf("the timing shows %q", got)
	}
	// The timing is for these lyrics: LRCLIB's of the track are as timed.
	other := ly
	other.Source = "LRCLIB"
	if got := s.svc.Catalog.LyricsOffset(t0, other); got != 0 {
		t.Errorf("another source's lyrics move by %v", got)
	}
	if got := s.svc.Catalog.LyricsOffset(testTracks[1], ly); got != 0 {
		t.Errorf("another track's lyrics move by %v", got)
	}
	click(t, s, tt, "reset")
	tt.Frame()
	if got := s.svc.Catalog.LyricsOffset(t0, ly); got != 0 || timingShown(s) != "as timed" || tt.HasText("reset") {
		t.Errorf("reset leaves the lyrics %v off, showing %q", got, timingShown(s))
	}
	// Later and later, to 120 s, where Later stops.
	for range 2 {
		pressNamed(t, s, tt, ui.Shift, locales.LyricsTimingLater())
	}
	if got := s.svc.Catalog.LyricsOffset(t0, ly); got != -2*time.Second || timingShown(s) != "2 s later" {
		t.Errorf("twice a second later: %v, showing %q", got, timingShown(s))
	}
	s.svc.Catalog.SetLyricsOffset(t0, ly, -119900*time.Millisecond)
	pressNamed(t, s, tt, ui.Shift, locales.LyricsTimingLater())
	if got := s.svc.Catalog.LyricsOffset(t0, ly); got != -120*time.Second {
		t.Errorf("Later went past 120 s: %v", got)
	}
	if a, ok := tt.AccessByRole(ui.RoleButton, locales.LyricsTimingLater()); !ok || !a.Disabled {
		t.Errorf("Later can be pressed at 120 s later: %+v", a)
	}
}

// TestLyricsTimingTyped types the song's timing in milliseconds: the field
// shows it so while it has the focus, and in words after. A timing typed
// isn't rounded to a step, and one past 120 s stops there.
func TestLyricsTimingTyped(t *testing.T) {
	s, tt := sungIn(t, "en", lanternLines, 59*time.Second)
	t0, ly := testTracks[0], shownLyrics(s)
	typeTiming := func(text string) {
		t.Helper()
		click(t, s, tt, locales.LyricsTimingField())
		tt.Frame()
		tt.Key(0, ui.KeyEnd)
		for range 12 {
			tt.Key(0, ui.KeyBackspace)
		}
		tt.Type(text)
		tt.Key(0, ui.KeyEnter)
		s.drain()
		tt.Frame()
	}
	click(t, s, tt, locales.LyricsTimingField())
	tt.Frame()
	if got := timingShown(s); got != "0" {
		t.Errorf("the field shows %q with the focus, not the milliseconds", got)
	}
	typeTiming("1234")
	if got := s.svc.Catalog.LyricsOffset(t0, ly); got != 1234*time.Millisecond {
		t.Errorf("1234 typed moves the lyrics by %v", got)
	}
	if got := timingShown(s); got != "1234" {
		t.Errorf("after Enter, the field shows %q", got)
	}
	// Earlier steps from there, as it is.
	pressNamed(t, s, tt, 0, locales.LyricsTimingEarlier())
	if got := s.svc.Catalog.LyricsOffset(t0, ly); got != 1284*time.Millisecond {
		t.Errorf("Earlier after 1234 ms: %v", got)
	}
	if got := timingShown(s); got != "1.284 s earlier" {
		t.Errorf("without the focus, the field shows %q", got)
	}
	typeTiming("-300000")
	if got := s.svc.Catalog.LyricsOffset(t0, ly); got != -120*time.Second {
		t.Errorf("300 s later typed moves the lyrics by %v", got)
	}
	typeTiming("soon")
	if got := s.svc.Catalog.LyricsOffset(t0, ly); got != -120*time.Second {
		t.Errorf("words typed moved the lyrics to %v", got)
	}
}

// TestLyricsTimingClick plays the song from a line clicked at its time
// less the timing, so that it is the line sung then, and never before the
// track's start.
func TestLyricsTimingClick(t *testing.T) {
	s, tt := sungIn(t, "en", lanternLines, 30*time.Second)
	t0, ly := testTracks[0], shownLyrics(s)
	for _, c := range []struct {
		shift time.Duration
		line  string
		want  time.Duration
	}{
		{2 * time.Second, "Quiet harbour", 58 * time.Second},
		{-3 * time.Second, "Quiet harbour", 63 * time.Second},
		{5 * time.Second, "Lanterns drifting home", 0},
	} {
		s.svc.Catalog.SetLyricsOffset(t0, ly, c.shift)
		tt.Frame()
		click(t, s, tt, c.line)
		s.settle(t, func(st player.State) bool { return st.Position == c.want })
		tt.Frame()
		if !lineLit(t, s, tt, c.line) {
			t.Errorf("%q moved by %v doesn't light once clicked", c.line, c.shift)
		}
	}
}

// TestLyricsTimingWords lights the words of the line sung now at the
// lyrics' time, moved by their timing: still without the lead of the
// lines.
func TestLyricsTimingWords(t *testing.T) {
	s, tt := sungIn(t, "en", lanterns, 2500*time.Millisecond)
	if lit(t, s, tt.Image(), wordsAt(t, s, tt, "Lanterns drifting home", "Lanterns", "drifting", "home")[2], "home") {
		t.Fatal("home lights at 0:02.5")
	}
	s.svc.Catalog.SetLyricsOffset(testTracks[0], shownLyrics(s), 2*time.Second)
	tt.Frame()
	if !lit(t, s, tt.Image(), wordsAt(t, s, tt, "Lanterns drifting home", "Lanterns", "drifting", "home")[2], "home") {
		t.Error("home doesn't light 2 s earlier")
	}
}

// TestLyricsTimingFollows has the mini player follow the song's timing,
// as the panel does.
func TestLyricsTimingFollows(t *testing.T) {
	s := miniShell(t, nil)
	tt, f := miniAt(t, s, 100, 100, 420, 132)
	click(t, s, tt, "Lyrics")
	f.settle(t, s)
	s.p.Time(2500*time.Millisecond, 200*time.Second)
	s.settle(t, func(st player.State) bool { return st.Position == 2500*time.Millisecond })
	tt.Frame()
	if !tt.HasText("Lanterns drifting home") {
		t.Fatalf("the mini player doesn't show the line sung: %q", tt.Texts())
	}
	s.svc.Catalog.SetLyricsOffset(testTracks[0], shownLyrics(s), 58*time.Second)
	tt.Frame()
	if tt.HasText("Lanterns drifting home") || !tt.HasText("Quiet harbour") {
		t.Errorf("the mini player's lyrics don't move earlier: %q", tt.Texts())
	}
}

// TestLyricsTimingNotch has the player around the notch follow the
// song's timing, as the panel does.
func TestLyricsTimingNotch(t *testing.T) {
	s := newTestShell(t)
	s.loads["lyrics:"+testTracks[0].ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "Unison", Lines: []lyrics.Line{
		{TimeMs: 0, Text: "Paper boats on the river"}, {TimeMs: 30000, Text: "Carry what the morning said"}, {TimeMs: 60000, Text: "Down to the open sea"}}}}
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	x := newNotchTest(t, s, testHousing)
	x.openPanel()
	if x.tt.HasText("Down to the open sea") {
		t.Fatalf("the notch shows the third line at the start: %q", x.tt.Texts())
	}
	// The line of 1:00 is sung, after the one of 0:30.
	s.svc.Catalog.SetLyricsOffset(testTracks[0], shownLyrics(s), 65*time.Second)
	x.tt.Frame()
	if x.tt.HasText("Paper boats on the river") || !x.tt.HasText("Down to the open sea") {
		t.Errorf("the notch's lyrics don't move earlier: %q", x.tt.Texts())
	}
}

// TestLyricsTimingEverySong moves every song's lyrics in Settings ›
// Playback › Lyrics: the slider and the field, which keeps a timing past
// the slider's ends, and reset. As a delay of the output, it grows with
// the speed, and it adds to the song's own.
func TestLyricsTimingEverySong(t *testing.T) {
	s, tt := sungIn(t, "en", lanternLines, 59*time.Second)
	tt.SetSize(1280, 2400)
	show(s, tt, "/settings/playback")
	showsAll(t, tt, "Settings › Playback", "Lyrics timing", locales.LyricsTimingDescription())
	if got := s.drafts["lyrics-offset-ms"].text; got != "as timed" {
		t.Errorf("the field shows %q at first", got)
	}
	click(t, s, tt, locales.LyricsTimingEveryField())
	tt.Frame()
	tt.Key(0, ui.KeyEnd)
	tt.Key(0, ui.KeyBackspace)
	tt.Type("-15000")
	tt.Key(0, ui.KeyEnter)
	s.drain()
	tt.Frame()
	tt.Frame()
	if got := s.svc.Preferences.Get().LyricsOffset; got != -15000 {
		t.Fatalf("15 s later typed keeps %d ms", got)
	}
	// The slider holds it at its end, and doesn't take it back to 10 s.
	tt.Frame()
	if got := s.svc.Preferences.Get().LyricsOffset; got != -15000 || s.set.LyricsOffset != -15000 {
		t.Errorf("the slider took the timing to %d ms", got)
	}
	if a, ok := tt.AccessByRole(ui.RoleSlider, locales.LyricsTimingEvery()); !ok || a.Value != "15 s later" {
		t.Errorf("the slider reads %+v", a)
	}
	ly := shownLyrics(s)
	s.svc.Catalog.SetLyricsOffset(testTracks[0], ly, 500*time.Millisecond)
	s.st.Speed = 1
	if got := s.lyricsShift(testTracks[0], ly); got != -14500*time.Millisecond {
		t.Errorf("0.5 s earlier and 15 s later move the lyrics by %v", got)
	}
	s.st.Speed = 1.5
	if got := s.lyricsShift(testTracks[0], ly); got != -22000*time.Millisecond {
		t.Errorf("at 1.5×, the lyrics move by %v, want 15 s × 1.5 later and 0.5 s earlier", got)
	}
	s.st.Speed = 1
	click(t, s, tt, "reset")
	if got := s.svc.Preferences.Get().LyricsOffset; got != 0 {
		t.Errorf("reset keeps %d ms", got)
	}
}

// TestPseudoLocaleLyricsTiming shows the timing of the lyrics, above them
// and in Settings, in en-XA: only the lyrics are English.
func TestPseudoLocaleLyricsTiming(t *testing.T) {
	usePseudoLocale(t)
	s, tt := sungIn(t, "en", lanternLines, 59*time.Second)
	tt.SetLocale("en-XA")
	xa := locales.In("en-XA")
	s.svc.Catalog.SetLyricsOffset(testTracks[0], shownLyrics(s), 350*time.Millisecond)
	tt.Frame()
	tt.Frame()
	names := []string{"Lanterns drifting home", "Quiet harbour", "Midnight City", "M83", "Hurry Up, We're Dreaming"}
	checkPseudo(t, "The lyrics panel", tt.Texts(), names...)
	if got := timingShown(s); !strings.HasPrefix(got, "[") || !strings.Contains(got, "0.35") {
		t.Errorf("the timing shows %q", got)
	}
	if !tt.HasText(xa.LyricsTimingReset()) {
		t.Errorf("no reset in en-XA: %q", tt.Texts())
	}
	for _, name := range []string{xa.LyricsTimingEarlier(), xa.LyricsTimingLater()} {
		if _, ok := tt.AccessByRole(ui.RoleButton, name); !ok {
			t.Errorf("no button %q", name)
		}
	}
	st := s.svc.Preferences.Get()
	st.LyricsOffset = -1200
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	tt.SetSize(1280, 2400)
	show(s, tt, "/settings/playback")
	checkPseudo(t, "Settings › Playback", tt.Texts(), names...)
	if got := s.drafts["lyrics-offset-ms"].text; !strings.HasPrefix(got, "[") || !strings.Contains(got, "1.2") {
		t.Errorf("the timing of every song shows %q", got)
	}
}

// TestRenderLyricsTiming draws the timing above the lyrics, in the panel
// at its narrowest and on the now playing page, moved and not, and its row
// of Settings, in each layout, to MIXMUSIC_RENDER, for a look.
func TestRenderLyricsTiming(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	for _, layout := range settings.Layouts {
		s, tt := sungIn(t, "en", lanternLines, 59*time.Second)
		st := s.svc.Preferences.Get()
		st.FollowSystem, st.Theme, st.Layout, st.DynamicAccent = false, "mix", layout, false
		if _, err := s.svc.Preferences.Set(st); err != nil {
			t.Fatal(err)
		}
		s.refresh()
		name := string(layout) + "-lyrics-timing-"
		tt.SetSize(1000, 700)
		renderTo(t, s, tt, dir, name+"panel")
		s.svc.Catalog.SetLyricsOffset(testTracks[0], shownLyrics(s), 350*time.Millisecond)
		renderTo(t, s, tt, dir, name+"panel-moved")
		tt.SetSize(1280, 900)
		show(s, tt, "/now-playing")
		renderTo(t, s, tt, dir, name+"page")
		st = s.svc.Preferences.Get()
		st.LyricsOffset = -15000
		if _, err := s.svc.Preferences.Set(st); err != nil {
			t.Fatal(err)
		}
		s.refresh()
		show(s, tt, "/settings/playback?q=lyrics+timing")
		renderTo(t, s, tt, dir, name+"settings")
	}
}

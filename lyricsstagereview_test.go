package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/romaji"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/lyrics"
)

// TestLyricSizeSliderReachesTheEnds takes the slider of the lyrics' text size
// to both its ends, which are 60% and 180%: stepping by a tenth from 0.6 goes
// to 1.8000000000000003, past the end, which would have it stop at 170%.
func TestLyricSizeSliderReachesTheEnds(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 3000)
	show(s, tt, "/settings/appearance")
	reveal(tt, "Lyrics text size")
	slider := func() ui.Rect {
		t.Helper()
		a, ok := tt.AccessByRole(ui.RoleSlider, "Lyrics text size")
		if !ok {
			t.Fatalf("Settings has no slider of the lyrics' text size: %q", tt.Texts())
		}
		return a.Bounds
	}
	b := slider()
	tt.ClickAt(b.X+b.W/2, b.Y+b.H/2)
	tt.Key(0, ui.KeyEnd)
	if got := s.set.LyricsSize; got != settings.MaxLyricsSize {
		t.Errorf("End takes the size to %v, not %v", got, settings.MaxLyricsSize)
	}
	tt.Key(0, ui.KeyHome)
	if got := s.set.LyricsSize; got != settings.MinLyricsSize {
		t.Errorf("Home takes the size to %v, not %v", got, settings.MinLyricsSize)
	}
	// Dragged to the far right.
	b = slider()
	tt.ClickAt(b.X+b.W-1, b.Y+b.H/2)
	if got := s.set.LyricsSize; got != settings.MaxLyricsSize {
		t.Errorf("a click at the far right takes the size to %v, not %v", got, settings.MaxLyricsSize)
	}
	// Every step in between is a tenth, as kept.
	for i := range 12 {
		tt.Key(0, ui.KeyHome)
		for range i {
			tt.Key(0, ui.KeyRight)
		}
		want := float64(60+10*i) / 100
		if got := s.set.LyricsSize; got != want {
			t.Errorf("%d steps from the start the size is %v, not %v", i, got, want)
		}
	}
	if got := newShell(s.a, s.p).set.LyricsSize; got != 1.7 {
		t.Errorf("the size kept is %v", got)
	}
}

// TestStageRomajiNotes keeps the notes of the romaji, that its readings of
// kanji download or couldn't, to the lyrics panel: the full-screen lyrics
// show the lyrics alone, kanji as they are until the readings come.
func TestStageRomajiNotes(t *testing.T) {
	g := &gate{open: make(chan struct{}), answer: func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }}
	s, tt := romajiFrom(t, g)
	const downloading = "The readings of kanji are downloading: kanji show as they are until then."
	const failed = "The readings of kanji couldn't be downloaded: kanji show as they are."
	// The panel says so, which also has the track's romaji made.
	until(t, s, tt, "the note of the download", func() bool { return tt.HasText(downloading) })
	// The stage, over a page with no lyrics of its own, and with the panel of
	// the lyrics closed, whose notes they would be, doesn't.
	show(s, tt, "/settings/shortcuts")
	s.panel = ""
	s.toggleStage()
	s.drain()
	tt.Frame()
	tt.Advance(time.Second)
	if !s.stage.open {
		t.Fatal("the stage didn't open")
	}
	if !tt.HasText("紙の凧") || !tt.HasText("紙 no 凧") {
		t.Errorf("the stage doesn't show the lyrics in romaji as they are: %q", tt.Texts())
	}
	if tt.HasText(downloading) || tt.HasText("Romaji is on its way…") {
		t.Errorf("the stage tells of the download: %q", tt.Texts())
	}
	g.let()
	until(t, s, tt, "the readings to fail", func() bool {
		l, ok := s.loads[romajiKey(testTracks[0])].(*loaded[romaji.Lines])
		return ok && l.done && l.value.Readings == romaji.Unavailable
	})
	tt.Advance(time.Second)
	if tt.HasText(failed) || tt.HasText("try again") {
		t.Errorf("the stage tells that the readings couldn't be downloaded: %q", tt.Texts())
	}
	if !tt.HasText("紙 no 凧") {
		t.Errorf("the stage loses the lyrics after the failure: %q", tt.Texts())
	}
}

// TestStageBallRoomNeedsSyncedLyrics keeps no room for the ball above the
// lines of lyrics shown as plain text, though their words have times: they
// don't light as they are sung, and show no ball.
func TestStageBallRoomNeedsSyncedLyrics(t *testing.T) {
	// pitch is how far apart two lines lie, with the ball on or off.
	pitch := func(ball bool) float32 {
		t.Helper()
		s, tt := stageOn(t, stageSong, 15200*time.Millisecond, false)
		s.set.LyricsPlain = true
		s.set.LyricsBall = ball
		s.drain()
		tt.Frame()
		tt.Advance(time.Second)
		return stageMid(t, tt, "Quiet as the harbour light") - stageMid(t, tt, "Sand keeps the shape of it")
	}
	if on, off := pitch(true), pitch(false); stageAbs(on-off) > 1 {
		t.Errorf("plain lyrics keep room for a ball they don't have: the lines lie %v apart with it on, %v with it off", on, off)
	}
}

// TestStageShortcutCanBeChanged gives the full-screen lyrics other keys from
// Settings › Shortcuts, as any command has: Ctrl+Shift+F is taken by some
// input methods on Windows. The old keys then open nothing, and the new ones
// open the stage.
func TestStageShortcutCanBeChanged(t *testing.T) {
	s := newTestShell(t)
	track := testTracks[0]
	s.loads["lyrics:"+track.ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "Unison", Language: "en", Lines: stageSong}}
	s.p.PlayTracks([]media.Track{track}, 0)
	s.settle(t, func(st player.State) bool { return st.Playing && !st.Loading })
	tt := testerOf(t, s, 1280, 4000)
	show(s, tt, "/settings/shortcuts")
	if !tt.HasText("Show or hide the full-screen lyrics") {
		t.Fatalf("Settings › Shortcuts doesn't list the full-screen lyrics: %q", tt.Texts())
	}
	if err := tt.Click("change the shortcut of Show or hide the full-screen lyrics"); err != nil {
		t.Fatalf("the full-screen lyrics have no shortcut to change: %v", err)
	}
	tt.Key(ui.Cmd|ui.Alt, ui.KeyF)
	if got := s.set.Shortcuts["view.lyricsStage"]; got != "Mod+Alt+F" {
		t.Fatalf("the shortcut is %q, not Mod+Alt+F; shortcuts %v", got, s.set.Shortcuts)
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyF)
	s.drain()
	tt.Frame()
	if s.stage.open {
		t.Error("the keys given up still open the full-screen lyrics")
	}
	tt.Key(ui.Cmd|ui.Alt, ui.KeyF)
	s.drain()
	tt.Frame()
	if !s.stage.open {
		t.Error("the new keys don't open the full-screen lyrics")
	}
}

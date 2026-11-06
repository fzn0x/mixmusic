package main

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/lyrics"
)

// TestLyricsNote says why lyrics show without their times, as those of a
// song with a longer music video of it (#290), above them.
func TestLyricsNote(t *testing.T) {
	s := newTestShell(t)
	note := "Not synced: the song's synced lyrics don't match this video."
	s.loads["lyrics:"+testTracks[0].ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Source: "LRCLIB", Note: note,
		Lines: []lyrics.Line{{Text: "Paper kites"}, {Text: "Over the river"}}}}
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := testerOf(t, s, 1280, 900)
	s.open("/lyrics")
	tt.Frame()
	if !tt.HasText(note) || !tt.HasText("Paper kites") {
		t.Errorf("texts: %q", tt.Texts())
	}
}

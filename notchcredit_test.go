package main

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/lyrics"
)

// TestNotchLyricsCredit has the player around the notch name where its
// lyrics come from, and who gave them to it, as the lyrics panel does:
// Unison's licence asks for it wherever its lyrics show.
func TestNotchLyricsCredit(t *testing.T) {
	cases := []struct {
		name string
		ly   lyrics.Lyrics
		want string
		not  string
	}{
		{"Unison", lyrics.Lyrics{Source: "Unison", SourceURL: "https://unison.betterlyrics.org/song/abcdefghijk",
			By: "Harbour Keeper", ByURL: "https://unison.betterlyrics.org/curator/0f1e2d3c"}, "lyrics from Unison · submitted by Harbour Keeper", ""},
		{"Better Lyrics", lyrics.Lyrics{Source: "Better Lyrics", SourceURL: "https://better-lyrics.boidu.dev"}, "lyrics from Better Lyrics", "submitted by"},
		{"no source", lyrics.Lyrics{}, "", "lyrics from"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newTestShell(t)
			c.ly.Synced = true
			c.ly.Lines = []lyrics.Line{{TimeMs: 0, Text: "Paper boats on the river"}, {TimeMs: 30000, Text: "Carry what the morning said"}}
			s.loads["lyrics:"+testTracks[0].ID] = &loaded[lyrics.Lyrics]{done: true, value: c.ly}
			s.p.PlayTracks(testTracks, 0)
			s.settle(t, func(st player.State) bool { return st.Playing })
			x := newNotchTest(t, s, testHousing)
			x.openPanel()
			if c.want != "" && !x.tt.HasText(c.want) {
				t.Errorf("the notch's lyrics show %q, want %q", x.tt.Texts(), c.want)
			}
			if c.not != "" && x.tt.HasText(c.not) {
				t.Errorf("the notch's lyrics show %q: %q", c.not, x.tt.Texts())
			}
		})
	}
}

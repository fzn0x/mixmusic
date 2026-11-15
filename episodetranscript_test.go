package main

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/lyrics"
)

// TestEpisodeTranscript shows an episode's transcript under its details,
// each line with its time, and plays the episode from a line clicked.
func TestEpisodeTranscript(t *testing.T) {
	s := detailsShell(t, &loaded[media.EpisodeDetails]{done: true, value: testDetails()})
	s.loads["transcript:"+detailsEpisode.ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "YouTube captions",
		Lines: []lyrics.Line{{TimeMs: 0, Text: "welcome back to night science"}, {TimeMs: 754_000, Text: "so what do owls dream of"}}}}
	tt := testerOf(t, s, 1280, 1800)
	s.open("/lyrics")
	tt.Frame()
	tt.Advance(time.Second)
	for _, want := range []string{"Transcript", "welcome back to night science", "12:34", "so what do owls dream of"} {
		if !tt.HasText(want) {
			t.Errorf("the details don't show %q: %q", want, tt.Texts())
		}
	}
	desc, _ := tt.Find("Why we dream, and what the owls know.")
	title, _ := tt.Find("Transcript")
	if title.Y <= desc.Y {
		t.Errorf("the transcript, at %v, isn't under the description, at %v", title, desc)
	}
	click(t, s, tt, "Play from 12:34, so what do owls dream of")
	s.settle(t, func(st player.State) bool { return st.Position >= 754*time.Second })
}

// TestEpisodeWithoutTranscript shows the details alone of an episode with
// no transcript.
func TestEpisodeWithoutTranscript(t *testing.T) {
	s := detailsShell(t, &loaded[media.EpisodeDetails]{done: true, value: testDetails()})
	tt := testerOf(t, s, 1280, 1800)
	s.open("/lyrics")
	tt.Frame()
	if tt.HasText("Transcript") || !tt.HasText("Why we dream, and what the owls know.") {
		t.Errorf("the details show %q", tt.Texts())
	}
}

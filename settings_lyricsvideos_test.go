package main

import (
	"testing"

	"github.com/fzn0x/mixmusic/lyrics"
)

// TestTimeVideosRow times music videos' lyrics to them at first, turns it
// off from its switch, and finds the row by a search for "music video".
func TestTimeVideosRow(t *testing.T) {
	s, tt := lyricsSourcesShell(t)
	if !s.svc.Preferences.Get().LyricsTimeVideos {
		t.Fatal("off at first")
	}
	clickSwitch(t, s, tt, "Time lyrics to music videos")
	if s.svc.Preferences.Get().LyricsTimeVideos {
		t.Error("the switch didn't save lyricsTimeVideos")
	}
	show(s, tt, settingsPath("", "music video"))
	if !tt.HasText("Time lyrics to music videos") {
		t.Errorf("searching \"music video\" doesn't find the row: %q", tt.Texts())
	}
}

// TestLyricsTimedLookAgain looks the lyrics of a video up again as the
// app timed them to it, showing those it has until the new ones come.
func TestLyricsTimedLookAgain(t *testing.T) {
	s := newTestShell(t)
	key := "lyrics:" + testTracks[0].ID
	kept := &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "LRCLIB", Note: "Timing these lyrics to the video…"}}
	s.loads[key] = kept
	s.lyricsTimed(testTracks[0].ID)
	if l, ok := s.loads[key]; !ok || l != any(kept) || !kept.again || !kept.done {
		t.Errorf("the lyrics shown aren't asked for again in place: %+v", s.loads[key])
	}
}

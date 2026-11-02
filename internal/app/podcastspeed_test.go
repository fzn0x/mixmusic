package app

import (
	"path/filepath"
	"testing"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/nowplaying"
)

// TestControlsRateOfEpisode keeps the rate the system's controls ask, while
// they show a podcast's episode, as the podcasts' speed, and the songs'
// speed as it was; while they show a song, as the songs' speed.
func TestControlsRateOfEpisode(t *testing.T) {
	dir := t.TempDir()
	a, err := New(Options{DataDir: filepath.Join(dir, "data"), CacheDir: filepath.Join(dir, "cache"), Builtin: builtin.FS, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	episode := media.Track{ID: "yt:e1", Title: "Episode", Episode: &media.Episode{Show: media.Ref{Name: "Show"}}}
	a.controls.np = NowPlaying{Track: &episode, Playing: true}
	a.controlsAsked(nowplaying.Request{Command: nowplaying.SetRate, Rate: 1.75})
	if st := a.settings.Get(); st.PodcastSpeed != 1.75 || st.SongSpeed != 1 {
		t.Errorf("with an episode, episodes play at %v and songs at %v", st.PodcastSpeed, st.SongSpeed)
	}
	song := media.Track{ID: "yt:1", Title: "Song"}
	a.controls.np = NowPlaying{Track: &song, Playing: true}
	a.controlsAsked(nowplaying.Request{Command: nowplaying.SetRate, Rate: 1.25})
	if st := a.settings.Get(); st.PodcastSpeed != 1.75 || st.SongSpeed != 1.25 {
		t.Errorf("with a song, episodes play at %v and songs at %v", st.PodcastSpeed, st.SongSpeed)
	}
}

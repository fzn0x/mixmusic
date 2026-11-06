package app

import (
	"net/http"
	"net/url"
	"slices"
	"sync/atomic"
	"testing"
)

// videoLyrics are lyrics sources, and YouTube Music as fakeMusic has it:
// the music video of Instant Crush is 5:40, and its song 5:38. lrclibAsked
// is the query of LRCLIB's last lookup.
type videoLyrics struct {
	*lyricsSources
	music       *fakeMusic
	lrclibAsked atomic.Value
}

func newVideoLyrics(t *testing.T) *videoLyrics {
	v := &videoLyrics{lyricsSources: newLyricsSources(t), music: newFakeMusic(t)}
	music := v.music.catalog(t)
	v.c.a.youtube, v.c.a.matcher = music.a.youtube, music.a.matcher
	v.lrclib = func(w http.ResponseWriter, r *http.Request) {
		v.lrclibAsked.Store(r.URL.Query())
		w.Write([]byte(lrclibSynced))
	}
	return v
}

// TestLyricsOfALongerVideo shows the synced lyrics of a song without
// their times with a music video longer than the song, as one that opens
// with a scene before it: they would light up every line early (#290).
func TestLyricsOfALongerVideo(t *testing.T) {
	v := newVideoLyrics(t)
	video := instantCrushVideo
	video.DurationMs = (5*60 + 38 + 29) * 1000
	l, err := v.c.Lyrics(t.Context(), video)
	if err != nil || l.Synced || l.Note == "" || l.Source != "LRCLIB" || len(l.Lines) != 2 || l.Lines[0].Text != "From LRCLIB" || l.Lines[1].TimeMs != 0 {
		t.Fatalf("lyrics = %+v, %v", l, err)
	}
	// LRCLIB was asked for the song, by its length.
	if q, _ := v.lrclibAsked.Load().(url.Values); q.Get("duration") != "338" {
		t.Errorf("LRCLIB asked %v", q)
	}
}

// TestLyricsOfALongerVideoFromYouTubeMusic keeps the note on YouTube
// Music's plain lyrics, when they show for being longer.
func TestLyricsOfALongerVideoFromYouTubeMusic(t *testing.T) {
	v := newVideoLyrics(t)
	withLyrics, page := recorded(t, "next"), recorded(t, "lyrics")
	v.music.radio = func(string) []byte { return withLyrics }
	v.music.browse = func(string) []byte { return page }
	video := instantCrushVideo
	video.DurationMs = (5*60 + 38 + 29) * 1000
	l, err := v.c.Lyrics(t.Context(), video)
	if err != nil || l.Synced || l.Source == "LRCLIB" || len(l.Lines) <= 2 || l.Note != videoNote {
		t.Fatalf("lyrics = %+v, %v", l, err)
	}
}

// TestLyricsOfAVideoAsLongAsItsSong keeps the synced lyrics of a song with
// its music video when they are as long, telling the video's length by
// its radio when the video doesn't.
func TestLyricsOfAVideoAsLongAsItsSong(t *testing.T) {
	v := newVideoLyrics(t)
	l, err := v.c.Lyrics(t.Context(), instantCrushVideo)
	if err != nil || !l.Synced || l.Note != "" || l.Lines[1].TimeMs != 4000 {
		t.Fatalf("lyrics = %+v, %v", l, err)
	}
	if _, radios := v.music.asked(); !slices.Contains(radios, "a5uQMwRMHcs") {
		t.Errorf("radios asked %v, not the video's for its length", radios)
	}
}

// TestLyricsOfAVideoOnUnison keeps the lyrics Unison has of a music video,
// which it finds by the video's own ID, timed for it, and looks no song
// up.
func TestLyricsOfAVideoOnUnison(t *testing.T) {
	v := newVideoLyrics(t)
	v.unison = status(http.StatusOK, unisonSynced)
	video := instantCrushVideo
	video.DurationMs = (5*60 + 38 + 29) * 1000
	l, err := v.c.Lyrics(t.Context(), video)
	if err != nil || !l.Synced || l.Note != "" || l.Source != "Unison" {
		t.Fatalf("lyrics = %+v, %v", l, err)
	}
	if searches, _ := v.music.asked(); len(searches) != 0 {
		t.Errorf("searched %v", searches)
	}
}

// TestLyricsOfAVideoWithoutASong asks LRCLIB for a video YouTube Music
// has no song of, as an upload, as it asks for any track.
func TestLyricsOfAVideoWithoutASong(t *testing.T) {
	v := newVideoLyrics(t)
	video := instantCrushVideo
	video.Title, video.DurationMs = "Something Else Live", 300_000
	l, err := v.c.Lyrics(t.Context(), video)
	if err != nil || !l.Synced || l.Note != "" {
		t.Fatalf("lyrics = %+v, %v", l, err)
	}
	if q, _ := v.lrclibAsked.Load().(url.Values); q.Get("track_name") != "Something Else Live" || q.Get("duration") != "300" {
		t.Errorf("LRCLIB asked %v", q)
	}
}

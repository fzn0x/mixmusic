package app

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/fzn0x/mixmusic/internal/match"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/lyrics"
)

// captionedVideo is the video of the captions recorded in
// ../stream/testdata: English, German, and English YouTube made from
// speech, 200 seconds long.
var captionedVideo = media.Track{ID: "yt:3IUqoyTxEU8", Source: media.SourceYouTube, Title: "Paper Kites (Live)",
	Artists: []media.Ref{{Name: "Nobody"}}, DurationMs: 200_000, Video: true}

// captionsServer is YouTube's player API answering with those captions,
// and their timedtext addresses, which count the captions asked for, by
// language.
type captionsServer struct {
	asks  atomic.Int32
	langs atomic.Value
	// speechOnly lists only the captions made from speech.
	speechOnly bool
}

// withCaptions has the catalog of ls read captions from a captionsServer.
func withCaptions(t *testing.T, ls *lyricsSources) *captionsServer {
	t.Helper()
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join("..", "stream", "testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	player, json3 := read("visionos-player-captions.json"), read("timedtext-json3.json")
	cs := &captionsServer{}
	cs.langs.Store("")
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/youtubei/v1/visitor_id":
			w.Write([]byte(`{"responseContext":{"visitorData":"CgtUZXN0VmlzaXRvcijAgICABjIKCgJVUxIEGgAgAA%3D%3D"}}`))
		case "/youtubei/v1/player":
			answer := bytes.ReplaceAll(player, []byte("{{timedtext}}"), []byte(srv.URL))
			if cs.speechOnly {
				answer = bytes.ReplaceAll(answer, []byte(`"vssId": ".`), []byte(`"kind": "asr", "vssId": ".`))
			}
			w.Write(answer)
		case "/api/timedtext":
			cs.asks.Add(1)
			cs.langs.Store(r.URL.Query().Get("lang"))
			w.Write(json3)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	d := stream.NewDirect()
	d.HTTP, d.BaseURL = srv.Client(), srv.URL
	ls.c.a.captions = d
	return cs
}

// TestLyricsFromCaptions shows the captions of a video no lyrics source
// has, timed, without what only tells a sound or the notes of music, and
// credits them.
func TestLyricsFromCaptions(t *testing.T) {
	ls := newLyricsSources(t)
	cs := withCaptions(t, ls)
	l, err := ls.c.Lyrics(context.Background(), captionedVideo)
	if err != nil {
		t.Fatal(err)
	}
	if !l.Synced || l.Source != "YouTube captions" || l.Language != "en" || len(l.Lines) != 3 ||
		l.Lines[0].TimeMs != 18640 || l.Lines[0].Text != "Paper kites over the bay" ||
		l.Lines[1].Text != "I'll wait for you in the night" || l.Lines[2].Text != "(crowd cheering)" {
		t.Fatalf("lyrics = %+v", l)
	}
	if got := cs.langs.Load(); got != "en" {
		t.Errorf("captions asked in %q", got)
	}
}

// TestLyricsCaptionsLast asks for captions only when no source before them
// has synced lyrics, and never once they're switched off.
func TestLyricsCaptionsLast(t *testing.T) {
	ls := newLyricsSources(t)
	cs := withCaptions(t, ls)
	ls.lrclib = status(http.StatusOK, lrclibSynced)
	if l, err := ls.c.Lyrics(context.Background(), captionedVideo); err != nil || l.Source != "LRCLIB" || cs.asks.Load() != 0 {
		t.Errorf("lyrics from %q, %v; captions asked %d times", l.Source, err, cs.asks.Load())
	}
	ls = newLyricsSources(t)
	cs = withCaptions(t, ls)
	withSources(t, ls, false, settings.LyricsSource{Key: settings.LyricsCaptions, Off: true})
	if _, err := ls.c.Lyrics(context.Background(), captionedVideo); !errors.Is(err, lyrics.ErrNotFound) || cs.asks.Load() != 0 {
		t.Errorf("switched off: %v; captions asked %d times", err, cs.asks.Load())
	}
}

// TestLyricsCaptionLanguage takes the captions in the language Settings
// chooses, when the video has them, else the video's own, and never those
// made from speech.
func TestLyricsCaptionLanguage(t *testing.T) {
	for lang, want := range map[string]string{"": "en", "de": "de-DE", "ja": "en"} {
		ls := newLyricsSources(t)
		cs := withCaptions(t, ls)
		ls.c.a.settings.Update(func(s *settings.Settings) error { s.LyricsCaptionLanguage = lang; return nil })
		l, err := ls.c.Lyrics(context.Background(), captionedVideo)
		if err != nil || cs.langs.Load() != want || l.Language != languageOf(want) {
			t.Errorf("%q: asked %q, lyrics in %q, %v", lang, cs.langs.Load(), l.Language, err)
		}
	}
	ls := newLyricsSources(t)
	cs := withCaptions(t, ls)
	cs.speechOnly = true
	if _, err := ls.c.Lyrics(context.Background(), captionedVideo); !errors.Is(err, lyrics.ErrNotFound) || cs.asks.Load() != 0 {
		t.Errorf("only made from speech: %v; captions asked %d times", err, cs.asks.Load())
	}
}

// TestLyricsCaptionsOfAMatch takes the captions of a Spotify track's
// recording on YouTube only when the two are as long, and never asks for
// those of a file of the computer.
func TestLyricsCaptionsOfAMatch(t *testing.T) {
	known, err := store.Open(filepath.Join(t.TempDir(), "matches.json"), map[string]media.Track{
		"spotify:1": {ID: captionedVideo.ID, Source: media.SourceYouTube}, "spotify:2": {ID: captionedVideo.ID, Source: media.SourceYouTube},
	})
	if err != nil {
		t.Fatal(err)
	}
	for id, length := range map[string]int{"spotify:1": 202_000, "spotify:2": 230_000} {
		ls := newLyricsSources(t)
		withCaptions(t, ls)
		ls.c.a.matcher = match.New(noSearch{}, known)
		_, err := ls.c.Lyrics(context.Background(), media.Track{ID: id, Source: media.SourceSpotify, Title: "Paper Kites", Artists: []media.Ref{{Name: "Nobody"}}, DurationMs: length})
		if fits := length == 202_000; fits != (err == nil) {
			t.Errorf("%d ms against 200 s: %v", length, err)
		}
	}
	ls := newLyricsSources(t)
	cs := withCaptions(t, ls)
	ls.c.a.matcher = match.New(noSearch{}, known)
	ls.c.Lyrics(context.Background(), media.Track{ID: "local:1", Source: media.SourceLocal, Title: "Paper Kites", Artists: []media.Ref{{Name: "Nobody"}}})
	if n := cs.asks.Load(); n != 0 {
		t.Errorf("captions asked %d times for a file", n)
	}
}

// TestTranscript reads an episode's captions as its transcript, those
// made from speech when it has no other, while captions are a source of
// lyrics.
func TestTranscript(t *testing.T) {
	episode := captionedVideo
	episode.Episode = &media.Episode{}
	ls := newLyricsSources(t)
	cs := withCaptions(t, ls)
	cs.speechOnly = true
	l, err := ls.c.Transcript(context.Background(), episode)
	if err != nil || !l.Synced || len(l.Lines) != 3 || cs.asks.Load() != 1 {
		t.Fatalf("transcript %+v, %v; captions asked %d times", l, err, cs.asks.Load())
	}
	withSources(t, ls, false, settings.LyricsSource{Key: settings.LyricsCaptions, Off: true})
	if _, err := ls.c.Transcript(context.Background(), episode); !errors.Is(err, lyrics.ErrNotFound) {
		t.Errorf("captions switched off: %v", err)
	}
}

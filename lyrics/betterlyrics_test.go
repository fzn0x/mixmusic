package lyrics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// betterLyricsFixture reads the recorded shape of an answer, whose words
// are made up.
func betterLyricsFixture(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("testdata/betterlyrics-getlyrics.json")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// betterLyricsServer answers every lookup with code and body, and tells
// each to seen.
func betterLyricsServer(t *testing.T, code int, body string, seen func(r *http.Request)) *BetterLyrics {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seen != nil {
			seen(r)
		}
		w.WriteHeader(code)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	b := NewBetterLyrics("test-agent")
	b.BaseURL = srv.URL
	return b
}

// TestBetterLyricsWordSynced reads lyrics timed word by word, asked for by
// the title, artist, album and length, and credits Better Lyrics.
func TestBetterLyricsWordSynced(t *testing.T) {
	b := betterLyricsServer(t, http.StatusOK, betterLyricsFixture(t), func(r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/getLyrics" || q.Get("s") != "Song" || q.Get("a") != "Artist" || q.Get("al") != "Album" || q.Get("d") != "200" ||
			r.Header.Get("User-Agent") != "test-agent" || r.Header.Get("X-API-Key") != "" {
			t.Errorf("request = %s, User-Agent %q", r.URL, r.Header.Get("User-Agent"))
		}
	})
	l, err := b.Get(context.Background(), song)
	if err != nil || !l.Synced || len(l.Lines) != 3 || l.Lines[0].Text != "Paper kites over town" {
		t.Fatalf("lyrics = %+v, %v", l, err)
	}
	if len(l.Lines[0].Words) != 4 || l.Lines[0].TimeMs != 10500 || l.Lines[1].Words[4].Text != "shore" {
		t.Errorf("words = %+v", l.Lines[0].Words)
	}
	if l.Source != "Better Lyrics" || l.SourceURL != "https://betterlyrics.org" {
		t.Errorf("credit = %q, %q", l.Source, l.SourceURL)
	}
}

// TestBetterLyricsNeedsAllFour asks nothing for a track without its album
// or length: the service answers those with a demand for a key.
func TestBetterLyricsNeedsAllFour(t *testing.T) {
	asked := false
	b := betterLyricsServer(t, http.StatusOK, betterLyricsFixture(t), func(*http.Request) { asked = true })
	for name, q := range map[string]Query{
		"no album":    {Title: "Song", Artist: "Artist", Duration: time.Minute},
		"no duration": {Title: "Song", Artist: "Artist", Album: "Album"},
		"no artist":   {Title: "Song", Album: "Album", Duration: time.Minute},
		"no title":    {Artist: "Artist", Album: "Album", Duration: time.Minute},
	} {
		if _, err := b.Get(context.Background(), q); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: err = %v, want ErrNotFound", name, err)
		}
	}
	if asked {
		t.Error("asked Better Lyrics without the four")
	}
	var none *BetterLyrics
	if _, err := none.Get(context.Background(), song); !errors.Is(err, ErrNotFound) {
		t.Errorf("nil client: err = %v", err)
	}
}

// TestBetterLyricsMisses reads a song it hasn't kept, as it says with
// 401, 404 or 422, as none, and lines timed only as none: Unison is asked
// for word-synced lyrics next.
func TestBetterLyricsMisses(t *testing.T) {
	lineSynced, _ := json.Marshal(map[string]string{"ttml": `<tt xmlns="http://www.w3.org/ns/ttml"><body><div><p begin="1.0" end="3.0">One line</p><p begin="4.0" end="6.0">Two lines</p></div></body></tt>`})
	for name, c := range map[string]struct {
		code int
		body string
	}{
		"401":         {http.StatusUnauthorized, `{"error":"API key required"}`},
		"404":         {http.StatusNotFound, `{"error":"not found"}`},
		"422":         {http.StatusUnprocessableEntity, `{}`},
		"empty":       {http.StatusOK, `{"ttml":""}`},
		"line synced": {http.StatusOK, string(lineSynced)},
		"unreadable":  {http.StatusOK, `{"ttml":"<tt><body><p begin=\"1\">"}`},
	} {
		_, err := betterLyricsServer(t, c.code, c.body, nil).Get(context.Background(), song)
		if !errors.Is(err, ErrNotFound) || errors.Is(err, ErrUnavailable) {
			t.Errorf("%s: err = %v, want ErrNotFound", name, err)
		}
	}
}

// TestBetterLyricsUnavailable passes over a service busy or failing.
func TestBetterLyricsUnavailable(t *testing.T) {
	for name, c := range map[string]struct {
		code int
		body string
	}{
		"429":      {http.StatusTooManyRequests, "Too Many Requests"},
		"500":      {http.StatusInternalServerError, `{"error":"upstream"}`},
		"503":      {http.StatusServiceUnavailable, `{}`},
		"not json": {http.StatusOK, `<html>`},
	} {
		_, err := betterLyricsServer(t, c.code, c.body, nil).Get(context.Background(), song)
		if !errors.Is(err, ErrUnavailable) || errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "Better Lyrics isn't answering") {
			t.Errorf("%s: err = %v, want ErrUnavailable", name, err)
		}
	}
}

// TestBetterLyricsSlow gives up on a service that doesn't answer in a few
// seconds, and on one that can't be reached.
func TestBetterLyricsSlow(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	t.Cleanup(func() { close(release); srv.Close() })
	b := NewBetterLyrics("test")
	b.BaseURL = srv.URL
	b.HTTP.Timeout = 50 * time.Millisecond
	if _, err := b.Get(context.Background(), song); !errors.Is(err, ErrUnavailable) {
		t.Errorf("slow: err = %v", err)
	}
	if betterLyricsTimeout > 3*time.Second {
		t.Errorf("timeout = %v, want at most 3 s", betterLyricsTimeout)
	}
	b.BaseURL = "http://127.0.0.1:1"
	if _, err := b.Get(context.Background(), song); !errors.Is(err, ErrUnavailable) {
		t.Errorf("unreachable: err = %v", err)
	}
}

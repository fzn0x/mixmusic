package lyrics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// unisonServer answers lookups as Unison does, with lyrics in format, and
// tells each lookup's query to seen.
func unisonServer(t *testing.T, format, text string, seen func(r *http.Request)) *Unison {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lyrics" || r.Method != http.MethodGet {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if seen != nil {
			seen(r)
		}
		json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{
			"lyrics": text, "format": format, "syncType": "linesync", "language": "en"}})
	}))
	t.Cleanup(srv.Close)
	u := NewUnison("test-agent")
	u.BaseURL = srv.URL
	return u
}

// TestUnisonByVideo looks lyrics up by the video ID alone, when there is
// one, and credits Unison with its address.
func TestUnisonByVideo(t *testing.T) {
	u := unisonServer(t, "ttml", richsync, func(r *http.Request) {
		if got := r.URL.RawQuery; got != "v=abcdefghijk" || r.Header.Get("User-Agent") != "test-agent" {
			t.Errorf("query = %q, User-Agent %q", got, r.Header.Get("User-Agent"))
		}
	})
	q := song
	q.VideoID = "abcdefghijk"
	l, err := u.Get(context.Background(), q)
	if err != nil || !l.Synced || len(l.Lines) != 3 || l.Lines[0].Text != "Paper kites" {
		t.Fatalf("lyrics = %+v, %v", l, err)
	}
	if l.Source != "Unison" || l.SourceURL != "https://unison.betterlyrics.org" || l.By != "" || l.ByURL != "" {
		t.Errorf("credit = %q, %q, %q, %q", l.Source, l.SourceURL, l.By, l.ByURL)
	}
	if l.Language != "en" {
		t.Errorf("language = %q", l.Language)
	}
}

// TestUnisonCredit credits Unison's lyrics as Unison asks: Unison, linked
// to the song's page there, and who submitted them, linked to their page.
// A submitter without a page is named alone, and lyrics without a
// submitter credit Unison alone.
func TestUnisonCredit(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		data                 map[string]any
		sourceURL, by, byURL string
	}{
		{"submitter", map[string]any{"videoId": "abcdefghijk", "submitter": map[string]any{"displayName": " Harbour Keeper ", "keyId": "0f1e2d3c"}},
			"https://unison.betterlyrics.org/song/abcdefghijk", "Harbour Keeper", "https://unison.betterlyrics.org/curator/0f1e2d3c"},
		{"no page", map[string]any{"videoId": "abcdefghijk", "submitter": map[string]any{"displayName": "Harbour Keeper"}},
			"https://unison.betterlyrics.org/song/abcdefghijk", "Harbour Keeper", ""},
		{"no submitter", map[string]any{"videoId": "abcdefghijk", "submitter": nil},
			"https://unison.betterlyrics.org/song/abcdefghijk", "", ""},
		{"no video", map[string]any{"submitter": map[string]any{"displayName": "Harbour Keeper", "keyId": "0f1e2d3c"}},
			"https://unison.betterlyrics.org", "Harbour Keeper", "https://unison.betterlyrics.org/curator/0f1e2d3c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				data := map[string]any{"lyrics": richsync, "format": "ttml", "syncType": "richsync", "language": "en"}
				for k, v := range tc.data {
					data[k] = v
				}
				json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
			}))
			t.Cleanup(srv.Close)
			u := NewUnison("test-agent")
			u.BaseURL = srv.URL
			q := song
			q.VideoID = "abcdefghijk"
			l, err := u.Get(context.Background(), q)
			if err != nil {
				t.Fatal(err)
			}
			if l.Source != "Unison" || l.SourceURL != tc.sourceURL || l.By != tc.by || l.ByURL != tc.byURL {
				t.Errorf("credit = %q, %q, %q, %q; want Unison, %q, %q, %q", l.Source, l.SourceURL, l.By, l.ByURL, tc.sourceURL, tc.by, tc.byURL)
			}
		})
	}
}

// TestUnisonBySong looks a track without a video ID up by its title,
// artist, album and duration in seconds.
func TestUnisonBySong(t *testing.T) {
	u := unisonServer(t, "lrc", "[00:01.00]Hello\n[00:02.50]Again", func(r *http.Request) {
		q := r.URL.Query()
		if q.Get("song") != "Song" || q.Get("artist") != "Artist" || q.Get("album") != "Album" || q.Get("duration") != "200" || q.Has("v") {
			t.Errorf("query = %v", q)
		}
	})
	l, err := u.Get(context.Background(), song)
	if err != nil || !l.Synced || len(l.Lines) != 2 || saidOf(l.Lines)[1] != (said{2500, "Again"}) {
		t.Fatalf("lyrics = %+v, %v", l, err)
	}

	// Unison needs the artist with the title: without one it isn't asked.
	if _, err := u.Get(context.Background(), Query{Title: "Song"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("no artist: %v", err)
	}
}

func TestUnisonFormats(t *testing.T) {
	for _, c := range []struct {
		format, text string
		synced       bool
		first        string
	}{
		{"plain", "One\nTwo", false, "One"},
		{"lrc", "no times here", false, "no times here"},
		// An unknown format is read as what it looks like.
		{"", "[00:03.00]Timed\n[00:06.00]Again", true, "Timed"},
		{"srt", `<tt><body><p begin="1s">Marked up</p><p begin="4s">Again</p></body></tt>`, true, "Marked up"},
		// Times that can't be followed: plain (#277).
		{"lrc", "[00:00.00]probe", false, "probe"},
		{"ttml", `<tt><body><p begin="0s">One</p><p begin="0s">Two</p></body></tt>`, false, "One"},
		{"", "Just text", false, "Just text"},
	} {
		l, err := unisonServer(t, c.format, c.text, nil).Get(context.Background(), song)
		if err != nil || l.Synced != c.synced || len(l.Lines) == 0 || l.Lines[0].Text != c.first {
			t.Errorf("%s %q: lyrics = %+v, %v", c.format, c.text, l, err)
		}
	}
	// Lyrics that can't be read are none at all, as are empty ones.
	for _, text := range []string{`<tt><body><p begin="1s">Cut`, "  "} {
		if _, err := unisonServer(t, "ttml", text, nil).Get(context.Background(), song); !errors.Is(err, ErrNotFound) {
			t.Errorf("%q: err = %v", text, err)
		}
	}
}

// TestUnisonMisses tells a track Unison has no lyrics of from Unison down,
// out of reach, or too slow.
func TestUnisonMisses(t *testing.T) {
	answer := func(status int, body string) *Unison {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			w.Write([]byte(body))
		}))
		t.Cleanup(srv.Close)
		u := NewUnison("test")
		u.BaseURL = srv.URL
		return u
	}
	notFound := `{"success":false,"error":"Lyrics not found","code":"NOT_FOUND","hint":"Try a different search."}`
	if _, err := answer(http.StatusNotFound, notFound).Get(context.Background(), song); !errors.Is(err, ErrNotFound) || errors.Is(err, ErrUnavailable) {
		t.Errorf("NOT_FOUND: err = %v", err)
	}
	for _, c := range []struct {
		status int
		body   string
	}{
		{http.StatusServiceUnavailable, `{"success":false}`},
		{http.StatusTooManyRequests, `{"success":false,"code":"RATE_LIMITED"}`},
		// A 404 that isn't Unison's NOT_FOUND, as from a proxy, is Unison
		// out of reach.
		{http.StatusNotFound, `<html>Not Found</html>`},
		{http.StatusOK, `{"success":tru`},
	} {
		_, err := answer(c.status, c.body).Get(context.Background(), song)
		if !errors.Is(err, ErrUnavailable) || errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "Unison isn't answering") {
			t.Errorf("HTTP %d %s: err = %v", c.status, c.body, err)
		}
	}

	gone := answer(http.StatusOK, "")
	gone.BaseURL = "http://127.0.0.1:1"
	if _, err := gone.Get(context.Background(), song); !errors.Is(err, ErrUnavailable) {
		t.Errorf("out of reach: err = %v", err)
	}

	stuck := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-stuck }))
	defer srv.Close()
	defer close(stuck)
	slow := NewUnison("test")
	slow.BaseURL = srv.URL
	slow.HTTP.Timeout = 50 * time.Millisecond
	if _, err := slow.Get(context.Background(), song); !errors.Is(err, ErrUnavailable) {
		t.Errorf("too slow: err = %v", err)
	}
	if NewUnison("test").HTTP.Timeout > 5*time.Second {
		t.Error("lyrics wait long on Unison")
	}
}

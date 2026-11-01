package lyrics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

func TestParseLRC(t *testing.T) {
	lrc := "[ar:Someone]\n[ti:Song]\n[00:12.50]First line\n[00:05.00][01:00.1]Repeated\n[00:20]\nno time here\n[00:30.123] Spaced  "
	got := ParseLRC(lrc)
	want := []Line{{5000, "Repeated"}, {12500, "First line"}, {20000, ""}, {30123, "Spaced"}, {60100, "Repeated"}}
	if len(got) != len(want) {
		t.Fatalf("lines = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseLRCOffset(t *testing.T) {
	got := ParseLRC("[offset:+500]\n[00:10.00]Line")
	if len(got) != 1 || got[0].TimeMs != 9500 {
		t.Errorf("lines = %+v", got)
	}
}

func TestPlain(t *testing.T) {
	l := Plain("a\r\nb\n\nc", "test")
	if l.Synced || len(l.Lines) != 4 || l.Lines[1].Text != "b" {
		t.Errorf("plain = %+v", l)
	}
}

var song = media.Track{Title: "Song", Artists: []media.Ref{{Name: "Artist"}}, Album: &media.Ref{Name: "Album"}, DurationMs: 200400}

func TestGetExact(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/get" || q.Get("duration") != "200" || q.Get("album_name") != "Album" || r.Header.Get("User-Agent") != "test-agent" {
			t.Errorf("request = %s %v", r.URL.Path, q)
		}
		w.Write([]byte(`{"syncedLyrics": "[00:01.00]Hello", "plainLyrics": "Hello"}`))
	}))
	defer srv.Close()
	c := New("test-agent")
	c.BaseURL = srv.URL
	l, err := c.Get(context.Background(), song)
	if err != nil || !l.Synced || l.Lines[0].Text != "Hello" {
		t.Fatalf("lyrics = %+v, %v", l, err)
	}
}

func TestGetFallsBackToSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/get" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(`[
			{"duration": 150, "syncedLyrics": "[00:01.00]Wrong length"},
			{"duration": 201, "plainLyrics": "Plain"},
			{"duration": 199, "syncedLyrics": "[00:01.00]Right"}]`))
	}))
	defer srv.Close()
	c := New("test")
	c.BaseURL = srv.URL
	l, err := c.Get(context.Background(), song)
	if err != nil || l.Lines[0].Text != "Right" {
		t.Fatalf("lyrics = %+v, %v", l, err)
	}
}

func TestGetNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/get" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	c := New("test")
	c.BaseURL = srv.URL
	if _, err := c.Get(context.Background(), song); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v", err)
	}
}

// LRCLIB down, or out of reach, is not a track without lyrics.
func TestGetUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	c := New("test")
	c.BaseURL = srv.URL
	_, err := c.Get(context.Background(), song)
	if !errors.Is(err, ErrUnavailable) || errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "HTTP 503") {
		t.Errorf("LRCLIB down: err = %v", err)
	}
	srv.Close()
	if _, err := c.Get(context.Background(), song); !errors.Is(err, ErrUnavailable) {
		t.Errorf("LRCLIB out of reach: err = %v", err)
	}
}

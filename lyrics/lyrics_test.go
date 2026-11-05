package lyrics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseLRC(t *testing.T) {
	lrc := "[ar:Someone]\n[ti:Song]\n[00:12.50]First line\n[00:05.00][01:00.1]Repeated\n[00:20]\nno time here\n[00:30.123] Spaced  "
	lines := ParseLRC(lrc)
	got := saidOf(lines)
	want := []said{{5000, "Repeated"}, {12500, "First line"}, {20000, ""}, {30123, "Spaced"}, {60100, "Repeated"}}
	if len(got) != len(want) {
		t.Fatalf("lines = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] || lines[i].Words != nil {
			t.Errorf("line %d = %+v, want %+v", i, lines[i], want[i])
		}
	}
}

// said is when a line is sung and its text, which tests compare.
type said struct {
	ms   int
	text string
}

func saidOf(lines []Line) []said {
	out := make([]said, len(lines))
	for i, l := range lines {
		out[i] = said{l.TimeMs, l.Text}
	}
	return out
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

var song = Query{Title: "Song", Artist: "Artist", Album: "Album", Duration: 200400 * time.Millisecond}

func TestGetExact(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/get" || q.Get("duration") != "200" || q.Get("album_name") != "Album" || r.Header.Get("User-Agent") != "test-agent" {
			t.Errorf("request = %s %v", r.URL.Path, q)
		}
		w.Write([]byte(`{"syncedLyrics": "[00:01.00]Hello\n[00:04.00]Again", "plainLyrics": "Hello\nAgain"}`))
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
			{"duration": 150, "syncedLyrics": "[00:01.00]Wrong length\n[00:04.00]Again"},
			{"duration": 201, "plainLyrics": "Plain"},
			{"duration": 199, "syncedLyrics": "[00:01.00]Right\n[00:04.00]Again"}]`))
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

// busy is LRCLIB too busy for its search and the lookups it makes on
// other sites, as it answered on 2026-11-01, whose own database answers
// with cached for the exact track.
func busy(cached func(w http.ResponseWriter)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/get-cached" {
			cached(w)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"message":"The server is busy, please retry in a moment","name":"ServerOverloaded","statusCode":503}`))
	}))
}

// TestGetBusy takes the lyrics LRCLIB's database has while LRCLIB is busy,
// and tells a track it hasn't from LRCLIB out of reach.
func TestGetBusy(t *testing.T) {
	srv := busy(func(w http.ResponseWriter) {
		w.Write([]byte(`{"duration": 200, "syncedLyrics": "[00:01.00]Cached\n[00:04.00]Again"}`))
	})
	defer srv.Close()
	c := New("test")
	c.BaseURL = srv.URL
	if l, err := c.Get(context.Background(), song); err != nil || l.Lines[0].Text != "Cached" {
		t.Errorf("busy, in the database: lyrics = %+v, %v", l, err)
	}

	missing := busy(func(w http.ResponseWriter) { w.WriteHeader(http.StatusNotFound) })
	defer missing.Close()
	c.BaseURL = missing.URL
	_, err := c.Get(context.Background(), song)
	if !errors.Is(err, ErrNotFoundYet) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrUnavailable) {
		t.Errorf("busy, not in the database: err = %v", err)
	}
}

// TestGetSearchBusy tells a track LRCLIB found no exact lyrics of, and
// was too busy to search for, as not found yet.
func TestGetSearchBusy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/get" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := New("test")
	c.BaseURL = srv.URL
	if _, err := c.Get(context.Background(), song); !errors.Is(err, ErrNotFoundYet) {
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

// TestTimedLyrics has lyrics synced only when they can be followed in time:
// two lines or more at different times. One line, or lines all at one
// time, are plain, without their times (#277).
func TestTimedLyrics(t *testing.T) {
	for _, c := range []struct {
		what   string
		lrc    string
		synced bool
	}{
		{"lines at their times", "[00:01.00]One\n[00:04.00]Two", true},
		{"one line", "[00:01.00]One", false},
		{"one line at the start", "[00:00.00]probe", false},
		{"lines all at one time", "[00:00.00]One\n[00:00.00]Two", false},
		{"one line among empty ones", "[00:00.00]\n[00:05.00]One\n[00:09.00]", false},
	} {
		l := timedLyrics(ParseLRC(c.lrc), "LRCLIB")
		if l.Synced != c.synced || l.Source != "LRCLIB" || len(l.Lines) == 0 {
			t.Errorf("%s: lyrics = %+v", c.what, l)
			continue
		}
		if !l.Synced && l.Lines[len(l.Lines)-1].TimeMs != 0 {
			t.Errorf("%s: plain lyrics keep times: %+v", c.what, l.Lines)
		}
	}
}

// TestGetJunkSynced takes LRCLIB's plain lyrics where its synced ones
// can't be followed in time, and the synced ones as plain where it has no
// others (#277).
func TestGetJunkSynced(t *testing.T) {
	for _, c := range []struct{ answer, first string }{
		{`{"syncedLyrics": "[00:00.00]probe", "plainLyrics": "Line one\nLine two"}`, "Line one"},
		{`{"syncedLyrics": "[00:00.00]probe"}`, "probe"},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(c.answer)) }))
		cl := New("test-agent")
		cl.BaseURL = srv.URL
		l, err := cl.Get(context.Background(), song)
		srv.Close()
		if err != nil || l.Synced || len(l.Lines) == 0 || l.Lines[0].Text != c.first {
			t.Errorf("%s: lyrics = %+v, %v", c.answer, l, err)
		}
	}
}

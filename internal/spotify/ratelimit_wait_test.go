package spotify

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// waitServer answers the page of a playlist with the 429 of each entry of
// answers, in turn, then with an empty last page.
func waitServer(t *testing.T, answers ...func(http.ResponseWriter)) (*Client, *atomic.Int32) {
	t.Helper()
	var asked atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n := int(asked.Add(1)) - 1; n < len(answers) {
			answers[n](w)
			return
		}
		w.Write([]byte(`{"items": []}`))
	}))
	t.Cleanup(srv.Close)
	return testClient(t, srv), &asked
}

func tooMany(retryAfter string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.Header().Set("Retry-After", retryAfter)
		w.WriteHeader(http.StatusTooManyRequests)
	}
}

// TestBackgroundReadWaitsOutRetryAfter has a page answered with a wait
// longer than the client's own limit: a read in the background waits it
// out and gets the page, with the time left told.
func TestBackgroundReadWaitsOutRetryAfter(t *testing.T) {
	retryAfterShorter = 50
	t.Cleanup(func() { retryAfterShorter = 1 })
	c, asked := waitServer(t, tooMany("30"))
	var told atomic.Int32
	ctx := media.WithPageWait(context.Background(), func(left time.Duration) {
		if left > 0 {
			told.Add(1)
		}
	})
	_, next, err := c.PlaylistPage(ctx, c.BaseURL+"/playlists/p1/items?offset=1")
	if err != nil || next != "" || asked.Load() != 2 {
		t.Fatalf("page: next %q, %v, asked %d times", next, err, asked.Load())
	}
	if told.Load() == 0 {
		t.Error("the wait was not told")
	}
}

// TestRetryAfterOverTheLimitFails keeps the wait on the error when it is
// longer than a background read waits.
func TestRetryAfterOverTheLimitFails(t *testing.T) {
	c, asked := waitServer(t, tooMany("90"))
	ctx := media.WithPageWait(context.Background(), nil)
	_, _, err := c.PlaylistPage(ctx, c.BaseURL+"/playlists/p1/items?offset=1")
	var api *APIError
	if !errors.As(err, &api) || api.Status != http.StatusTooManyRequests || api.RetryAfter != 90*time.Second || asked.Load() != 1 {
		t.Fatalf("got %v (asked %d times)", err, asked.Load())
	}
}

// TestListenerCallKeepsTheShortLimit fails a call that is not a read in
// the background at once, as before.
func TestListenerCallKeepsTheShortLimit(t *testing.T) {
	c, asked := waitServer(t, tooMany("30"))
	_, _, err := c.PlaylistPage(context.Background(), c.BaseURL+"/playlists/p1/items?offset=1")
	var api *APIError
	if !errors.As(err, &api) || api.RetryAfter != 30*time.Second || asked.Load() != 1 {
		t.Fatalf("got %v (asked %d times)", err, asked.Load())
	}
}

// TestPlainTextRefusalIsNotAWait does not wait for the 403 an app in
// development mode answers in plain text.
func TestPlainTextRefusalIsNotAWait(t *testing.T) {
	c, asked := waitServer(t, func(w http.ResponseWriter) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("User not registered in the Developer Dashboard"))
	})
	ctx := media.WithPageWait(context.Background(), nil)
	_, _, err := c.PlaylistPage(ctx, c.BaseURL+"/playlists/p1/items?offset=1")
	if !errors.Is(err, ErrUnregistered) || asked.Load() != 1 {
		t.Fatalf("got %v (asked %d times)", err, asked.Load())
	}
}

// TestWaitEndsWithTheContext stops a wait at once when the read is
// dropped.
func TestWaitEndsWithTheContext(t *testing.T) {
	c, _ := waitServer(t, tooMany("30"))
	ctx, cancel := context.WithCancel(context.Background())
	ctx = media.WithPageWait(ctx, func(left time.Duration) {
		if left > 0 {
			cancel()
		}
	})
	start := time.Now()
	_, _, err := c.PlaylistPage(ctx, c.BaseURL+"/playlists/p1/items?offset=1")
	if !errors.Is(err, context.Canceled) || time.Since(start) > 10*time.Second {
		t.Fatalf("got %v after %v", err, time.Since(start))
	}
}

// TestPlaylistReadFinishesAfterALongWait reads a playlist of three pages
// whose second meets a 429 with a Retry-After of 30 seconds (shortened):
// all its tracks come, with no error.
func TestPlaylistReadFinishesAfterALongWait(t *testing.T) {
	retryAfterShorter = 100
	t.Cleanup(func() { retryAfterShorter = 1 })
	var asked atomic.Int32
	var base string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("offset")
		if page == "2" && asked.Add(1) == 1 {
			tooMany("30")(w)
			return
		}
		item := func(id string) string {
			return `{"item": {"id": "` + id + `", "name": "T", "duration_ms": 1000, "type": "track", "uri": "spotify:track:` + id + `", "artists": [{"id": "a", "name": "A"}]}}`
		}
		switch page {
		case "1":
			w.Write([]byte(`{"total": 3, "items": [` + item("t2") + `], "next": "` + base + `/playlists/p/items?offset=2"}`))
		default:
			w.Write([]byte(`{"total": 3, "items": [` + item("t3") + `]}`))
		}
	}))
	t.Cleanup(srv.Close)
	base = srv.URL
	c := testClient(t, srv)
	p := media.Playlist{Source: media.SourceSpotify, TrackCount: 3, More: base + "/playlists/p/items?offset=1"}
	err := p.ReadOn(media.WithPageWait(context.Background(), nil), c.PlaylistPage, nil)
	if err != nil || len(p.Tracks) != 2 || p.More != "" {
		t.Fatalf("read: %d tracks, more %q, %v", len(p.Tracks), p.More, err)
	}
}

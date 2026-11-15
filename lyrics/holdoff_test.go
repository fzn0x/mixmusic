package lyrics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// countingServer answers every request with status and header, and keeps
// the paths asked.
func countingServer(t *testing.T, status int, retryAfter string) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		if retryAfter != "" {
			w.Header().Set("Retry-After", retryAfter)
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), paths...)
	}
}

func TestLRCLIB429HoldsOffWithRetryAfter(t *testing.T) {
	srv, asked := countingServer(t, http.StatusTooManyRequests, "60")
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	c := New("test")
	c.BaseURL = srv.URL
	c.wait.now = func() time.Time { return now }

	_, err := c.Get(context.Background(), Query{Title: "a", Artist: "b"})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Get = %v, want ErrUnavailable", err)
	}
	if got := asked(); len(got) != 1 || got[0] != "/get" {
		t.Fatalf("asked %v, want /get only: a 429 isn't followed by /get-cached", got)
	}
	// The wait is 60 s plus up to 30%.
	if until := c.wait.until.Sub(now); until < 60*time.Second || until > 78*time.Second {
		t.Fatalf("held off for %v, want 60 s to 78 s", until)
	}

	now = now.Add(59 * time.Second)
	if _, err := c.Get(context.Background(), Query{Title: "c", Artist: "d"}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Get during the wait = %v, want ErrUnavailable", err)
	}
	if got := asked(); len(got) != 1 {
		t.Fatalf("asked %v during the wait, want no new request", got)
	}

	now = now.Add(20 * time.Second)
	c.Get(context.Background(), Query{Title: "c", Artist: "d"})
	if got := asked(); len(got) != 2 || got[1] != "/get" {
		t.Fatalf("asked %v after the wait, want /get again", got)
	}
}

func TestLRCLIB429WithoutRetryAfterWaitsAMinute(t *testing.T) {
	srv, asked := countingServer(t, http.StatusTooManyRequests, "")
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	c := New("test")
	c.BaseURL = srv.URL
	c.wait.now = func() time.Time { return now }

	c.Get(context.Background(), Query{Title: "a", Artist: "b"})
	if until := c.wait.until.Sub(now); until < defaultHoldoff || until > defaultHoldoff*13/10 {
		t.Fatalf("held off for %v, want a minute to 78 s", until)
	}
	c.Get(context.Background(), Query{Title: "c", Artist: "d"})
	if got := asked(); len(got) != 1 {
		t.Fatalf("asked %v, want one request", got)
	}
}

func TestLRCLIB429RetryAfterDate(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	srv, _ := countingServer(t, http.StatusTooManyRequests, now.Add(10*time.Minute).UTC().Format(http.TimeFormat))
	c := New("test")
	c.BaseURL = srv.URL
	c.wait.now = func() time.Time { return now }

	c.Get(context.Background(), Query{Title: "a", Artist: "b"})
	if until := c.wait.until.Sub(now); until < 10*time.Minute || until > 13*time.Minute {
		t.Fatalf("held off for %v, want 10 to 13 minutes", until)
	}
}

func TestLRCLIB503StillAsksGetCached(t *testing.T) {
	srv, asked := countingServer(t, http.StatusServiceUnavailable, "")
	c := New("test")
	c.BaseURL = srv.URL
	_, err := c.Get(context.Background(), Query{Title: "a", Artist: "b"})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Get = %v, want ErrUnavailable", err)
	}
	if got := asked(); len(got) != 2 || got[1] != "/get-cached" {
		t.Fatalf("asked %v, want /get then /get-cached", got)
	}
	if c.wait.active() {
		t.Fatal("a 503 held LRCLIB off")
	}
}

func TestOtherSources429HoldOff(t *testing.T) {
	q := Query{Title: "a", Artist: "b", Album: "c", Duration: 3 * time.Minute}
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	t.Run("Better Lyrics", func(t *testing.T) {
		srv, asked := countingServer(t, http.StatusTooManyRequests, "30")
		b := NewBetterLyrics("test")
		b.BaseURL = srv.URL
		b.wait.now = func() time.Time { return now }
		for range 2 {
			if _, err := b.Get(context.Background(), q); !errors.Is(err, ErrUnavailable) {
				t.Fatalf("Get = %v, want ErrUnavailable", err)
			}
		}
		if got := asked(); len(got) != 1 {
			t.Fatalf("asked %v, want one request", got)
		}
		now = now.Add(time.Minute)
		b.Get(context.Background(), q)
		if got := asked(); len(got) != 2 {
			t.Fatalf("asked %v after the wait, want a second request", got)
		}
	})
	t.Run("Unison", func(t *testing.T) {
		srv, asked := countingServer(t, http.StatusTooManyRequests, "30")
		u := NewUnison("test")
		u.BaseURL = srv.URL
		u.wait.now = func() time.Time { return now }
		for range 2 {
			if _, err := u.Get(context.Background(), q); !errors.Is(err, ErrUnavailable) {
				t.Fatalf("Get = %v, want ErrUnavailable", err)
			}
		}
		if got := asked(); len(got) != 1 {
			t.Fatalf("asked %v, want one request", got)
		}
		now = now.Add(time.Minute)
		u.Get(context.Background(), q)
		if got := asked(); len(got) != 2 {
			t.Fatalf("asked %v after the wait, want a second request", got)
		}
	})
}

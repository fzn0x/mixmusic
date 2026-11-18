package tools

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock stands in for the clock of fetch's tries: its waits pass at
// once and move its time on, and it records them.
type fakeClock struct {
	now   time.Time
	waits []time.Duration
}

// fakeWaits has fetch's tries wait on a fake clock until the test ends.
func fakeWaits(t *testing.T) *fakeClock {
	t.Helper()
	c := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	savedNow, savedWait := retryNow, retryWait
	retryNow = func() time.Time { return c.now }
	retryWait = func(ctx context.Context, d time.Duration) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		c.waits = append(c.waits, d)
		c.now = c.now.Add(d)
		return nil
	}
	t.Cleanup(func() { retryNow, retryWait = savedNow, savedWait })
	return c
}

// logged returns what the log says until the test ends.
func logged(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	saved := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(saved) })
	return &buf
}

// seconds turns seconds into durations.
func seconds(s ...int) []time.Duration {
	d := make([]time.Duration, len(s))
	for i, n := range s {
		d[i] = time.Duration(n) * time.Second
	}
	return d
}

// TestInstallTriesAgain installs yt-dlp when GitHub answers a 502, breaks
// the download off, then answers a 504 and a 429: a second after the
// first failure, then twice as long after each, with the progress from
// zero at each try (#538).
func TestInstallTriesAgain(t *testing.T) {
	g := newGitHub(t, &github{fails: []int{502, cut, 504, 429}})
	logs := logged(t)
	m := New(t.TempDir())
	type report struct{ done, total int64 }
	var reports []report
	err := m.InstallYTDLP(context.Background(), func(done, total int64) {
		reports = append(reports, report{done, total})
	})
	if err != nil {
		t.Fatalf("InstallYTDLP: %v; asked %q", err, g.asked)
	}
	if got := installed(m); got != string(g.program) {
		t.Errorf("installed %q, want the program", got)
	}
	if n := g.times("/latest/download/yt-dlp_linux"); n != 5 {
		t.Errorf("the program was asked %d times, want 5", n)
	}
	if g.askedFor("/download/") {
		t.Errorf("asked the release's address too: %q", g.asked)
	}
	if want := seconds(1, 2, 4, 8); !slices.Equal(g.clock.waits, want) {
		t.Errorf("waited %v, want %v", g.clock.waits, want)
	}
	size, half := int64(len(g.program)), int64(len(g.program)/2)
	broken := slices.Index(reports, report{half, size})
	if broken < 0 || !slices.Contains(reports[broken:], report{0, 0}) {
		t.Errorf("progress %v: want half of it, then from zero", reports)
	}
	if len(reports) == 0 || reports[len(reports)-1] != (report{size, size}) {
		t.Errorf("progress %v: want all of it last", reports)
	}
	if !strings.Contains(logs.String(), "/latest/download/yt-dlp_linux after 5 tries") {
		t.Errorf("the log doesn't say how many tries: %q", logs)
	}
}

// server answers each request with answer, counting them.
func server(t *testing.T, answer int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var asked atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.Add(1)
		w.WriteHeader(answer)
		w.Write([]byte("x"))
	}))
	t.Cleanup(srv.Close)
	return srv, &asked
}

// TestFetchGivesUp stops trying after two minutes in all, the last try
// at two minutes, and tells the last answer.
func TestFetchGivesUp(t *testing.T) {
	clock := fakeWaits(t)
	logs := logged(t)
	srv, asked := server(t, http.StatusServiceUnavailable)
	_, err := New(t.TempDir()).fetch(context.Background(), srv.URL+"/file", nil)
	if err == nil || err.Error() != "HTTP 503" {
		t.Errorf("fetch = %v, want HTTP 503", err)
	}
	if want := seconds(1, 2, 4, 8, 16, 32, 57); !slices.Equal(clock.waits, want) {
		t.Errorf("waited %v, want %v", clock.waits, want)
	}
	if n := asked.Load(); n != 8 {
		t.Errorf("asked %d times, want 8", n)
	}
	if !strings.Contains(logs.String(), "after 8 tries: HTTP 503") {
		t.Errorf("the log doesn't say how many tries: %q", logs)
	}
}

// TestFetchDoesNotRetry asks once for what won't come later: a 404, or
// another answer that isn't a 5xx or a 429.
func TestFetchDoesNotRetry(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusNotFound, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			clock := fakeWaits(t)
			logs := logged(t)
			srv, asked := server(t, status)
			_, err := New(t.TempDir()).fetch(context.Background(), srv.URL+"/file", nil)
			if (status == http.StatusOK) != (err == nil) {
				t.Errorf("fetch = %v", err)
			}
			if n := asked.Load(); n != 1 || len(clock.waits) != 0 || logs.Len() != 0 {
				t.Errorf("asked %d times, waited %v, logged %q; want once", n, clock.waits, logs)
			}
		})
	}
}

// TestFetchStopsWithContext stops trying once the caller's context ends,
// during a wait, with the last try's error.
func TestFetchStopsWithContext(t *testing.T) {
	clock := fakeWaits(t)
	logs := logged(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fake := retryWait
	retryWait = func(ctx context.Context, d time.Duration) error {
		if len(clock.waits) == 1 {
			cancel()
		}
		return fake(ctx, d)
	}
	srv, asked := server(t, http.StatusBadGateway)
	_, err := New(t.TempDir()).fetch(ctx, srv.URL+"/file", nil)
	if err == nil || err.Error() != "HTTP 502" {
		t.Errorf("fetch = %v, want HTTP 502", err)
	}
	if n := asked.Load(); n != 2 || !slices.Equal(clock.waits, seconds(1)) {
		t.Errorf("asked %d times, waited %v; want twice, a second between", n, clock.waits)
	}
	if !strings.Contains(logs.String(), "after 2 tries") {
		t.Errorf("the log doesn't say how many tries: %q", logs)
	}
}

// TestRetryWait waits as long as asked, and no longer than the context.
func TestRetryWait(t *testing.T) {
	if err := retryWait(context.Background(), time.Millisecond); err != nil {
		t.Errorf("a wait: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if err := retryWait(ctx, time.Hour); !errors.Is(err, context.Canceled) || time.Since(start) > time.Minute {
		t.Errorf("a wait of an hour, the context ended: %v after %v", err, time.Since(start))
	}
}

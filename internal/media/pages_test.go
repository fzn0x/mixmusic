package media

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"
)

// quickTries shortens the waits between the tries of a page, for a test.
func quickTries(t *testing.T) {
	t.Helper()
	waits := PageWaits
	PageWaits = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	t.Cleanup(func() { PageWaits = waits })
}

// book is a service's long list of tracks, read a page of size at a time
// by the number of its first track; fails says how many times the page of
// a token fails before it's read.
type book struct {
	mu    sync.Mutex
	total int
	size  int
	fails map[string]int
	asked []string
}

func (b *book) read(ctx context.Context, token string) ([]Track, string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.asked = append(b.asked, token)
	if b.fails[token] > 0 {
		b.fails[token]--
		return nil, "", errors.New("HTTP 503")
	}
	from, err := strconv.Atoi(token)
	if err != nil {
		return nil, "", err
	}
	var tracks []Track
	for i := from; i < min(from+b.size, b.total); i++ {
		tracks = append(tracks, Track{ID: "yt:" + strconv.Itoa(i)})
	}
	next := ""
	if from+b.size < b.total {
		next = strconv.Itoa(from + b.size)
	}
	return tracks, next, nil
}

// first is the playlist of b as its first page has it.
func (b *book) first(t *testing.T) Playlist {
	t.Helper()
	tracks, next, err := b.read(context.Background(), "0")
	if err != nil {
		t.Fatal(err)
	}
	return Playlist{ID: "yt:PL", Source: SourceYouTube, Tracks: tracks, More: next, TrackCount: b.total}
}

// inOrder reports whether tracks are the first of b's, each once.
func inOrder(tracks []Track) bool {
	for i, tr := range tracks {
		if tr.ID != "yt:"+strconv.Itoa(i) {
			return false
		}
	}
	return true
}

// TestReadOnReadsEveryPage reads a playlist to its end, a page at a time,
// telling each page as it comes, and leaves the tracks it was given as
// they were.
func TestReadOnReadsEveryPage(t *testing.T) {
	b := &book{total: 2350, size: 100}
	p := b.first(t)
	// What a caller shows holds the first 50, with room after them that
	// isn't its own.
	room := make([]Track, 51, 100)
	copy(room, p.Tracks[:50])
	room[50] = Track{ID: "someone else's"}
	p.Tracks, p.More = room[:50], "50"
	var told []int
	err := p.ReadOn(context.Background(), b.read, func(now Playlist) {
		told = append(told, len(now.Tracks))
		if cap(now.Tracks) != len(now.Tracks) {
			t.Errorf("a page told with room after its tracks, that the next page fills")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Tracks) != 2350 || !inOrder(p.Tracks) || p.More != "" || p.Cut {
		t.Fatalf("read %d tracks, in order %v, more %q, cut %v", len(p.Tracks), inOrder(p.Tracks), p.More, p.Cut)
	}
	if len(told) != 23 || told[0] != 150 || told[22] != 2350 {
		t.Errorf("told the pages at %v", told)
	}
	if room[50].ID != "someone else's" {
		t.Errorf("the pages went into the room after the tracks it was given: %+v", room[50])
	}
}

// TestReadOnTriesAgain asks again for a page that fails, a few times; one
// that fails each time stops the reading with its error, with the tracks
// read before it, and reads on from it when asked again.
func TestReadOnTriesAgain(t *testing.T) {
	quickTries(t)
	b := &book{total: 450, size: 100, fails: map[string]int{"200": 3}}
	p := b.first(t)
	if err := p.ReadOn(context.Background(), b.read, nil); err != nil || len(p.Tracks) != 450 || !inOrder(p.Tracks) {
		t.Fatalf("a page that failed three times: %d tracks, %v", len(p.Tracks), err)
	}

	b = &book{total: 450, size: 100, fails: map[string]int{"200": 4}}
	p = b.first(t)
	err := p.ReadOn(context.Background(), b.read, nil)
	if err == nil || len(p.Tracks) != 200 || p.More != "200" || p.Cut {
		t.Fatalf("a page that failed four times: %d tracks, more %q, %v", len(p.Tracks), p.More, err)
	}
	if asked := len(b.asked); asked != 2+4 {
		t.Errorf("asked %d times, %v", asked, b.asked)
	}
	if err := p.ReadOn(context.Background(), b.read, nil); err != nil || len(p.Tracks) != 450 || !inOrder(p.Tracks) {
		t.Errorf("read on: %d tracks, %v", len(p.Tracks), err)
	}
}

// TestReadOnStopsWaiting ends the waits between the tries of a page with
// the reading's context.
func TestReadOnStopsWaiting(t *testing.T) {
	b := &book{total: 450, size: 100, fails: map[string]int{"100": 4}}
	p := b.first(t)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	start := time.Now()
	if err := p.ReadOn(ctx, b.read, nil); err == nil {
		t.Fatal("a reading stopped gave no error")
	}
	if waited := time.Since(start); waited > time.Second {
		t.Errorf("stopped after %v", waited)
	}
}

// TestReadOnBound reads a playlist up to MaxTracks: one with more is cut
// there, and says so; one of that many tracks isn't.
func TestReadOnBound(t *testing.T) {
	for _, tc := range []struct {
		name  string
		total int
		size  int
		cut   bool
	}{
		{"more", MaxTracks + 1, 1000, true},
		{"a page past it", MaxTracks + 600, 1500, true},
		{"as many", MaxTracks, 1000, false},
	} {
		b := &book{total: tc.total, size: tc.size}
		p := b.first(t)
		if err := p.ReadOn(context.Background(), b.read, nil); err != nil {
			t.Fatal(err)
		}
		if len(p.Tracks) != MaxTracks || !inOrder(p.Tracks) || p.Cut != tc.cut || p.More != "" {
			t.Errorf("%s: %d tracks, cut %v, more %q", tc.name, len(p.Tracks), p.Cut, p.More)
		}
		if asked := len(b.asked); asked != (MaxTracks+tc.size-1)/tc.size {
			t.Errorf("%s: asked for %d pages", tc.name, asked)
		}
	}
}

// TestReadOnEnds ends a list whose service gives pages of nothing, or the
// same page again, with the token of another, and counts the tracks of a
// playlist that didn't say how many it has.
func TestReadOnEnds(t *testing.T) {
	asked := 0
	empty := func(ctx context.Context, token string) ([]Track, string, error) {
		asked++
		return nil, "next-" + strconv.Itoa(asked), nil
	}
	p := Playlist{Tracks: []Track{{ID: "yt:0"}}, More: "next-0"}
	if err := p.ReadOn(context.Background(), empty, nil); err != nil || asked != maxEmptyPages || p.More != "" {
		t.Errorf("pages of nothing: asked %d times, more %q, %v", asked, p.More, err)
	}
	if p.TrackCount != 1 {
		t.Errorf("the count of a playlist read whole is %d", p.TrackCount)
	}
	asked = 0
	same := func(ctx context.Context, token string) ([]Track, string, error) {
		asked++
		return []Track{{ID: "yt:" + strconv.Itoa(asked)}}, token, nil
	}
	p = Playlist{More: "again"}
	if err := p.ReadOn(context.Background(), same, nil); err != nil || asked != 1 || len(p.Tracks) != 1 {
		t.Errorf("the same page again: asked %d times, %d tracks, %v", asked, len(p.Tracks), err)
	}
}

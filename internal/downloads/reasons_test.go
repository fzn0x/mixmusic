package downloads

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/internal/match"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/tools"
)

// TestFailureReasons keeps the reason of a failure the windows tell in
// their language, beside its English words, and in the index too, so that
// it is told after MixMusic starts again.
func TestFailureReasons(t *testing.T) {
	dir := t.TempDir()
	m := open(t, dir, Options{Fetcher: &fetcher{fail: func(string, int) error { return tools.ErrNoYTDLP }}, Matcher: &matcher{}})
	m.Add([]media.Track{yt("qqqqqqqqqqq", "Kids"), sp("none", "Nothing")}, "best")
	m.Start()
	until(t, "the failures", func() bool { return kind(m, "yt:qqqqqqqqqqq") == Failed && kind(m, "sp:none") == Failed })
	for id, want := range map[string]string{"yt:qqqqqqqqqqq": ReasonNeedsYTDLP, "sp:none": ReasonNoMatch} {
		if st := m.State(id); st.Reason != want || st.Why == "" {
			t.Errorf("%s: %+v, want the reason %q", id, st, want)
		}
	}
	m.Close()
	m = open(t, dir, Options{Fetcher: &fetcher{}})
	if st := m.State("sp:none"); st.Kind != Failed || st.Reason != ReasonNoMatch {
		t.Errorf("after a start: %+v", st)
	}
	// Tried again, it forgets why it failed.
	m.RetryFailed()
	if st := m.State("sp:none"); st.Reason != "" {
		t.Errorf("tried again: %+v", st)
	}
}

// TestReasonOf names the failures the windows tell in their language.
func TestReasonOf(t *testing.T) {
	for err, want := range map[error]string{
		tools.ErrNoYTDLP: ReasonNeedsYTDLP, match.ErrNoMatch: ReasonNoMatch, errBadAudio: ReasonBadAudio,
		errNotYouTube: ReasonNotYouTube, errNoMatcher: ReasonNoMatcher, errors.New("something else"): "",
	} {
		if got := reasonOf(err); got != want {
			t.Errorf("reasonOf(%v) = %q, want %q", err, got, want)
		}
	}
}

// TestHoldKinds tells the kind of what holds the downloads, and what it
// names, beside its English words.
func TestHoldKinds(t *testing.T) {
	m := open(t, t.TempDir(), Options{Fetcher: &fetcher{}})
	m.SetPaused(true)
	if st := m.Status(); st.HoldKind != HoldPaused || st.Hold != "Paused" {
		t.Errorf("paused: %+v", st)
	}
	m.SetPaused(false)
	m.SetMetered(true, false)
	if st := m.Status(); st.HoldKind != HoldMetered {
		t.Errorf("metered: %+v", st)
	}
	m.SetMetered(false, false)
	if st := m.Status(); st.HoldKind != "" || len(st.HoldArgs) != 0 {
		t.Errorf("released: %+v", st)
	}
	file := filepath.Join(t.TempDir(), "file")
	os.WriteFile(file, []byte("x"), 0o644)
	gone := filepath.Join(file, "downloads")
	m = open(t, gone, Options{Fetcher: &fetcher{}})
	if st := m.Status(); st.HoldKind != HoldFolder || !slices.Equal(st.HoldArgs, []string{gone}) {
		t.Errorf("unreachable: %+v", st)
	}
}

// TestMoveErrors returns an error of its own for each move refused.
func TestMoveErrors(t *testing.T) {
	from := t.TempDir()
	cache := t.TempDir()
	m := open(t, from, Options{Fetcher: &fetcher{}, Avoid: []string{cache}})
	for dir, want := range map[string]error{from: ErrSameFolder, filepath.Join(from, "audio"): ErrIntoTheirs, cache: ErrCacheFolder} {
		if _, err := m.Move(context.Background(), dir, true, nil); !errors.Is(err, want) {
			t.Errorf("Move(%s) = %v, want %v", dir, err, want)
		}
	}
	var none *Manager
	if _, err := none.Move(context.Background(), from, true, nil); !errors.Is(err, ErrNoDownloads) {
		t.Errorf("no downloads: %v", err)
	}
}

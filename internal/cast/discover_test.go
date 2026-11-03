package cast_test

import (
	"context"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/cast"
	"github.com/fzn0x/mixmusic/internal/cast/casttest"
)

func browse(t *testing.T, r *casttest.Responder) []cast.Device {
	t.Helper()
	devices, err := cast.Browse(context.Background(), cast.BrowseOptions{To: r.Addr(), Wait: 300 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	return devices
}

// TestBrowse finds the cast devices that answer, by name, sorted, with
// their model and where they speak the protocol, but groups of speakers.
func TestBrowse(t *testing.T) {
	r := casttest.NewResponder(t,
		casttest.Entry{ID: "b2", Name: "Living Room TV", Model: "Chromecast", Port: 8009},
		casttest.Entry{ID: "a1", Name: "kitchen", Model: "Google Nest Mini", Port: 8010, Host: "192.168.1.20"},
		casttest.Entry{ID: "c3", Name: "Everywhere", Model: "Google Cast Group", Port: 32187},
	)
	got := browse(t, r)
	want := []cast.Device{
		{ID: "a1", Name: "kitchen", Model: "Google Nest Mini", Host: "192.168.1.20", Port: 8010},
		{ID: "b2", Name: "Living Room TV", Model: "Chromecast", Host: "127.0.0.1", Port: 8009},
	}
	if len(got) != len(want) {
		t.Fatalf("found %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("device %d: %+v, want %+v", i, got[i], want[i])
		}
	}
	if got[1].Addr() != "127.0.0.1:8009" {
		t.Errorf("address %q", got[1].Addr())
	}
}

// TestBrowseAsksForTheRest asks again for where a device is and what it
// says of itself, when it answered the first question alone; one that
// tells no address is where its answer came from.
func TestBrowseAsksForTheRest(t *testing.T) {
	r := casttest.NewResponder(t, casttest.Entry{ID: "d4", Name: "Bedroom", Model: "Google Home", Port: 8009, Host: "-"})
	r.Sparse()
	got := browse(t, r)
	if len(got) != 1 || got[0].Name != "Bedroom" || got[0].Host != "127.0.0.1" || got[0].Port != 8009 {
		t.Fatalf("found %+v", got)
	}
	if n := r.Queries(); n != 2 {
		t.Errorf("%d queries, want 2: the pointers, then the rest", n)
	}
}

// TestBrowseNothing finds nothing where nothing answers, after the wait,
// and stops as its context ends.
func TestBrowseNothing(t *testing.T) {
	r := casttest.NewResponder(t)
	if got := browse(t, r); len(got) != 0 {
		t.Errorf("found %+v", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := cast.Browse(ctx, cast.BrowseOptions{To: r.Addr(), Wait: time.Minute}); err == nil {
		t.Error("a browse whose context ended went on")
	}
}

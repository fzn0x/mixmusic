package scrobble

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"
)

// pickyAPI rejects every batch that holds a play of a title it refuses,
// as a service rejects a whole request for one listen it can't take.
type pickyAPI struct {
	refuse    string
	sizes     []int
	scrobbled []Play
}

func (f *pickyAPI) NowPlaying(context.Context, string, Track) error { return nil }

func (f *pickyAPI) Scrobble(_ context.Context, _ string, plays []Play) error {
	f.sizes = append(f.sizes, len(plays))
	if f.refuse != "" && slices.ContainsFunc(plays, func(p Play) bool { return p.Title == f.refuse }) {
		return rejection{"invalid listen"}
	}
	f.scrobbled = append(f.scrobbled, plays...)
	return nil
}

// TestScrobblerSplitsARejectedBatch drops the play at fault alone: the
// batch it was in goes again in halves, until it goes by itself.
func TestScrobblerSplitsARejectedBatch(t *testing.T) {
	api := &pickyAPI{refuse: "Bad"}
	s := New(api, "SK", Options{})
	var queue []Play
	for i := range 50 {
		title := fmt.Sprint("Song ", i)
		if i == 30 {
			title = "Bad"
		}
		queue = append(queue, Play{song(title, 3*time.Minute), t0.Add(time.Duration(i) * 3 * time.Minute)})
	}
	s.SetQueue(queue)
	ctx := context.Background()
	if err := s.Send(ctx); !IsRejected(err) {
		t.Fatalf("err = %v", err)
	}
	if err := s.Send(ctx); err != nil {
		t.Fatal(err)
	}
	wantQueue(t, s)
	want := slices.Delete(slices.Clone(queue), 30, 31)
	if !samePlays(api.scrobbled, want) {
		t.Errorf("scrobbled %d plays, want the 49 good ones in order", len(api.scrobbled))
	}
	// 50 are rejected; 25 go, the 25 left are rejected, 12 too, and 6;
	// 3 go, 3 are rejected, then one at a time; once the bad one is
	// dropped, the 19 left go at once.
	if want := []int{50, 25, 25, 12, 6, 3, 3, 1, 1, 1, 19}; !slices.Equal(api.sizes, want) {
		t.Errorf("batches of %v, want %v", api.sizes, want)
	}
}

// TestScrobblerBatchOption sends as many plays per call as the Options
// say.
func TestScrobblerBatchOption(t *testing.T) {
	api := &pickyAPI{}
	s := New(api, "SK", Options{Batch: 1000})
	s.SetQueue(make([]Play, 1500))
	if err := s.Send(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := []int{1000, 500}; !slices.Equal(api.sizes, want) {
		t.Errorf("batches of %v, want %v", api.sizes, want)
	}
}

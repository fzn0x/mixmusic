package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestBring asks the computer controlled for what plays there: it hands
// over its queue cleaned, a file without its path, and pauses as told
// that a track or more was taken, not when none was; with nothing
// playing, it hands nothing over.
func TestBring(t *testing.T) {
	h, _, d, sess := controlled(t)
	if !sess.Can(FeatureBring) || !sess.Can(FeatureModes) || !sess.Can(FeatureOutputs) {
		t.Fatalf("the computer controlled can %q", sess.features)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := sess.Bring(ctx); !errors.Is(err, ErrNothingThere) {
		t.Errorf("nothing playing there: %v", err)
	}
	h.mu.Lock()
	h.give = &HandOver{Queue: []QueueTrack{
		{Source: "youtube", ID: "yt:aaaaaaaaaaa", Title: "Paper Boats", Artwork: []Image{{URL: "http://example.com/a.jpg"}}},
		{Source: "file", ID: "file:L1VzZXJzL21lL3NhbHQuZmxhYw", Title: "Salt\x07", Artists: []Ref{{Name: "Kelp"}}, DurationMs: 200000},
	}, Index: 1, PositionMs: 61000, Playing: true, Shuffle: true, Repeat: "once"}
	h.mu.Unlock()
	got, err := sess.Bring(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 2 || len(got.Queue) != 2 || got.Index != 1 || got.PositionMs != 61000 || !got.Playing || !got.Shuffle || got.Repeat != "" {
		t.Errorf("brought %+v", got)
	}
	if f := got.Queue[1]; f.ID != "" || f.Title != "Salt" || f.DurationMs != 200000 || len(got.Queue[0].Artwork) != 0 {
		t.Errorf("the tracks brought %+v", got.Queue)
	}
	given := func() []Taken {
		h.mu.Lock()
		defer h.mu.Unlock()
		return slices.Clone(h.given)
	}
	if err := sess.Took(got, Taken{Kept: 0, Left: 2, CurrentLeft: true}); err != nil {
		t.Fatal(err)
	}
	// A pause comes after what came before it: the one taken below.
	if err := sess.Took(HandOver{ID: 9}, Taken{Kept: 1}); err != nil {
		t.Fatal(err)
	}
	if again, err := sess.Bring(ctx); err != nil || again.ID != 3 {
		t.Fatalf("brought again %+v, %v", again, err)
	}
	if err := sess.Took(HandOver{ID: 3}, Taken{Kept: 1, Left: 1}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "told it was taken", func() bool { return len(given()) == 1 }, func() any { return given() })
	if g := given(); g[0] != (Taken{ID: 3, Kept: 1, Left: 1}) {
		t.Errorf("told %+v", g)
	}

	// Removed, it hands nothing over.
	h.revoke(d.Identity.Fingerprint())
	if _, err := sess.Bring(ctx); err == nil {
		t.Error("handed over to a computer removed")
	}
}

// TestModes sends the commands of shuffle, repeat, mute, like and the
// place to play, cleaned, and the state tells them, with up to 40 tracks
// after the one that plays and the places, cleaned.
func TestModes(t *testing.T) {
	h, _, _, sess := controlled(t)
	next := make([]Track, 45)
	for i := range next {
		next[i] = Track{Title: fmt.Sprint("Track ", i), Entry: fmt.Sprint("e", i)}
	}
	h.setState(State{Track: &Track{Title: "Low Tide", Entry: "e"}, Next: next, Shuffle: true, Repeat: "one", Liked: true,
		Outputs: []Output{{ID: "default", Kind: OutputDefault}, {ID: "s1", Kind: OutputSound, Name: "Speakers\x00"},
			{ID: "c1", Kind: OutputGroup, Name: "Everywhere", Model: "Google Cast Group"}, {ID: "c1", Kind: OutputCast, Name: "Again"},
			{ID: "x", Kind: "teleporter"}, {ID: "has space", Kind: OutputSound}},
		Output: "c1"})
	waitFor(t, "the state", func() bool { st, _, _ := sess.State(); return st.Liked }, func() any { st, _, _ := sess.State(); return st })
	st, _, _ := sess.State()
	want := []Output{{ID: "default", Kind: OutputDefault}, {ID: "s1", Kind: OutputSound, Name: "Speakers"},
		{ID: "c1", Kind: OutputGroup, Name: "Everywhere", Model: "Google Cast Group"}}
	if len(st.Next) != MaxNext || !st.Shuffle || st.Repeat != "one" || st.Output != "c1" || !reflect.DeepEqual(st.Outputs, want) {
		t.Errorf("state %d next, %+v", len(st.Next), st)
	}
	sent := []Command{{Op: OpShuffle, Shuffle: true}, {Op: OpRepeat, Repeat: "all", Volume: 1}, {Op: OpMute, Muted: true},
		{Op: OpLike, Liked: true, Entry: "e3"}, {Op: OpLike}, {Op: OpOutput, Output: "s1"}}
	for _, c := range sent {
		if err := sess.Do(c); err != nil {
			t.Fatal(err)
		}
	}
	want2 := slices.Clone(sent)
	want2[1].Volume = 0
	waitFor(t, "the commands", func() bool { return reflect.DeepEqual(h.got(), want2) }, func() any { return h.got() })
	for _, c := range []Command{{Op: OpRepeat, Repeat: "twice"}, {Op: OpLike, Entry: "e 3"}, {Op: OpOutput}, {Op: OpOutput, Output: "a b"}} {
		if err := sess.Do(c); err == nil {
			t.Errorf("sent %+v", c)
		}
	}
}

// TestOlderHostModes neither asks an older computer for what plays there
// nor sends it the commands it doesn't take.
func TestOlderHostModes(t *testing.T) {
	_, key, addr := serveOlder(t)
	sess := connect(t, newDialer(t, "Laptop", fast), addr, key)
	waitFor(t, "the state", func() bool { _, _, ok := sess.State(); return ok }, func() any { return sess.Err() })
	if _, err := sess.Bring(context.Background()); !errors.Is(err, ErrOlder) {
		t.Errorf("bring: %v", err)
	}
	for _, c := range []Command{{Op: OpShuffle, Shuffle: true}, {Op: OpRepeat}, {Op: OpMute}, {Op: OpLike}, {Op: OpOutput, Output: "s1"}} {
		if err := sess.Do(c); !errors.Is(err, ErrOlder) {
			t.Errorf("%s: %v", c.Op, err)
		}
	}
}

// TestFitState sends no state longer than the protocol takes: the tracks
// after the one that plays are fewer then.
func TestFitState(t *testing.T) {
	long := strings.Repeat("\U0001F3B5", 300)
	next := make([]Track, MaxNext)
	for i := range next {
		artists := make([]string, 10)
		for j := range artists {
			artists[j] = strings.Repeat("\U0001F3B6", 200)
		}
		next[i] = Track{Title: long, Artists: artists, Album: long, Artwork: "https://example.com/" + strings.Repeat("a", 2000)}
	}
	big := State{Track: &Track{Title: "Low Tide"}, Next: next}
	for range 3 {
		big.Next = append(big.Next, big.Next...)
	}
	st := fitState(big)
	data, err := json.Marshal(message{Type: "state", State: &st})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) > maxMessage || len(st.Next) == 0 {
		t.Errorf("%d bytes, %d tracks next", len(data), len(st.Next))
	}
	if small := fitState(State{Next: next[:3]}); len(small.Next) != 3 {
		t.Errorf("a short list cut: %d", len(small.Next))
	}
}

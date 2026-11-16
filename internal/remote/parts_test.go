package remote

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/sponsorblock"
)

// TestCleanParts keeps the parts of the kinds this MixMusic knows, within
// the track, one highlight with no length, and MaxParts at most.
func TestCleanParts(t *testing.T) {
	got := cleanParts([]Part{
		{Kind: sponsorblock.Sponsor, FromMs: -500, ToMs: 20_000},
		{Kind: "teleport", FromMs: 30_000, ToMs: 40_000},
		{Kind: sponsorblock.Intro, FromMs: 50_000, ToMs: 50_000},
		{Kind: sponsorblock.Highlight, FromMs: 0},
		{Kind: sponsorblock.Highlight, FromMs: 150_000, ToMs: 151_000},
		{Kind: sponsorblock.Highlight, FromMs: 160_000},
		{Kind: sponsorblock.Outro, FromMs: 200_000, ToMs: 400_000},
		{Kind: sponsorblock.Filler, FromMs: 300_000, ToMs: 310_000},
	}, 211_000)
	want := []Part{{Kind: sponsorblock.Sponsor, FromMs: 0, ToMs: 20_000}, {Kind: sponsorblock.Highlight, FromMs: 150_000},
		{Kind: sponsorblock.Outro, FromMs: 200_000, ToMs: 211_000}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cleaned %+v, want %+v", got, want)
	}
	// A track of no length known bounds nothing.
	if got := cleanParts([]Part{{Kind: sponsorblock.Outro, FromMs: 200_000, ToMs: 400_000}}, 0); len(got) != 1 || got[0].ToMs != 400_000 {
		t.Errorf("of no length: %+v", got)
	}
	many := make([]Part, 2*MaxParts)
	for i := range many {
		many[i] = Part{Kind: sponsorblock.Sponsor, FromMs: int64(i) * 1000, ToMs: int64(i)*1000 + 500}
	}
	if got := cleanParts(many, 0); len(got) != MaxParts {
		t.Errorf("%d parts kept", len(got))
	}
	if got := cleanParts(nil, 0); got != nil {
		t.Errorf("none: %+v", got)
	}
}

// marked is the state of a computer whose player passes over a sponsor
// read and marks a highlight.
var marked = State{Track: &Track{Title: "Low Tide", Entry: "e0"}, Playing: true, PositionMs: 100_000, DurationMs: 211_000,
	Parts: []Part{{Kind: sponsorblock.Sponsor, FromMs: 20_000, ToMs: 60_000}, {Kind: sponsorblock.Highlight, FromMs: 150_000}}}

// TestPartsInState tells the parts of the computer controlled, cleaned,
// to the one that controls it.
func TestPartsInState(t *testing.T) {
	h, _, _, sess := controlled(t)
	st := marked
	st.Parts = append(slices.Clone(st.Parts), Part{Kind: "teleport", FromMs: 1, ToMs: 2})
	h.setState(st)
	waitFor(t, "the parts", func() bool {
		got, _, _ := sess.State()
		return reflect.DeepEqual(got.Parts, marked.Parts)
	}, func() any { got, _, _ := sess.State(); return got })
}

// TestPartsFromOlderHost shows what plays on a computer whose MixMusic is
// older, which tells no parts, as before: its states hold none.
func TestPartsFromOlderHost(t *testing.T) {
	_, key, addr := serveOlder(t)
	sess := connect(t, newDialer(t, "Laptop", fast), addr, key)
	waitFor(t, "the state", func() bool { _, _, ok := sess.State(); return ok }, func() any { return sess.Err() })
	st, _, _ := sess.State()
	if st.Track == nil || st.Track.Title != "Low Tide" || st.Parts != nil {
		t.Errorf("the state of an older computer: %+v", st)
	}
}

// olderState is State as an older MixMusic has it, without Parts.
type olderState struct {
	Track      *Track  `json:"track,omitempty"`
	Playing    bool    `json:"playing,omitempty"`
	Loading    bool    `json:"loading,omitempty"`
	Live       bool    `json:"live,omitempty"`
	PositionMs int64   `json:"positionMs,omitempty"`
	DurationMs int64   `json:"durationMs,omitempty"`
	Volume     float64 `json:"volume"`
	Muted      bool    `json:"muted,omitempty"`
	Next       []Track `json:"next,omitempty"`
}

// TestPartsToOlderController sends the parts in a field that a computer
// whose MixMusic is older, which controls this one, doesn't know: it reads
// the rest of the state as before.
func TestPartsToOlderController(t *testing.T) {
	h := newFakeHost()
	h.setState(marked)
	_, addr := serve(t, h, fast)
	d := newDialer(t, "Old laptop", fast)
	peer, err := pair(t, d, addr, h, same)
	if err != nil {
		t.Fatal(err)
	}
	<-h.ended
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tc, _, err := d.connect(ctx, addr, &peer.Key)
	if err != nil {
		t.Fatal(err)
	}
	defer tc.Close()
	// As an older MixMusic says hello: without features.
	w := newWire(tc)
	if err := w.send(message{Type: "hello", Want: wantControl, Name: d.Name, Version: version}, time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := w.expect("welcome", 5*time.Second); err != nil {
		t.Fatal(err)
	}
	var raw json.RawMessage
	if err := w.f.Receive(&raw, 5*time.Second); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"parts":[`) {
		t.Fatalf("no parts sent: %s", raw)
	}
	// What an older MixMusic reads it as.
	var old struct {
		Type  string      `json:"type"`
		State *olderState `json:"state,omitempty"`
	}
	if err := json.Unmarshal(raw, &old); err != nil {
		t.Fatalf("an older MixMusic can't read the state: %v", err)
	}
	if st := old.State; old.Type != "state" || st == nil || st.Track == nil || st.Track.Title != "Low Tide" || st.PositionMs != 100_000 || st.DurationMs != 211_000 || !st.Playing {
		t.Errorf("an older MixMusic reads %+v", old.State)
	}
}

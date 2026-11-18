package library

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestMergeData joins two libraries: a like here wins over a dislike in
// the file, a dislike here over a like, the mix changed last stays, the
// history is merged by time, each play once, and kept to its last 1,000.
func TestMergeData(t *testing.T) {
	tr := func(id string) media.Track { return media.Track{ID: id, Title: id} }
	at := func(h int) time.Time { return time.Date(2026, 11, 1, h, 0, 0, 0, time.UTC) }
	here := Data{
		Liked:     []media.Track{tr("a")},
		Disliked:  []media.Track{tr("b")},
		Playlists: []Playlist{{ID: "m1", Title: "older", Updated: at(1)}, {ID: "m2", Title: "only here", Updated: at(5)}},
		History:   []Play{{tr("a"), at(3)}, {tr("b"), at(1)}},
		Searches:  []string{"x"},
	}
	file := Data{
		Liked:     []media.Track{tr("b"), tr("c")},
		Disliked:  []media.Track{tr("a"), tr("d")},
		Playlists: []Playlist{{ID: "m1", Title: "newer", Updated: at(2)}, {ID: "m3", Title: "only there", Updated: at(9)}},
		History:   []Play{{tr("c"), at(2)}, {tr("b"), at(1)}},
		Searches:  []string{"y", "x"},
		Artists:   []FollowedArtist{{ID: "ar"}},
	}
	got, added := MergeData(here, file)
	ids := func(ts []media.Track) []string {
		var out []string
		for _, t := range ts {
			out = append(out, t.ID)
		}
		return out
	}
	if l := ids(got.Liked); !slices.Equal(l, []string{"a", "c"}) {
		t.Errorf("liked %q", l)
	}
	if d := ids(got.Disliked); !slices.Equal(d, []string{"b", "d"}) {
		t.Errorf("disliked %q", d)
	}
	var mixes []string
	for _, p := range got.Playlists {
		mixes = append(mixes, p.Title)
	}
	if !slices.Equal(mixes, []string{"only there", "only here", "newer"}) {
		t.Errorf("mixes %q", mixes)
	}
	var history []string
	for _, p := range got.History {
		history = append(history, p.Track.ID)
	}
	if !slices.Equal(history, []string{"a", "c", "b"}) {
		t.Errorf("history %q", history)
	}
	if !slices.Equal(got.Searches, []string{"x", "y"}) || len(got.Artists) != 1 {
		t.Errorf("searches %q, artists %d", got.Searches, len(got.Artists))
	}
	if added != (Added{Liked: 1, Disliked: 1, Mixes: 1, Artists: 1}) {
		t.Errorf("added %+v", added)
	}
	long := make([]Play, 0, 1500)
	for i := range 1500 {
		long = append(long, Play{tr("t"), at(0).Add(-time.Duration(i) * time.Minute)})
	}
	if got, _ := MergeData(Data{History: long[:800]}, Data{History: long[700:]}); len(got.History) != maxHistory || !got.History[0].PlayedAt.Equal(at(0)) {
		t.Errorf("a long history merged to %d plays", len(got.History))
	}
}

// TestMergePlays merges two play logs: every play of both, the oldest
// first, a play both hold once.
func TestMergePlays(t *testing.T) {
	dir := t.TempDir()
	here, file, out := filepath.Join(dir, "here.jsonl"), filepath.Join(dir, "file.jsonl"), filepath.Join(dir, "out.jsonl")
	os.WriteFile(here, []byte(`{"at":"2026-11-01T10:00:00Z","id":"a","title":"A"}`+"\n"+`{"at":"2026-11-03T10:00:00Z","id":"b","title":"B"}`+"\n"), 0o644)
	os.WriteFile(file, []byte(`{"at":"2026-11-02T10:00:00Z","id":"c","title":"C"}`+"\n"+`{"at":"2026-11-03T10:00:00Z","id":"b","title":"B"}`+"\n"), 0o644)
	if n, err := PlaysAdded(here, file); err != nil || n != 1 {
		t.Errorf("PlaysAdded = %d, %v", n, err)
	}
	if n, err := MergePlays(here, file, out); err != nil || n != 1 {
		t.Fatalf("MergePlays = %d, %v", n, err)
	}
	g, _, err := openPlayLog(out)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, p := range g.plays {
		order = append(order, g.tracks[p.track].ID)
	}
	if !slices.Equal(order, []string{"a", "c", "b"}) {
		t.Errorf("merged plays %q", order)
	}
}

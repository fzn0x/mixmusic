package main

import (
	"context"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestViewTracksByPlays puts the most played first; tracks played as
// often, and those never played, keep the list's order.
func TestViewTracksByPlays(t *testing.T) {
	tracks := []media.Track{{ID: "a", Title: "A"}, {ID: "b", Title: "B"}, {ID: "c", Title: "C"}, {ID: "d", Title: "D"}, {ID: "e", Title: "E"}}
	v := viewTracks(tracks, "plays", "", map[string]int{"b": 1, "c": 5, "e": 1})
	if !slices.Equal(v.indices, []int{2, 1, 4, 0, 3}) {
		t.Errorf("by plays: %v", v.indices)
	}
	// Without plays counted, the list keeps its order.
	if v := viewTracks(tracks, "plays", "", nil); !slices.Equal(v.indices, []int{0, 1, 2, 3, 4}) {
		t.Errorf("nothing played: %v", v.indices)
	}
}

// titlesShown are the titles of tracks the window shows, from the top.
func titlesShown(t *testing.T, tt *ui.Tester, tracks []media.Track) []string {
	t.Helper()
	type shown struct {
		title string
		y     float32
	}
	var rows []shown
	for _, tr := range tracks {
		r, ok := tt.Find(tr.Title)
		if !ok {
			t.Fatalf("%q doesn't show: %q", tr.Title, tt.Texts())
		}
		rows = append(rows, shown{tr.Title, r.Y})
	}
	slices.SortFunc(rows, func(a, b shown) int {
		switch {
		case a.y < b.y:
			return -1
		case a.y > b.y:
			return 1
		}
		return 0
	})
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.title
	}
	return out
}

// TestSortByPlays sorts the liked songs by how often MixMusic played
// them, from Sort by's Plays, and follows the plays counted after.
func TestSortByPlays(t *testing.T) {
	s := newTestShell(t)
	ctx := context.Background()
	for _, tr := range testTracks[:4] {
		if err := s.svc.Collection.SetLiked(ctx, tr, true); err != nil {
			t.Fatal(err)
		}
	}
	play := func(tr media.Track, n int) {
		for range n {
			if err := s.svc.Collection.RecordPlay(tr); err != nil {
				t.Fatal(err)
			}
		}
	}
	play(testTracks[2], 3)
	play(testTracks[0], 1)
	tt := tester(t, s)
	show(s, tt, "/library/liked")
	title := func(i int) string { return testTracks[i].Title }
	// The last liked first.
	if got, want := titlesShown(t, tt, testTracks[:4]), []string{title(3), title(2), title(1), title(0)}; !slices.Equal(got, want) {
		t.Fatalf("the liked songs show as %q, want %q", got, want)
	}
	click(t, s, tt, "Sort by")
	click(t, s, tt, "Plays")
	if got, want := titlesShown(t, tt, testTracks[:4]), []string{title(2), title(0), title(3), title(1)}; !slices.Equal(got, want) {
		t.Fatalf("sorted by plays, the liked songs show as %q, want %q", got, want)
	}
	// Plays counted later move a track up.
	play(testTracks[1], 4)
	s.drain()
	tt.Frame()
	if got, want := titlesShown(t, tt, testTracks[:4]), []string{title(1), title(2), title(0), title(3)}; !slices.Equal(got, want) {
		t.Errorf("after more plays, the liked songs show as %q, want %q", got, want)
	}
}

package main

import (
	"context"
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

func TestViewTracks(t *testing.T) {
	titles := func(v trackView) []string {
		var out []string
		for _, t := range v.tracks {
			out = append(out, t.Title)
		}
		return out
	}
	tracks := []media.Track{
		{Title: "Track 10", Artists: []media.Ref{{Name: "Björk"}}, DurationMs: 3},
		{Title: "track 2", Artists: []media.Ref{{Name: "Air"}}, DurationMs: 1},
		{Title: "Éclair", Artists: []media.Ref{{Name: "Air"}}, DurationMs: 2, Album: &media.Ref{Name: "Moon Safari"}},
		{Title: "Alpha", Artists: []media.Ref{{Name: "bjork"}}, DurationMs: 2},
	}
	for _, c := range []struct {
		sort, query string
		want        []string
		indices     []int
	}{
		{"order", "", []string{"Track 10", "track 2", "Éclair", "Alpha"}, []int{0, 1, 2, 3}},
		{"title", "", []string{"Alpha", "Éclair", "track 2", "Track 10"}, []int{3, 2, 1, 0}},
		{"artist", "", []string{"Éclair", "track 2", "Alpha", "Track 10"}, []int{2, 1, 3, 0}},
		// Equal lengths keep the list's order.
		{"length", "", []string{"track 2", "Éclair", "Alpha", "Track 10"}, []int{1, 2, 3, 0}},
		// Words match anywhere, regardless of case and accents.
		{"order", "BJORK", []string{"Track 10", "Alpha"}, []int{0, 3}},
		{"order", "eclair safari", []string{"Éclair"}, []int{2}},
		{"title", "air", []string{"Éclair", "track 2"}, []int{2, 1}},
		{"order", "nothing like it", nil, []int{}},
	} {
		v := viewTracks(tracks, c.sort, c.query)
		if !slices.Equal(titles(v), c.want) || !slices.Equal(v.indices, c.indices) {
			t.Errorf("%s, %q: %q at %v, want %q at %v", c.sort, c.query, titles(v), v.indices, c.want, c.indices)
		}
	}
}

func TestCompareText(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"track 2", "Track 10", -1}, {"Track 007", "track 7", 0}, {"éclair", "Eclair", 0}, {"a", "ab", -1}, {"b", "A", 1},
	} {
		if got := compareText(c.a, c.b); got != c.want {
			t.Errorf("compareText(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// TestLikedSearch searches the liked songs, and plays what it found.
func TestLikedSearch(t *testing.T) {
	s := newTestShell(t)
	for _, tr := range testTracks[:4] {
		if err := s.svc.Collection.SetLiked(context.Background(), tr, true); err != nil {
			t.Fatal(err)
		}
	}
	tt := tester(t, s)
	show(s, tt, "/library/liked")
	if err := tt.Click("Search this list"); err != nil {
		t.Fatal(err)
	}
	tt.Type("aphex")
	tt.Frame()
	if !tt.HasText("1 of 4") || tt.HasText(testTracks[0].Title) {
		t.Fatalf("searching the liked songs shows %q", tt.Texts())
	}
	// What plays is what the search found.
	click(t, s, tt, "Play "+testTracks[3].Title)
	s.settle(t, func(st player.State) bool {
		cur, ok := st.Current()
		return ok && cur.Track.ID == testTracks[3].ID && len(st.Queue) == 1
	})
	// Escape clears the box, which shows them all again.
	if err := tt.Click("Search this list"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if tt.HasText("1 of 4") || !tt.HasText(testTracks[0].Title) {
		t.Errorf("clearing the search shows %q", tt.Texts())
	}
}

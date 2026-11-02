package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
)

// likedShell is the main window of n liked songs.
func likedShell(tb testing.TB, n int) *shell {
	tb.Helper()
	s := newTestShell(tb)
	for i := range n {
		tr := testTracks[i%len(testTracks)]
		tr.ID = fmt.Sprintf("%s-%d", tr.ID, i)
		tr.Title = fmt.Sprintf("%s %d", tr.Title, i)
		if err := s.svc.Collection.SetLiked(context.Background(), tr, true); err != nil {
			tb.Fatal(err)
		}
	}
	return s
}

// shelves are n shelves of albums, of so many each.
func shelves(n, albums int) []media.Shelf {
	out := make([]media.Shelf, n)
	for i := range out {
		out[i].Title = "Shelf " + count(i)
		out[i].Source = media.SourceYouTube
		for j := range albums {
			a := media.Album{ID: fmt.Sprintf("al-%d-%d", i, j), Source: media.SourceYouTube, Title: "Album " + count(j),
				Artists: []media.Ref{{ID: "ar-m83", Name: "M83"}}, Year: "2011"}
			out[i].Items = append(out[i].Items, media.Item{Kind: media.ItemAlbum, Album: &a})
		}
	}
	return out
}

// scrollFrames scrolls the page by the wheel, a frame at a time, down and
// back up.
func scrollFrames(b *testing.B, tt *ui.Tester) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		dy := float32(60)
		if i/60%2 == 1 {
			dy = -60
		}
		tt.Wheel(640, 400, 0, dy)
		tt.Advance(16 * time.Millisecond)
	}
}

// BenchmarkScroll times a frame of the pages as they scroll.
func BenchmarkScroll(b *testing.B) {
	for _, n := range []int{100, 1000, 5000} {
		b.Run(fmt.Sprintf("liked-%d", n), func(b *testing.B) {
			s := likedShell(b, n)
			tt := testerOf(b, s, 1280, 820)
			show(s, tt, "/library/liked")
			scrollFrames(b, tt)
		})
	}
	b.Run("liked-1000-by-title", func(b *testing.B) {
		s := likedShell(b, 1000)
		s.tools["liked"] = &trackTool{sort: "title"}
		tt := testerOf(b, s, 1280, 820)
		show(s, tt, "/library/liked")
		scrollFrames(b, tt)
	})
	b.Run("home", func(b *testing.B) {
		s := newTestShell(b)
		tt := testerOf(b, s, 1280, 820)
		s.loads["home:youtube"] = &loaded[[]media.Shelf]{done: true, value: shelves(10, 25)}
		show(s, tt, "/")
		scrollFrames(b, tt)
	})
	b.Run("settings", func(b *testing.B) {
		s := newTestShell(b)
		tt := testerOf(b, s, 1280, 820)
		show(s, tt, "/settings/playback")
		scrollFrames(b, tt)
	})
}

// TestFarRowsBuildNothing scrolls a long list: the rows in view show their
// tracks, wherever it scrolled to, and the others build none.
func TestFarRowsBuildNothing(t *testing.T) {
	s := likedShell(t, 300)
	tt := testerOf(t, s, 1280, 820)
	show(s, tt, "/library/liked")
	// The song liked last comes first.
	if !tt.HasText("Glósóli 299") || tt.HasText("Midnight City 0") {
		t.Fatalf("at the top, the window shows %q", tt.Texts())
	}
	// Far down: the rows there build, and the first ones no more.
	for range 40 {
		tt.Wheel(640, 400, 0, 400)
		tt.Advance(16 * time.Millisecond)
	}
	tt.Advance(time.Second)
	if tt.HasText("Glósóli 299") {
		t.Error("the first row still builds, far from the view")
	}
	shown := false
	for i := 0; i < 300; i += 6 {
		if r, ok := tt.Find(fmt.Sprintf("Midnight City %d", i)); ok && r.Y > 0 && r.Y < 820 {
			shown = true
		}
	}
	if !shown {
		t.Errorf("no row in view shows its track: %q", tt.Texts())
	}
	settled(t, tt)
}

// settled fails when the window, still, asks for frames: what tells the
// rows near the view asks for one once what it follows moved, not again.
func settled(t *testing.T, tt *ui.Tester) {
	t.Helper()
	tt.Frame()
	if tt.FramePending() {
		t.Error("the window, still, asks for frames")
	}
}

// TestFarShelvesBuildNothing scrolls the home page of many shelves to its
// end: the shelves there show, and the first builds nothing.
func TestFarShelvesBuildNothing(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 820)
	s.loads["home:youtube"] = &loaded[[]media.Shelf]{done: true, value: shelves(12, 6)}
	show(s, tt, "/")
	if !tt.HasText("Shelf 0") || tt.HasText("Shelf 11") {
		t.Fatalf("at the top, the window shows %q", tt.Texts())
	}
	for range 40 {
		tt.Wheel(640, 400, 0, 400)
		tt.Advance(16 * time.Millisecond)
	}
	tt.Advance(time.Second)
	if !tt.HasText("Shelf 11") || tt.HasText("Shelf 0") {
		t.Errorf("at the end, the window shows %q", tt.Texts())
	}
	settled(t, tt)
}

// TestFarTilesBuildNothing scrolls a shelf of many tiles to its end: the
// tiles there show, and the first builds nothing.
func TestFarTilesBuildNothing(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 820)
	s.loads["home:youtube"] = &loaded[[]media.Shelf]{done: true, value: shelves(1, 40)}
	show(s, tt, "/")
	if !tt.HasText("Album 0") || tt.HasText("Album 39") {
		t.Fatalf("at the start, the shelf shows %q", tt.Texts())
	}
	// Its arrows, under the home's tiles, come into view.
	tt.Wheel(640, 400, 0, 2000)
	tt.Advance(time.Second)
	for range 24 {
		click(t, s, tt, "Scroll Shelf 0 on")
		tt.Advance(time.Second)
	}
	if !tt.HasText("Album 39") || tt.HasText("Album 0") {
		t.Errorf("at the end, the shelf shows %q", tt.Texts())
	}
	settled(t, tt)
}

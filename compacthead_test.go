package main

import (
	"context"
	"fmt"
	"image/color"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// longPlaylist is a playlist of YouTube Music of n tracks, which the page
// of s shows.
func longPlaylist(s *shell, id, title string, n int) media.Playlist {
	p := media.Playlist{ID: id, Source: media.SourceYouTube, Title: title, Owner: "Ada",
		URL: "https://music.youtube.com/playlist?list=" + id}
	for i := range n {
		tr := testTracks[i%len(testTracks)]
		tr.ID = fmt.Sprintf("%s-%d", tr.ID, i)
		tr.Title = fmt.Sprintf("%s %d", tr.Title, i)
		p.Tracks = append(p.Tracks, tr)
	}
	s.loads["playlist:"+p.ID] = &loaded[media.Playlist]{done: true, value: p}
	return p
}

// scrollBy scrolls the page under the pointer by dy, and lets the window
// settle: the pointer is left of the side panel, at any size.
func scrollBy(tt *ui.Tester, dy float32) {
	tt.Wheel(300, 400, 0, dy)
	tt.Advance(16 * time.Millisecond)
	tt.Advance(time.Second)
}

// topBarHas reports whether label shows in the top bar, 48 high: not
// in what the page scrolled out of view.
func topBarHas(tt *ui.Tester, label string) bool {
	r, ok := tt.Find(label)
	return ok && r.W > 0 && r.H > 0 && r.Y >= 0 && r.Y+r.H <= 48
}

// TestBarHeadPlays scrolls a playlist past its head: the top bar shows
// its name and its buttons, whose play plays the playlist, and they go
// once the head comes back.
func TestBarHeadPlays(t *testing.T) {
	s := newTestShell(t)
	p := longPlaylist(s, "yt:PLlong", "Late night drive", 80)
	tt := tester(t, s)
	show(s, tt, playlistPath(p.ID))
	if _, ok := tt.Find("Play " + p.Title); ok {
		t.Fatalf("at the top, the top bar shows the head: %q", tt.Texts())
	}
	// Above the head's buttons, it shows none yet.
	scrollBy(tt, 100)
	if _, ok := tt.Find("Play " + p.Title); ok {
		t.Errorf("with the head's buttons in view, the top bar shows them too")
	}
	scrollBy(tt, 700)
	for _, label := range []string{s.lk.words(p.Title), "Play " + p.Title, "Shuffle " + p.Title, "download", "More for " + p.Title} {
		if !topBarHas(tt, label) {
			t.Errorf("scrolled, the top bar has no %q: %q", label, tt.Texts())
		}
	}
	click(t, s, tt, "Play "+p.Title)
	s.settle(t, func(st player.State) bool {
		cur, ok := st.Current()
		return ok && st.Playing && cur.Track.ID == p.Tracks[0].ID && len(st.Queue) == len(p.Tracks)
	})
	click(t, s, tt, "Shuffle "+p.Title)
	s.settle(t, func(st player.State) bool { return st.Shuffle && st.Playing })
	scrollBy(tt, -2000)
	if _, ok := tt.Find("Play " + p.Title); ok {
		t.Errorf("back at the top, the top bar still shows the head")
	}
	settled(t, tt)
}

// TestBarHeadFades fades the head in the top bar in, but shows it at once
// while the computer asks for less motion.
func TestBarHeadFades(t *testing.T) {
	for _, reduce := range []bool{false, true} {
		s := newTestShell(t)
		p := longPlaylist(s, "yt:PLlong", "Late night drive", 80)
		tt := tester(t, s)
		tt.SetPreferences(ui.Preferences{ReduceMotion: reduce})
		show(s, tt, playlistPath(p.ID))
		// The page jumps past its head, as a press of End does.
		s.here.scroll.Y = 1500
		tt.Frame()
		tt.Advance(40 * time.Millisecond)
		r, ok := tt.Find("Play " + p.Title)
		if !ok {
			t.Fatalf("reduce motion %v: no play button in the top bar: %q", reduce, tt.Texts())
		}
		// The edge of the play button, a disc of the text's colour.
		x, y := int(r.X+3), int(r.Y+r.H/2)
		first := tt.Image().At(x, y)
		tt.Advance(time.Second)
		last := tt.Image().At(x, y)
		if same := sameColor(first, last); same != reduce {
			t.Errorf("reduce motion %v: the play button shows %v, then %v", reduce, first, last)
		}
		settled(t, tt)
	}
}

func sameColor(a, b color.Color) bool {
	r1, g1, b1, a1 := a.RGBA()
	r2, g2, b2, a2 := b.RGBA()
	return r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2
}

// TestPinnedSearch keeps the search and the sort of a playlist's tracks
// under the top bar once the page scrolled past them, in every layout
// design. What the user types there keeps the focus, and shows the
// tracks found under them, however few.
func TestPinnedSearch(t *testing.T) {
	const label = "Search this playlist"
	for _, layout := range settings.Layouts {
		s := newTestShell(t)
		s.set.Layout = layout
		s.restyle()
		p := longPlaylist(s, "yt:PLlong", "Late night drive", 80)
		tt := tester(t, s)
		show(s, tt, playlistPath(p.ID))
		before, _ := tt.Find(label)
		scrollBy(tt, 1500)
		pinned, ok := tt.Find(label)
		if !ok || pinned.Y < 48+pinPad || pinned.Y > 48+pinPad+40 {
			t.Fatalf("%s: scrolled, the search is at %v (%v), at %v before", layout, pinned, ok, before)
		}
		scrollBy(tt, 600)
		if r, _ := tt.Find(label); r.Y != pinned.Y {
			t.Errorf("%s: scrolled further, the search moved from %v to %v", layout, pinned, r)
		}
		if _, ok := tt.Find("Midnight City 0"); ok {
			t.Errorf("%s: the list's start shows, scrolled far", layout)
		}
		click(t, s, tt, label)
		tt.Type("teardrop 7")
		s.drain()
		tt.Frame()
		tt.Advance(time.Second)
		if !tt.Focused(label) {
			t.Errorf("%s: typing, the search lost the focus", layout)
		}
		if r, _ := tt.Find(label); r.Y != pinned.Y {
			t.Errorf("%s: typing, the search moved from %v to %v", layout, pinned, r)
		}
		found, ok := tt.Find("Teardrop 7")
		if !ok || found.Y < pinned.Y+pinned.H || found.Y > pinned.Y+pinned.H+120 {
			t.Errorf("%s: the first track found is at %v (%v), the search at %v", layout, found, ok, pinned)
		}
		if pin := s.here.pins["playlist:"+p.ID+"-tools"]; pin == nil || math.Abs(float64(s.here.scroll.Y-pin.at())) > 0.5 {
			t.Errorf("%s: searching, the page is at %v, pinned at %+v", layout, s.here.scroll.Y, pin)
		}
		tt.Key(0, ui.KeyEscape)
		s.drain()
		tt.Frame()
		tt.Advance(time.Second)
		if r, _ := tt.Find(label); r.Y != pinned.Y || !tt.HasText("Midnight City 0") {
			t.Errorf("%s: cleared, the search is at %v, and the list shows %q", layout, r, tt.Texts())
		}
		scrollBy(tt, -3000)
		if r, _ := tt.Find(label); r.Y != before.Y {
			t.Errorf("%s: back at the top, the search is at %v, not %v", layout, r, before)
		}
		settled(t, tt)
	}
}

// TestBarHeadNarrow moves the download of the head in the top bar into its
// menu in a narrow window.
func TestBarHeadNarrow(t *testing.T) {
	s := newTestShell(t)
	p := longPlaylist(s, "yt:PLlong", "Late night drive", 80)
	tt := testerOf(t, s, 800, 600)
	show(s, tt, playlistPath(p.ID))
	scrollBy(tt, 1500)
	if !topBarHas(tt, "Play "+p.Title) || topBarHas(tt, "download") {
		t.Fatalf("narrow, the top bar shows %q", tt.Texts())
	}
	if err := tt.Click("More for " + p.Title); err != nil {
		t.Fatal(err)
	}
	if items := tt.Menu(); !slices.Contains(items, "Download") {
		t.Errorf("the top bar's menu is %q", items)
	}
	tt.CloseMenu()
}

// TestBarHeadOfPages shows the heads of albums, artists and the liked
// songs in the top bar too, once they scrolled away: an artist's plays
// its top songs, and has no shuffle.
func TestBarHeadOfPages(t *testing.T) {
	s := newTestShell(t)
	pl := longPlaylist(s, "yt:PLlong", "Late night drive", 40)
	s.loads["album:al-long"] = &loaded[media.Album]{done: true, value: media.Album{ID: "al-long", Source: media.SourceYouTube,
		Title: "Hurry Up, We're Dreaming", Artists: []media.Ref{{ID: "ar-m83", Name: "M83"}}, Tracks: pl.Tracks}}
	var albums []media.Album
	for i := range 4 {
		albums = append(albums, media.Album{ID: "al-" + count(i), Source: media.SourceYouTube, Title: "Album " + count(i)})
	}
	s.loads["artist:ar-m83"] = &loaded[media.Artist]{done: true, value: media.Artist{ID: "ar-m83", Source: media.SourceYouTube,
		Name: "M83", TopTracks: testTracks[:5], Albums: albums, Singles: albums}}
	for _, tr := range pl.Tracks {
		if err := s.svc.Collection.SetLiked(context.Background(), tr, true); err != nil {
			t.Fatal(err)
		}
	}
	tt := testerOf(t, s, 1280, 600)
	for _, c := range []struct {
		path, title       string
		shuffle, download bool
		first             string
	}{
		{albumPath("al-long"), "Hurry Up, We're Dreaming", true, true, pl.Tracks[0].ID},
		{artistPath("ar-m83"), "M83", false, false, testTracks[0].ID},
		{"/library/liked", "Liked songs", true, true, pl.Tracks[len(pl.Tracks)-1].ID},
	} {
		show(s, tt, c.path)
		if _, ok := tt.Find("Play " + c.title); ok {
			t.Errorf("%s: at the top, the top bar shows the head", c.path)
		}
		scrollBy(tt, 1500)
		if !topBarHas(tt, "Play "+c.title) || topBarHas(tt, "Shuffle "+c.title) != c.shuffle ||
			topBarHas(tt, "download") != c.download {
			t.Errorf("%s: scrolled, the top bar shows %q", c.path, tt.Texts())
			continue
		}
		click(t, s, tt, "Play "+c.title)
		s.settle(t, func(st player.State) bool {
			cur, ok := st.Current()
			return ok && st.Playing && cur.Track.ID == c.first
		})
	}
	settled(t, tt)
}

// TestLikedSongsPlay plays and shuffles the liked songs from their page.
func TestLikedSongsPlay(t *testing.T) {
	s := likedShell(t, 3)
	tt := tester(t, s)
	show(s, tt, "/library/liked")
	liked := s.svc.Collection.Liked()
	click(t, s, tt, "play")
	s.settle(t, func(st player.State) bool {
		cur, ok := st.Current()
		return ok && st.Playing && !st.Shuffle && cur.Track.ID == liked[0].ID
	})
	click(t, s, tt, "shuffle")
	s.settle(t, func(st player.State) bool { return st.Shuffle && st.Playing })
}

// TestBarHeadOtherPage shows no head of the page left in the top bar of
// the next, nor a page's own offset on another; going back finds the
// page where it was, its head in the top bar.
func TestBarHeadOtherPage(t *testing.T) {
	s := newTestShell(t)
	a := longPlaylist(s, "yt:PLa", "Late night drive", 80)
	b := longPlaylist(s, "yt:PLb", "Morning run", 80)
	tt := tester(t, s)
	show(s, tt, playlistPath(a.ID))
	scrollBy(tt, 1500)
	at := s.here.scroll.Y
	if !topBarHas(tt, "Play "+a.Title) {
		t.Fatalf("scrolled, the top bar shows %q", tt.Texts())
	}
	s.navigate(playlistPath(b.ID))
	for range 20 {
		tt.Advance(16 * time.Millisecond)
		if _, ok := tt.Find("Play " + a.Title); ok {
			t.Fatalf("on the next page, the top bar shows the last one's head")
		}
	}
	tt.Advance(time.Second)
	if _, ok := tt.Find("Play " + b.Title); ok || s.here.scroll.Y != 0 {
		t.Errorf("the next page starts at %v, its head in the top bar: %v", s.here.scroll.Y, ok)
	}
	s.router.Back()
	tt.Frame()
	tt.Advance(time.Second)
	if s.here.scroll.Y != at || !topBarHas(tt, "Play "+a.Title) {
		t.Errorf("back, the page is at %v, not %v, and the top bar shows %q", s.here.scroll.Y, at, tt.Texts())
	}
	settled(t, tt)
}

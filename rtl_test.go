package main

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
	"github.com/fzn0x/mixmusic/lyrics"
)

// rtlShell is a shell in layout playing the test's second track a minute
// in, with made-up covers, liked songs, plays and lyrics: what the tests of
// the right-to-left layout look at (#564).
func rtlShell(t *testing.T, layout settings.Layout) *shell {
	t.Helper()
	s := newTestShell(t)
	base := coverServer(t, s)
	tracks := make([]media.Track, len(testTracks))
	for i, tr := range testTracks {
		tr.Artwork = []media.Image{{URL: base + "/cover/" + strconv.Itoa(i) + ".png", Width: 300, Height: 300}}
		tracks[i] = tr
	}
	st := s.svc.Preferences.Get()
	st.Layout = layout
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	var items []media.Item
	for i := range tracks {
		items = append(items, media.Item{Kind: media.ItemTrack, Track: &tracks[i]})
	}
	for _, key := range []string{"home:spotify", "explore", "charts"} {
		s.loads[key] = &loaded[[]media.Shelf]{done: true}
	}
	s.loads["home:youtube"] = &loaded[[]media.Shelf]{done: true, value: []media.Shelf{{Title: "Listen again", Source: media.SourceYouTube, Items: items}}}
	s.loads["release"] = &loaded[app.Release]{done: true}
	s.loads["lyrics:"+tracks[1].ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "LRCLIB", Lines: []lyrics.Line{
		{TimeMs: 0, Text: "Stones remember every tide"}, {TimeMs: 30000, Text: "Sand keeps the shape of it"},
		{TimeMs: 55000, Text: "Quiet as the harbour light"}, {TimeMs: 90000, Text: "Slow wind over the pier"}}}}
	for _, tr := range tracks {
		if err := s.svc.Collection.RecordPlay(tr); err != nil {
			t.Fatal(err)
		}
		if err := s.svc.Collection.SetLiked(context.Background(), tr, true); err != nil {
			t.Fatal(err)
		}
	}
	s.p.PlayTracks(tracks, 1)
	s.settle(t, func(st player.State) bool { return st.Playing })
	s.p.Time(time.Minute, 211*time.Second)
	s.settle(t, func(st player.State) bool { return st.Position == time.Minute })
	return s
}

// rtlTester is the main window of s, w by h, laid out in d.
func rtlTester(s *shell, w, h int, d ui.Direction) *ui.Tester {
	tt := ui.NewTester(s.view, w, h)
	tt.SetDirection(d)
	return tt
}

// placeOf is where tt shows label.
func placeOf(t *testing.T, tt *ui.Tester, label string) ui.Rect {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("no %q in %q", label, tt.Texts())
	}
	return r
}

// inOrder checks that tt shows labels from the left to the right.
func inOrder(t *testing.T, tt *ui.Tester, what string, labels ...string) {
	t.Helper()
	for i := 1; i < len(labels); i++ {
		if a, b := placeOf(t, tt, labels[i-1]), placeOf(t, tt, labels[i]); a.X+a.W > b.X {
			t.Errorf("%s: %q at %+v is not left of %q at %+v", what, labels[i-1], a, labels[i], b)
		}
	}
}

// TestRightToLeftMirrors lays the main window out right to left, as in
// Arabic, Hebrew, Persian and Urdu (#564), in every layout design: what
// isn't a media control mirrors, as the rail, now on the right, the top
// bar's arrows, the side panel, now on the left, and the player bar's
// sides.
func TestRightToLeftMirrors(t *testing.T) {
	const w = 1280
	for _, layout := range settings.Layouts {
		s := rtlShell(t, layout)
		s.panel = "queue"
		ltr, rtl := rtlTester(s, w, 820, ui.LeftToRight), rtlTester(s, w, 820, ui.RightToLeft)
		show(s, ltr, "/")
		show(s, rtl, "/")
		for _, label := range []string{locales.RailCollapse(), locales.TopbarBack(), locales.TopbarForward(), locales.TopbarPalette(),
			locales.PanelClearQueue(), locales.PlayerDislike(), locales.PlayerMiniPlayer(), locales.PlayerQueue()} {
			l, r := placeOf(t, ltr, label), placeOf(t, rtl, label)
			if mirrored := w - l.X - l.W; r.X < mirrored-1 || r.X > mirrored+1 || r.Y != l.Y {
				t.Errorf("%s: %q is at %+v right to left, and at %+v left to right: not mirrored", layout, label, r, l)
			}
		}
		if back := placeOf(t, rtl, locales.TopbarBack()); back.X < w/2 {
			t.Errorf("%s: the back arrow is at %+v, not at the right", layout, back)
		}
	}
}

// TestRightToLeftTitleBar keeps clear of the window's buttons under a
// hidden title bar, which macOS puts at the left or, for an app laid out
// right to left, at the right: the top bar keeps clear at its end, the
// left, and the rail at its start, the right.
func TestRightToLeftTitleBar(t *testing.T) {
	const w = 1280
	s := rtlShell(t, settings.LayoutTiles)
	tt := rtlTester(s, w, 820, ui.RightToLeft)
	left := ui.TitleBar{Height: 28, Left: 69}
	tt.SetTitleBar(left)
	show(s, tt, "/")
	if r := placeOf(t, tt, locales.TopbarPalette()); r.X < left.Left {
		t.Errorf("the top bar's end is at %+v, under the window's buttons %+v", r, left)
	}
	right := ui.TitleBar{Height: 28, Right: 69}
	tt.SetTitleBar(right)
	show(s, tt, "/")
	for _, label := range []string{locales.RailCollapse(), "mixmusic"} {
		if r := placeOf(t, tt, label); r.X+r.W > w-right.Right && r.Y < right.Height {
			t.Errorf("%q is at %+v, under the window's buttons %+v", label, r, right)
		}
	}
}

// TestLineIconTurns turns the icons that point along the line right to
// left, and only those.
func TestLineIconTurns(t *testing.T) {
	for _, d := range []ui.Direction{ui.LeftToRight, ui.RightToLeft} {
		got := map[string]*ui.SVG{}
		tt := ui.NewTester(func(c *ui.Context) {
			ui.Row(c).Children(func() {
				for _, name := range []string{"back", "forward", "sidebar", "next", "play", "search"} {
					got[name] = lineIcon(c, name)
				}
			})
		}, 100, 100)
		tt.SetDirection(d)
		for name, svg := range got {
			want := icon(name)
			if m, ok := mirroredIcons[name]; ok && d == ui.RightToLeft {
				want = m
			}
			if svg != want {
				t.Errorf("%v: the icon %q isn't the one that points the way the line reads", d, name)
			}
		}
	}
}

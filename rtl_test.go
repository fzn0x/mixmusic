package main

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/remote/remotetest"
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

// TestRightToLeftMediaControls keeps the media controls left to right in a
// window laid out right to left, as playback runs one way in every
// language (#564): the transport, the seek bar and its times, the volume,
// the mini player's and the now playing page's, and the equalizer's bands.
// The bars fill from the left: a click a quarter along seeks a quarter in.
func TestRightToLeftMediaControls(t *testing.T) {
	s := rtlShell(t, settings.LayoutTiles)
	s.panel = ""
	tt := rtlTester(s, 1280, 820, ui.RightToLeft)
	show(s, tt, "/")
	inOrder(t, tt, "the player bar", locales.PlayerShuffle(), locales.PlayerPrevious(), locales.PlayerPause(), locales.PlayerNext(),
		locales.PlayerRepeatOff())
	inOrder(t, tt, "the seek bar", "1:00", locales.PlayerSeek())
	inOrder(t, tt, "the volume", locales.VolumeMute(), locales.PlayerVolume())
	seek := placeOf(t, tt, locales.PlayerSeek())
	tt.ClickAt(seek.X+seek.W/4, seek.Y+seek.H/2)
	s.settle(t, func(st player.State) bool { return st.Position > 40*time.Second && st.Position < 60*time.Second })
	volume := placeOf(t, tt, locales.PlayerVolume())
	tt.ClickAt(volume.X+volume.W/4, volume.Y+volume.H/2)
	s.settle(t, func(st player.State) bool { return st.Volume > 0.1 && st.Volume < 0.4 })

	show(s, tt, "/now-playing")
	inOrder(t, tt, "the now playing page", locales.PlayerPause(), locales.PlayerNext())

	mini := ui.NewTester(s.miniView, 420, 132)
	mini.SetDirection(ui.RightToLeft)
	inOrder(t, mini, "the mini player", locales.PlayerShuffle(), locales.PlayerPrevious(), locales.PlayerPause(), locales.PlayerNext())
	if title := placeOf(t, mini, testTracks[1].Title); title.X+title.W > 420-100 {
		t.Errorf("the mini player's title is at %+v, over its artwork, which is at the right", title)
	}
	at := s.p.State().Position
	if want := clock(at) + " / " + clockLength(211*time.Second); !mini.HasText(want) {
		t.Errorf("the mini player doesn't show %q: %q", want, mini.Texts())
	}
	bar := placeOf(t, mini, locales.PlayerSeek())
	mini.ClickAt(bar.X+bar.W*3/4, bar.Y+bar.H/2)
	s.settle(t, func(st player.State) bool { return st.Position > 150*time.Second && st.Position < 170*time.Second })

	eq := rtlTester(s, 1280, 3600, ui.RightToLeft)
	show(s, eq, "/settings/playback")
	inOrder(t, eq, "the equalizer", locales.EqualizerBandDB(locales.EqualizerPreamp()), locales.EqualizerBandDB("32"),
		locales.EqualizerBandDB("1k"), locales.EqualizerBandDB("16k"))
}

// TestRightToLeftRemote keeps the controls of another computer left to
// right on its page, as the player bar's: its transport, its volume, and
// its seek bar, which a click a quarter along moves a quarter in.
func TestRightToLeftRemote(t *testing.T) {
	n := remotetest.New()
	desk := newRemoteShell(t, n, "Desktop")
	lap := newRemoteShell(t, n, "Laptop")
	dt, lt := tester(t, desk), tester(t, lap)
	if err := desk.a.Remote().SetOn(true); err != nil {
		t.Fatal(err)
	}
	desk.p.PlayTracks(testTracks, 0)
	desk.settle(t, func(st player.State) bool { return st.Playing })
	pc := found(t, lap, "Desktop")
	show(lap, lt, remotePath(pc.ID))
	click(t, lap, lt, "pair")
	var code string
	untilBoth(t, desk, dt, lap, lt, "the code", func() bool {
		c, ok := desk.a.Remote().Code()
		code = c.Code
		return ok
	})
	lt.Type(code)
	lap.drain()
	lt.Frame()
	click(t, lap, lt, "pair")
	lt.SetDirection(ui.RightToLeft)
	untilBoth(t, desk, dt, lap, lt, "what plays there", func() bool { return lt.HasText("Midnight City") })
	inOrder(t, lt, "the remote", locales.PlayerPrevious(), locales.PlayerPause(), locales.PlayerNext(), locales.PlayerVolume())
	seek := placeOf(t, lt, locales.PlayerSeek())
	lt.ClickAt(seek.X+seek.W/4, seek.Y+seek.H/2)
	lap.drain()
	desk.settle(t, func(st player.State) bool { return st.Position > 40*time.Second && st.Position < 65*time.Second })
}

// TestRightToLeftNotch swaps the sides of the player around the notch right
// to left, as its panel's body below: the button that opens it is at the
// right of the housing, and the transport in the open panel reads left to
// right.
func TestRightToLeftNotch(t *testing.T) {
	x := playingNotch(t, testHousing)
	x.tt.SetDirection(ui.RightToLeft)
	x.frame()
	w, _ := x.size()
	if r := placeOf(t, x.tt, locales.NotchPanelOpenPlayer()); r.X < float32(w)/2 {
		t.Errorf("the button that opens the panel is at %+v, at the left of the housing", r)
	}
	x.click(locales.NotchPanelOpenPlayer())
	x.advance(time.Second)
	inOrder(t, x.tt, "the notch", locales.PlayerPrevious(), locales.PlayerPause(), locales.PlayerNext())
	if r := placeOf(t, x.tt, locales.NotchPanelClosePlayer()); r.X < float32(w)/2 {
		t.Errorf("the button that closes the panel is at %+v, at the left of the housing", r)
	}
}

// TestRightToLeftStats keeps the chart of Stats left to right, as it draws,
// with today at its right under it.
func TestRightToLeftStats(t *testing.T) {
	s := rtlShell(t, settings.LayoutTiles)
	tt := rtlTester(s, 1280, 1600, ui.RightToLeft)
	show(s, tt, "/stats")
	if today := placeOf(t, tt, locales.StatsToday()); today.X < 1280/2 {
		t.Errorf("today is at %+v, at the left of the chart", today)
	}
}

// TestRightToLeftPseudoLocale shows the main screens in MyGo's pseudo
// language ar-XB, English words written right to left in a window laid out
// right to left, as a translation into Arabic would be: the rail is on the
// right, and the transport left to right.
func TestRightToLeftPseudoLocale(t *testing.T) {
	was := i18n.Locale()
	i18n.SetLocaleFunc(func() string { return i18n.PseudoBidi })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return was }) })
	xb := locales.In(i18n.PseudoBidi)
	s := rtlShell(t, settings.LayoutBlend)
	s.panel = "lyrics"
	tt := rtlTester(s, 1280, 820, ui.RightToLeft)
	tt.SetLocale(i18n.PseudoBidi)
	for _, p := range []string{"/", "/library/liked", "/now-playing", "/settings/playback", "/stats"} {
		show(s, tt, p)
		if r := placeOf(t, tt, xb.RailCollapse()); r.X < 1280/2 {
			t.Errorf("%s: the rail is at %+v, not at the right", p, r)
		}
		inOrder(t, tt, p+": the player bar", xb.PlayerPrevious(), xb.PlayerPause(), xb.PlayerNext())
	}
	mini := ui.NewTester(s.miniView, 420, 132)
	mini.SetDirection(ui.RightToLeft)
	mini.SetLocale(i18n.PseudoBidi)
	inOrder(t, mini, "the mini player", xb.PlayerPrevious(), xb.PlayerPause(), xb.PlayerNext())
}

// TestRenderRightToLeft draws the main screens left to right, right to left
// and in ar-XB to MIXMUSIC_RENDER, for a look.
func TestRenderRightToLeft(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	for _, run := range []struct {
		name   string
		d      ui.Direction
		locale string
	}{{"ltr", ui.LeftToRight, ""}, {"rtl", ui.RightToLeft, ""}, {"ar-XB", ui.RightToLeft, i18n.PseudoBidi}} {
		if run.locale != "" {
			was := i18n.Locale()
			i18n.SetLocaleFunc(func() string { return run.locale })
			t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return was }) })
		}
		for _, layout := range []settings.Layout{settings.LayoutBlend, settings.LayoutTiles} {
			s := rtlShell(t, layout)
			name := run.name + "-" + string(layout)
			tt := rtlTester(s, 1280, 820, run.d)
			tall := rtlTester(s, 1280, 1700, run.d)
			mini := ui.NewTester(s.miniView, 420, 132)
			mini.SetDirection(run.d)
			if run.locale != "" {
				tt.SetLocale(run.locale)
				tall.SetLocale(run.locale)
				mini.SetLocale(run.locale)
			}
			for _, page := range []struct{ name, path, panel string }{
				{"home", "/", ""}, {"home-queue", "/", "queue"}, {"library-liked", "/library/liked", "lyrics"},
				{"now-playing", "/now-playing", ""}, {"stats", "/stats", ""},
			} {
				s.panel = page.panel
				s.navigate(page.path)
				renderTo(t, s, tt, dir, name+"-"+page.name)
			}
			s.panel = ""
			s.navigate("/settings/playback")
			renderTo(t, s, tall, dir, name+"-settings-playback")
			renderTo(t, s, mini, dir, name+"-mini")
		}
	}
}

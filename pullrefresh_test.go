package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/lyrics"
)

// The tests pull the page of an album of a malformed ID, whose refresh
// fails at once, offline, as refresh_test.go's do. The words are the
// Tiles layout's, lowered.

// pullAlbum is the page of an album of n tracks that showed, long enough
// to scroll, whose ID names no service.
func pullAlbum(s *shell, n int) *loaded[media.Album] {
	a := media.Album{ID: "al-x", Title: "Paper Lanterns"}
	for i := range n {
		a.Tracks = append(a.Tracks, testTrack(fmt.Sprintf("yt:lantern-%d", i), fmt.Sprintf("Lantern %d", i), "The Kites", a.Title, media.SourceYouTube))
	}
	l := &loaded[media.Album]{done: true, value: a}
	s.loads["album:al-x"] = l
	return l
}

// pullAt is where the tests pull the page: over its content, beside the
// queue.
const pullX, pullY = 640, 400

// swipe scrolls by each of dys at x, y as a touchpad does, a frame apart.
func swipe(tt *ui.Tester, x, y float32, dys ...float32) {
	for _, dy := range dys {
		tt.Scroll(x, y, 0, dy)
		tt.Advance(16 * time.Millisecond)
	}
}

// notches turns the wheel up by n notches at x, y, 50 ms apart.
func notches(tt *ui.Tester, x, y float32, n int) {
	for range n {
		tt.Wheel(x, y, 0, -100)
		tt.Advance(50 * time.Millisecond)
	}
}

// rested lets the gesture end, and takes in what the page fetched.
func rested(s *shell, tt *ui.Tester) {
	tt.Advance(pullRest)
	s.drain()
	tt.Frame()
}

// pulling tells whether the window shows the strip of a pull, and whether
// it says letting go refreshes.
func pulling(tt *ui.Tester) (shown, far bool) {
	far = tt.HasText("release to refresh")
	return far || tt.HasText("pull to refresh"), far
}

// TestPullThreshold refreshes the page pulled past pullFar once let go,
// with the strip saying so, and not one pulled short of it, or pulled
// back.
func TestPullThreshold(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	l := pullAlbum(s, 40)
	show(s, tt, "/album/al-x")
	swipe(tt, pullX, pullY, -30, -30, -30)
	if shown, far := pulling(tt); !shown || far {
		t.Fatalf("pulled 90 DIPs, the strip shows %v, far %v: %q", shown, far, tt.Texts())
	}
	rested(s, tt)
	if l.asks != 0 || s.refreshAt != "" {
		t.Fatalf("a pull short of the threshold refreshed: asks %d", l.asks)
	}
	tt.Advance(time.Second)
	if shown, _ := pulling(tt); shown {
		t.Error("the strip stays open after the pull")
	}

	// Past it, and back.
	swipe(tt, pullX, pullY, -80, -80)
	if _, far := pulling(tt); !far {
		t.Fatalf("pulled 160 DIPs, the strip doesn't say letting go refreshes: %q", tt.Texts())
	}
	swipe(tt, pullX, pullY, 60)
	if shown, far := pulling(tt); !shown || far {
		t.Errorf("pulled back to 100 DIPs, the strip shows %v, far %v", shown, far)
	}
	if y := s.pageScrolled().Y; y != 0 {
		t.Errorf("the pull going back scrolled the page to %v", y)
	}
	rested(s, tt)
	if l.asks != 0 {
		t.Fatal("a pull back short of the threshold refreshed")
	}

	// Past it: the refresh, with the button's spinner, and its toast.
	tt.Advance(time.Second)
	swipe(tt, pullX, pullY, -50, -50, -50)
	rested(s, tt)
	if l.asks != 1 {
		t.Fatalf("a pull past the threshold asked %d times", l.asks)
	}
	if _, ok := tt.Find("Refreshing"); !ok && s.refreshing() {
		t.Error("the button isn't a spinner while the pull's refresh is out")
	}
	waitRefreshed(t, s, tt)
	if got := refreshToasts(tt); !slices.Equal(got, []string{"Couldn't refresh: media: malformed id"}) {
		t.Errorf("the toasts are %q", got)
	}
	tt.Advance(time.Second)
	if shown, _ := pulling(tt); shown {
		t.Error("the strip stays open after the refresh")
	}
}

// TestPullWheel takes three notches of the wheel turned up at the top for
// a pull, and never one or two.
func TestPullWheel(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	l := pullAlbum(s, 40)
	show(s, tt, "/album/al-x")
	for n := 1; n <= 2; n++ {
		notches(tt, pullX, pullY, n)
		if shown, far := pulling(tt); !shown || far {
			t.Errorf("%d notches: the strip shows %v, far %v", n, shown, far)
		}
		rested(s, tt)
		if l.asks != 0 {
			t.Fatalf("%d notches refreshed", n)
		}
	}
	notches(tt, pullX, pullY, 3)
	if _, far := pulling(tt); !far {
		t.Errorf("3 notches: the strip doesn't say letting go refreshes: %q", tt.Texts())
	}
	rested(s, tt)
	if l.asks != 1 {
		t.Errorf("3 notches asked %d times", l.asks)
	}
	waitRefreshed(t, s, tt)
}

// TestPullMomentum never pulls the page with scrolling that came up to
// its top, as a fling's momentum does on Windows, where nothing tells it
// apart, until it stopped; nor with momentum macOS marks.
func TestPullMomentum(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	l := pullAlbum(s, 60)
	show(s, tt, "/album/al-x")
	swipe(tt, pullX, pullY, 900)
	tt.Advance(time.Second)
	if y := s.pageScrolled().Y; y < 800 {
		t.Fatalf("the album scrolled to %v", y)
	}
	// A fling up, on past the top for three times the threshold.
	fling := make([]float32, 25)
	for i := range fling {
		fling[i] = -60
	}
	swipe(tt, pullX, pullY, fling...)
	if y := s.pageScrolled().Y; y != 0 {
		t.Fatalf("the fling stopped at %v", y)
	}
	if shown, _ := pulling(tt); shown {
		t.Error("the fling's momentum at the top pulled the page")
	}
	rested(s, tt)
	if l.asks != 0 {
		t.Fatal("the fling's momentum refreshed the page")
	}
	// Once it stopped, a pull.
	swipe(tt, pullX, pullY, -70, -70)
	rested(s, tt)
	if l.asks != 1 {
		t.Errorf("a pull after the fling asked %d times", l.asks)
	}
	waitRefreshed(t, s, tt)

	// macOS marks momentum: it never pulls, and lets go of the fingers'
	// pull, once.
	clock := time.Unix(1, 0)
	ev := func(dy float32, momentum bool) ui.InputEvent {
		clock = clock.Add(16 * time.Millisecond)
		return ui.InputEvent{Kind: ui.InputScroll, DY: dy, Precise: true, Momentum: momentum, Time: clock}
	}
	var p pullState
	for range 20 {
		if taken, refresh := p.take(ev(-40, true), true, false); taken || refresh {
			t.Fatal("momentum at the top pulled")
		}
	}
	if p.by != 0 {
		t.Fatalf("momentum pulled %v", p.by)
	}
	// The fingers come back, the momentum gone: they pull.
	p.take(ev(-80, false), true, false)
	p.take(ev(-80, false), true, false)
	if p.by != 160 {
		t.Fatalf("the fingers pulled %v", p.by)
	}
	refreshes := 0
	for range 20 {
		if _, refresh := p.take(ev(-40, true), true, false); refresh {
			refreshes++
		}
	}
	if refreshes != 1 || p.by != 0 {
		t.Errorf("the momentum after the fingers' pull refreshed %d times, and pulled %v", refreshes, p.by)
	}
	if p.rest(clock.Add(time.Second)) {
		t.Error("the pull refreshed again once it rested")
	}
}

// TestPullSideways never pulls with scrolling sideways: Shift+wheel, which
// pages a shelf, or a touchpad's, which scrolls it.
func TestPullSideways(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 1600)
	s.loads["home:youtube"] = &loaded[[]media.Shelf]{done: true, value: shelves(2, 25)}
	show(s, tt, "/")
	r, ok := tt.Find("Album 3")
	if !ok {
		t.Fatalf("no shelf: %q", tt.Texts())
	}
	x, y := r.X+r.W/2, r.Y+r.H/2
	// A touchpad's scrolling sideways, a little up as fingers go.
	for range 8 {
		tt.Scroll(x, y, 60, -20)
		tt.Advance(16 * time.Millisecond)
	}
	if st := s.shelfScroll["/ home:youtube-0"]; st == nil || st.X <= 0 {
		t.Errorf("the shelf didn't scroll sideways: %+v", st)
	}
	if shown, _ := pulling(tt); shown {
		t.Error("scrolling a shelf sideways pulled the page")
	}
	rested(s, tt)
	if s.refreshing() || s.refreshAt != "" {
		t.Error("scrolling a shelf sideways refreshed the page")
	}

	// Shift+wheel, which the Tester can't hold: the pull's own.
	clock := time.Unix(1, 0)
	var p pullState
	for range 5 {
		clock = clock.Add(50 * time.Millisecond)
		if taken, refresh := p.take(ui.InputEvent{Kind: ui.InputScroll, DY: -100, Mods: ui.Shift, Time: clock}, true, false); taken || refresh {
			t.Fatal("Shift+wheel at the top pulled")
		}
	}
	// Shift let go of, the gesture goes on sideways: it pulls once it ended.
	clock = clock.Add(50 * time.Millisecond)
	if taken, _ := p.take(ui.InputEvent{Kind: ui.InputScroll, DY: -100, Time: clock}, true, false); taken {
		t.Error("the wheel turned on after Shift+wheel pulled")
	}
	clock = clock.Add(pullRest)
	if taken, _ := p.take(ui.InputEvent{Kind: ui.InputScroll, DY: -100, Time: clock}, true, false); !taken {
		t.Error("the wheel after the gesture ended didn't pull")
	}
	// Shift pressed in a pull lets go of it, without refreshing.
	p = pullState{}
	clock = clock.Add(time.Second)
	for range 4 {
		clock = clock.Add(16 * time.Millisecond)
		p.take(ui.InputEvent{Kind: ui.InputScroll, DY: -60, Precise: true, Time: clock}, true, false)
	}
	clock = clock.Add(16 * time.Millisecond)
	if taken, refresh := p.take(ui.InputEvent{Kind: ui.InputScroll, DY: -60, Precise: true, Mods: ui.Shift, Time: clock}, true, false); taken || refresh || p.by != 0 {
		t.Errorf("Shift in a pull: taken %v, refresh %v, pulled %v", taken, refresh, p.by)
	}
	if p.rest(clock.Add(time.Second)) {
		t.Error("the pull Shift let go of refreshed")
	}
}

// TestPullInside never pulls the page with the scrolling of a scroller
// inside it, as the lyrics of the now playing page, nor of the side panel.
func TestPullInside(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	var lines []lyrics.Line
	for i := range 40 {
		lines = append(lines, lyrics.Line{Text: fmt.Sprintf("Paper boats on the river, verse %d", i)})
	}
	lyricsKey := "lyrics:" + testTracks[0].ID
	s.loads[lyricsKey] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Source: "LRCLIB", Lines: lines}}
	tt := testerOf(t, s, 1280, 1200)
	show(s, tt, "/now-playing")
	quiet := func(what string) {
		t.Helper()
		if shown, _ := pulling(tt); shown {
			t.Errorf("scrolling %s pulled the page", what)
		}
		rested(s, tt)
		if s.refreshAt != "" || s.refreshingKey(lyricsKey) {
			t.Errorf("scrolling %s refreshed the page", what)
		}
		tt.Advance(time.Second)
	}
	// The lyrics, scrolled down, then up past their top.
	r, ok := tt.Find(lines[2].Text)
	if !ok {
		t.Fatalf("no lyrics: %q", tt.Texts())
	}
	lx, ly := r.X+r.W/2, r.Y+r.H/2
	swipe(tt, lx, ly, 200)
	tt.Advance(time.Second)
	swipe(tt, lx, ly, -60, -60, -60, -60, -60, -60)
	if y := s.pageScrolled().Y; y != 0 {
		t.Fatalf("scrolling the lyrics scrolled the page to %v", y)
	}
	quiet("the lyrics")
	// The queue, in the side panel.
	if s.panel != "queue" {
		t.Fatalf("the side panel is %q", s.panel)
	}
	r, ok = tt.Find(testTracks[3].Title)
	if !ok {
		t.Fatalf("no queue: %q", tt.Texts())
	}
	swipe(tt, r.X+r.W/2, r.Y+r.H/2, -60, -60, -60, -60)
	quiet("the queue")
	// Over its title, the page pulls: short of the threshold, as the
	// lyrics would ask LRCLIB again.
	r, ok = tt.Find("midnight city")
	if !ok {
		t.Fatalf("no title: %q", tt.Texts())
	}
	swipe(tt, r.X+20, r.Y+r.H/2, -30)
	if shown, _ := pulling(tt); !shown {
		t.Errorf("the page over its title doesn't pull: %q", tt.Texts())
	}
	rested(s, tt)
}

// TestPullDimmed shows nothing and does nothing on a page that has
// nothing to refresh, where the button is dimmed, nor while a refresh is
// out.
func TestPullDimmed(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	for _, path := range []string{"/stats", "/library/liked", "/settings/appearance"} {
		show(s, tt, path)
		if !s.refreshOff {
			t.Fatalf("the button isn't dimmed on %s", path)
		}
		swipe(tt, pullX, pullY, -80, -80, -80)
		if shown, _ := pulling(tt); shown {
			t.Errorf("a pull of %s shows the strip", path)
		}
		rested(s, tt)
		notches(tt, pullX, pullY, 4)
		rested(s, tt)
		if s.refreshAt != "" || s.refreshing() {
			t.Errorf("a pull of %s refreshed", path)
		}
		tt.Advance(time.Second)
	}
}

// TestPullOnce refreshes once a pull, however far it goes and whatever
// scrolling of its gesture comes after it was let go; a pull while the
// refresh is out does nothing.
func TestPullOnce(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	l := pullAlbum(s, 40)
	show(s, tt, "/album/al-x")
	far := make([]float32, 12)
	for i := range far {
		far[i] = -60
	}
	swipe(tt, pullX, pullY, far...)
	// The fingers leave, which an event that scrolls nothing tells.
	tt.Scroll(pullX, pullY, 0, 0)
	tt.Frame()
	if l.asks != 1 {
		t.Fatalf("letting go of a long pull asked %d times", l.asks)
	}
	if !tt.HasText("refreshing") {
		t.Errorf("the strip doesn't say the page refreshes as it closes: %q", tt.Texts())
	}
	// What the gesture scrolls on, at once, pulls nothing.
	swipe(tt, pullX, pullY, far...)
	if _, far := pulling(tt); far || s.pull.by != 0 {
		t.Errorf("the gesture let go of pulled again, %v DIPs", s.pull.by)
	}
	rested(s, tt)
	if l.asks != 1 {
		t.Errorf("the gesture after letting go asked %d times in all", l.asks)
	}
	waitRefreshed(t, s, tt)
	tt.Advance(time.Second)

	// While a refresh is out, a pull does nothing.
	s.refreshPage()
	tt.Frame()
	if !s.refreshing() {
		t.Fatal("no refresh is out")
	}
	swipe(tt, pullX, pullY, far...)
	if shown, _ := pulling(tt); shown {
		t.Error("a pull while a refresh is out shows the strip")
	}
	tt.Advance(pullRest)
	if l.asks != 2 {
		t.Errorf("a pull while a refresh is out asked: %d in all", l.asks)
	}
	waitRefreshed(t, s, tt)
	tt.Advance(time.Second)
	swipe(tt, pullX, pullY, far...)
	rested(s, tt)
	if l.asks != 3 {
		t.Errorf("a pull once the refresh ended asked: %d in all", l.asks)
	}
	waitRefreshed(t, s, tt)
}

// TestPullStill opens the strip whole at once, while the computer asks
// for less motion, and closes it at once.
func TestPullStill(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	pullAlbum(s, 40)
	show(s, tt, "/album/al-x")
	swipe(tt, pullX, pullY, -10)
	if s.pull.shown != pullBar {
		t.Errorf("a pull of 10 DIPs opened the strip %v tall", s.pull.shown)
	}
	rested(s, tt)
	tt.Frame()
	if shown, _ := pulling(tt); shown || s.pull.shown != 0 {
		t.Errorf("the strip didn't close at once: %v tall", s.pull.shown)
	}
}

// TestPullHeight opens the strip in step with the pull to pullBar at
// pullFar, then less and less.
func TestPullHeight(t *testing.T) {
	if h := pullHeight(pullFar / 2); h != pullBar/2 {
		t.Errorf("half the way, the strip is %v", h)
	}
	if h := pullHeight(pullFar); h != pullBar {
		t.Errorf("at the threshold, the strip is %v", h)
	}
	if a, b := pullHeight(2*pullFar)-pullHeight(pullFar), pullHeight(3*pullFar)-pullHeight(2*pullFar); b >= a || pullHeight(100*pullFar) > pullBar+pullStretch {
		t.Errorf("past the threshold, the strip opens %v, then %v more", a, b)
	}
}

// TestRenderPull draws a pull short of the threshold and one past it, to
// MIXMUSIC_RENDER, for a look.
func TestRenderPull(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	s := newTestShell(t)
	pullAlbum(s, 12)
	tt := tester(t, s)
	show(s, tt, "/album/al-x")
	swipe(tt, pullX, pullY, -35, -35)
	shot(t, tt, dir, "pull-short")
	swipe(tt, pullX, pullY, -60, -60)
	shot(t, tt, dir, "pull-far")
}

// shot draws the window as it is to dir, as name.
func shot(t *testing.T, tt *ui.Tester, dir, name string) {
	t.Helper()
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

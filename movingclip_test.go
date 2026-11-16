package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// testClips is a source of clips of two renditions of every album, which
// counts what it was asked.
type testClips struct {
	mu    sync.Mutex
	url   string
	asked int
}

func (c *testClips) Clips(_ context.Context, q app.ClipQuery) ([]app.ClipRendition, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.asked++
	return []app.ClipRendition{
		{URL: c.url + "/720.mp4", Mime: "video/mp4", Codec: "avc1", Width: 720, Height: 720, Bitrate: 1_000_000},
		{URL: c.url + "/1080.mp4", Mime: "video/mp4", Codec: "avc1", Width: 1080, Height: 1080, Bitrate: 2_000_000},
	}, nil
}

// clipShell is a test shell whose albums have clips, playing a song on
// the now playing page with moving covers on: its web view is a nil one,
// which places nowhere, and its page fake.
func clipShell(t *testing.T) (*shell, *ui.Tester, *fakePage, *testClips) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "a clip of "+r.URL.Path)
	}))
	t.Cleanup(srv.Close)
	src := &testClips{url: srv.URL}
	s := newTestShellOf(t, func(o *app.Options) { o.Clips = src })
	set := s.svc.Preferences.Get()
	set.MovingCovers = true
	if _, err := s.svc.Preferences.Set(set); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	s.panel = ""
	page := newFakePage()
	s.video.view = (*mygo.WebView)(nil)
	s.video.families = []string{"vp9", "avc1"}
	s.video.link = newVideoLink(page.call, func() time.Duration { return 0 }, func(st videoStatus) {
		s.update(func() { s.videoTold(st) })
	})
	t.Cleanup(s.video.link.close)
	s.p.PlayTracks([]media.Track{testTracks[0]}, 0)
	s.settle(t, func(st player.State) bool { return st.Playing && st.Source != nil })
	tt := testerOf(t, s, 1600, 1000)
	show(s, tt, "/now-playing")
	return s, tt, page, src
}

// TestNowPlayingClip loops the clip of the album that plays in the
// sleeve's square, muted, in the loop mode of the page of the picture,
// over the cover until its first frame; it pauses with the music, and
// stops on another page. A box over it opens the track's menu.
func TestNowPlayingClip(t *testing.T) {
	s, tt, page, src := clipShell(t)
	track := testTracks[0]
	js := framesUntil(t, s, tt, page, "mv.load(")
	if !strings.Contains(js, `"loop":true`) || !strings.Contains(js, "moving-cover/") {
		t.Errorf("loads %s", js)
	}
	framesUntil(t, s, tt, page, "playing: true, active: true")
	tt.Frame()
	r, ok := tt.Find("Album clip: " + track.Title)
	if !ok {
		t.Fatalf("no clip: %q", tt.Texts())
	}
	if r.W != r.H || r.W < 260 {
		t.Errorf("the clip is %v × %v", r.W, r.H)
	}
	if _, ok := tt.Find("Visualizer"); ok {
		t.Error("the visualizer draws over the clip")
	}
	tt.RightClickAt(r.X+r.W/2, r.Y+r.H/2)
	if !slices.Contains(tt.Menu(), "Find the moving cover again") {
		t.Errorf("right-clicking the clip opens %q", tt.Menu())
	}
	tt.CloseMenu()

	s.p.Toggle()
	s.settle(t, func(st player.State) bool { return !st.Playing })
	tt.Frame()
	framesUntil(t, s, tt, page, "playing: false, active: true")
	show(s, tt, "/")
	framesUntil(t, s, tt, page, "active: false")
	if src.asked != 1 {
		t.Errorf("the source was asked %d times", src.asked)
	}
}

// TestNowPlayingClipFails tries the clip of another rendition once when
// the page can't play one, then keeps the cover, and forgets the clip.
func TestNowPlayingClipFails(t *testing.T) {
	s, tt, page, _ := clipShell(t)
	first := framesUntil(t, s, tt, page, "mv.load(")
	framesUntil(t, s, tt, page, "playing: true")
	fail := func(js string) {
		t.Helper()
		var src string
		if _, rest, ok := strings.Cut(js, `"src":"`); ok {
			src, _, _ = strings.Cut(rest, `"`)
		}
		page.mu.Lock()
		page.status = videoStatus{Src: src, Failed: "the clip showed no new frame for 5 seconds"}
		page.mu.Unlock()
		s.p.Toggle()
		s.settle(t, func(st player.State) bool { return st.Playing == !s.st.Playing })
		tt.Frame()
	}
	fail(first)
	second := framesUntil(t, s, tt, page, "mv.load(")
	if second == first {
		t.Fatalf("loads the same clip again: %s", second)
	}
	fail(second)
	until(t, s, tt, "the cover again", func() bool { return s.moving.clips.broken[testTracks[0].ID] })
	if _, ok := tt.Find("Album clip: " + testTracks[0].Title); ok {
		t.Error("a clip that failed twice still shows")
	}
	if u := s.svc.System.MovingCoverUsage(); u.Clips != 0 {
		t.Errorf("the clip that failed is kept: %+v", u)
	}
}

// TestNowPlayingClipReducedMotion keeps the cover while the system asks
// for less motion, with a button that plays the clip.
func TestNowPlayingClipReducedMotion(t *testing.T) {
	s, tt, _, _ := clipShell(t)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	until(t, s, tt, "the button", func() bool { return tt.HasText("Play the moving cover") || hasLabel(tt, "Play the moving cover") })
	if _, ok := tt.Find("Album clip: " + testTracks[0].Title); ok {
		t.Error("the clip plays by itself")
	}
	click(t, s, tt, "Play the moving cover")
	tt.Advance(time.Second)
	if _, ok := tt.Find("Album clip: " + testTracks[0].Title); !ok {
		t.Errorf("the button didn't play it: %q", tt.Texts())
	}
}

// framesUntil draws frames until the page runs a script that contains
// part, which it returns.
func framesUntil(t *testing.T, s *shell, tt *ui.Tester, page *fakePage, part string) string {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		select {
		case js := <-page.ran:
			if strings.Contains(js, part) {
				return js
			}
		case <-time.After(10 * time.Millisecond):
			s.drain()
			tt.Frame()
		}
	}
	t.Fatalf("the page ran nothing with %q", part)
	return ""
}

func hasLabel(tt *ui.Tester, label string) bool {
	_, ok := tt.Find(label)
	return ok
}

// TestNowPlayingNoClips looks for no clip while no source of them is set,
// as until one is chosen, nor offers to look again.
func TestNowPlayingNoClips(t *testing.T) {
	s := newTestShell(t)
	tt := movingShell(t, s, testTracks[0], true)
	tt.Advance(time.Second)
	for key := range s.loads {
		if strings.HasPrefix(key, "clip:") {
			t.Errorf("looks for a clip: %s", key)
		}
	}
	if err := tt.RightClick("Midnight City"); err == nil && slices.Contains(tt.Menu(), "Find the moving cover again") {
		t.Error("offers to look for a clip again")
	}
}

// TestVideoSendLoop tells the page of a clip that loops nothing of the
// engine's time while it plays: it follows no clock.
func TestVideoSendLoop(t *testing.T) {
	now := time.Now()
	src := &app.VideoSource{Src: "stream://localhost/moving-cover/a.mp4", Loop: true}
	want := videoWant{src: src, active: true, playing: true, pos: 5 * time.Second}
	prev := videoSent{src: src.Src, active: true, playing: true, pos: time.Second, at: now}
	if videoSend(prev, want, now.Add(time.Minute)) {
		t.Error("the page of a clip hears the time")
	}
	want.playing = false
	if !videoSend(prev, want, now.Add(time.Second)) {
		t.Error("the page of a clip doesn't hear it paused")
	}
}

package main

import (
	"context"
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

func TestVideoSend(t *testing.T) {
	src := &app.VideoSource{Src: "stream://localhost/video/a?itag=243"}
	other := &app.VideoSource{Src: "stream://localhost/video/b?itag=243"}
	at := time.Unix(1000, 0)
	playing := videoSent{src: src.Src, active: true, playing: true, pos: 10 * time.Second, at: at}
	paused := videoSent{src: src.Src, active: true, pos: 10 * time.Second, at: at}
	idle := videoSent{src: src.Src, at: at}
	for _, c := range []struct {
		name string
		prev videoSent
		want videoWant
		now  time.Time
		send bool
	}{
		{"the first picture", videoSent{}, videoWant{src: src, active: true, playing: true}, at, true},
		{"another picture", playing, videoWant{src: other, active: true, playing: true}, at, true},
		{"playing, soon after", playing, videoWant{src: src, active: true, playing: true, pos: 11 * time.Second}, at.Add(100 * time.Millisecond), false},
		{"playing, a while after", playing, videoWant{src: src, active: true, playing: true}, at.Add(videoEvery), true},
		{"paused", playing, videoWant{src: src, active: true, pos: 10 * time.Second}, at.Add(time.Millisecond), true},
		{"buffering", playing, videoWant{src: src, active: true, pos: 10 * time.Second}, at.Add(time.Millisecond), true},
		{"paused, a while after", paused, videoWant{src: src, active: true, pos: 10 * time.Second}, at.Add(time.Hour), false},
		{"a seek while paused", paused, videoWant{src: src, active: true, pos: 42 * time.Second}, at.Add(time.Millisecond), true},
		{"resumed", paused, videoWant{src: src, active: true, playing: true, pos: 10 * time.Second}, at.Add(time.Millisecond), true},
		{"no longer shown", playing, videoWant{src: src}, at.Add(time.Millisecond), true},
		{"still not shown", idle, videoWant{src: src}, at.Add(time.Hour), false},
		{"shown again", idle, videoWant{src: src, active: true, playing: true}, at.Add(time.Millisecond), true},
		{"a theme's tint", playing, videoWant{src: src, active: true, playing: true, filter: videoFilter}, at.Add(time.Millisecond), true},
		{"nothing to play", videoSent{}, videoWant{}, at, false},
	} {
		if got := videoSend(c.prev, c.want, c.now); got != c.send {
			t.Errorf("%s: send %v, want %v", c.name, got, c.send)
		}
	}
}

func TestVideoShape(t *testing.T) {
	wide := []media.Image{{URL: "s", Width: 60, Height: 60}, {URL: "l", Width: 400, Height: 225}}
	for _, c := range []struct {
		name   string
		images []media.Image
		src    *app.VideoSource
		w, h   float32
	}{
		{"a thumbnail of 16:9", wide, nil, 420, 236},
		{"its stream's shape first", wide, &app.VideoSource{Width: 480, Height: 854}, 236, 420},
		{"a square stream", wide, &app.VideoSource{Width: 480, Height: 480}, 420, 420},
		{"no thumbnail", nil, nil, 420, 236},
		{"a strip", []media.Image{{Width: 1000, Height: 10}}, nil, 420, 175},
	} {
		w, h := videoShape(videoAspect(c.images, c.src), 420)
		if w != c.w || h != c.h {
			t.Errorf("%s: %v × %v, want %v × %v", c.name, w, h, c.w, c.h)
		}
	}
}

// fakePage is the page of the picture as a videoLink talks to it: it keeps
// the scripts it runs, and answers with status.
type fakePage struct {
	mu      sync.Mutex
	scripts []string
	status  videoStatus
	err     error // what the next call fails with
	ran     chan string
}

func newFakePage() *fakePage { return &fakePage{ran: make(chan string, 100)} }

func (f *fakePage) call(_ context.Context, js string) (videoStatus, error) {
	f.mu.Lock()
	f.scripts = append(f.scripts, js)
	st, err := f.status, f.err
	f.err = nil
	f.mu.Unlock()
	f.ran <- js
	return st, err
}

// fails has the next call fail with err.
func (f *fakePage) fails(err error) {
	f.mu.Lock()
	f.err = err
	f.mu.Unlock()
}

// next returns the next script the page runs.
func (f *fakePage) next(t *testing.T) string {
	t.Helper()
	select {
	case js := <-f.ran:
		return js
	case <-time.After(5 * time.Second):
		t.Fatal("the page heard nothing")
		return ""
	}
}

// none checks that the page runs nothing more for a while.
func (f *fakePage) none(t *testing.T) {
	t.Helper()
	select {
	case js := <-f.ran:
		t.Errorf("the page ran %s", js)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestVideoLink(t *testing.T) {
	page := newFakePage()
	reports := make(chan videoStatus, 10)
	l := newVideoLink(page.call, func() time.Duration { return 61500 * time.Millisecond }, func(st videoStatus) { reports <- st })
	t.Cleanup(l.close)
	src := &app.VideoSource{Src: "stream://localhost/video/a?itag=243", MimeType: `video/webm; codecs="vp9"`, Init: 220}

	page.status = videoStatus{Src: src.Src}
	l.post(videoCmd{src: src, active: true, playing: true})
	if js := page.next(t); !strings.HasPrefix(js, "return mv.load(") || !strings.Contains(js, `"init":220`) {
		t.Errorf("loads with %s", js)
	}
	if js := page.next(t); !strings.Contains(js, "mv.sync({at: 61.5000, sentAt: ") || !strings.Contains(js, "playing: true, active: true") {
		t.Errorf("syncs with %s", js)
	}
	if st := <-reports; st.Src != src.Src || st.Shown {
		t.Errorf("reported %+v", st)
	}
	// The same picture loads once; a status that changed but for its
	// drift isn't told again.
	page.mu.Lock()
	page.status.Drift.Max = 12
	page.mu.Unlock()
	l.post(videoCmd{src: src, active: true})
	if js := page.next(t); !strings.Contains(js, "playing: false, active: true") {
		t.Errorf("pauses with %s", js)
	}
	page.mu.Lock()
	page.status.Shown = true
	page.mu.Unlock()
	l.post(videoCmd{src: src, active: false})
	if js := page.next(t); !strings.Contains(js, "active: false") {
		t.Errorf("stops with %s", js)
	}
	if st := <-reports; !st.Shown {
		t.Errorf("reported %+v", st)
	}
	select {
	case st := <-reports:
		t.Errorf("reported %+v more", st)
	default:
	}
	// A page that doesn't answer in time is told the picture again.
	page.fails(context.DeadlineExceeded)
	l.post(videoCmd{src: src, active: true, playing: true})
	page.next(t)
	l.post(videoCmd{src: src, active: true, playing: true})
	if js := page.next(t); !strings.HasPrefix(js, "return mv.load(") {
		t.Errorf("after no answer, %s", js)
	}
	page.next(t)
	// So is one that lost it, as when it loaded again.
	page.mu.Lock()
	page.status = videoStatus{}
	page.mu.Unlock()
	l.post(videoCmd{src: src, active: true, playing: true})
	page.next(t)
	if st := <-reports; st.Src != "" {
		t.Errorf("reported %+v", st)
	}
	page.mu.Lock()
	page.status = videoStatus{Src: src.Src}
	page.mu.Unlock()
	l.post(videoCmd{src: src, active: true, playing: true})
	if js := page.next(t); !strings.HasPrefix(js, "return mv.load(") {
		t.Errorf("after a page that lost the picture, %s", js)
	}
	page.next(t)
	<-reports
	// What the page throws is why the picture failed.
	page.fails(pageError{"mv is not defined"})
	l.post(videoCmd{src: src, active: true, playing: true})
	page.next(t)
	if st := <-reports; st.Src != src.Src || st.Failed != "mv is not defined" {
		t.Errorf("reported %+v", st)
	}

	l.post(videoCmd{})
	if js := page.next(t); js != "return mv.clear()" {
		t.Errorf("clears with %s", js)
	}
	l.post(videoCmd{})
	page.none(t)
}

// videoShell is a test shell playing a music video, whose picture shows:
// its web view is a nil one, which places nowhere, and its page fake.
func videoShell(t *testing.T, track media.Track) (*shell, *ui.Tester, *fakePage, *app.VideoSource) {
	t.Helper()
	s := newTestShell(t)
	set := s.svc.Preferences.Get()
	set.MusicVideos = true
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
	src := &app.VideoSource{Src: "stream://localhost/video/x?itag=243", MimeType: `video/webm; codecs="vp9"`, Width: 640, Height: 360, Init: 220}
	w, h := videoShape(videoAspect(track.Artwork, nil), videoFrame)
	s.loads["video:"+track.ID+":"+itoa(s.artPixels(w))+"x"+itoa(s.artPixels(h))] = &loaded[app.VideoSource]{done: true, value: *src}
	s.loads["related-"+track.ID] = &loaded[[]media.Shelf]{done: true}
	s.p.PlayTracks([]media.Track{track}, 0)
	s.settle(t, func(st player.State) bool { return st.Playing && st.Source != nil })
	tt := testerOf(t, s, 1600, 1000)
	show(s, tt, "/now-playing")
	return s, tt, page, src
}

func itoa(n int) string { return count(n) }

// musicVideo is a music video whose thumbnail is 16:9, as theirs are.
func musicVideo() media.Track {
	t := testTrack("yt:Kq-7hp9i4lE", "Ajen ajen", "Laila Ayu", "", media.SourceYouTube)
	t.Album = nil
	t.Video, t.VideoType = true, media.VideoOfficial
	t.Artwork = []media.Image{{URL: "https://i.ytimg.com/vi/Kq-7hp9i4lE/sddefault.jpg", Width: 640, Height: 360}}
	return t
}

// waitScript returns the first script the page runs that contains part.
func waitScript(t *testing.T, s *shell, page *fakePage, part string) string {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		select {
		case js := <-page.ran:
			if strings.Contains(js, part) {
				return js
			}
		case <-time.After(10 * time.Millisecond):
			s.drain()
		}
	}
	t.Fatalf("the page ran nothing with %q", part)
	return ""
}

// TestNowPlayingVideo plays the picture of a music video in place of its
// cover, in a frame of its shape, without the visualizer; a box over it
// opens the track's menu, and keeps the pointer from the page.
func TestNowPlayingVideo(t *testing.T) {
	track := musicVideo()
	s, tt, page, src := videoShell(t, track)
	r, ok := tt.Find("Music video: " + track.Title)
	if !ok {
		t.Fatalf("no picture: %q", tt.Texts())
	}
	if r.W != 420 || r.H != 236 {
		t.Errorf("the frame is %v × %v", r.W, r.H)
	}
	if _, ok := tt.Find("Visualizer"); ok {
		t.Error("the visualizer draws over the picture")
	}
	if js := waitScript(t, s, page, "mv.load("); !strings.Contains(js, src.Src) {
		t.Errorf("loads %s", js)
	}
	waitScript(t, s, page, "playing: true, active: true")
	x, y := r.X+r.W/2, r.Y+r.H/2
	if v := tt.WebViewAt(x, y); v != nil {
		t.Error("the page takes the pointer")
	}
	tt.RightClickAt(x, y)
	if !slices.Contains(tt.Menu(), "Add to queue") {
		t.Errorf("right-clicking the picture opens %q", tt.Menu())
	}
	tt.CloseMenu()
	if _, ok := tt.Find("Show the cover"); !ok {
		t.Error("no button shows the cover")
	}

	// Another page stops it: nothing more is fetched while it doesn't show.
	show(s, tt, "/")
	waitScript(t, s, page, "active: false")
	show(s, tt, "/now-playing")
	waitScript(t, s, page, "playing: true, active: true")

	// So does the window hiding, as in the tray: no frame comes then.
	s.idleVideo()
	waitScript(t, s, page, "active: false")
	tt.Frame()
	tt.Advance(time.Second)
	waitScript(t, s, page, "active: true")

	// And pausing.
	s.p.Toggle()
	s.settle(t, func(st player.State) bool { return !st.Playing })
	tt.Frame()
	waitScript(t, s, page, "playing: false, active: true")
}

// TestNowPlayingVideoOff keeps the cover and the visualizer of songs, of
// music videos with the setting off, and of a picture that failed.
func TestNowPlayingVideoOff(t *testing.T) {
	track := musicVideo()
	s, tt, page, src := videoShell(t, track)
	click(t, s, tt, "Show the cover")
	if s.svc.Preferences.Get().MusicVideos {
		t.Error("the button left the setting on")
	}
	tt.Advance(time.Second)
	if _, ok := tt.Find("Music video: " + track.Title); ok {
		t.Error("the picture shows with the setting off")
	}
	if _, ok := tt.Find("Visualizer"); !ok {
		t.Error("no visualizer over the cover")
	}
	if _, ok := tt.Find("Show the video"); !ok {
		t.Error("no button shows the video")
	}
	click(t, s, tt, "Show the video")
	tt.Advance(time.Second)
	if _, ok := tt.Find("Music video: " + track.Title); !ok {
		t.Fatal("the picture doesn't show again")
	}
	// The page tells the picture failed: the cover comes back.
	page.mu.Lock()
	page.status = videoStatus{Src: src.Src, Failed: "the picture's data doesn't decode"}
	page.mu.Unlock()
	s.p.Toggle()
	s.settle(t, func(st player.State) bool { return !st.Playing })
	for deadline := time.Now().Add(5 * time.Second); !s.video.broken[track.ID]; time.Sleep(10 * time.Millisecond) {
		s.drain()
		tt.Frame()
		if time.Now().After(deadline) {
			t.Fatal("the failure didn't come back")
		}
	}
	tt.Frame()
	if _, ok := tt.Find("Music video: " + track.Title); ok {
		t.Error("a picture that failed still shows")
	}
	if _, ok := tt.Find("Show the video"); ok {
		t.Error("a picture that failed still offers to show")
	}
}

func TestNowPlayingSongKeepsItsCover(t *testing.T) {
	for _, track := range []media.Track{
		testTracks[0],
		func() media.Track {
			t := musicVideo()
			t.VideoType = "MUSIC_VIDEO_TYPE_PODCAST_EPISODE"
			return t
		}(),
	} {
		s, tt, _, _ := videoShell(t, track)
		if _, ok := tt.Find("Music video: " + track.Title); ok {
			t.Errorf("%s shows a picture", track.Title)
		}
		if _, ok := tt.Find("Show the cover"); ok {
			t.Errorf("%s offers its video", track.Title)
		}
		if s.video.shown || s.video.lastSrc != nil {
			t.Errorf("%s: the page was told to play", track.Title)
		}
	}
}

// TestNowPlayingVideoReducedMotion keeps the cover while the system asks
// for less motion, with a button that plays the picture.
func TestNowPlayingVideoReducedMotion(t *testing.T) {
	track := musicVideo()
	s, tt, _, _ := videoShell(t, track)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	if _, ok := tt.Find("Music video: " + track.Title); ok {
		t.Error("the picture plays by itself")
	}
	click(t, s, tt, "Play the video")
	tt.Advance(time.Second)
	if _, ok := tt.Find("Music video: " + track.Title); !ok {
		t.Errorf("the button didn't play it: %q", tt.Texts())
	}
}

// TestNowPlayingVideoNoWebView keeps the cover where the system has no
// web view, and says so in the settings.
func TestNowPlayingVideoNoWebView(t *testing.T) {
	saved := webViewProblem
	webViewProblem = func(*shell) string { return "Showing music videos needs WebKitGTK" }
	t.Cleanup(func() { webViewProblem = saved })
	track := musicVideo()
	_, tt, _, _ := videoShell(t, track)
	if _, ok := tt.Find("Music video: " + track.Title); ok {
		t.Error("a picture without a web view")
	}
	if _, ok := tt.Find("Show the video"); ok {
		t.Error("the button shows without a web view")
	}
}

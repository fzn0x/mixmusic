package main

// The picture of music videos on the now playing page. MyGo draws no
// video, so a web view of the main window shows a page of MixMusic's own,
// public/video.html, in place of the cover: it plays the picture, muted,
// through Media Source Extensions, from the proxy (internal/app/video.go),
// and follows the engine's clock, which the window sends it a few times a
// second. The web view is made the first time a picture shows, and closed a
// minute after the last one did.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

const (
	// videoKeep is how long the web view of the picture stays once no
	// picture shows: it takes memory, and making it again a moment.
	videoKeep = time.Minute
	// videoEvery is how often, at most, the page hears the engine's time
	// while the picture plays.
	videoEvery = 250 * time.Millisecond
	// videoLoad bounds how long the page of the picture takes to load.
	videoLoad = 15 * time.Second
	// videoFade is how long the picture takes to show over the cover.
	videoFade = 400 * time.Millisecond
	// videoFrame is the largest side of the frame, at which the stream is
	// chosen, whatever the frame's size now: one stream from a narrow
	// window to a wide one.
	videoFrame = 420
)

// videoFilter is Terminal's tint of artwork, for the page: what
// phosphorize does to covers.
const videoFilter = "grayscale(1) sepia(1) hue-rotate(60deg) saturate(3) brightness(0.85)"

// webViewProblem tells why the picture of music videos can't show, or ""
// when it can: it asks MyGo's main loop, which tests don't run.
var webViewProblem = func(s *shell) string { return s.svc.System.WebViewProblem() }

// videoStatus is what the page tells of the picture it plays.
type videoStatus struct {
	// Src is the stream it plays, Shown set once its first frame showed,
	// and Failed why it couldn't play.
	Src    string `json:"src"`
	Shown  bool   `json:"shown"`
	Failed string `json:"failed"`
}

// videoWant is what a frame wants the page to do: play the picture of
// src, of the recording track, while active and playing, at pos, the
// player's position.
type videoWant struct {
	src     *app.VideoSource
	track   string
	active  bool
	playing bool
	pos     time.Duration
	filter  string
}

// videoSent is what the page was told last, and when.
type videoSent struct {
	src     string
	active  bool
	playing bool
	pos     time.Duration
	filter  string
	at      time.Time
}

// videoSend tells whether the page is to be told want, when it was told
// prev last: of another picture, as it shows or hides, plays or pauses,
// of a move while paused, as a seek, or of the engine's time, every
// videoEvery while it plays.
func videoSend(prev videoSent, want videoWant, now time.Time) bool {
	src := ""
	if want.src != nil {
		src = want.src.Src
	}
	switch {
	case src != prev.src, want.active != prev.active, want.playing != prev.playing, want.filter != prev.filter:
		return true
	case !want.active || src == "":
		return false
	case want.playing:
		return now.Sub(prev.at) >= videoEvery
	}
	return want.pos != prev.pos
}

// videoCmd is what the link tells the page.
type videoCmd struct {
	src     *app.VideoSource
	active  bool
	playing bool
	filter  string
}

// videoLink talks to the page of the picture from a goroutine of its own,
// one command at a time, the newest: the page gets a call at a time, and
// none waits behind a slow one.
type videoLink struct {
	// call runs js in the page and reads the status it returns; clock is
	// the engine's time, and report takes what the page tells when it
	// changed.
	call   func(ctx context.Context, js string) (videoStatus, error)
	clock  func() time.Duration
	report func(videoStatus)

	mu   sync.Mutex
	next *videoCmd
	wake chan struct{}
	stop chan struct{}
	once sync.Once
	// loaded is the stream the page plays, and last what it told last; the
	// goroutine's own.
	loaded string
	last   videoStatus
}

func newVideoLink(call func(context.Context, string) (videoStatus, error), clock func() time.Duration, report func(videoStatus)) *videoLink {
	l := &videoLink{call: call, clock: clock, report: report, wake: make(chan struct{}, 1), stop: make(chan struct{})}
	go l.run()
	return l
}

// post has the page do cmd, in place of a command not sent yet.
func (l *videoLink) post(cmd videoCmd) {
	l.mu.Lock()
	l.next = &cmd
	l.mu.Unlock()
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

// close stops the goroutine.
func (l *videoLink) close() { l.once.Do(func() { close(l.stop) }) }

func (l *videoLink) run() {
	for {
		select {
		case <-l.stop:
			return
		case <-l.wake:
		}
		l.mu.Lock()
		cmd := l.next
		l.next = nil
		l.mu.Unlock()
		if cmd != nil {
			l.send(*cmd)
		}
	}
}

// send tells the page cmd: the picture to load, when it changed, then the
// engine's time and whether the picture plays, which the time is read
// just before, and stamped with.
func (l *videoLink) send(cmd videoCmd) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var st videoStatus
	var err error
	switch {
	case cmd.src == nil && l.loaded == "":
		return
	case cmd.src == nil:
		l.loaded = ""
		st, err = l.call(ctx, "return mv.clear()")
	default:
		if cmd.src.Src != l.loaded {
			desc, _ := json.Marshal(cmd.src)
			if _, err = l.call(ctx, "return mv.load("+string(desc)+")"); err != nil {
				break
			}
			l.loaded = cmd.src.Src
		}
		filter, _ := json.Marshal(cmd.filter)
		at := l.clock()
		sentAt := float64(time.Now().UnixMicro()) / 1000
		st, err = l.call(ctx, fmt.Sprintf("return mv.sync({at: %s, sentAt: %s, playing: %t, active: %t, filter: %s})",
			strconv.FormatFloat(at.Seconds(), 'f', 4, 64), strconv.FormatFloat(sentAt, 'f', 3, 64), cmd.playing, cmd.active, filter))
	}
	var thrown pageError
	switch {
	case err == nil:
		if cmd.src != nil && st.Src != cmd.src.Src {
			// The page lost the picture, as when it loaded again: the next
			// command loads it.
			l.loaded = ""
		}
	case cmd.src != nil && errors.As(err, &thrown):
		st = videoStatus{Src: cmd.src.Src, Failed: thrown.msg}
	default:
		// No answer in time, or no page: what the page holds isn't known,
		// so the next command loads the picture anew, or clears it.
		l.loaded = unknownPicture
		return
	}
	if st != l.last {
		l.last = st
		l.report(st)
	}
}

// unknownPicture is what a videoLink takes the page to play when it
// didn't answer: no picture's stream.
const unknownPicture = "?"

// pageError is what the page threw, as it words it.
type pageError struct{ msg string }

func (e pageError) Error() string { return e.msg }

// pageCall returns the call of a videoLink to page: it runs js there and
// reads what it returns as a videoStatus. What the page throws is a
// pageError.
func pageCall(page *mygo.Page) func(context.Context, string) (videoStatus, error) {
	return func(ctx context.Context, js string) (videoStatus, error) {
		var st videoStatus
		v, err := page.EvalContext(ctx, js)
		if err != nil {
			var evalErr *mygo.EvalError
			if errors.As(err, &evalErr) {
				return st, pageError{strings.TrimPrefix(evalErr.Message, "Error: ")}
			}
			return st, err
		}
		data, err := json.Marshal(v)
		if err == nil {
			err = json.Unmarshal(data, &st)
		}
		return st, err
	}
}

// videoView is the picture of music videos in the main window, whose
// state belongs to the main thread.
type videoView struct {
	// problem is why no picture shows here, once checked: no web view,
	// or a page that didn't load.
	problem string
	checked bool
	// web is the web view, once made, and view what ui.WebView shows of
	// it, link what talks to its page and families the codecs it plays,
	// in the order it prefers them; making is set while it is made.
	web      *mygo.WebView
	view     ui.NativeWebView
	link     *videoLink
	families []string
	making   bool
	// status is what the page told last.
	status videoStatus
	// broken holds the tracks whose picture couldn't play, which keep
	// their cover, and played those whose picture the user asked for while
	// the system asks for less motion.
	broken, played map[string]bool
	// want is what the frame built wants the page to do, built whether it
	// built the picture, and shown whether the frame before did; sent is
	// what the page was told last, and lastSrc the picture it plays, of
	// lastTrack, which it keeps while none shows, to go on with when it
	// shows again.
	want      videoWant
	built     bool
	shown     bool
	sent      videoSent
	lastSrc   *app.VideoSource
	lastTrack string
	// keep closes the web view once no picture showed for videoKeep.
	keep *time.Timer
}

// videoProblem tells why no picture of music videos can show, or "".
func (s *shell) videoProblem() string {
	v := &s.video
	if !v.checked {
		v.checked = true
		v.problem = webViewProblem(s)
	}
	return v.problem
}

// videoTrack returns the recording that plays, when the now playing page
// shows its picture: a music video playing in the engine, with the setting
// on, a web view to show it, and the mini player closed, the main window
// showing no picture while it is open.
func (s *shell) videoTrack() (media.Track, bool) {
	src := s.st.Source
	if !s.set.MusicVideos || src == nil || src.Mode != player.ModeStream || !src.Track.MusicVideo() ||
		s.video.broken[src.Track.ID] || s.videoProblem() != "" || s.svc.System.MiniPlayerOpen() {
		return media.Track{}, false
	}
	return src.Track, true
}

// sleeve is the now playing page's sleeve, size wide: the cover, with the
// visualizer over its foot while it plays, or the picture of the music
// video that plays.
func (s *shell) sleeve(c *ui.Context, t media.Track, size float32) {
	if rec, ok := s.videoTrack(); ok {
		s.videoSleeve(c, t, rec, size)
		return
	}
	ui.Box(c).Size(size, size).Shrink(0).Children(func() {
		s.cover(c, "sleeve", t.Artwork, size, false)
		if s.st.Playing {
			s.visualizer(c, size)
		}
	})
}

// videoSleeve is the sleeve of the music video rec, the recording of t
// that plays: a frame of the picture's shape inside the square of side
// size, which shows the video's thumbnail until the picture's first frame,
// and the picture then. A box over it keeps the pointer and the keyboard
// from the page, and opens the track's menu.
func (s *shell) videoSleeve(c *ui.Context, t, rec media.Track, size float32) {
	k := &s.lk
	v := &s.video
	art := rec.Artwork
	if len(art) == 0 {
		art = t.Artwork
	}
	res := s.videoSource(rec)
	var src *app.VideoSource
	if res != nil && res.done && res.err == nil {
		src = &res.value
	}
	w, h := videoShape(videoAspect(art, src), size)
	ui.Box(c.Key("sleeve-video")).Size(w, h).Shrink(0).Radius(k.radiusSm).Clip().Children(func() {
		s.videoCover(c, art, w, h)
		switch {
		case c.Preferences().ReduceMotion && !v.played[rec.ID]:
			// No motion the user didn't ask for: a play button asks.
			b := s.buttonBase(c.Key("sleeve-video-play")).Size(42, 42).Radius(21).Background(k.accent).Absolute().
				Right(8).Bottom(8).Label("Play the video").Tooltip("Play the video").Cursor(ui.CursorPointer)
			if b.Clicked() {
				if v.played == nil {
					v.played = map[string]bool{}
				}
				v.played[rec.ID] = true
			}
			b.Children(func() { ui.Icon(c, icon("play")).FontSize(18).TextColor(k.accentText) })
			return
		case v.view == nil:
			s.makeVideoView()
			return
		case src == nil:
			if res != nil && res.done {
				s.videoFailed(rec, res.err.Error())
			}
			return
		}
		shown := v.status.Src == src.Src && v.status.Shown
		web := ui.WebView(c.Key("video "+src.Src), v.view).Absolute().Left(0).Top(0).Size(w, h).Label("Music video: " + t.Title)
		target := float32(0)
		if shown {
			target = 1
		}
		// Until its first frame, the page shows nothing over the cover.
		web.Opacity(web.Animate("fade", target, videoFade))
		ui.Box(c.Key("sleeve-video-over")).Absolute().Left(0).Top(0).Size(w, h).ContextMenu(func(m *ui.Menu) { s.trackMenu(m, t) })
		filter := ""
		if k.fx.phosphor {
			filter = videoFilter
		}
		v.want = videoWant{src: src, track: rec.ID, active: true, playing: s.st.Playing && !s.st.Buffering, pos: s.st.Position, filter: filter}
		v.built = true
	})
}

// videoSource returns the stream of the picture of rec for the page, once
// the page told the codecs it plays: nil until then. It is chosen for the
// largest frame, so that a window resized keeps it.
func (s *shell) videoSource(rec media.Track) *loaded[app.VideoSource] {
	v := &s.video
	if v.view == nil || len(v.families) == 0 {
		return nil
	}
	w, h := videoShape(videoAspect(rec.Artwork, nil), videoFrame)
	pw, ph := s.artPixels(w), s.artPixels(h)
	families := v.families
	return fetch(s, "video:"+rec.ID+":"+strconv.Itoa(pw)+"x"+strconv.Itoa(ph), func(ctx context.Context) (app.VideoSource, error) {
		return s.svc.Player.Video(ctx, rec, pw, ph, families)
	})
}

// videoCover is the thumbnail of a music video, filling a frame of w × h.
func (s *shell) videoCover(c *ui.Context, images []media.Image, w, h float32) {
	frame := ui.Box(c.Key("sleeve")).Size(w, h).Shrink(0)
	var b *ui.Bitmap
	if s.inView(c, frame) {
		b = s.art.get(c, artworkURL(images, int(w*s.zoomOf())), s.artPixels(w))
	}
	if b == nil {
		frame.Background(s.lk.surface3)
	}
	frame.Children(func() {
		if b != nil {
			ui.Image(c, b).Size(w, h).Fit(ui.Cover)
		}
	})
}

// videoAspect is the shape of the picture of a music video, its width over
// its height: the stream's, once known, else its thumbnail's, which music
// videos' are, else 16:9. A shape out of reason is kept within it.
func videoAspect(images []media.Image, src *app.VideoSource) float32 {
	aspect := float32(16) / 9
	switch {
	case src != nil && src.Width > 0 && src.Height > 0:
		aspect = float32(src.Width) / float32(src.Height)
	default:
		largest := 0
		for _, img := range images {
			if img.Width > largest && img.Height > 0 {
				largest, aspect = img.Width, float32(img.Width)/float32(img.Height)
			}
		}
	}
	return min(max(aspect, 0.5), 2.4)
}

// videoShape returns the frame of a picture of aspect inside a square of
// side size: a landscape picture keeps the square's width, a portrait
// one its height, in whole points.
func videoShape(aspect, size float32) (w, h float32) {
	if aspect >= 1 {
		return size, float32(int(size/aspect + 0.5))
	}
	return float32(int(size*aspect + 0.5)), size
}

// videoFailed keeps the cover of a recording whose picture couldn't play,
// and logs why, once.
func (s *shell) videoFailed(rec media.Track, why string) {
	v := &s.video
	if v.broken[rec.ID] {
		return
	}
	if v.broken == nil {
		v.broken = map[string]bool{}
	}
	v.broken[rec.ID] = true
	log.Printf("mixmusic: the picture of %s doesn't play: %s", rec.ID, why)
}

// makeVideoView makes the web view of the picture, beside the main thread,
// and loads its page. Where it can't, the covers stay, and the log tells
// why, once.
func (s *shell) makeVideoView() {
	v := &s.video
	if v.making || v.view != nil || s.win == nil || v.problem != "" {
		return
	}
	v.making = true
	win := s.win
	go func() {
		web, families, err := openVideoPage(win)
		s.update(func() {
			v.making = false
			if err != nil {
				v.problem = "The picture of music videos couldn't load here."
				log.Printf("mixmusic: the picture of music videos can't show: %v", err)
				if web != nil {
					go web.Destroy()
				}
				return
			}
			v.web, v.view, v.families = web, web, families
			s.a.SetVideoPage(web.Page())
			v.link = newVideoLink(pageCall(web.Page()), s.svc.Player.Clock, func(st videoStatus) {
				s.update(func() { s.videoTold(st) })
			})
		})
	}()
}

// openVideoPage makes the web view of the picture in win, and waits for its
// page, which tells the codecs it plays.
func openVideoPage(win *mygo.Window) (*mygo.WebView, []string, error) {
	web, err := win.NewWebView(mygo.WebViewOptions{URL: "/video.html", BackgroundColor: "#000000"})
	if err != nil {
		return nil, nil, err
	}
	page := web.Page()
	for deadline := time.Now().Add(videoLoad); ; time.Sleep(100 * time.Millisecond) {
		if ok, _ := mygo.EvalAs[bool](page, "return typeof mv === 'object'"); ok {
			break
		}
		if time.Now().After(deadline) || web.IsDestroyed() {
			return web, nil, errors.New("its page didn't load")
		}
	}
	families, err := mygo.EvalAs[[]string](page, "return await mv.codecs()")
	switch {
	case err != nil:
		return web, nil, err
	case len(families) == 0:
		return web, nil, errors.New("the web view plays neither VP9 nor H.264 through Media Source Extensions")
	}
	return web, families, nil
}

// videoTold takes in what the page told: the first frame shown, or why
// the picture couldn't play.
func (s *shell) videoTold(st videoStatus) {
	v := &s.video
	v.status = st
	if st.Failed == "" {
		return
	}
	if src := s.st.Source; src != nil {
		if res := s.videoSource(src.Track); res != nil && res.done && res.err == nil && res.value.Src == st.Src {
			s.videoFailed(src.Track, st.Failed)
		}
	}
}

// followVideo tells the page what the frame just built wants: to play the
// picture it built, or, when it built none, to stop. Once no picture
// shows, the web view closes after videoKeep.
func (s *shell) followVideo() {
	v := &s.video
	built, want := v.built, v.want
	v.built, v.want = false, videoWant{}
	if v.link == nil {
		return
	}
	if built != v.shown {
		v.shown = built
		if built {
			if v.keep != nil {
				v.keep.Stop()
			}
		} else {
			s.closeVideoLater()
		}
	}
	if !built {
		// The picture of what still plays waits to show again, where it
		// was; another track's goes.
		want = videoWant{}
		if cur := s.st.Source; cur != nil && cur.Track.ID == v.lastTrack {
			want = videoWant{src: v.lastSrc, track: v.lastTrack, filter: v.sent.filter}
		}
	}
	now := time.Now()
	if !videoSend(v.sent, want, now) {
		return
	}
	src := ""
	if want.src != nil {
		src = want.src.Src
	}
	v.sent = videoSent{src: src, active: want.active, playing: want.playing, pos: want.pos, filter: want.filter, at: now}
	v.lastSrc, v.lastTrack = want.src, want.track
	v.link.post(videoCmd{src: want.src, active: want.active, playing: want.playing, filter: want.filter})
}

// idleVideo tells the page to stop, as the window hides or minimizes: no
// frame comes then to tell it.
func (s *shell) idleVideo() {
	v := &s.video
	v.built = false
	if v.shown {
		v.shown = false
		s.closeVideoLater()
	}
	if v.link != nil && v.sent.active {
		v.sent.active, v.sent.at = false, time.Now()
		v.link.post(videoCmd{src: v.lastSrc, active: false, filter: v.sent.filter})
	}
}

// closeVideoLater closes the web view once no picture showed for
// videoKeep.
func (s *shell) closeVideoLater() {
	v := &s.video
	if v.keep != nil {
		v.keep.Stop()
	}
	v.keep = time.AfterFunc(videoKeep, func() {
		s.update(func() {
			if !v.shown {
				s.closeVideo()
			}
		})
	})
}

// closeVideo closes the web view and its page, from a goroutine: closed
// from the view, a web view crashes GTK, as a window does (#37). The next
// picture makes another.
func (s *shell) closeVideo() {
	v := &s.video
	if v.keep != nil {
		v.keep.Stop()
	}
	if v.link != nil {
		v.link.close()
	}
	if web := v.web; web != nil {
		s.a.SetVideoPage(nil)
		go web.Destroy()
	}
	v.web, v.view, v.link, v.families = nil, nil, nil, nil
	v.status, v.sent, v.lastSrc, v.lastTrack, v.shown = videoStatus{}, videoSent{}, nil, "", false
}

// videoToggle shows or hides the picture of music videos, beside the
// transport, while a music video plays where its picture can show.
func (s *shell) videoToggle(c *ui.Context) {
	src := s.st.Source
	if src == nil || src.Mode != player.ModeStream || !src.Track.MusicVideo() || s.videoProblem() != "" {
		return
	}
	label := "Show the video"
	if s.set.MusicVideos {
		label = "Show the cover"
	}
	if s.iconButton(c, "now-video", "video", label, s.set.MusicVideos, 18).Clicked() {
		s.toggleVideos()
	}
}

// toggleVideos turns the picture of music videos on or off.
func (s *shell) toggleVideos() {
	s.set.MusicVideos = !s.set.MusicVideos
	s.save()
}

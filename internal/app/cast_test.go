package app

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/cast"
	"github.com/fzn0x/mixmusic/internal/cast/casttest"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/store"
)

func init() {
	castTick = 20 * time.Millisecond
	castAskEvery = 100 * time.Millisecond
	castEcho = 50 * time.Millisecond
}

// TestCastServer serves the proxy's audio to the cast device alone, under
// its token, and nothing else.
func TestCastServer(t *testing.T) {
	var got []string
	var mu sync.Mutex
	proxy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, r.URL.Path+"?"+r.URL.RawQuery)
		mu.Unlock()
		io.WriteString(w, "audio")
	})
	loop := net.IPv4(127, 0, 0, 1)
	s, err := serveToCast(loop, loop, proxy)
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	u := s.url("/yt/abcdefghijk")
	if !strings.HasPrefix(u, "http://127.0.0.1:") || len(s.token) != 32 {
		t.Fatalf("served at %q", u)
	}
	res, err := http.Get(u + "?retry=1")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || string(body) != "audio" || s.fetched.Load() != 1 {
		t.Errorf("fetched %d %q, counted %d", res.StatusCode, body, s.fetched.Load())
	}
	mu.Lock()
	if !slices.Equal(got, []string{"/yt/abcdefghijk?retry=1"}) {
		t.Errorf("the proxy got %q", got)
	}
	mu.Unlock()

	for _, path := range []string{"/yt/abcdefghijk", "/" + s.token + "/settings.json", "/" + strings.ToUpper(s.token) + "/yt/abcdefghijk"} {
		res, err := http.Get("http://" + s.ln.Addr().String() + path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("%s: %d", path, res.StatusCode)
		}
	}
	// Another computer on the network gets nothing.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, s.url("/yt/abcdefghijk"), nil)
	req.RemoteAddr = "192.168.1.30:50000"
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("another address: %d", rec.Code)
	}
	s.close()
	if _, err := http.Get(u); err == nil {
		t.Error("served after casting stopped")
	}
}

// fakeSession is a cast session that keeps the commands it gets.
type fakeSession struct {
	mu     sync.Mutex
	calls  []string
	loads  []cast.Media
	volume cast.Volume
	// loadErr fails the loads.
	loadErr error
	session int
}

func (f *fakeSession) record(format string, args ...any) {
	f.mu.Lock()
	f.calls = append(f.calls, fmt.Sprintf(format, args...))
	f.mu.Unlock()
}

func (f *fakeSession) Load(_ context.Context, m cast.Media) (cast.MediaStatus, error) {
	f.mu.Lock()
	f.loads = append(f.loads, m)
	err := f.loadErr
	f.session++
	n := f.session
	f.mu.Unlock()
	f.record("load %s %v %v", m.URL, m.Start, m.Autoplay)
	if err != nil {
		return cast.MediaStatus{}, err
	}
	return cast.MediaStatus{Session: n, State: cast.Buffering, Position: m.Start, Rate: 1, At: time.Now()}, nil
}

func (f *fakeSession) Play(context.Context) error  { f.record("play"); return nil }
func (f *fakeSession) Pause(context.Context) error { f.record("pause"); return nil }
func (f *fakeSession) Stop(context.Context) error  { f.record("stop"); return nil }
func (f *fakeSession) Seek(_ context.Context, t time.Duration) error {
	f.record("seek %v", t)
	return nil
}
func (f *fakeSession) SetRate(_ context.Context, r float64) error { f.record("rate %v", r); return nil }
func (f *fakeSession) Status(context.Context) error               { return nil }
func (f *fakeSession) SetVolume(_ context.Context, v float64) error {
	f.record("volume %v", v)
	return nil
}
func (f *fakeSession) SetMuted(_ context.Context, m bool) error { f.record("muted %v", m); return nil }
func (f *fakeSession) Volume() cast.Volume                      { return f.volume }

func (f *fakeSession) has(call string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Contains(f.calls, call)
}

func (f *fakeSession) wait(t *testing.T, call string) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !f.has(call); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			f.mu.Lock()
			t.Fatalf("no %q: %q", call, f.calls)
		}
	}
}

// heardEvents are the engine's events the cast output told.
type heardEvents struct {
	mu  sync.Mutex
	got []string
}

func (h *heardEvents) add(format string, args ...any) {
	h.mu.Lock()
	h.got = append(h.got, fmt.Sprintf(format, args...))
	h.mu.Unlock()
}

func (h *heardEvents) Time(p, d time.Duration) { h.add("time") }
func (h *heardEvents) Ended()                  { h.add("ended") }
func (h *heardEvents) Error(m string)          { h.add("error %s", m) }
func (h *heardEvents) Playing(p bool)          { h.add("playing %v", p) }
func (h *heardEvents) Buffering(b bool)        { h.add("buffering %v", b) }

func (h *heardEvents) all() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.got)
}

func (h *heardEvents) wait(t *testing.T, event string) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !slices.Contains(h.all(), event); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("no %q: %q", event, h.all())
		}
	}
}

func newTestOut(t *testing.T) (*castOut, *fakeSession, *heardEvents) {
	ev := &heardEvents{}
	f := &fakeSession{volume: cast.Volume{Level: 0.5}}
	o := newCastOut(ev)
	o.served = func(path string) string { return "http://lan" + path }
	fetched := int64(0)
	o.fetched = func() int64 { return fetched }
	o.contentType = func(string, bool) string { return "audio/webm" }
	o.start(f)
	t.Cleanup(o.close)
	return o, f, ev
}

// TestCastOutLoads loads a stream on the device at the address it fetches
// it at, with what shows on screens, and the speed it plays at; a live
// stream, or an address of no stream, fails.
func TestCastOutLoads(t *testing.T) {
	o, f, ev := newTestOut(t)
	track := media.Track{ID: "yt:abcdefghijk", Title: "Song", Artists: []media.Ref{{Name: "A"}, {Name: "B"}},
		Album: &media.Ref{Name: "Album"}, DurationMs: 200000,
		Artwork: []media.Image{{URL: "https://img/s", Width: 60, Height: 60}, {URL: "https://img/l", Width: 544, Height: 544}}}
	o.Load(streamURL("abcdefghijk")+"?retry=5", audio.Cue{Start: 12 * time.Second, Autoplay: true, Speed: 1.5, Track: &track})
	f.wait(t, "load http://lan/yt/abcdefghijk?retry=5 12s true")
	f.wait(t, "rate 1.5")
	f.mu.Lock()
	m := f.loads[0]
	f.mu.Unlock()
	if m.Title != "Song" || m.Artist != "A, B" || m.Album != "Album" || m.Image != "https://img/l" ||
		m.Duration != 200*time.Second || m.ContentType != "audio/webm" {
		t.Errorf("media %+v", m)
	}
	ev.wait(t, "playing true")
	ev.wait(t, "time")

	o.Load(schemeURL(streamScheme)+"live/abcdefghijk", audio.Cue{Autoplay: true})
	ev.wait(t, "error "+errLive.Error())
	o.Load("url:yt:x", audio.Cue{Autoplay: true})
	ev.wait(t, "error this track's audio can't play on a cast device")
}

// TestCastOutStatuses tells the player what the device does, as the
// engine would: it plays, buffers, pauses by its remote, ends, fails;
// statuses of media it left are stale.
func TestCastOutStatuses(t *testing.T) {
	o, f, ev := newTestOut(t)
	o.Load(streamURL("abcdefghijk"), audio.Cue{Autoplay: true})
	f.wait(t, "load http://lan/yt/abcdefghijk 0s true")
	ev.wait(t, "buffering true")
	o.onMedia(cast.MediaStatus{Session: 1, State: cast.Playing, Position: time.Second, Rate: 1, At: time.Now()})
	ev.wait(t, "buffering false")
	// The position moves on between the device's statuses.
	before := len(ev.all())
	time.Sleep(5 * castTick)
	if n := strings.Count(strings.Join(ev.all()[before:], ","), "time"); n < 2 {
		t.Errorf("%d ticks of the time while playing", n)
	}
	o.onMedia(cast.MediaStatus{Session: 1, State: cast.Paused, Position: 3 * time.Second, Rate: 1, At: time.Now()})
	ev.wait(t, "playing false")
	o.onMedia(cast.MediaStatus{Session: 7, State: cast.Idle, IdleReason: cast.Finished})
	o.onMedia(cast.MediaStatus{Session: 1, State: cast.Idle, IdleReason: cast.Finished})
	ev.wait(t, "ended")
	if n := strings.Count(strings.Join(ev.all(), ","), "ended"); n != 1 {
		t.Errorf("ended %d times; a status of other media came through", n)
	}

	// The media that ended plays again from where it's seeked to.
	o.Seek(5 * time.Second)
	o.Play()
	f.wait(t, "load http://lan/yt/abcdefghijk 5s true")

	o.onMedia(cast.MediaStatus{Session: 2, State: cast.Idle, IdleReason: cast.Failed})
	ev.wait(t, "error The cast device stopped playing this track's audio")
}

// TestCastOutCommands sends the player's commands to the device in their
// order, and the last of the volumes asked at once.
func TestCastOutCommands(t *testing.T) {
	o, f, _ := newTestOut(t)
	o.Load(streamURL("abcdefghijk"), audio.Cue{Autoplay: true})
	o.Pause()
	o.Seek(40 * time.Second)
	o.Play()
	o.SetSpeed(1.25)
	o.Unload()
	f.wait(t, "stop")
	f.mu.Lock()
	want := []string{"load http://lan/yt/abcdefghijk 0s true", "pause", "seek 40s", "play", "rate 1.25", "stop"}
	if !slices.Equal(f.calls, want) {
		t.Errorf("commands %q, want %q", f.calls, want)
	}
	f.calls = nil
	f.mu.Unlock()

	f.volume.Muted = true
	o.onVolume(cast.Volume{Level: 0.4, Muted: true}, func(float64, bool) {})
	for _, v := range []float64{0.1, 0.2, 0.3} {
		o.SetVolume(v)
	}
	f.wait(t, "volume 0.3")
	if f.has("volume 0.2") {
		t.Error("a volume asked in between was sent")
	}
	if !f.has("muted false") {
		t.Error("a volume above 0 left the device muted")
	}
}

// TestCastOutVolumeEcho shows the device's own volume, but not the echo of
// one MixMusic set.
func TestCastOutVolumeEcho(t *testing.T) {
	o, f, _ := newTestOut(t)
	var shown []float64
	show := func(v float64, _ bool) { shown = append(shown, v) }
	o.SetVolume(0.6)
	f.wait(t, "volume 0.6")
	o.onVolume(cast.Volume{Level: 0.6}, show)
	time.Sleep(2 * castEcho)
	o.onVolume(cast.Volume{Level: 0.25}, show)
	if !slices.Equal(shown, []float64{0.25}) {
		t.Errorf("showed %v", shown)
	}
}

// TestCastOutLoadFailure tells why a load failed: a device that never
// fetched the audio can't reach MixMusic.
func TestCastOutLoadFailure(t *testing.T) {
	o, f, ev := newTestOut(t)
	f.loadErr = &cast.RequestError{Type: "LOAD_FAILED"}
	o.Load(streamURL("abcdefghijk"), audio.Cue{Autoplay: true})
	ev.wait(t, "error The cast device couldn't reach MixMusic: let MixMusic take connections from your network in the firewall")
	n := int64(1)
	o.fetched = func() int64 { n++; return n }
	o.Load(streamURL("abcdefghijk"), audio.Cue{Autoplay: true})
	ev.wait(t, "error The cast device couldn't play this track's audio")
}

// castServices are the app's, but their streams are those of the videos
// of YouTube tracks, as PrepareStream gives them, without the network.
type castServices struct{ playerServices }

func (castServices) Prepare(_ context.Context, t media.Track) (player.Source, error) {
	_, id, err := media.ParseID(t.ID)
	if err != nil {
		return player.Source{}, err
	}
	return player.Source{Mode: player.ModeStream, URL: streamURL(id), Track: t}, nil
}

func (s castServices) PrepareStream(ctx context.Context, t media.Track) (player.Source, error) {
	return s.Prepare(ctx, t)
}

// keptAudio serves a few bytes of audio of every video, as downloads.
type keptAudio struct{}

func (keptAudio) Serve(w http.ResponseWriter, r *http.Request, videoID string) bool {
	w.Header().Set("Content-Type", "audio/webm")
	io.WriteString(w, "audio of "+videoID)
	return true
}

// newCastApp is an app whose player plays YouTube tracks silently on the
// computer, and which finds the cast devices r answers for.
func newCastApp(t *testing.T, r *casttest.Responder) (*App, *player.Player, *quietStream) {
	t.Helper()
	dir := t.TempDir()
	opts := Options{DataDir: filepath.Join(dir, "data"), CacheDir: filepath.Join(dir, "cache"), Builtin: builtin.FS, Version: "test"}
	a, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	st := a.settings.Get()
	st.DiscordPresence, st.Notifications = false, false
	if err := a.settings.Set(st); err != nil {
		t.Fatal(err)
	}
	if a.queue, err = store.Open(filepath.Join(opts.DataDir, "queue.json"), player.Saved{}); err != nil {
		t.Fatal(err)
	}
	a.proxy.Kept = keptAudio{}
	a.opts.CastBrowse = func(ctx context.Context) ([]cast.Device, error) {
		return cast.Browse(ctx, cast.BrowseOptions{To: r.Addr(), Wait: 200 * time.Millisecond})
	}
	engine := &quietStream{}
	p := player.New(castServices{playerServices{a}}, engine, connectDevice{a}, player.Saved{})
	engine.p.Store(p)
	a.goPlayer.Store(p)
	t.Cleanup(func() {
		a.stopCasting()
		p.Close()
		p.Flush()
	})
	return a, p, engine
}

func castTrack(id string) media.Track {
	return media.Track{ID: "yt:" + id, Source: media.SourceYouTube, Title: "Song " + id, DurationMs: 200000,
		Artists: []media.Ref{{Name: "Artist"}}}
}

// TestCasting finds a cast device, moves the track that plays there from
// where it was, with the address the device fetches its audio at, plays,
// pauses, seeks and moves on to the next track there, sets the device's
// volume and shows the device's own, and brings the track back.
func TestCasting(t *testing.T) {
	dev := casttest.New(t)
	r := casttest.NewResponder(t, dev.Entry())
	a, p, _ := newCastApp(t, r)
	svc := &Player{a}
	var whys []string
	var mu sync.Mutex
	a.OnCast(func(why string) {
		mu.Lock()
		whys = append(whys, why)
		mu.Unlock()
	})

	a.browseCast()
	devices := svc.CastDevices()
	if len(devices) != 1 || devices[0] != (CastDevice{ID: dev.ID, Name: "Living Room", Model: "Chromecast"}) {
		t.Fatalf("found %+v", devices)
	}
	p.PlayTracks([]media.Track{castTrack("aaaaaaaaaaa"), castTrack("bbbbbbbbbbb"), castTrack("ccccccccccc")}, 0)
	waitPlayer(t, p, "a on the computer", func(s player.State) bool { return s.Playing && !s.Loading })
	p.Time(20*time.Second, 200*time.Second)
	waitPlayer(t, p, "a at 20s", func(s player.State) bool { return s.Position >= 20*time.Second })

	if err := svc.Cast(context.Background(), dev.ID); err != nil {
		t.Fatal(err)
	}
	if d, ok := svc.CastingTo(); !ok || d.ID != dev.ID {
		t.Errorf("casting to %+v %v", d, ok)
	}
	load := dev.Wait(t, "LOAD", 1)
	m := load.Media()
	contentID, _ := m["contentId"].(string)
	meta, _ := m["metadata"].(map[string]any)
	if !strings.HasPrefix(contentID, "http://127.0.0.1:") || !strings.HasSuffix(contentID, "/yt/aaaaaaaaaaa") ||
		load.Float("currentTime") < 20 || load.Payload["autoplay"] != true || meta["title"] != "Song aaaaaaaaaaa" ||
		m["contentType"] != "audio/webm" {
		t.Errorf("loaded %v", load.Payload)
	}
	res, err := http.Get(contentID)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if string(body) != "audio of aaaaaaaaaaa" {
		t.Errorf("the device fetched %d %q", res.StatusCode, body)
	}
	waitPlayer(t, p, "a sounding on the device", func(s player.State) bool {
		return s.Remote && s.Playing && !s.Loading && s.Volume == 0.5
	})

	p.Pause()
	dev.Wait(t, "PAUSE", 1)
	waitPlayer(t, p, "paused", func(s player.State) bool { return !s.Playing })
	p.Play()
	dev.Wait(t, "PLAY", 1)
	p.Seek(60 * time.Second)
	if seek := dev.Wait(t, "SEEK", 1); seek.Float("currentTime") != 60 {
		t.Errorf("seeked %v", seek.Payload)
	}

	p.Next()
	if next := dev.Wait(t, "LOAD", 2); !strings.HasSuffix(next.Media()["contentId"].(string), "/yt/bbbbbbbbbbb") {
		t.Errorf("next loaded %v", next.Payload)
	}
	waitPlayer(t, p, "b on the device", func(s player.State) bool {
		cur, _ := s.Current()
		return cur.Track.ID == "yt:bbbbbbbbbbb" && s.Playing && !s.Loading
	})
	// The device plays the track to its end: the next one loads there.
	dev.Finish()
	if next := dev.Wait(t, "LOAD", 3); !strings.HasSuffix(next.Media()["contentId"].(string), "/yt/ccccccccccc") {
		t.Errorf("after the end loaded %v", next.Payload)
	}

	p.SetVolume(0.7)
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
		if level, _ := dev.Volume(); level == 0.7 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the device's volume didn't change")
		}
	}
	time.Sleep(2 * castEcho)
	dev.SetVolume(0.2, false)
	waitPlayer(t, p, "the device's own volume", func(s player.State) bool { return s.Volume == 0.2 })

	svc.StopCasting()
	// The receiver stops, beside the media that stopped as the tracks
	// changed.
	for deadline := time.Now().Add(5 * time.Second); !slices.ContainsFunc(dev.Messages(), func(m casttest.Message) bool {
		return m.Namespace == casttest.NSReceiver && m.Type == "STOP" && m.Payload["sessionId"] == "session-1"
	}); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the receiver didn't stop")
		}
	}
	waitPlayer(t, p, "back on the computer", func(s player.State) bool { return !s.Remote && s.Playing && s.Volume != 0.2 })
	if _, ok := svc.CastingTo(); ok {
		t.Error("still casting")
	}
	if _, err := http.Get(contentID); err == nil {
		t.Error("the audio is served after casting stopped")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(whys) == 0 || slices.ContainsFunc(whys, func(w string) bool { return w != "" }) {
		t.Errorf("told %q", whys)
	}
}

// TestCastingEnds brings the track back to the computer, paused, when the
// device stops playing from MixMusic, and tells why.
func TestCastingEnds(t *testing.T) {
	dev := casttest.New(t)
	r := casttest.NewResponder(t, dev.Entry())
	a, p, _ := newCastApp(t, r)
	svc := &Player{a}
	whys := make(chan string, 10)
	a.OnCast(func(why string) {
		if why != "" {
			whys <- why
		}
	})
	a.browseCast()
	p.PlayTracks([]media.Track{castTrack("aaaaaaaaaaa")}, 0)
	waitPlayer(t, p, "playing", func(s player.State) bool { return s.Playing && !s.Loading })
	if err := svc.Cast(context.Background(), dev.ID); err != nil {
		t.Fatal(err)
	}
	waitPlayer(t, p, "on the device", func(s player.State) bool { return s.Remote && s.Playing && !s.Loading })
	dev.StopApp("")
	select {
	case why := <-whys:
		if why != "Casting to Living Room stopped: it stopped playing from MixMusic" {
			t.Errorf("told %q", why)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("not told")
	}
	waitPlayer(t, p, "back, paused", func(s player.State) bool { return !s.Remote && !s.Playing })
}

// TestCastToNothing turns down a device no longer found, and one that
// can't be reached.
func TestCastToNothing(t *testing.T) {
	dev := casttest.New(t)
	r := casttest.NewResponder(t, dev.Entry())
	a, _, _ := newCastApp(t, r)
	svc := &Player{a}
	if err := svc.Cast(context.Background(), "nobody"); err == nil {
		t.Error("cast to a device never found")
	}
	a.browseCast()
	dev.Close()
	err := svc.Cast(context.Background(), dev.ID)
	if err == nil || err.Error() != "Couldn't connect to Living Room: check that it's on, on this computer's network" {
		t.Errorf("cast to a device gone: %v", err)
	}
	if _, ok := svc.CastStarting(); ok {
		t.Error("still starting")
	}
}

// TestEngineEventsWhileCasting gives the player none of the engine's
// events while its streams play on a cast device.
func TestEngineEventsWhileCasting(t *testing.T) {
	a := newQuietApp(t, nil)
	events := engineEvents{a: a}
	if _, ok := events.to().(*player.Player); !ok {
		t.Fatal("the engine's events go nowhere")
	}
	a.cast.remote.Store(true)
	if _, ok := events.to().(noEvents); !ok {
		t.Error("the engine's events reach the player while casting")
	}
}

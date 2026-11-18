package app

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/cast"
	"github.com/fzn0x/mixmusic/internal/fcast"
	"github.com/fzn0x/mixmusic/internal/fcast/fcasttest"
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/upnp"
	"github.com/fzn0x/mixmusic/internal/upnp/upnptest"
	"github.com/fzn0x/mixmusic/locales"
)

// farServices are castServices, whose tracks on this computer play from
// their file, as prepareFile gives it.
type farServices struct{ castServices }

func (s farServices) Prepare(ctx context.Context, t media.Track) (player.Source, error) {
	if source, native, err := media.ParseID(t.ID); err == nil && source == media.SourceFile {
		return player.Source{Mode: player.ModeStream, URL: schemeURL(streamScheme) + "file/" + native, Track: t}, nil
	}
	return s.castServices.Prepare(ctx, t)
}

func (s farServices) PrepareStream(ctx context.Context, t media.Track) (player.Source, error) {
	return s.Prepare(ctx, t)
}

// newFarApp is newCastApp of an app that finds no Google Cast device, and
// the renderers and FCast receivers set gives it: none reached on the
// network.
func newFarApp(t *testing.T, set func(o *Options)) (*App, *player.Player) {
	t.Helper()
	dir := t.TempDir()
	opts := Options{DataDir: filepath.Join(dir, "data"), CacheDir: filepath.Join(dir, "cache"), Builtin: builtin.FS, Version: "test",
		Controls: noControls}
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
	a.opts.CastBrowse = func(context.Context) ([]cast.Device, error) { return nil, nil }
	set(&a.opts)
	engine := &quietStream{}
	p := player.New(farServices{castServices{playerServices{a}}}, engine, connectDevice{a}, player.Saved{})
	engine.p.Store(p)
	a.goPlayer.Store(p)
	t.Cleanup(func() {
		a.stopCasting()
		p.Close()
		p.Flush()
	})
	return a, p
}

// youtubeAAC has the app's resolver find, of every video, Opus beside an
// AAC whose audio srv serves, as yt-dlp lists them, without the network.
func youtubeAAC(a *App, srv *httptest.Server) {
	a.resolver.Direct = nil
	a.resolver.Run = func(context.Context, ...string) ([]byte, error) {
		expire := time.Now().Add(6 * time.Hour).Unix()
		return fmt.Appendf(nil, `{"url": "%s/opus?expire=%d", "ext": "webm", "acodec": "opus", "filesize": 5, "formats": [
			{"format_id": "140", "url": "%s/aac?expire=%d", "ext": "m4a", "protocol": "https", "acodec": "mp4a.40.2",
			"vcodec": "none", "audio_channels": 2, "filesize": 9}]}`, srv.URL, expire, srv.URL, expire), nil
	}
}

func audioServer(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/aac":
			w.Header().Set("Content-Type", "audio/mp4")
			io.WriteString(w, "aac audio")
		default:
			w.Header().Set("Content-Type", "audio/webm")
			io.WriteString(w, "opus!")
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestCastingToRenderer finds a DLNA renderer by SSDP beside Google Cast's
// devices, lists it by its name and what it is, moves the track there from
// where it was, in the format it plays, YouTube's AAC as it plays no WebM,
// with DLNA's headers and what shows on its screen, plays, pauses and
// seeks there, sets its volume, plays at its own speed as it turns
// another down, hands it the next track ahead, which it moves on to with
// no gap, and brings the track back.
func TestCastingToRenderer(t *testing.T) {
	fake := upnptest.New(t, upnptest.Options{Name: "Kitchen TV", Sink: "http-get:*:audio/mpeg:*,http-get:*:audio/x-m4a:DLNA.ORG_PN=AAC_ISO"})
	ssdp := upnptest.NewResponder(t, fake.Location())
	a, p := newFarApp(t, func(o *Options) {
		o.RendererSearch = func(ctx context.Context) ([]upnp.Renderer, error) {
			return upnp.Search(ctx, upnp.SearchOptions{To: ssdp.Addr(), From: "127.0.0.1:0", Wait: 200 * time.Millisecond})
		}
	})
	youtubeAAC(a, audioServer(t))
	svc := &Player{a}
	a.browseCast()
	devices := svc.CastDevices()
	want := CastDevice{ID: "upnp:" + fake.UDN(), Name: "Kitchen TV", Kind: CastRenderer}
	if len(devices) != 1 || devices[0] != want {
		t.Fatalf("found %+v", devices)
	}
	if got := ModelOf(devices[0]); got != locales.CastRendererModel() || got != "DLNA renderer" {
		t.Errorf("shown as %q", got)
	}
	p.PlayTracks([]media.Track{castTrack("aaaaaaaaaaa"), castTrack("bbbbbbbbbbb"), castTrack("ccccccccccc")}, 0)
	waitPlayer(t, p, "a on the computer", func(s player.State) bool { return s.Playing && !s.Loading })
	p.Time(20*time.Second, 200*time.Second)
	waitPlayer(t, p, "a at 20s", func(s player.State) bool { return s.Position >= 20*time.Second })

	if err := svc.Cast(context.Background(), want.ID); err != nil {
		t.Fatal(err)
	}
	set := fake.Wait(t, "SetAVTransportURI", 1)
	uri := set.Args["CurrentURI"]
	if !strings.HasPrefix(uri, "http://127.0.0.1:") || !strings.HasSuffix(uri, "/yt/aaaaaaaaaaa/aac.m4a") ||
		!strings.Contains(set.Args["CurrentURIMetaData"], "<dc:title>Song aaaaaaaaaaa</dc:title>") ||
		!strings.Contains(set.Args["CurrentURIMetaData"], `protocolInfo="http-get:*:audio/x-m4a:DLNA.ORG_OP=01;`) {
		t.Errorf("loaded %v", set.Args)
	}
	if seek := fake.Wait(t, "Seek", 1); seek.Args["Target"] != "0:00:20" {
		t.Errorf("started from %v", seek.Args)
	}
	waitPlayer(t, p, "a playing on the renderer", func(s player.State) bool {
		return s.Remote && s.Playing && !s.Loading && s.Volume == 0.4
	})
	fetches := fake.Fetches()
	if len(fetches) != 1 || fetches[0].Status != 200 || fetches[0].Header.Get("Content-Type") != "audio/x-m4a" ||
		fetches[0].Header.Get("transferMode.dlna.org") != "Streaming" ||
		!strings.HasPrefix(fetches[0].Header.Get("contentFeatures.dlna.org"), "DLNA.ORG_OP=01") {
		t.Errorf("fetched %+v", fetches)
	}

	p.Pause()
	fake.Wait(t, "Pause", 1)
	p.Play()
	fake.Wait(t, "Play", 2)
	p.Seek(time.Minute)
	if seek := fake.Wait(t, "Seek", 2); seek.Args["Target"] != "0:01:00" {
		t.Errorf("seeked %v", seek.Args)
	}
	p.SetVolume(0.3)
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
		if v, _ := fake.Volume(); v == 30 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the renderer's volume didn't change")
		}
	}

	a.cast.mu.Lock()
	cur := a.cast.active
	a.cast.mu.Unlock()
	// A renderer of one speed plays at its own, which the speed button
	// tells.
	if _, own := svc.CastOwnSpeed(); own {
		t.Error("its own speed before it was asked another")
	}
	cur.out.SetSpeed(1.5)
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
		if d, own := svc.CastOwnSpeed(); own && d.Name == "Kitchen TV" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the speed it turned down isn't told")
		}
	}

	// The next track goes ahead, as the player preloads it.
	cur.out.Preload(streamURL("bbbbbbbbbbb"))
	next := fake.Wait(t, "SetNextAVTransportURI", 1)
	if !strings.HasSuffix(next.Args["NextURI"], "/yt/bbbbbbbbbbb/aac.m4a") ||
		!strings.Contains(next.Args["NextURIMetaData"], "<dc:title>Song bbbbbbbbbbb</dc:title>") {
		t.Errorf("handed ahead %v", next.Args)
	}
	fake.Finish()
	waitPlayer(t, p, "b on the renderer", func(s player.State) bool {
		c, _ := s.Current()
		return c.Track.ID == "yt:bbbbbbbbbbb" && s.Playing && !s.Loading
	})
	if n := len(slices.DeleteFunc(fake.Actions(), func(a upnptest.Action) bool { return a.Name != "SetAVTransportURI" })); n != 1 {
		t.Errorf("loaded %d times: the next track wasn't taken in", n)
	}

	svc.StopCasting()
	fake.Wait(t, "Stop", 1)
	waitPlayer(t, p, "back on the computer", func(s player.State) bool { return !s.Remote && s.Playing })
	if _, err := http.Get(uri); err == nil {
		t.Error("the audio is served after casting stopped")
	}
}

// TestCastingSongsToRenderer plays a song on this computer on a renderer
// that plays its format, from its file, and tells, naming the format, of
// one that doesn't.
func TestCastingSongsToRenderer(t *testing.T) {
	file := filepath.Join(t.TempDir(), "Song.flac")
	if err := os.WriteFile(file, []byte("fLaC and more"), 0o644); err != nil {
		t.Fatal(err)
	}
	song := media.Track{ID: localfiles.TrackID(file), Source: media.SourceFile, Title: "Song", DurationMs: 100000}
	for _, c := range []struct {
		sink  string
		plays bool
	}{{"http-get:*:audio/x-flac:*,http-get:*:audio/mp4:*", true}, {"http-get:*:audio/mpeg:*,http-get:*:audio/mp4:*", false}} {
		fake := upnptest.New(t, upnptest.Options{Name: "Hall speaker", Sink: c.sink})
		ssdp := upnptest.NewResponder(t, fake.Location())
		a, p := newFarApp(t, func(o *Options) {
			o.RendererSearch = func(ctx context.Context) ([]upnp.Renderer, error) {
				return upnp.Search(ctx, upnp.SearchOptions{To: ssdp.Addr(), From: "127.0.0.1:0", Wait: 200 * time.Millisecond})
			}
		})
		a.browseCast()
		p.PlayTracks([]media.Track{song}, 0)
		waitPlayer(t, p, "the song", func(s player.State) bool { return s.Playing && !s.Loading })
		if err := (&Player{a}).Cast(context.Background(), "upnp:"+fake.UDN()); err != nil {
			t.Fatal(err)
		}
		if !c.plays {
			waitPlayer(t, p, "told the format", func(s player.State) bool { return s.Error == "Hall speaker doesn't play FLAC audio" })
			continue
		}
		set := fake.Wait(t, "SetAVTransportURI", 1)
		if !strings.HasSuffix(set.Args["CurrentURI"], ".flac") || !strings.Contains(set.Args["CurrentURIMetaData"], ":audio/x-flac:") {
			t.Errorf("loaded %v", set.Args)
		}
		waitPlayer(t, p, "the song on the renderer", func(s player.State) bool { return s.Remote && s.Playing && !s.Loading })
		if f := fake.Fetches(); len(f) != 1 || f[0].Status != 200 || f[0].Header.Get("Content-Type") != "audio/x-flac" {
			t.Errorf("fetched %+v", f)
		}
	}
}

// TestCastingToFCast finds FCast receivers by mDNS and plays on them: WebM
// on a v4 receiver that lists it, over TLS, AAC on a v3 one; and brings
// the track back, paused, as another app takes the receiver over.
func TestCastingToFCast(t *testing.T) {
	for _, version := range []int{4, 3} {
		fake := fcasttest.New(t, fcasttest.Options{Name: "Bedroom TV", Version: version})
		mdns := fake.Responder(t)
		a, p := newFarApp(t, func(o *Options) {
			o.FCastBrowse = func(ctx context.Context) ([]fcast.Receiver, error) {
				return fcast.Browse(ctx, fcast.BrowseOptions{To: mdns, From: "127.0.0.1:0", Wait: 200 * time.Millisecond})
			}
		})
		youtubeAAC(a, audioServer(t))
		svc := &Player{a}
		whys := make(chan string, 10)
		a.OnCast(func(why string) {
			if why != "" {
				whys <- why
			}
		})
		a.browseCast()
		devices := svc.CastDevices()
		if len(devices) != 1 || devices[0].Name != "Bedroom TV" || devices[0].Kind != CastFCast || ModelOf(devices[0]) != "FCast receiver" {
			t.Fatalf("v%d: found %+v", version, devices)
		}
		p.PlayTracks([]media.Track{castTrack("aaaaaaaaaaa")}, 0)
		waitPlayer(t, p, "playing", func(s player.State) bool { return s.Playing && !s.Loading })
		if err := svc.Cast(context.Background(), devices[0].ID); err != nil {
			t.Fatalf("v%d: %v", version, err)
		}
		kind, path, container := "Load", "/yt/aaaaaaaaaaa.webm", "audio/webm"
		if version == 3 {
			kind, path, container = "Play", "/yt/aaaaaaaaaaa/aac.m4a", "audio/mp4"
		}
		load := fake.Wait(t, kind, 1)
		if !strings.HasSuffix(load.URL, path) || load.Container != container || load.Title != "Song aaaaaaaaaaa" {
			t.Errorf("v%d: loaded %+v", version, load)
		}
		waitPlayer(t, p, "on the receiver", func(s player.State) bool { return s.Remote && s.Playing && !s.Loading })
		fake.TakeOver("http://192.168.1.77/other.mp4")
		select {
		case why := <-whys:
			if why != "Casting to Bedroom TV stopped: it stopped playing from MixMusic" {
				t.Errorf("v%d: told %q", version, why)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("v%d: not told", version)
		}
		waitPlayer(t, p, "back, paused", func(s player.State) bool { return !s.Remote && !s.Playing })
	}
}

// TestCastToFCastWrongKey doesn't play on a receiver whose key isn't the
// one it announced, and tells why.
func TestCastToFCastWrongKey(t *testing.T) {
	fake := fcasttest.New(t, fcasttest.Options{Name: "Bedroom TV", WrongKey: true})
	mdns := fake.Responder(t)
	a, _ := newFarApp(t, func(o *Options) {
		o.FCastBrowse = func(ctx context.Context) ([]fcast.Receiver, error) {
			return fcast.Browse(ctx, fcast.BrowseOptions{To: mdns, From: "127.0.0.1:0", Wait: 200 * time.Millisecond})
		}
	})
	a.browseCast()
	svc := &Player{a}
	err := svc.Cast(context.Background(), "fcast:"+fake.Fingerprint())
	if err == nil || err.Error() != "Couldn't cast to Bedroom TV: its key isn't the one it announced on the network" {
		t.Errorf("cast: %v", err)
	}
	if _, ok := svc.CastingTo(); ok {
		t.Error("casting")
	}
}

// TestFarFindersInTests finds nothing of the kinds a test leaves unset:
// no test asks the network.
func TestFarFindersInTests(t *testing.T) {
	a := &App{opts: Options{CastBrowse: func(context.Context) ([]cast.Device, error) { return nil, nil }}}
	if far := a.browseFar(context.Background()); len(far) != 0 {
		t.Errorf("found %v", far)
	}
}

// TestCastServerRenderer serves a renderer the addresses handed to it, as
// their extension and the type it was told, with DLNA's headers, the files
// of songs on this computer by ranges, over eight connections at once at
// most, and nothing else.
func TestCastServerRenderer(t *testing.T) {
	var mu sync.Mutex
	var got []string
	proxy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", `audio/mp4; codecs="mp4a.40.2"`)
		w.WriteHeader(200)
		io.WriteString(w, "aac")
	})
	loop := net.IPv4(127, 0, 0, 1)
	s, err := serveCast(loop, loop, proxy, true)
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	file := filepath.Join(t.TempDir(), "a.mp3")
	os.WriteFile(file, []byte("0123456789"), 0o644)
	s.serve("/yt/abcdefghijk/aac.m4a", castServed{path: "/yt/abcdefghijk/aac", contentType: "audio/x-m4a"})
	s.serve("/file/xyz.mp3", castServed{file: file, contentType: "audio/mpeg"})

	res, err := http.Get(s.url("/yt/abcdefghijk/aac.m4a"))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	mu.Lock()
	if string(body) != "aac" || res.Header.Get("Content-Type") != "audio/x-m4a" || !slices.Equal(got, []string{"/yt/abcdefghijk/aac"}) ||
		res.Header.Get("transferMode.dlna.org") != "Streaming" || res.Header.Get("contentFeatures.dlna.org") != upnp.Features {
		t.Errorf("served %q as %q, the proxy got %q, headers %v", body, res.Header.Get("Content-Type"), got, res.Header)
	}
	mu.Unlock()
	if !s.ours(s.url("/yt/x")) || s.ours("http://192.168.1.4:9/yt/x") {
		t.Error("told the addresses apart wrongly")
	}

	req, _ := http.NewRequest(http.MethodGet, s.url("/file/xyz.mp3"), nil)
	req.Header.Set("Range", "bytes=2-5")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusPartialContent || string(body) != "2345" || res.Header.Get("Content-Type") != "audio/mpeg" {
		t.Errorf("the file: %d %q %q", res.StatusCode, body, res.Header.Get("Content-Type"))
	}
	for _, path := range []string{"/file/other.mp3", "/settings.json"} {
		res, err := http.Get(s.url(path))
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("%s: %d", path, res.StatusCode)
		}
	}

	// Eight connections at once: the ninth waits.
	http.DefaultClient.CloseIdleConnections()
	time.Sleep(50 * time.Millisecond)
	var conns []net.Conn
	for range castConns {
		c, err := net.Dial("tcp", s.ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		conns = append(conns, c)
		fmt.Fprintf(c, "GET /%s/yt/abcdefghijk/aac.m4a HTTP/1.1\r\nHost: x\r\n\r\n", s.token)
	}
	time.Sleep(100 * time.Millisecond)
	ninth := make(chan error, 1)
	go func() {
		c := &http.Client{Timeout: 300 * time.Millisecond}
		res, err := c.Get(s.url("/yt/abcdefghijk/aac.m4a"))
		if err == nil {
			res.Body.Close()
		}
		ninth <- err
	}()
	if err := <-ninth; err == nil {
		t.Error("a ninth connection was served beside eight")
	}
	for _, c := range conns {
		c.Close()
	}
}

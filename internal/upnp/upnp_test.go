package upnp

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/cast"
	"github.com/fzn0x/mixmusic/internal/upnp/upnptest"
)

func init() {
	pollPlaying = 30 * time.Millisecond
	pollOther = 20 * time.Millisecond
	volumeEvery = 50 * time.Millisecond
	loadPoll = 10 * time.Millisecond
}

// TestSearchFindsRenderers finds the renderers that answer a search, by
// their description, and leaves out what isn't one, and one whose
// description is another host's.
func TestSearchFindsRenderers(t *testing.T) {
	tv := upnptest.New(t, upnptest.Options{Name: "Living Room TV", Model: "Bravia"})
	speaker := upnptest.New(t, upnptest.Options{Name: "kitchen", Model: ""})
	notRenderer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `<root><device><deviceType>urn:schemas-upnp-org:device:MediaServer:1</deviceType><UDN>uuid:s</UDN></device></root>`)
	}))
	defer notRenderer.Close()
	elsewhere := strings.Replace(tv.Location(), "127.0.0.1", "192.0.2.9", 1)
	r := upnptest.NewResponder(t, tv.Location(), speaker.Location(), tv.Location(), notRenderer.URL+"/d.xml", elsewhere)
	found, err := Search(context.Background(), SearchOptions{To: r.Addr(), From: "127.0.0.1:0", Wait: 300 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 {
		t.Fatalf("found %+v", found)
	}
	k, l := found[0], found[1]
	if k.Name != "kitchen" || l.Name != "Living Room TV" || l.Model != "Bravia" || l.ID != tv.UDN() || l.Host != "127.0.0.1" {
		t.Errorf("found %+v", found)
	}
	if !strings.HasSuffix(l.AVTransport.Control, "/AVTransport/control") || l.AVTransport.Type != avTransport+"2" ||
		!strings.HasSuffix(l.RenderingControl.Control, "/RenderingControl/control") || l.ConnectionManager.Control == "" {
		t.Errorf("services %+v", l)
	}
}

// TestDescriptionAddresses keeps the control addresses relative to
// URLBase, of the renderer inside a root device, and drops those of
// another host.
func TestDescriptionAddresses(t *testing.T) {
	body := `<?xml version="1.0"?><root xmlns="urn:schemas-upnp-org:device-1-0"><URLBase>http://192.168.1.20:52235/dmr/</URLBase>
<device><deviceType>urn:schemas-upnp-org:device:Basic:1</deviceType><UDN>uuid:root</UDN><deviceList><device>
<deviceType>urn:schemas-upnp-org:device:MediaRenderer:3</deviceType><friendlyName>[TV] Samsung</friendlyName><modelName>UE55</modelName><UDN>uuid:tv</UDN>
<serviceList>
<service><serviceType>urn:schemas-upnp-org:service:AVTransport:3</serviceType><controlURL>upnp/control/AVTransport1</controlURL></service>
<service><serviceType>urn:schemas-upnp-org:service:RenderingControl:1</serviceType><controlURL>http://203.0.113.5/rc</controlURL></service>
</serviceList></device></deviceList></device></root>`
	r, err := parseDescription([]byte(body), "http://192.168.1.20:52235/dmr/desc.xml", "192.168.1.20")
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != "uuid:tv" || r.Name != "[TV] Samsung" || r.Model != "UE55" ||
		r.AVTransport.Control != "http://192.168.1.20:52235/dmr/upnp/control/AVTransport1" || r.RenderingControl.Control != "" {
		t.Errorf("read %+v", r)
	}
	if _, err := parseDescription([]byte(strings.ReplaceAll(body, "AVTransport:3", "Other:1")), "http://192.168.1.20/d.xml", "192.168.1.20"); !errors.Is(err, errNotRenderer) {
		t.Errorf("a renderer without AVTransport: %v", err)
	}
}

// TestLocalAddress keeps renderers at home only.
func TestLocalAddress(t *testing.T) {
	for _, c := range []struct {
		ip   string
		want bool
	}{{"192.168.1.2", true}, {"10.0.0.7", true}, {"172.20.1.1", true}, {"169.254.3.4", true},
		{"8.8.8.8", false}, {"127.0.0.1", false}, {"fe80::1", false}} {
		if got := localAddress(parseIP(t, c.ip)); got != c.want {
			t.Errorf("%s: %v", c.ip, got)
		}
	}
}

func parseIP(t *testing.T, s string) net.IP {
	ip := net.ParseIP(s)
	if ip == nil {
		t.Fatalf("not an IP: %q", s)
	}
	return ip
}

// TestSearchMessage asks for media renderers, as SSDP has it.
func TestSearchMessage(t *testing.T) {
	msg := string(searchMessage(2))
	for _, line := range []string{"M-SEARCH * HTTP/1.1\r\n", "HOST: 239.255.255.250:1900\r\n", "MAN: \"ssdp:discover\"\r\n",
		"MX: 2\r\n", "ST: urn:schemas-upnp-org:device:MediaRenderer:1\r\n"} {
		if !strings.Contains(msg, line) {
			t.Errorf("no %q in %q", line, msg)
		}
	}
	if !strings.HasSuffix(msg, "\r\n\r\n") {
		t.Errorf("unended %q", msg)
	}
}

// TestSink tells which of the types MixMusic serves a renderer plays, as
// it names them.
func TestSink(t *testing.T) {
	s := parseSink("http-get:*:audio/mpeg:DLNA.ORG_PN=MP3, http-get:*:audio/x-m4a:*,rtsp-rtp-udp:*:audio/webm:*")
	if got := s.Type("audio/webm", "audio/mp4", "audio/x-m4a"); got != "audio/x-m4a" {
		t.Errorf("chose %q", got)
	}
	if got := s.Type("audio/flac"); got != "" {
		t.Errorf("chose %q of none", got)
	}
	if got := parseSink("http-get:*:*:*").Type("audio/webm"); got != "audio/webm" {
		t.Errorf("of any: %q", got)
	}
}

// TestDIDL tells what shows on the renderer's screen, escaped, and how
// its audio is fetched.
func TestDIDL(t *testing.T) {
	got := didl(cast.Media{URL: "http://192.168.1.2:4000/t/yt/a.m4a?x=1&y=2", ContentType: "audio/mp4", Title: "Rock & Roll",
		Artist: "A <B>", Album: "Album", Image: "https://img/l", Duration: 3*time.Minute + 20*time.Second})
	for _, part := range []string{"<dc:title>Rock &amp; Roll</dc:title>", "<upnp:artist>A &lt;B&gt;</upnp:artist>",
		"<upnp:album>Album</upnp:album>", "<upnp:albumArtURI>https://img/l</upnp:albumArtURI>",
		"<upnp:class>object.item.audioItem.musicTrack</upnp:class>",
		`protocolInfo="http-get:*:audio/mp4:DLNA.ORG_OP=01;`, `duration="0:03:20.000"`, ">http://192.168.1.2:4000/t/yt/a.m4a?x=1&amp;y=2</res>"} {
		if !strings.Contains(got, part) {
			t.Errorf("no %q in %s", part, got)
		}
	}
}

// TestClock writes and reads UPnP's times.
func TestClock(t *testing.T) {
	if got := clock(time.Hour + 2*time.Minute + 3*time.Second + 400*time.Millisecond); got != "1:02:03" {
		t.Errorf("wrote %q", got)
	}
	for in, want := range map[string]time.Duration{"0:03:20": 200 * time.Second, "00:00:01.500": 1500 * time.Millisecond,
		"10:00:00": 10 * time.Hour} {
		if got, ok := parseClock(in); !ok || got != want {
			t.Errorf("%q read %v %v", in, got, ok)
		}
	}
	for _, in := range []string{"NOT_IMPLEMENTED", "", "1:2", "-0:00:01"} {
		if _, ok := parseClock(in); ok {
			t.Errorf("%q read", in)
		}
	}
}

// heard keeps what a session told.
type heard struct {
	mu      sync.Mutex
	media   []cast.MediaStatus
	volumes []cast.Volume
	ended   chan error
}

func newHeard() *heard { return &heard{ended: make(chan error, 1)} }

func (h *heard) options() SessionOptions {
	return SessionOptions{
		OnMedia: func(st cast.MediaStatus) {
			h.mu.Lock()
			h.media = append(h.media, st)
			h.mu.Unlock()
		},
		OnVolume: func(v cast.Volume) {
			h.mu.Lock()
			h.volumes = append(h.volumes, v)
			h.mu.Unlock()
		},
		OnEnd: func(err error) { h.ended <- err },
	}
}

// wait waits for a status that ok takes.
func (h *heard) wait(t *testing.T, what string, ok func(cast.MediaStatus) bool) cast.MediaStatus {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
		h.mu.Lock()
		i := slices.IndexFunc(h.media, ok)
		var st cast.MediaStatus
		if i >= 0 {
			st = h.media[i]
		}
		h.mu.Unlock()
		if i >= 0 {
			return st
		}
		if time.Now().After(deadline) {
			h.mu.Lock()
			t.Fatalf("no %s: %+v", what, h.media)
		}
	}
}

// openOn finds the fake renderer and opens a session on it.
func openOn(t *testing.T, fake *upnptest.Renderer, h *heard) *Session {
	t.Helper()
	r, err := describe(context.Background(), http.DefaultClient, fake.Location(), "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	s, err := Open(context.Background(), r, h.options())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close(false) })
	return s
}

// media serves a few bytes of audio, as MixMusic's cast server does.
func media(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mp4")
		io.WriteString(w, "audio")
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestSession loads media on a renderer with its metadata, waits for it to
// play from where it starts, pauses, plays, seeks, sets the volume and
// hears the renderer's own, and hands it the next media, which it moves
// on to with no gap.
func TestSession(t *testing.T) {
	fake := upnptest.New(t, upnptest.Options{})
	srv := media(t)
	h := newHeard()
	s := openOn(t, fake, h)
	s.SetOurs(func(uri string) bool { return strings.HasPrefix(uri, srv.URL) })
	if s.Volume().Level != 0.4 || len(s.Sink()) != 3 || s.RemoteIP().String() != "127.0.0.1" || s.LocalAddr() == nil {
		t.Errorf("volume %+v, sink %q", s.Volume(), s.Sink())
	}
	ctx := context.Background()
	st, err := s.Load(ctx, cast.Media{URL: srv.URL + "/a.m4a", ContentType: "audio/mp4", Title: "Song", Artist: "A",
		Duration: 200 * time.Second, Start: 20 * time.Second, Autoplay: true})
	if err != nil {
		t.Fatal(err)
	}
	if st.Session != 1 || st.State != cast.Playing || st.Position != 20*time.Second {
		t.Errorf("loaded %+v", st)
	}
	set := fake.Wait(t, "SetAVTransportURI", 1)
	if set.Args["CurrentURI"] != srv.URL+"/a.m4a" || !strings.Contains(set.Args["CurrentURIMetaData"], "<dc:title>Song</dc:title>") {
		t.Errorf("set %+v", set.Args)
	}
	if seek := fake.Wait(t, "Seek", 1); seek.Args["Target"] != "0:00:20" || seek.Args["Unit"] != "REL_TIME" {
		t.Errorf("seeked %+v", seek.Args)
	}
	h.wait(t, "playing at 20s", func(st cast.MediaStatus) bool { return st.State == cast.Playing && st.Position >= 20*time.Second })

	if err := s.Pause(ctx); err != nil {
		t.Fatal(err)
	}
	h.wait(t, "paused", func(st cast.MediaStatus) bool { return st.State == cast.Paused })
	if err := s.Play(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.Seek(ctx, time.Minute); err != nil {
		t.Fatal(err)
	}
	h.wait(t, "at 1m", func(st cast.MediaStatus) bool { return st.State == cast.Playing && st.Position >= time.Minute })

	if err := s.SetVolume(ctx, 0.25); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMuted(ctx, true); err != nil {
		t.Fatal(err)
	}
	if v, m := fake.Volume(); v != 25 || !m {
		t.Errorf("volume %d %v", v, m)
	}
	fake.SetVolume(70, false)
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
		h.mu.Lock()
		got := slices.Contains(h.volumes, cast.Volume{Level: 0.7})
		h.mu.Unlock()
		if got {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the renderer's own volume wasn't told")
		}
	}

	if err := s.Next(ctx, cast.Media{URL: srv.URL + "/b.m4a", ContentType: "audio/mp4", Title: "Next"}); err != nil {
		t.Fatal(err)
	}
	if _, _, next := fake.State(); next != srv.URL+"/b.m4a" {
		t.Errorf("queued %q", next)
	}
	fake.Finish()
	h.wait(t, "the first ended", func(st cast.MediaStatus) bool {
		return st.Session == 1 && st.State == cast.Idle && st.IdleReason == cast.Finished
	})
	h.wait(t, "the next playing", func(st cast.MediaStatus) bool { return st.Session == 2 && st.State == cast.Playing })
	// The next track's load takes in what plays.
	st, err = s.Load(ctx, cast.Media{URL: srv.URL + "/b.m4a", ContentType: "audio/mp4", Autoplay: true})
	if err != nil || st.Session != 2 {
		t.Errorf("loaded the next %+v %v", st, err)
	}
	if n := len(slices.DeleteFunc(fake.Actions(), func(a upnptest.Action) bool { return a.Name != "SetAVTransportURI" })); n != 1 {
		t.Errorf("loaded %d times", n)
	}
}

// TestSessionStops tells the media ended when the renderer stops near its
// end, and otherwise that it stopped.
func TestSessionStops(t *testing.T) {
	fake := upnptest.New(t, upnptest.Options{NoNext: true})
	srv := media(t)
	h := newHeard()
	s := openOn(t, fake, h)
	ctx := context.Background()
	if _, err := s.Load(ctx, cast.Media{URL: srv.URL + "/a", ContentType: "audio/mp4", Duration: 3 * time.Second, Autoplay: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.Next(ctx, cast.Media{URL: srv.URL + "/b", ContentType: "audio/mp4"}); err == nil {
		t.Error("a renderer without gapless playback queued")
	}
	h.wait(t, "playing", func(st cast.MediaStatus) bool { return st.State == cast.Playing })
	fake.Finish()
	h.wait(t, "ended", func(st cast.MediaStatus) bool { return st.State == cast.Idle && st.IdleReason == cast.Finished })

	if _, err := s.Load(ctx, cast.Media{URL: srv.URL + "/c", ContentType: "audio/mp4", Duration: time.Hour, Autoplay: true}); err != nil {
		t.Fatal(err)
	}
	h.wait(t, "c playing", func(st cast.MediaStatus) bool { return st.Session == 2 && st.State == cast.Playing })
	fake.Finish()
	st := h.wait(t, "c stopped", func(st cast.MediaStatus) bool { return st.Session == 2 && st.State == cast.Idle })
	if st.IdleReason != cast.Cancelled {
		t.Errorf("stopped early as %q", st.IdleReason)
	}
}

// TestSessionWaitsPaused loads media to wait paused, which plays from
// where it was asked to once played.
func TestSessionWaitsPaused(t *testing.T) {
	fake := upnptest.New(t, upnptest.Options{})
	srv := media(t)
	h := newHeard()
	s := openOn(t, fake, h)
	ctx := context.Background()
	st, err := s.Load(ctx, cast.Media{URL: srv.URL + "/a", ContentType: "audio/mp4", Start: 30 * time.Second})
	if err != nil || st.State != cast.Paused || st.Position != 30*time.Second {
		t.Fatalf("loaded %+v %v", st, err)
	}
	if state, _, _ := fake.State(); state != "STOPPED" || len(fake.Fetches()) != 0 {
		t.Errorf("waiting, the renderer is %s", state)
	}
	s.Seek(ctx, 40*time.Second)
	if err := s.Play(ctx); err != nil {
		t.Fatal(err)
	}
	if seek := fake.Wait(t, "Seek", 1); seek.Args["Target"] != "0:00:40" {
		t.Errorf("seeked %+v", seek.Args)
	}
	h.wait(t, "playing at 40s", func(st cast.MediaStatus) bool { return st.State == cast.Playing && st.Position >= 40*time.Second })
}

// TestSessionLoadFails fails a load the renderer couldn't fetch, as cast
// does, and one it turned down.
func TestSessionLoadFails(t *testing.T) {
	fake := upnptest.New(t, upnptest.Options{})
	h := newHeard()
	s := openOn(t, fake, h)
	gone := httptest.NewServer(http.NotFoundHandler())
	defer gone.Close()
	_, err := s.Load(context.Background(), cast.Media{URL: gone.URL + "/a", ContentType: "audio/mp4", Autoplay: true})
	var re *cast.RequestError
	if !errors.As(err, &re) || re.Type != "LOAD_FAILED" {
		t.Errorf("loaded what couldn't be fetched: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	silent := upnptest.New(t, upnptest.Options{NoFetch: true})
	s2 := openOn(t, silent, newHeard())
	if _, err := s2.Load(ctx, cast.Media{URL: gone.URL + "/b", ContentType: "audio/mp4", Autoplay: true}); err != nil {
		t.Errorf("a renderer that plays at once: %v", err)
	}
}

// TestSessionSpeed plays at another speed where the renderer does, and
// tells where it turned the speed down.
func TestSessionSpeed(t *testing.T) {
	ctx := context.Background()
	srv := media(t)
	for _, speeds := range []bool{true, false} {
		fake := upnptest.New(t, upnptest.Options{Speeds: speeds})
		s := openOn(t, fake, newHeard())
		if _, err := s.Load(ctx, cast.Media{URL: srv.URL + "/a", ContentType: "audio/mp4", Autoplay: true}); err != nil {
			t.Fatal(err)
		}
		err := s.SetRate(ctx, 1.5)
		if speeds && (err != nil || s.SpeedRefused()) {
			t.Errorf("a renderer with speeds: %v", err)
		}
		if !speeds && (!errors.Is(err, ErrNoSpeed) || !s.SpeedRefused()) {
			t.Errorf("a renderer of one speed: %v", err)
		}
		if state, _, _ := fake.State(); state != "PLAYING" {
			t.Errorf("speeds %v: %s", speeds, state)
		}
	}
}

// TestSessionTakenOver ends the session once another app plays on the
// renderer, and once it stopped answering.
func TestSessionTakenOver(t *testing.T) {
	fake := upnptest.New(t, upnptest.Options{})
	srv := media(t)
	h := newHeard()
	s := openOn(t, fake, h)
	s.SetOurs(func(uri string) bool { return strings.HasPrefix(uri, srv.URL) })
	if _, err := s.Load(context.Background(), cast.Media{URL: srv.URL + "/a", ContentType: "audio/mp4", Autoplay: true}); err != nil {
		t.Fatal(err)
	}
	fake.TakeOver("http://192.168.1.50:8000/other.mp3")
	select {
	case err := <-h.ended:
		if !errors.Is(err, ErrTaken) || !errors.Is(err, cast.ErrStopped) {
			t.Errorf("ended for %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("not ended")
	}

	fake2 := upnptest.New(t, upnptest.Options{})
	h2 := newHeard()
	s2 := openOn(t, fake2, h2)
	if _, err := s2.Load(context.Background(), cast.Media{URL: srv.URL + "/a", ContentType: "audio/mp4", Autoplay: true}); err != nil {
		t.Fatal(err)
	}
	fake2.Break(true)
	select {
	case err := <-h2.ended:
		if !errors.Is(err, ErrLost) {
			t.Errorf("ended for %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("not ended")
	}
	if n := len(slices.DeleteFunc(fake2.Actions(), func(a upnptest.Action) bool { return a.Name != "GetTransportInfo" })); n < maxFailedPolls {
		t.Errorf("ended after %d polls", n)
	}
}

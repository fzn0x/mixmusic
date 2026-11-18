package fcast

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/cast"
	"github.com/fzn0x/mixmusic/internal/fcast/fcasttest"
)

func init() {
	pingAfter = 100 * time.Millisecond
	deadAfter = 300 * time.Millisecond
	heartbeat = 20 * time.Millisecond
}

// TestFlat writes tables the reader reads back: scalars, the Time struct,
// strings, vectors of strings, tables and unions, each aligned.
func TestFlat(t *testing.T) {
	inner := (&table{}).str(0, "artist").str(1, "album")
	root := (&table{}).u8(0, 7).time(1, 1500*time.Millisecond).f32(2, 0.25).str(3, "héllo").
		strs(4, []string{"webm", "mp4"}).union(5, 2, inner)
	buf := root.finish()
	r, err := rootOf(buf)
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := r.u8(0); v != 7 {
		t.Errorf("u8 %d", v)
	}
	if p, _ := r.at(1, 8); p%8 != 0 {
		t.Errorf("the Time struct at %d", p)
	}
	if d, _ := r.time(1); d != 1500*time.Millisecond {
		t.Errorf("time %v", d)
	}
	if v, _ := r.f32(2); v != 0.25 {
		t.Errorf("f32 %v", v)
	}
	if s := r.str(3); s != "héllo" {
		t.Errorf("str %q", s)
	}
	if s := r.strs(4); !slices.Equal(s, []string{"webm", "mp4"}) {
		t.Errorf("strs %q", s)
	}
	typ, u := r.union(5)
	if typ != 2 || u.str(0) != "artist" || u.str(1) != "album" {
		t.Errorf("union %d %q %q", typ, u.str(0), u.str(1))
	}
	if _, ok := r.table(9); ok {
		t.Error("a field the table hasn't")
	}
	// Broken buffers read as nothing, without a panic.
	for i := range buf {
		_, _ = readV4(buf[:i])
	}
	if _, err := readV4([]byte{0xff, 0xff, 0xff, 0x7f, 1, 2}); err == nil {
		t.Error("read a broken buffer")
	}
}

// TestPacket frames packets as the protocol does, and turns down one that
// is too large or empty.
func TestPacket(t *testing.T) {
	var b strings.Builder
	writePacket(&b, opVersion, []byte(`{"version":3}`))
	got := []byte(b.String())
	if got[0] != 14 || got[1] != 0 || got[4] != opVersion {
		t.Errorf("framed % x", got[:5])
	}
	p, err := readPacket(strings.NewReader(b.String()), maxPacketV3)
	if err != nil || p.op != opVersion || string(p.body) != `{"version":3}` {
		t.Errorf("read %+v %v", p, err)
	}
	for _, raw := range []string{"\x00\x00\x00\x00", "\xff\xff\x00\x00\x01"} {
		if _, err := readPacket(strings.NewReader(raw), maxPacketV3); !errors.Is(err, errPacket) {
			t.Errorf("%q: %v", raw, err)
		}
	}
}

// TestBrowse finds a receiver by mDNS, with its version and fingerprint.
func TestBrowse(t *testing.T) {
	fake := fcasttest.New(t, fcasttest.Options{Name: "Bedroom TV"})
	found, err := Browse(context.Background(), BrowseOptions{To: fake.Responder(t), From: "127.0.0.1:0", Wait: 200 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 {
		t.Fatalf("found %+v", found)
	}
	r := found[0]
	if r.Name != "Bedroom TV" || r.Version != 4 || r.Fingerprint != fake.Fingerprint() || r.Port != fake.Port() ||
		r.ID != "fcast:"+fake.Fingerprint() {
		t.Errorf("found %+v", r)
	}
}

func receiverOf(fake *fcasttest.Receiver, version int) Receiver {
	return Receiver{ID: "fcast:test", Name: "Bedroom TV", Host: fake.Host(), Port: fake.Port(), Version: version,
		Fingerprint: fake.Fingerprint()}
}

type heard struct {
	mu      sync.Mutex
	media   []cast.MediaStatus
	volumes []cast.Volume
	ended   chan error
}

func newHeard() *heard { return &heard{ended: make(chan error, 1)} }

func (h *heard) options() SessionOptions {
	return SessionOptions{App: "MixMusic", Version: "test",
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

func (h *heard) wait(t *testing.T, what string, ok func(cast.MediaStatus) bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
		h.mu.Lock()
		found := slices.ContainsFunc(h.media, ok)
		h.mu.Unlock()
		if found {
			return
		}
		if time.Now().After(deadline) {
			h.mu.Lock()
			t.Fatalf("no %s: %+v", what, h.media)
		}
	}
}

func (h *heard) waitVolume(t *testing.T, v cast.Volume) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
		h.mu.Lock()
		found := slices.Contains(h.volumes, v)
		h.mu.Unlock()
		if found {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("no volume %+v: %+v", v, h.volumes)
		}
	}
}

func audio(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/gone") {
			http.NotFound(w, r)
			return
		}
		io.WriteString(w, "audio")
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestSessionV4 speaks v4 over TLS to a receiver that announces it: it
// introduces MixMusic, reads what the receiver plays, loads media with its
// metadata, plays, pauses, seeks, sets the speed, the volume and the mute,
// hears the end, and another app taking the receiver over.
func TestSessionV4(t *testing.T) {
	fake := fcasttest.New(t, fcasttest.Options{Audio: []string{"opus", "aac", "mp3"}})
	srv := audio(t)
	h := newHeard()
	s, err := Open(context.Background(), receiverOf(fake, 4), h.options())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(false)
	s.SetOurs(func(u string) bool { return strings.HasPrefix(u, srv.URL) })
	if s.Version() != 4 || !s.Plays("webm", "opus") || s.Plays("ogg", "opus") || !s.PlaysAudio("mp3") || s.PlaysAudio("flac") {
		t.Errorf("version %d, plays %q %q", s.Version(), s.containers, s.audio)
	}
	if v := fake.Wait(t, "Version", 1); v.State != 4 {
		t.Errorf("asked for version %d", v.State)
	}
	if in := fake.Wait(t, "SenderIntroduction", 1); in.App != "MixMusic" {
		t.Errorf("introduced %+v", in)
	}
	ctx := context.Background()
	st, err := s.Load(ctx, cast.Media{URL: srv.URL + "/a.webm", ContentType: "audio/webm", Title: "Song", Artist: "A", Album: "Al",
		Image: "https://img/l", Start: 20 * time.Second, Duration: 200 * time.Second, Autoplay: true})
	if err != nil {
		t.Fatal(err)
	}
	if st.Session != 1 || st.State != cast.Playing {
		t.Errorf("loaded %+v", st)
	}
	load := fake.Wait(t, "Load", 1)
	if load.URL != srv.URL+"/a.webm" || load.Container != "audio/webm" || load.Start != 20*time.Second || load.Title != "Song" ||
		load.Artist != "A" || load.Album != "Al" || load.Image != "https://img/l" || load.Speed != 0 {
		t.Errorf("loaded %+v", load)
	}
	s.Pause(ctx)
	if m := fake.Wait(t, "PlaybackStateChanged", 1); m.State != statePaused {
		t.Errorf("paused as %d", m.State)
	}
	h.wait(t, "paused", func(st cast.MediaStatus) bool { return st.State == cast.Paused })
	s.Play(ctx)
	if m := fake.Wait(t, "PlaybackStateChanged", 2); m.State != statePlaying {
		t.Errorf("played as %d", m.State)
	}
	s.Seek(ctx, time.Minute)
	if m := fake.Wait(t, "ProgressChanged", 1); m.Start != time.Minute {
		t.Errorf("seeked %+v", m)
	}
	h.wait(t, "at 1m", func(st cast.MediaStatus) bool { return st.Position == time.Minute })
	s.SetRate(ctx, 1.5)
	if m := fake.Wait(t, "SpeedChanged", 1); m.Speed != 1.5 {
		t.Errorf("speed %+v", m)
	}
	s.SetVolume(ctx, 0.5)
	s.SetMuted(ctx, true)
	if m := fake.Wait(t, "VolumeChanged", 2); m.Volume != 0 {
		t.Errorf("muted as %+v", m)
	}
	if v := s.Volume(); v.Level != 0.5 || !v.Muted {
		t.Errorf("volume %+v", v)
	}
	s.SetMuted(ctx, false)
	if m := fake.Wait(t, "VolumeChanged", 3); m.Volume != 0.5 {
		t.Errorf("unmuted as %+v", m)
	}
	fake.SetVolume(0.75)
	h.waitVolume(t, cast.Volume{Level: 0.75})
	fake.Finish()
	h.wait(t, "the end", func(st cast.MediaStatus) bool { return st.State == cast.Idle && st.IdleReason == cast.Finished })

	if _, err := s.Load(ctx, cast.Media{URL: srv.URL + "/b", ContentType: "audio/mp4", Autoplay: true}); err != nil {
		t.Fatal(err)
	}
	fake.TakeOver("http://192.168.1.9/other.mp4")
	select {
	case err := <-h.ended:
		if !errors.Is(err, ErrTaken) || !errors.Is(err, cast.ErrStopped) {
			t.Errorf("ended for %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("not ended")
	}
}

// TestSessionV4Key turns down a receiver whose key isn't the one it
// announced, one that announced v4 without its key, and one that answers
// an older version than it announced: never plain v3 then.
func TestSessionV4Key(t *testing.T) {
	wrong := fcasttest.New(t, fcasttest.Options{WrongKey: true})
	if _, err := Open(context.Background(), receiverOf(wrong, 4), newHeard().options()); !errors.Is(err, ErrKey) {
		t.Errorf("a wrong key: %v", err)
	}
	r := receiverOf(wrong, 4)
	r.Fingerprint = ""
	if _, err := Open(context.Background(), r, newHeard().options()); !errors.Is(err, ErrNoKey) {
		t.Errorf("no key: %v", err)
	}
	old := fcasttest.New(t, fcasttest.Options{Version: 3, Announce: 4})
	if _, err := Open(context.Background(), receiverOf(old, 4), newHeard().options()); !errors.Is(err, ErrDowngrade) {
		t.Errorf("a downgrade: %v", err)
	}
	if slices.ContainsFunc(old.Messages(), func(m fcasttest.Message) bool { return m.Kind != "Version" }) {
		t.Errorf("spoke v3 to it: %+v", old.Messages())
	}
}

// TestSessionV3 speaks v3 to an older receiver: Initial, the end's event,
// Play with metadata, Pause, Resume, Seek, SetVolume, the end, a failed
// load, and another app taking over.
func TestSessionV3(t *testing.T) {
	fake := fcasttest.New(t, fcasttest.Options{Version: 3})
	srv := audio(t)
	h := newHeard()
	s, err := Open(context.Background(), receiverOf(fake, 3), h.options())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(false)
	if s.Version() != 3 || s.Plays("webm", "opus") {
		t.Errorf("version %d", s.Version())
	}
	fake.Wait(t, "Initial", 1)
	if ev := fake.Wait(t, "SubscribeEvent", 1); ev.State != 1 {
		t.Errorf("subscribed to %+v", ev)
	}
	ctx := context.Background()
	if _, err := s.Load(ctx, cast.Media{URL: srv.URL + "/a.m4a", ContentType: "audio/mp4", Title: "Song", Start: 5 * time.Second,
		Duration: 200 * time.Second, Autoplay: true}); err != nil {
		t.Fatal(err)
	}
	if p := fake.Wait(t, "Play", 1); p.URL != srv.URL+"/a.m4a" || p.Container != "audio/mp4" || p.Start != 5*time.Second || p.Title != "Song" {
		t.Errorf("played %+v", p)
	}
	s.Pause(ctx)
	fake.Wait(t, "Pause", 1)
	s.Play(ctx)
	fake.Wait(t, "Resume", 1)
	s.Seek(ctx, 90*time.Second)
	if m := fake.Wait(t, "Seek", 1); m.Start != 90*time.Second {
		t.Errorf("seeked %+v", m)
	}
	s.SetVolume(ctx, 0.3)
	if m := fake.Wait(t, "SetVolume", 1); m.Volume != 0.3 {
		t.Errorf("volume %+v", m)
	}
	fake.Finish()
	h.wait(t, "the end", func(st cast.MediaStatus) bool { return st.State == cast.Idle && st.IdleReason == cast.Finished })

	_, err = s.Load(ctx, cast.Media{URL: srv.URL + "/gone", ContentType: "audio/mp4", Autoplay: true})
	var re *cast.RequestError
	if !errors.As(err, &re) || re.Type != "LOAD_FAILED" {
		t.Errorf("loaded what couldn't be fetched: %v", err)
	}
	s.SetOurs(func(u string) bool { return strings.HasPrefix(u, srv.URL) })
	fake.TakeOver("http://192.168.1.9/other.mp4")
	select {
	case err := <-h.ended:
		if !errors.Is(err, ErrTaken) {
			t.Errorf("ended for %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("not ended")
	}
}

// TestSessionWaitsPaused loads nothing until media loaded to wait paused
// is played, from where it was asked to.
func TestSessionWaitsPaused(t *testing.T) {
	fake := fcasttest.New(t, fcasttest.Options{})
	srv := audio(t)
	s, err := Open(context.Background(), receiverOf(fake, 4), newHeard().options())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(false)
	ctx := context.Background()
	st, err := s.Load(ctx, cast.Media{URL: srv.URL + "/a", ContentType: "audio/mp4", Start: 30 * time.Second})
	if err != nil || st.State != cast.Paused || st.Position != 30*time.Second {
		t.Fatalf("loaded %+v %v", st, err)
	}
	s.Seek(ctx, 45*time.Second)
	if slices.ContainsFunc(fake.Messages(), func(m fcasttest.Message) bool { return m.Kind == "Load" }) {
		t.Error("loaded before it played")
	}
	if err := s.Play(ctx); err != nil {
		t.Fatal(err)
	}
	if m := fake.Wait(t, "Load", 1); m.Start != 45*time.Second {
		t.Errorf("loaded from %v", m.Start)
	}
}

// TestSessionLost ends a session whose receiver stopped answering, after
// it was asked whether it's there.
func TestSessionLost(t *testing.T) {
	fake := fcasttest.New(t, fcasttest.Options{Silent: true})
	h := newHeard()
	s, err := Open(context.Background(), receiverOf(fake, 4), h.options())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(false)
	select {
	case err := <-h.ended:
		if !errors.Is(err, ErrLost) {
			t.Errorf("ended for %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("not ended")
	}
}

package cast

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/cast/casttest"
)

func init() {
	pingEvery = 20 * time.Millisecond
	silentFor = time.Second
	launchPoll = 10 * time.Millisecond
}

// heard is what a session told its options.
type heard struct {
	mu      sync.Mutex
	media   []MediaStatus
	volumes []Volume
	ended   chan error
}

func newHeard() *heard { return &heard{ended: make(chan error, 1)} }

func (h *heard) options() SessionOptions {
	return SessionOptions{
		OnMedia: func(st MediaStatus) {
			h.mu.Lock()
			h.media = append(h.media, st)
			h.mu.Unlock()
		},
		OnVolume: func(v Volume) {
			h.mu.Lock()
			h.volumes = append(h.volumes, v)
			h.mu.Unlock()
		},
		OnEnd: func(err error) { h.ended <- err },
	}
}

// waitMedia waits for a status ok takes.
func (h *heard) waitMedia(t *testing.T, what string, ok func(MediaStatus) bool) MediaStatus {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		h.mu.Lock()
		for _, st := range h.media {
			if ok(st) {
				h.mu.Unlock()
				return st
			}
		}
		h.mu.Unlock()
	}
	t.Fatalf("no status of %s: %+v", what, h.media)
	return MediaStatus{}
}

func (h *heard) waitEnd(t *testing.T) error {
	t.Helper()
	select {
	case err := <-h.ended:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("the session didn't end")
		return nil
	}
}

func fakeDevice(d *casttest.Device) Device {
	port, _ := strconv.Atoi(d.Addr()[len("127.0.0.1:"):])
	return Device{ID: d.ID, Name: d.Name, Model: d.Model, Host: "127.0.0.1", Port: port}
}

func open(t *testing.T, d *casttest.Device, h *heard) *Session {
	t.Helper()
	s, err := Open(context.Background(), fakeDevice(d), h.options())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close(false) })
	return s
}

// TestOpenLaunches connects to the device and launches its default media
// receiver, then connects to the receiver, whose volume it tells.
func TestOpenLaunches(t *testing.T) {
	d := casttest.New(t)
	h := newHeard()
	s := open(t, d, h)
	connect := d.Wait(t, "CONNECT", 1)
	if connect.Destination != "receiver-0" || connect.Namespace != casttest.NSConnection {
		t.Errorf("first connected %+v", connect)
	}
	if launch := d.Wait(t, "LAUNCH", 1); launch.Payload["appId"] != DefaultMediaReceiver {
		t.Errorf("launched %v", launch.Payload)
	}
	if c := d.Wait(t, "CONNECT", 2); c.Destination != "web-1" {
		t.Errorf("connected to %q, not the receiver's transport", c.Destination)
	}
	if v := s.Volume(); v.Level != 0.5 || v.Muted {
		t.Errorf("volume %+v", v)
	}
	if s.RemoteIP().String() != "127.0.0.1" || s.LocalAddr() == nil {
		t.Errorf("addresses %v %v", s.RemoteIP(), s.LocalAddr())
	}
}

// TestOpenJoins joins the receiver that runs, and waits for one that
// takes a moment to start.
func TestOpenJoins(t *testing.T) {
	d := casttest.New(t)
	open(t, d, newHeard()).Close(false)
	open(t, d, newHeard())
	if n := d.Count("LAUNCH"); n != 1 {
		t.Errorf("launched %d times; the receiver ran", n)
	}

	late := casttest.New(t)
	late.LaunchLate()
	open(t, late, newHeard())
	if n := late.Count("GET_STATUS"); n < 2 {
		t.Errorf("asked the status %d times; the receiver ran after its launch", n)
	}
}

// TestLoadAndControl loads a track, with what shows on screens, and plays,
// pauses and seeks it, hearing each status.
func TestLoadAndControl(t *testing.T) {
	d := casttest.New(t)
	h := newHeard()
	s := open(t, d, h)
	st, err := s.Load(context.Background(), Media{URL: "http://192.168.1.2:4000/t/yt/abc", ContentType: "audio/webm",
		Title: "Song", Artist: "Artist", Album: "Album", Image: "https://example.com/cover.jpg",
		Duration: 200 * time.Second, Start: 12 * time.Second, Autoplay: true})
	if err != nil {
		t.Fatal(err)
	}
	if st.Session != 1 || st.State != Playing || st.Position != 12*time.Second {
		t.Errorf("load's status %+v", st)
	}
	load := d.Wait(t, "LOAD", 1)
	media := load.Media()
	meta, _ := media["metadata"].(map[string]any)
	images, _ := meta["images"].([]any)
	if media["contentId"] != "http://192.168.1.2:4000/t/yt/abc" || media["contentType"] != "audio/webm" ||
		media["streamType"] != "BUFFERED" || media["duration"] != 200.0 || load.Float("currentTime") != 12 ||
		load.Payload["autoplay"] != true || load.Payload["sessionId"] != "session-1" || load.Destination != "web-1" {
		t.Errorf("loaded %v", load.Payload)
	}
	if meta["metadataType"] != 3.0 || meta["title"] != "Song" || meta["artist"] != "Artist" || meta["albumName"] != "Album" ||
		len(images) != 1 || images[0].(map[string]any)["url"] != "https://example.com/cover.jpg" {
		t.Errorf("metadata %v", meta)
	}
	h.waitMedia(t, "the load", func(st MediaStatus) bool { return st.Session == 1 && st.State == Playing })

	ctx := context.Background()
	if err := s.Pause(ctx); err != nil {
		t.Fatal(err)
	}
	h.waitMedia(t, "the pause", func(st MediaStatus) bool { return st.State == Paused })
	if err := s.Seek(ctx, 90*time.Second); err != nil {
		t.Fatal(err)
	}
	if seek := d.Wait(t, "SEEK", 1); seek.Float("currentTime") != 90 || seek.Float("mediaSessionId") != 1 {
		t.Errorf("seeked %v", seek.Payload)
	}
	h.waitMedia(t, "the seek", func(st MediaStatus) bool { return st.Position == 90*time.Second })
	if err := s.Play(ctx); err != nil {
		t.Fatal(err)
	}
	if _, state, pos, _ := d.Media(); state != "PLAYING" || pos != 90 {
		t.Errorf("the device %s at %v", state, pos)
	}
	if err := s.SetRate(ctx, 1.25); err != nil {
		t.Fatal(err)
	}
	if err := s.Status(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	h.waitMedia(t, "the stop", func(st MediaStatus) bool { return st.State == Idle && st.IdleReason == Cancelled })
}

// TestDeviceControls hears what the device does by itself: its remote
// pausing, the media ending, its volume changing.
func TestDeviceControls(t *testing.T) {
	d := casttest.New(t)
	h := newHeard()
	s := open(t, d, h)
	if _, err := s.Load(context.Background(), Media{URL: "http://x/1", Autoplay: true}); err != nil {
		t.Fatal(err)
	}
	d.SetState("PAUSED", 30)
	h.waitMedia(t, "the remote's pause", func(st MediaStatus) bool { return st.State == Paused && st.Position == 30*time.Second })
	d.SetDuration(180)
	h.waitMedia(t, "the length", func(st MediaStatus) bool { return st.Duration == 180*time.Second })
	d.Finish()
	h.waitMedia(t, "the end", func(st MediaStatus) bool { return st.State == Idle && st.IdleReason == Finished })

	d.SetVolume(0.3, false)
	for deadline := time.Now().Add(5 * time.Second); s.Volume().Level != 0.3; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("volume %+v", s.Volume())
		}
	}
	if err := s.SetVolume(context.Background(), 0.8); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMuted(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if level, muted := d.Volume(); level != 0.8 || !muted {
		t.Errorf("the device's volume %v, muted %v", level, muted)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.volumes) == 0 || h.volumes[0].Level != 0.3 {
		t.Errorf("heard volumes %+v", h.volumes)
	}
}

// TestStaleStatuses drops the statuses of media a later load took over
// from, and tells of another sender's media as the session's interrupted.
func TestStaleStatuses(t *testing.T) {
	d := casttest.New(t)
	h := newHeard()
	s := open(t, d, h)
	ctx := context.Background()
	if _, err := s.Load(ctx, Media{URL: "http://x/1", Autoplay: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(ctx, Media{URL: "http://x/2", Autoplay: true}); err != nil {
		t.Fatal(err)
	}
	h.waitMedia(t, "the second load", func(st MediaStatus) bool { return st.Session == 2 })

	other := newHeard()
	o := open(t, d, other)
	if _, err := o.Load(ctx, Media{URL: "http://elsewhere/3", Autoplay: true}); err != nil {
		t.Fatal(err)
	}
	h.waitMedia(t, "the other load", func(st MediaStatus) bool { return st.Session == 2 && st.IdleReason == Interrupted })
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, st := range h.media {
		if st.Session != 2 && st.Session != 1 {
			t.Errorf("heard the status of another sender's media %+v", st)
		}
	}
}

// TestLoadFails tells a load the device turned down.
func TestLoadFails(t *testing.T) {
	d := casttest.New(t)
	s := open(t, d, newHeard())
	d.FailLoads(1)
	_, err := s.Load(context.Background(), Media{URL: "http://x/1"})
	var re *RequestError
	if !errors.As(err, &re) || re.Type != "LOAD_FAILED" {
		t.Fatalf("load: %v", err)
	}
	if err := s.Play(context.Background()); !errors.Is(err, errNoMedia) {
		t.Errorf("play with nothing loaded: %v", err)
	}
}

// TestSessionEnds ends the session as the receiver stops, another app
// takes the device over, or the device stops answering; Close stops the
// receiver, and tells nothing.
func TestSessionEnds(t *testing.T) {
	t.Run("stopped", func(t *testing.T) {
		d := casttest.New(t)
		h := newHeard()
		open(t, d, h)
		d.StopApp("")
		if err := h.waitEnd(t); !errors.Is(err, ErrStopped) {
			t.Errorf("ended for %v", err)
		}
	})
	t.Run("taken over", func(t *testing.T) {
		d := casttest.New(t)
		h := newHeard()
		open(t, d, h)
		d.StopApp("YouTube")
		if err := h.waitEnd(t); err == nil || (err.Error() != "YouTube plays on the cast device now" && !errors.Is(err, ErrStopped)) {
			t.Errorf("ended for %v", err)
		}
	})
	t.Run("silent", func(t *testing.T) {
		d := casttest.New(t)
		h := newHeard()
		open(t, d, h)
		d.Silence()
		if err := h.waitEnd(t); !errors.Is(err, errSilent) {
			t.Errorf("ended for %v", err)
		}
	})
	t.Run("dropped", func(t *testing.T) {
		d := casttest.New(t)
		h := newHeard()
		open(t, d, h)
		d.Drop()
		if err := h.waitEnd(t); err == nil {
			t.Error("ended for no reason")
		}
	})
	t.Run("closed", func(t *testing.T) {
		d := casttest.New(t)
		h := newHeard()
		s := open(t, d, h)
		s.Close(true)
		if stop := d.Wait(t, "STOP", 1); stop.Payload["sessionId"] != "session-1" {
			t.Errorf("stopped %v", stop.Payload)
		}
		d.Wait(t, "CLOSE", 1)
		select {
		case err := <-h.ended:
			t.Errorf("Close told the end: %v", err)
		case <-time.After(50 * time.Millisecond):
		}
	})
}

// TestHeartbeat pings the device while the session lasts: the device
// answers each ping, so the session outlasts silentFor. It waits for the
// pings, not for some pings' time, as CI's runners can be slow.
func TestHeartbeat(t *testing.T) {
	d := casttest.New(t)
	h := newHeard()
	open(t, d, h)
	start := time.Now()
	deadline := time.After(10 * time.Second)
	for d.Pings() < 3 || time.Since(start) < silentFor+2*pingEvery {
		select {
		case err := <-h.ended:
			t.Fatalf("ended after %v and %d pings: %v", time.Since(start), d.Pings(), err)
		case <-deadline:
			t.Fatalf("%d pings in %v", d.Pings(), time.Since(start))
		case <-time.After(pingEvery):
		}
	}
}

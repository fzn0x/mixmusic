package app

import (
	"net"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/player/playertest"
	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/remote/remotetest"
	"github.com/fzn0x/mixmusic/internal/room"
	"github.com/fzn0x/mixmusic/internal/room/roomtest"
	"github.com/fzn0x/mixmusic/internal/store"
)

// fastRoom is the rooms' timing in these tests.
var fastRoom = room.Timing{Ping: 100 * time.Millisecond, Silence: 3 * time.Second, Keep: time.Minute, Gate: 2 * time.Second,
	Lead: 100 * time.Millisecond, Wait: time.Second, Changes: 10 * time.Millisecond}

// roomEngine is an engine that plays a track in time, at its speed: where
// it is heard is where it would be, as the engine's Position tells.
type roomEngine struct {
	playertest.Stream
	p atomic.Pointer[player.Player]

	mu      sync.Mutex
	url     string
	playing bool
	pos     time.Duration
	at      time.Time
	speed   float64
	loads   int
}

func (e *roomEngine) now() time.Duration {
	if !e.playing {
		return e.pos
	}
	return e.pos + time.Duration(float64(time.Since(e.at))*e.speed)
}

func (e *roomEngine) Load(url string, cue audio.Cue) error {
	e.mu.Lock()
	e.url, e.pos, e.at, e.playing = url, cue.Start, time.Now(), false
	e.loads++
	e.mu.Unlock()
	if cue.Autoplay {
		return e.Play()
	}
	return nil
}

func (e *roomEngine) Play() error {
	e.mu.Lock()
	e.pos, e.at, e.playing = e.now(), time.Now(), true
	pos := e.pos
	e.mu.Unlock()
	e.p.Load().Playing(true)
	e.p.Load().Time(pos+time.Millisecond, 0)
	return nil
}

func (e *roomEngine) Pause() {
	e.mu.Lock()
	e.pos, e.playing = e.now(), false
	e.mu.Unlock()
	e.p.Load().Playing(false)
}

func (e *roomEngine) Seek(t time.Duration) {
	e.mu.Lock()
	e.pos, e.at = t, time.Now()
	e.mu.Unlock()
	e.p.Load().Time(t, 0)
}

func (e *roomEngine) Unload() {
	e.mu.Lock()
	e.pos, e.playing = e.now(), false
	e.mu.Unlock()
}

func (e *roomEngine) Stop() { e.Unload() }

// heard is where the engine's track is heard now.
func (e *roomEngine) heard(player.State) time.Duration {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.now()
}

func (e *roomEngine) setSpeed(speed float64) {
	e.mu.Lock()
	e.pos, e.at, e.speed = e.now(), time.Now(), speed
	e.mu.Unlock()
}

func (e *roomEngine) state() (url string, playing bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.url, e.playing
}

// killer listens as a room does, and drops every connection it took at
// once, as a network that went.
type killer struct {
	net.Listener
	mu    sync.Mutex
	conns []net.Conn
}

func (k *killer) Accept() (net.Conn, error) {
	c, err := k.Listener.Accept()
	if err == nil {
		k.mu.Lock()
		k.conns = append(k.conns, c)
		k.mu.Unlock()
	}
	return c, err
}

func (k *killer) kill() {
	k.mu.Lock()
	defer k.mu.Unlock()
	for _, c := range k.conns {
		c.Close()
	}
	k.conns = nil
}

// roomApp is an app named name on the network n, which plays through an
// engine of its own, wrapped as the app wraps its own engine.
type roomApp struct {
	*App
	engine *roomEngine
	ln     *killer
}

func newRoomApp(t *testing.T, n *roomtest.Network, name string) *roomApp {
	t.Helper()
	id, err := remote.NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	ra := &roomApp{engine: &roomEngine{speed: 1}}
	dir := t.TempDir()
	opts := Options{DataDir: filepath.Join(dir, "data"), CacheDir: filepath.Join(dir, "cache"), Builtin: builtin.FS, Version: "test",
		Remote: &RemoteOptions{Discovery: remotetest.New(), Listen: remotetest.Listen, Identity: id, Name: name, Timing: fastRemote},
		Room: &RoomOptions{Discovery: n, Timing: fastRoom, Heard: ra.engine.heard, SetSpeed: ra.engine.setSpeed,
			Listen: func() (net.Listener, error) {
				ln, err := roomtest.Listen()
				if err != nil {
					return nil, err
				}
				ra.ln = &killer{Listener: ln}
				return ra.ln, nil
			}}}
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
	svc := endingServices{offlineServices{playerServices{a}}, &plays{}}
	p := player.New(svc, roomStream{ra.engine, a}, connectDevice{a}, player.Saved{})
	ra.engine.p.Store(p)
	a.goPlayer.Store(p)
	ra.App = a
	t.Cleanup(func() {
		a.closeRooms()
		p.Close()
		p.Flush()
		svc.plays.end()
	})
	return ra
}

// vid is the ID of a YouTube video of the letter id, 11 of it, as a
// video's ID is.
func vid(id string) string { return "yt:" + strings.Repeat(id, 11) }

func tracksOf(ids ...string) []media.Track {
	var out []media.Track
	for _, id := range ids {
		out = append(out, media.Track{ID: vid(id), Source: media.SourceYouTube, Title: "Song " + id, DurationMs: 300000,
			Artists: []media.Ref{{Name: "Band"}}})
	}
	return out
}

// untilGuest waits for the room the app is a guest of to show what ok
// wants.
func untilGuest(t *testing.T, a *App, what string, ok func(GuestView) bool) GuestView {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		v, _ := a.Rooms().Guest()
		if ok(v) {
			return v
		}
		if time.Now().After(deadline) {
			t.Fatalf("waiting for %s; the guest shows %+v", what, v)
		}
	}
}

// joinRoom has guest knock at host's room with its code, and join it.
func joinRoom(t *testing.T, guest, host *roomApp) {
	t.Helper()
	hosted, ok := host.Rooms().Hosted()
	if !ok {
		t.Fatal("no room hosted")
	}
	guest.browseRooms()
	hostID := host.opts.Remote.Identity.Fingerprint().ID()
	if f := guest.Rooms().Found(); len(f) != 1 || f[0].ID != hostID || f[0].Name != "Desk" {
		t.Fatalf("rooms found: %+v", f)
	}
	guest.Rooms().Knock(hostID, strings.ToLower(hosted.Code[:3])+" "+hosted.Code[3:])
	untilGuest(t, guest.App, "the preview", func(v GuestView) bool { return v.Step == RoomPreview })
	guest.Rooms().Join()
	untilGuest(t, guest.App, "the room", func(v GuestView) bool { return v.Step == RoomIn && v.HasState })
}

// off is how far the guest's track is from the host's, now.
func off(host, guest *roomApp) time.Duration {
	d := guest.engine.heard(player.State{}) - host.engine.heard(player.State{})
	return max(d, -d)
}

// TestRoomBetweenApps hosts a room on one app, which another joins by its
// code: the guest plays what the host plays, in step, as it changes track,
// pauses and seeks, and gets its own queue back as it leaves.
func TestRoomBetweenApps(t *testing.T) {
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	lap := newRoomApp(t, n, "Lap")
	// The guest's own queue, which it gets back.
	lp := lap.GoPlayer()
	lp.PlayTracks(tracksOf("x", "y"), 1)
	waitPlayer(t, lp, "y playing", func(s player.State) bool { return s.Playing })
	lp.Pause()
	dp := desk.GoPlayer()
	dp.PlayTracks(tracksOf("a", "b", "c"), 0)
	waitPlayer(t, dp, "a playing", func(s player.State) bool { return s.Playing })
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	if n.Announced() != 1 {
		t.Fatal("the room isn't announced")
	}
	joinRoom(t, lap, desk)
	v, _ := lap.Rooms().Guest()
	if v.Host != "Desk" || v.Preview.Title != "Song a" || v.Preview.Artists != "Band" {
		t.Fatalf("the guest shows %+v", v)
	}
	waitPlayer(t, lp, "a playing on the guest", func(s player.State) bool {
		cur, ok := s.Current()
		return ok && cur.Track.ID == vid("a") && s.Playing && len(s.Queue) == 1
	})
	until(t, "the guest in step", func() bool { return off(desk, lap) < room.InStep })
	hosted, _ := desk.Rooms().Hosted()
	if len(hosted.Guests) != 1 || hosted.Guests[0].Name != "Lap" || !hosted.Guests[0].Here {
		t.Fatalf("the host shows %+v", hosted)
	}
	// The next track waits for the guest, and starts on both at once.
	loads := lap.engine.loads
	dp.Next()
	waitPlayer(t, lp, "b on the guest", func(s player.State) bool {
		cur, ok := s.Current()
		return ok && cur.Track.ID == vid("b") && s.Playing
	})
	waitPlayer(t, dp, "b playing on the host", func(s player.State) bool { return s.Index == 1 && s.Playing && !s.Loading })
	if lap.engine.loads == loads {
		t.Fatal("the guest didn't load b")
	}
	until(t, "b in step", func() bool { return off(desk, lap) < room.InStep })
	// A pause, and a seek, follow.
	dp.Pause()
	waitPlayer(t, lp, "the guest paused", func(s player.State) bool { return !s.Playing })
	dp.Play()
	waitPlayer(t, lp, "the guest playing", func(s player.State) bool { return s.Playing })
	dp.Seek(2 * time.Minute)
	until(t, "the seek followed", func() bool {
		return lap.engine.heard(player.State{}) > 2*time.Minute-time.Second && off(desk, lap) < room.InStep
	})
	// The guest's queue, apart from its own, isn't kept.
	saved := lap.queue.Get()
	if len(saved.Tracks) != 2 || saved.Index != 1 {
		t.Fatalf("queue.json keeps %+v while in the room", saved)
	}
	if s := (playerServices{lap.App}).Settings(); s.Autoplay || s.Crossfade != 0 || s.SongSpeed != 1 {
		t.Fatalf("the guest's settings while it follows: %+v", s.Playback)
	}
	lap.Rooms().Leave()
	waitPlayer(t, lp, "the guest's own queue back", func(s player.State) bool {
		cur, ok := s.Current()
		return ok && cur.Track.ID == vid("y") && len(s.Queue) == 2 && !s.Playing
	})
	if (playerServices{lap.App}).Settings().Autoplay != lap.settings.Get().Autoplay {
		t.Fatal("the guest's settings didn't come back")
	}
	until(t, "the host to see the guest leave", func() bool {
		h, _ := desk.Rooms().Hosted()
		return len(h.Guests) == 0 && len(h.Activity) == 2 && h.Activity[1].What == room.Left
	})
}

// TestRoomWrongCodeThenLink knocks with a wrong code, then joins by the
// invite link.
func TestRoomWrongCodeThenLink(t *testing.T) {
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	lap := newRoomApp(t, n, "Lap")
	desk.GoPlayer().PlayTracks(tracksOf("a"), 0)
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	hosted, _ := desk.Rooms().Hosted()
	lap.browseRooms()
	hostID := desk.opts.Remote.Identity.Fingerprint().ID()
	wrong := "ZZZZZZ"
	if hosted.Code == wrong {
		wrong = "YYYYYY"
	}
	lap.Rooms().Knock(hostID, wrong)
	v := untilGuest(t, lap.App, "the wrong code refused", func(v GuestView) bool { return v.Step == RoomEnded })
	if !strings.Contains(v.Problem, "isn't the code") {
		t.Fatalf("problem %q", v.Problem)
	}
	lap.Rooms().Knock(hostID, "AB")
	v = untilGuest(t, lap.App, "a code too short", func(v GuestView) bool { return v.Step == RoomEnded && strings.Contains(v.Problem, "6 letters") })
	if err := lap.Rooms().OpenLink(hosted.Link); err != nil {
		t.Fatal(err)
	}
	untilGuest(t, lap.App, "the preview", func(v GuestView) bool { return v.Step == RoomPreview && v.Host == "Desk" })
	lap.Rooms().Join()
	untilGuest(t, lap.App, "the room", func(v GuestView) bool { return v.Step == RoomIn })
	if err := lap.Rooms().OpenLink("mixmusic://join/ABC123"); err == nil {
		t.Fatal("a link without its key and secret was taken")
	}
}

// TestRoomRemoveGivesTheQueueBack removes a guest: it is told so, and gets
// its own queue back.
func TestRoomRemoveGivesTheQueueBack(t *testing.T) {
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	lap := newRoomApp(t, n, "Lap")
	lp := lap.GoPlayer()
	lp.PlayTracks(tracksOf("x"), 0)
	waitPlayer(t, lp, "x playing", func(s player.State) bool { return s.Playing })
	desk.GoPlayer().PlayTracks(tracksOf("a"), 0)
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	joinRoom(t, lap, desk)
	hosted, _ := desk.Rooms().Hosted()
	desk.Rooms().Remove(hosted.Guests[0].Key)
	v := untilGuest(t, lap.App, "removed", func(v GuestView) bool { return v.Step == RoomEnded })
	if !strings.Contains(v.Problem, "removed you") {
		t.Fatalf("problem %q", v.Problem)
	}
	waitPlayer(t, lp, "x back", func(s player.State) bool {
		cur, ok := s.Current()
		return ok && cur.Track.ID == vid("x") && !s.Playing
	})
	lap.Rooms().Dismiss()
	if _, ok := lap.Rooms().Guest(); ok {
		t.Fatal("the room that ended stays once dismissed")
	}
}

// TestRoomGuestComesBack drops a guest's connection: it comes back by its
// key, and plays on in step.
func TestRoomGuestComesBack(t *testing.T) {
	was := roomRetry
	roomRetry = 20 * time.Millisecond
	t.Cleanup(func() { roomRetry = was })
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	lap := newRoomApp(t, n, "Lap")
	desk.GoPlayer().PlayTracks(tracksOf("a"), 0)
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	joinRoom(t, lap, desk)
	desk.ln.kill()
	untilGuest(t, lap.App, "back in the room", func(v GuestView) bool { return v.Step == RoomIn && v.HasState })
	hosted, _ := desk.Rooms().Hosted()
	if len(hosted.Guests) != 1 || len(hosted.Activity) != 1 {
		t.Fatalf("the host shows %+v", hosted)
	}
	until(t, "in step again", func() bool { return off(desk, lap) < room.InStep })
}

// TestRoomEndReleasesTheStart ends a room while the host's next track
// waits for a guest that never has it ready: it plays at once, and the
// guest is told the room ended.
func TestRoomEndReleasesTheStart(t *testing.T) {
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	lap := newRoomApp(t, n, "Lap")
	dp := desk.GoPlayer()
	dp.PlayTracks(tracksOf("a", "b"), 0)
	waitPlayer(t, dp, "a playing", func(s player.State) bool { return s.Playing })
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	joinRoom(t, lap, desk)
	// The guest stops following, as a computer too busy to: it never tells
	// it has b ready.
	g := lap.guestRoom()
	lap.room.mu.Lock()
	f := g.follow
	lap.room.mu.Unlock()
	f.stop()
	dp.Next()
	until(t, "b held", func() bool { h, _ := desk.Rooms().Hosted(); return h.Waiting })
	if _, playing := desk.engine.state(); playing {
		t.Fatal("b plays before the guest has it")
	}
	desk.Rooms().End()
	waitPlayer(t, dp, "b playing once the room ended", func(s player.State) bool { return s.Index == 1 && s.Playing && !s.Loading })
	v := untilGuest(t, lap.App, "the room ended", func(v GuestView) bool { return v.Step == RoomEnded })
	if v.Problem != "The room ended." {
		t.Fatalf("problem %q", v.Problem)
	}
	if n.Announced() != 0 {
		t.Fatal("the room is still announced")
	}
}

// TestRoomQuitKeepsTheOwnQueue quits MixMusic while in a room: the host
// hears the guest left, and queue.json keeps the guest's own queue, which
// the next start plays.
func TestRoomQuitKeepsTheOwnQueue(t *testing.T) {
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	lap := newRoomApp(t, n, "Lap")
	lp := lap.GoPlayer()
	lp.PlayTracks(tracksOf("x", "y"), 1)
	waitPlayer(t, lp, "y playing", func(s player.State) bool { return s.Playing })
	desk.GoPlayer().PlayTracks(tracksOf("a"), 0)
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	joinRoom(t, lap, desk)
	waitPlayer(t, lp, "a on the guest", func(s player.State) bool { cur, ok := s.Current(); return ok && cur.Track.ID == vid("a") })
	lap.closeRooms()
	lp.Flush()
	if saved := lap.queue.Get(); len(saved.Tracks) != 2 || saved.Tracks[saved.Index].ID != vid("y") {
		t.Fatalf("queue.json keeps %+v", saved)
	}
	until(t, "the host to see the guest leave", func() bool {
		h, _ := desk.Rooms().Hosted()
		return len(h.Guests) == 0
	})
}

// TestRoomSettingsRestWhileFollowing changes the settings while a guest
// follows its host: the player still hears them as a guest's.
func TestRoomSettingsRestWhileFollowing(t *testing.T) {
	n := roomtest.New()
	desk := newRoomApp(t, n, "Desk")
	lap := newRoomApp(t, n, "Lap")
	desk.GoPlayer().PlayTracks(tracksOf("a"), 0)
	if err := desk.Rooms().Start(); err != nil {
		t.Fatal(err)
	}
	joinRoom(t, lap, desk)
	lp := lap.GoPlayer()
	waitPlayer(t, lp, "a on the guest", func(s player.State) bool { return s.Playing })
	st := lap.settings.Get()
	st.SongSpeed = 1.5
	lap.apply(st)
	// The player takes its tasks in turn: once it took a volume set after
	// the settings, it took them.
	lp.SetVolume(0.42)
	waitPlayer(t, lp, "the volume after the settings", func(s player.State) bool { return s.Volume == 0.42 })
	if sp := lp.State().Speed; sp != 1 {
		t.Fatalf("the guest plays at %v, its own speed, while it follows the host", sp)
	}
}

package app

// Casting: the cast devices on the network, which MixMusic looks for by
// mDNS (internal/cast), and the player's streams played on one of them in
// the engine's place (castOut), which fetches their audio from MixMusic
// (castServer). Spotify tracks that play on a Spotify device stay there.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/cast"
	"github.com/fzn0x/mixmusic/internal/player"
)

// These times are casting's; tests shorten them.
var (
	// castFirstBrowse is how long after start MixMusic first looks for
	// cast devices, and castBrowseEvery how often it looks again.
	castFirstBrowse = 3 * time.Second
	castBrowseEvery = 45 * time.Second
	// castForget is how long a device not seen stays listed.
	castForget = 3 * time.Minute
	// castBrowseAgain is how soon after a look the menu looks again.
	castBrowseAgain = 5 * time.Second
	// castConnectTimeout bounds starting to cast.
	castConnectTimeout = 30 * time.Second
)

// CastDevice is a cast device the player can play on, as the windows show
// it.
type CastDevice struct {
	ID string
	// Name is the name the user gave it, which tells its room, as
	// "Living Room TV"; Model what it is, as "Google Nest Mini".
	Name, Model string
}

// castState is the app's casting: the devices found, and the one the
// player's streams play on.
type castState struct {
	mu    sync.Mutex
	found map[string]castFound
	// active is the cast under way, and starting the device a cast starts
	// on, until it plays there or fails.
	active   *casting
	starting *CastDevice
	watchers []*func(string)
	browsing bool
	browsed  time.Time
	// shown is the devices as the windows list them, sorted by name.
	shown atomic.Pointer[[]CastDevice]
	// remote is set while the player's streams play on a cast device: the
	// engine's events are nobody's then.
	remote atomic.Bool
}

type castFound struct {
	device cast.Device
	seen   time.Time
}

// casting is a cast under way.
type casting struct {
	device  CastDevice
	out     *castOut
	session *cast.Session
	server  *castServer
}

// end ends the cast: stop stops the receiver on the device too.
func (c *casting) end(stop bool) {
	c.out.close()
	c.session.Close(stop)
	c.server.close()
}

func castDeviceOf(d cast.Device) CastDevice {
	return CastDevice{ID: d.ID, Name: d.Name, Model: d.Model}
}

// watchCast looks for cast devices a moment after start, then every
// castBrowseEvery, and stops the cast device's receiver as MixMusic quits.
func (a *App) watchCast() {
	a.OnFlush(a.quitCasting)
	go func() {
		time.Sleep(castFirstBrowse)
		for {
			a.browseCast()
			time.Sleep(castBrowseEvery)
		}
	}()
}

// browseCast looks for cast devices once, unless a look is under way, and
// tells the windows when the list changes.
func (a *App) browseCast() {
	c := &a.cast
	c.mu.Lock()
	if c.browsing {
		c.mu.Unlock()
		return
	}
	c.browsing = true
	browse := a.opts.CastBrowse
	c.mu.Unlock()
	if browse == nil {
		browse = func(ctx context.Context) ([]cast.Device, error) { return cast.Browse(ctx, cast.BrowseOptions{}) }
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	devices, err := browse(ctx)
	cancel()
	if err != nil {
		log.Printf("mixmusic: looking for cast devices: %v", err)
	}
	now := time.Now()
	c.mu.Lock()
	c.browsing, c.browsed = false, now
	if c.found == nil {
		c.found = map[string]castFound{}
	}
	for _, d := range devices {
		c.found[d.ID] = castFound{device: d, seen: now}
	}
	for id, f := range c.found {
		if now.Sub(f.seen) > castForget && (c.active == nil || c.active.device.ID != id) {
			delete(c.found, id)
		}
	}
	changed := c.list()
	c.mu.Unlock()
	if changed {
		a.notifyCast("")
	}
}

// list makes the list the windows show of what was found, and reports
// whether it changed. c.mu is held.
func (c *castState) list() bool {
	shown := make([]CastDevice, 0, len(c.found))
	for _, f := range c.found {
		shown = append(shown, castDeviceOf(f.device))
	}
	slices.SortFunc(shown, func(x, y CastDevice) int {
		if n := strings.Compare(strings.ToLower(x.Name), strings.ToLower(y.Name)); n != 0 {
			return n
		}
		return strings.Compare(x.ID, y.ID)
	})
	if old := c.shown.Load(); old != nil && slices.Equal(*old, shown) {
		return false
	}
	c.shown.Store(&shown)
	return true
}

// OnCast has fn told, from any goroutine, when the cast devices found or
// the cast change, until cancel: why tells, when not "", why a cast
// stopped by itself, in the interface's words.
func (a *App) OnCast(fn func(why string)) (cancel func()) {
	f := &fn
	c := &a.cast
	c.mu.Lock()
	c.watchers = append(slices.Clone(c.watchers), f)
	c.mu.Unlock()
	return func() {
		c.mu.Lock()
		c.watchers = slices.DeleteFunc(slices.Clone(c.watchers), func(g *func(string)) bool { return g == f })
		c.mu.Unlock()
	}
}

func (a *App) notifyCast(why string) {
	a.cast.mu.Lock()
	fns := a.cast.watchers
	a.cast.mu.Unlock()
	for _, fn := range fns {
		(*fn)(why)
	}
}

// CastDevices returns the cast devices on the network, sorted by name, for
// a frame: don't change it.
func (p *Player) CastDevices() []CastDevice {
	if l := p.a.cast.shown.Load(); l != nil {
		return *l
	}
	return nil
}

// FindCastDevices looks for cast devices at once, as the menu that lists
// them opens, unless it looked a moment ago.
func (p *Player) FindCastDevices() {
	c := &p.a.cast
	c.mu.Lock()
	recent := time.Since(c.browsed) < castBrowseAgain
	c.mu.Unlock()
	if !recent {
		go p.a.browseCast()
	}
}

// CastingTo returns the device the player plays on, and whether it plays
// on one; CastStarting the device a cast starts on, until it plays there
// or fails.
func (p *Player) CastingTo() (CastDevice, bool) {
	c := &p.a.cast
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active == nil {
		return CastDevice{}, false
	}
	return c.active.device, true
}

func (p *Player) CastStarting() (CastDevice, bool) {
	c := &p.a.cast
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.starting == nil {
		return CastDevice{}, false
	}
	return *c.starting, true
}

// Cast has the player play on the cast device id, from where it is: it
// connects, starts the device's media player, serves it the audio, and
// moves the track there. A device cast to before lets go.
func (p *Player) Cast(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, castConnectTimeout)
	defer cancel()
	return p.a.castTo(ctx, id)
}

// StopCasting brings the player's streams back to the computer, playing as
// they played, and stops the cast device's receiver.
func (p *Player) StopCasting() { p.a.stopCasting() }

func (a *App) castTo(ctx context.Context, id string) error {
	gp := a.goPlayer.Load()
	if gp == nil {
		return errNoNative
	}
	c := &a.cast
	c.mu.Lock()
	f, ok := c.found[id]
	switch {
	case !ok:
		c.mu.Unlock()
		return errors.New("That cast device is no longer on the network")
	case c.active != nil && c.active.device.ID == id:
		c.mu.Unlock()
		return nil
	case c.starting != nil:
		c.mu.Unlock()
		return errors.New("MixMusic is connecting to a cast device already")
	}
	d := castDeviceOf(f.device)
	c.starting = &d
	c.mu.Unlock()
	a.notifyCast("")
	defer func() {
		c.mu.Lock()
		c.starting = nil
		c.mu.Unlock()
		a.notifyCast("")
	}()

	out := newCastOut(castEvents{a, nil})
	out.events = castEvents{a, out}
	session, err := cast.Open(ctx, f.device, cast.SessionOptions{
		OnMedia: out.onMedia,
		OnVolume: func(v cast.Volume) {
			out.onVolume(v, func(level float64, muted bool) {
				if a.castOn(out) {
					gp.OutputVolume(level, muted)
				}
			})
		},
		OnEnd: func(err error) { a.castEnded(out, err) },
	})
	if err != nil {
		log.Printf("mixmusic: connecting to a cast device: %v", err)
		var ne net.Error
		if errors.As(err, &ne) || errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("Couldn't connect to %s: check that it's on, on this computer's network", d.Name)
		}
		return fmt.Errorf("Couldn't cast to %s: %v", d.Name, err)
	}
	server, err := serveToCast(localIP(session.LocalAddr()), session.RemoteIP(), a.proxy)
	if err != nil {
		session.Close(true)
		return fmt.Errorf("Couldn't serve %s the audio: %v", d.Name, err)
	}
	out.served, out.fetched, out.contentType, out.native = server.url, server.fetched.Load, a.castContentType, a.setNative
	out.start(session)

	cur := &casting{device: d, out: out, session: session, server: server}
	c.mu.Lock()
	prev := c.active
	c.active = cur
	c.remote.Store(true)
	c.mu.Unlock()
	v := session.Volume()
	gp.SetOutput(player.Output{Stream: out, Volume: v.Level, Muted: v.Muted})
	if prev != nil {
		prev.end(true)
	}
	log.Printf("mixmusic: casting to a %s", d.Model)
	return nil
}

// castOn reports whether out is the output of the cast under way.
func (a *App) castOn(out *castOut) bool {
	a.cast.mu.Lock()
	defer a.cast.mu.Unlock()
	return a.cast.active != nil && a.cast.active.out == out
}

func (a *App) stopCasting() {
	c := &a.cast
	c.mu.Lock()
	cur := c.active
	c.active = nil
	c.remote.Store(false)
	c.mu.Unlock()
	if cur == nil {
		return
	}
	if gp := a.goPlayer.Load(); gp != nil {
		gp.SetOutput(player.Output{})
	}
	cur.end(true)
	a.notifyCast("")
}

// castEnded takes in the session of out ending by itself: the track comes
// back to the computer, paused, and the windows tell why.
func (a *App) castEnded(out *castOut, err error) {
	c := &a.cast
	c.mu.Lock()
	cur := c.active
	if cur == nil || cur.out != out {
		c.mu.Unlock()
		return
	}
	c.active = nil
	c.remote.Store(false)
	c.mu.Unlock()
	if gp := a.goPlayer.Load(); gp != nil {
		gp.SetOutput(player.Output{Paused: true})
	}
	cur.end(false)
	why := "the connection was lost"
	switch {
	case errors.Is(err, cast.ErrStopped):
		why = "it stopped playing from MixMusic"
	case err != nil && strings.HasSuffix(err.Error(), "plays on the cast device now"):
		why = err.Error()
	}
	log.Printf("mixmusic: casting stopped: %v", err)
	a.notifyCast(fmt.Sprintf("Casting to %s stopped: %s", cur.device.Name, why))
}

// quitCasting stops the cast device's receiver as MixMusic quits: it could
// fetch no more audio.
func (a *App) quitCasting() {
	c := &a.cast
	c.mu.Lock()
	cur := c.active
	c.active = nil
	c.mu.Unlock()
	if cur != nil {
		cur.end(true)
	}
}

// castEvents are the cast output's events, which the player hears while
// out is the cast under way's.
type castEvents struct {
	a   *App
	out *castOut
}

func (e castEvents) to() audio.Events {
	if p := e.a.goPlayer.Load(); p != nil && e.out != nil && e.a.castOn(e.out) {
		return p
	}
	return noEvents{}
}

func (e castEvents) Time(position, duration time.Duration) { e.to().Time(position, duration) }
func (e castEvents) Ended()                                { e.to().Ended() }
func (e castEvents) Error(message string)                  { e.to().Error(message) }
func (e castEvents) Playing(playing bool)                  { e.to().Playing(playing) }
func (e castEvents) Buffering(buffering bool)              { e.to().Buffering(buffering) }

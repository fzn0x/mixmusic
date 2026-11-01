// Package nowplaying shows what MixMusic's own audio engine plays in the
// system's media controls, and passes on what they ask: macOS's Now
// Playing, which Control Center, the media keys and headphones reach, an
// MPRIS player on Linux, which desktops show and pass their media keys to,
// and Windows' media flyout, which the volume keys and Quick Settings
// show. The webview does this for the audio it plays itself; the engine
// plays outside it.
package nowplaying

import (
	"errors"
	"strings"
	"sync"
	"time"
)

// ErrUnsupported is returned where MixMusic can't reach the system's media
// controls.
var ErrUnsupported = errors.New("nowplaying: the system's media controls are not supported here")

// Info is what plays.
type Info struct {
	// ID tells tracks apart.
	ID      string
	Title   string
	Artists []string
	Album   string
	// Artwork is the address of an https image of the track, or empty.
	Artwork  string
	Duration time.Duration
	// Position is where the track was at At.
	Position time.Duration
	At       time.Time
	Playing  bool
	// Volume is from 0 to 1.
	Volume float64
	// Rate is the speed the track plays at, from MinRate to MaxRate, at
	// which the position moves on: 1 is its own, as is 0.
	Rate float64
}

// The speeds MixMusic plays at, which the system's controls may ask.
const (
	MinRate = 0.5
	MaxRate = 3
)

// rate returns the speed the track plays at.
func (i *Info) rate() float64 {
	if i.Rate > 0 {
		return i.Rate
	}
	return 1
}

// position returns where the track is at now.
func (i *Info) position(now time.Time) time.Duration {
	p := i.Position
	if i.Playing && !i.At.IsZero() {
		p += time.Duration(float64(now.Sub(i.At)) * i.rate())
	}
	if i.Duration > 0 {
		p = min(p, i.Duration)
	}
	return max(p, 0)
}

// seekSkew is how far a position may be from where the one before went on
// to before it counts as a seek: positions come from the page about every
// second.
const seekSkew = 1500 * time.Millisecond

// change tells what differs from one Info to the next.
type change struct {
	// track is set for another track, or other details of it.
	track   bool
	playing bool
	// seeked is set when the position jumped in the same track.
	seeked bool
	volume bool
	// rate is set when the track plays at another speed.
	rate bool
}

func (c change) any() bool { return c.track || c.playing || c.seeked || c.volume || c.rate }

// diff returns what changed from prev, what the system's controls go on
// from, to next.
func diff(prev, next *Info) change {
	if prev == nil || next == nil {
		return change{track: true, playing: true}
	}
	var c change
	c.track = prev.ID != next.ID || prev.Title != next.Title || prev.Album != next.Album ||
		prev.Artwork != next.Artwork || prev.Duration != next.Duration || !equal(prev.Artists, next.Artists)
	c.playing = prev.Playing != next.Playing
	if !c.track {
		d := next.Position - prev.position(next.At)
		c.seeked = d > seekSkew || d < -seekSkew
	}
	c.volume = prev.Volume != next.Volume
	c.rate = prev.rate() != next.rate()
	return c
}

func valid(s string) string { return strings.ToValidUTF8(s, "�") }

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Command is what the system's media controls ask of the player.
type Command int

const (
	Play Command = iota
	Pause
	Toggle
	Next
	Previous
	// Seek moves to the Request's Position.
	Seek
	// SetVolume sets the Request's Volume. MPRIS asks it.
	SetVolume
	// Raise shows the player's window. MPRIS asks it.
	Raise
	// SetRate plays at the Request's Rate, from MinRate to MaxRate. MPRIS
	// asks it.
	SetRate
)

// Request is a Command with what it needs.
type Request struct {
	Command  Command
	Position time.Duration
	Volume   float64
	Rate     float64
}

// requests is how many requests wait for the handler before more are
// dropped: the system's controls never wait for it.
const requests = 16

// Controls shows what plays in the system's media controls.
type Controls struct {
	sys      platform
	mu       sync.Mutex
	want     *Info
	kick     chan struct{}
	requests chan Request
	stop     chan struct{}
	done     chan struct{}
}

// platform is a system's media controls.
type platform interface {
	// show shows info, or nothing for nil.
	show(info *Info)
	close()
}

// Open reaches the system's media controls, which show nothing until Show.
// handle gets what they ask, one request at a time, on a goroutine of its
// own. run runs a function on the main thread and waits for it, which
// macOS's controls need; nil runs it in place.
func Open(handle func(Request), run func(func())) (*Controls, error) {
	if run == nil {
		run = func(fn func()) { fn() }
	}
	c := newControls()
	sys, err := open(c.ask, run)
	if err != nil {
		return nil, err
	}
	c.start(sys, handle)
	return c, nil
}

func newControls() *Controls {
	return &Controls{
		kick:     make(chan struct{}, 1),
		requests: make(chan Request, requests),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// start shows what Show asks on sys, and passes on what it asks to handle.
func (c *Controls) start(sys platform, handle func(Request)) {
	c.sys = sys
	go c.loop()
	go func() {
		for {
			select {
			case r := <-c.requests:
				handle(r)
			case <-c.stop:
				return
			}
		}
	}()
}

// ask passes on what the system's controls ask.
func (c *Controls) ask(r Request) {
	select {
	case c.requests <- r:
	default:
	}
}

// Show shows info in the system's media controls, or nothing for nil. It
// returns at once: the controls follow in the background.
func (c *Controls) Show(info *Info) {
	if info != nil {
		// The systems take only UTF-8: NSString makes nil of the rest, and
		// GVariant refuses it.
		cp := *info
		cp.Title, cp.Album, cp.Artwork = valid(info.Title), valid(info.Album), valid(info.Artwork)
		cp.Artists = make([]string, len(info.Artists))
		for i, a := range info.Artists {
			cp.Artists[i] = valid(a)
		}
		info = &cp
	}
	c.mu.Lock()
	c.want = info
	c.mu.Unlock()
	select {
	case c.kick <- struct{}{}:
	default:
	}
}

// Close takes MixMusic out of the system's media controls.
func (c *Controls) Close() {
	close(c.stop)
	<-c.done
}

func (c *Controls) loop() {
	defer close(c.done)
	shown := false
	for {
		select {
		case <-c.stop:
			if shown {
				c.sys.show(nil)
			}
			c.sys.close()
			return
		case <-c.kick:
		}
		c.mu.Lock()
		want := c.want
		c.mu.Unlock()
		if want == nil && !shown {
			continue
		}
		c.sys.show(want)
		shown = want != nil
	}
}

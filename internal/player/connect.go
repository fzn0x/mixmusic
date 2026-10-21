package player

// Playback on a Spotify Connect device: the player sends commands and asks
// what the device plays. Spotify's reports lag its commands by a few
// seconds, and may name a track by another ID, that of the copy Spotify
// relinked for the user's country, so the engine waits for the device to
// settle and knows its track by more than its ID.

import (
	"context"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// Device is the Spotify device the player plays on and follows.
type Device interface {
	Play(ctx context.Context, track media.Track, position time.Duration) error
	Pause(ctx context.Context) error
	Resume(ctx context.Context) error
	Seek(ctx context.Context, position time.Duration) error
	// SetVolume sets the volume, from 0 to 1.
	SetVolume(ctx context.Context, v float64) error
	// State tells what the device plays, or nil when it plays nothing.
	State(ctx context.Context) (*DeviceState, error)
}

// DeviceState is what a device plays, as it reports it.
type DeviceState struct {
	Playing  bool
	Progress time.Duration
	// Track is nil when the device plays nothing.
	Track *media.Track
}

// These times are the device's; tests shorten them.
var (
	// connectTick moves the progress on between the device's reports.
	connectTick = time.Second
	// connectAskEvery is how often the device is asked what it plays.
	connectAskEvery = 3 * time.Second
	// connectSettle is how long the device's reports may still tell what
	// it did before a command.
	connectSettle = 5 * time.Second
)

const (
	// connectEnd is how close to its end a track is ending.
	connectEnd = 6 * time.Second
	// connectStart is how close to its start a track that Spotify plays
	// again is: it repeats it after its end.
	connectStart = 3 * time.Second
	// connectStill is how little a track waiting where it started moved.
	connectStill = 1500 * time.Millisecond
)

// connectEngine plays Spotify tracks on the device. It belongs to the
// player's goroutine, and tells the player what the device does as the
// stream engine does.
type connectEngine struct {
	p      *Player
	device Device
	// tickEvery, askDefault and settle are connectTick, connectAskEvery
	// and connectSettle as the engine was made; askEvery is how often the
	// device it plays on is asked what it plays.
	tickEvery, askEvery, askDefault, settle time.Duration
	// ticking is closed to stop the ticks of a track.
	ticking chan struct{}
	track   *media.Track
	// progress is where the track is; startedAt where it started.
	progress  time.Duration
	startedAt time.Duration
	playing   bool
	// sentAt is when the device got the last command; askedAt when it
	// was last asked what it plays.
	sentAt  time.Time
	askedAt time.Time
	asking  bool
	// nudged is set once the device was told again to play the track.
	nudged bool
	// ending is set once the device told the track is in its last seconds.
	ending bool
	// strays counts the reports in a row of the device playing something
	// else.
	strays int
	// lost is set once the device plays something else: playing starts
	// the track there again.
	lost bool
}

func (c *connectEngine) active() bool { return c.track != nil }

// asker is a device that tells how often to ask it what it plays: one at
// hand answers at once.
type asker interface{ AskEvery() time.Duration }

// use has the engine play on d: the device it leaves pauses.
func (c *connectEngine) use(d Device) {
	if c.device == d {
		return
	}
	c.stop(true)
	c.device = d
	c.askEvery = c.askDefault
	if a, ok := d.(asker); ok {
		c.askEvery = a.AskEvery()
	}
}

func (c *connectEngine) duration() time.Duration { return ms(c.track.DurationMs) }

// play plays track on the device from position, and then tells how it
// went. A track the player left meanwhile stops on the device.
func (c *connectEngine) play(track media.Track, position time.Duration, then func(error)) {
	// Until the device plays it, its reports would end the track before.
	c.stop(false)
	token := c.p.loadToken
	await(c.p, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, c.device.Play(ctx, track, position)
	}, func(_ struct{}, err error) {
		if err == nil && token != c.p.loadToken {
			c.send(c.device.Pause)
			return
		}
		if err == nil {
			c.track = &track
			c.progress, c.startedAt = position, position
			c.nudged, c.ending, c.lost, c.strays = false, false, false, 0
			c.sentAt = time.Now()
			c.playing = true
			c.p.onPlaying(true)
			c.p.onTime(position, c.duration())
			c.poll()
		}
		then(err)
	})
}

// send sends the device a command whose outcome nothing waits for.
func (c *connectEngine) send(command func(context.Context) error) {
	go func() { _ = command(c.p.ctx) }()
}

// command sends the device a command, and then runs done if the device
// took it and the track still plays there.
func (c *connectEngine) command(command func(context.Context) error, done func()) {
	track := c.track
	await(c.p, func(ctx context.Context) (struct{}, error) { return struct{}{}, command(ctx) }, func(_ struct{}, err error) {
		if err == nil && c.track == track {
			done()
		}
	})
}

func (c *connectEngine) pause() {
	if c.track == nil {
		return
	}
	paused := func() {
		c.sentAt = time.Now()
		c.playing = false
		c.p.onPlaying(false)
	}
	if c.lost {
		// A device that plays something else is not paused for it.
		paused()
		return
	}
	c.command(c.device.Pause, paused)
}

func (c *connectEngine) resume() {
	if c.track == nil {
		return
	}
	if c.lost {
		c.play(*c.track, c.progress, func(error) {})
		return
	}
	c.command(c.device.Resume, func() {
		c.sentAt = time.Now()
		c.playing = true
		c.p.onPlaying(true)
	})
}

func (c *connectEngine) seek(t time.Duration) {
	if c.track == nil {
		return
	}
	moved := func() {
		c.progress = t
		c.ending = false
		c.sentAt = time.Now()
		c.p.onTime(t, c.duration())
	}
	if c.lost {
		// The track starts from there when it plays again.
		moved()
		return
	}
	c.command(func(ctx context.Context) error { return c.device.Seek(ctx, t) }, moved)
}

func (c *connectEngine) setVolume(v float64) {
	if c.track != nil {
		c.send(func(ctx context.Context) error { return c.device.SetVolume(ctx, v) })
	}
}

// stop stops following the device, pausing it when asked.
func (c *connectEngine) stop(pause bool) {
	if c.ticking != nil {
		close(c.ticking)
		c.ticking = nil
	}
	if pause && c.track != nil && !c.lost {
		c.send(c.device.Pause)
	}
	c.track = nil
	c.playing = false
}

// poll ticks every connectTick while the track plays on the device.
func (c *connectEngine) poll() {
	ticking := make(chan struct{})
	c.ticking = ticking
	go func() {
		t := time.NewTicker(c.tickEvery)
		defer t.Stop()
		for {
			select {
			case <-ticking:
				return
			case <-c.p.ctx.Done():
				return
			case <-t.C:
				c.p.do(func() {
					if c.ticking == ticking {
						c.tick()
					}
				})
			}
		}
	}()
}

func (c *connectEngine) tick() {
	track := c.track
	dur := c.duration()
	if c.playing {
		c.progress = min(c.progress+c.tickEvery, dur)
		c.p.onTime(c.progress, dur)
	}
	asked := time.Now()
	near := dur-c.progress < connectEnd
	if c.asking || (asked.Sub(c.askedAt) < c.askEvery && !near) {
		return
	}
	c.asking = true
	c.askedAt = asked
	await(c.p, c.device.State, func(state *DeviceState, err error) {
		c.asking = false
		// A report asked for before the last command tells what the device
		// did before it.
		if err != nil || c.track != track || c.sentAt.After(asked) {
			return
		}
		c.follow(state, time.Since(c.sentAt) < c.settle)
	})
}

// follow takes in what the device reported, while it settles after a
// command or not.
func (c *connectEngine) follow(state *DeviceState, settling bool) {
	dur := c.duration()
	if state != nil && sameTrack(state.Track, c.track) {
		c.strays = 0
		c.lost = false
		// After its end, Spotify plays the track again from its start when
		// it repeats, or stops on it, at its start or at its end.
		back := state.Progress < connectStart || (!state.Playing && dur-state.Progress < connectStill)
		if c.ending && !settling && back {
			c.finish()
			return
		}
		// The device may still report the track loading, paused.
		if settling && c.playing && !state.Playing {
			return
		}
		// A device may take the track and wait, paused, where it was to
		// start: it is told once more to play.
		if c.playing && !state.Playing && !c.nudged && state.Progress-c.startedAt < connectStill {
			c.nudged = true
			c.sentAt = time.Now()
			c.send(c.device.Resume)
			return
		}
		c.progress = state.Progress
		c.ending = dur-state.Progress < connectEnd
		if state.Playing != c.playing {
			c.playing = state.Playing
			c.p.onPlaying(state.Playing)
		}
		c.p.onTime(state.Progress, dur)
		return
	}
	// The device may still report what it played before the command.
	if settling {
		return
	}
	// Around the end, the device moving on or stopping means the track
	// ended.
	if c.ending || (c.playing && dur-c.progress < connectEnd) {
		c.finish()
		return
	}
	// Otherwise someone plays something else on it: twice in a row, it is
	// lost.
	c.strays++
	if c.strays < 2 || c.lost {
		return
	}
	c.lost = true
	if c.playing {
		c.playing = false
		c.p.onPlaying(false)
	}
}

func (c *connectEngine) finish() {
	c.stop(false)
	c.p.advance(true)
}

// sameTrack tells whether the device plays track: by its ID, or for a
// relinked copy, by its title, first artist and length.
func sameTrack(playing, track *media.Track) bool {
	if playing == nil || track == nil {
		return false
	}
	if playing.ID == track.ID {
		return true
	}
	first := func(t *media.Track) string {
		if len(t.Artists) == 0 {
			return ""
		}
		return t.Artists[0].Name
	}
	d := ms(playing.DurationMs) - ms(track.DurationMs)
	return plain(playing.Title) == plain(track.Title) && plain(first(playing)) == plain(first(track)) &&
		d < 3*time.Second && d > -3*time.Second
}

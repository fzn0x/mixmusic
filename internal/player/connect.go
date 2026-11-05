package player

// Playback on a Spotify Connect device: the player sends commands and asks
// what the device plays. Spotify's reports lag its commands by a few
// seconds, and may name a track by another ID, that of the copy Spotify
// relinked for the user's country, so the engine waits for the device to
// settle and knows its track by more than its ID.
//
// A device that plays a run of tracks by itself (aheadPlayer), as
// MixMusic's own does, gets the Spotify tracks the queue moves on to after
// the current one along with it: it goes on to them without a gap, and the
// engine follows it there rather than telling it to play them.

import (
	"context"
	"log"
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
	// connectGoOn is how long a device that ended a track may take to go
	// on by itself to the one it was given after it.
	connectGoOn = 2 * time.Second
)

// maxAhead is how many tracks after the current one a device that plays
// them by itself gets at most.
const maxAhead = 20

// aheadPlayer is a device that plays tracks after the one it's asked to,
// by itself and without a gap between them, as MixMusic's own does.
type aheadPlayer interface {
	PlayAhead(ctx context.Context, track media.Track, position time.Duration, ahead []media.Track) error
}

// settler is a device that tells how long its reports may still tell what
// it did before a command: one at hand settles at once.
type settler interface{ Settle() time.Duration }

// husher is a device that silences what it plays at once, and stops it,
// until a track it is told to play sounds: pausing a track it still loads
// doesn't hold, as it plays it once loaded.
type husher interface {
	Hush(ctx context.Context) error
}

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
	// tickEvery, askDefault, settleDefault and goOn are connectTick,
	// connectAskEvery, connectSettle and connectGoOn as the engine was
	// made; askEvery and settle are the device's it plays on.
	tickEvery, askEvery, askDefault, settle, settleDefault, goOn time.Duration
	// ticking is closed to stop the ticks of a track.
	ticking chan struct{}
	track   *media.Track
	// ahead are the entries the device was given to play after track, by
	// itself; followed is the move the device made to the first of them,
	// which the player makes too, until it plays it (take).
	ahead    []Item
	followed *following
	// progress is where the track is; startedAt where it started.
	progress  time.Duration
	startedAt time.Duration
	playing   bool
	// sentAt is when the device got the last command; askedAt when it
	// was last asked what it plays.
	sentAt  time.Time
	askedAt time.Time
	asking  bool
	// again is set when the device changed while it was being asked: it is
	// asked once more.
	again bool
	// nudged is set once the device was told again to play the track.
	nudged bool
	// ending is set once the device told the track is in its last seconds.
	ending bool
	// endedAt is when the device was first seen to end the track without
	// having gone on yet to the entry it was given after it (ended).
	endedAt time.Time
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
	c.settle = c.settleDefault
	if s, ok := d.(settler); ok {
		c.settle = s.Settle()
	}
}

func (c *connectEngine) duration() time.Duration { return ms(c.track.DurationMs) }

// play plays track on the device from position, with the entries ahead
// after it on a device that plays them by itself, and then tells how it
// went. A track the player left meanwhile stops on the device.
func (c *connectEngine) play(track media.Track, ahead []Item, position time.Duration, then func(error)) {
	// Until the device plays it, its reports would end the track before.
	c.stop(false)
	ap, ok := c.device.(aheadPlayer)
	if !ok {
		ahead = nil
	}
	token := c.p.loadToken
	await(c.p, func(ctx context.Context) (struct{}, error) {
		if len(ahead) == 0 {
			return struct{}{}, c.device.Play(ctx, track, position)
		}
		tracks := make([]media.Track, len(ahead))
		for i, it := range ahead {
			tracks[i] = it.Track
		}
		return struct{}{}, ap.PlayAhead(ctx, track, position, tracks)
	}, func(_ struct{}, err error) {
		if err == nil && token != c.p.loadToken {
			c.send(c.device.Pause)
			return
		}
		if err == nil {
			c.track = &track
			c.ahead = ahead
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
		c.play(*c.track, c.p.aheadOnDevice(), c.progress, func(error) {})
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
		// Sought into its last seconds, the track may end before the
		// device, once settled, tells where it is.
		c.ending = c.duration()-t < connectEnd
		c.endedAt = time.Time{}
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

// stop stops following the device, pausing it when asked. A move the
// device made by itself that the player didn't make yet is dropped.
func (c *connectEngine) stop(pause bool) {
	if c.ticking != nil {
		close(c.ticking)
		c.ticking = nil
	}
	if pause && (c.track != nil && !c.lost || c.followed != nil) {
		c.send(c.device.Pause)
	}
	c.track = nil
	c.ahead = nil
	c.followed = nil
	c.endedAt = time.Time{}
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
	dur := c.duration()
	if c.playing {
		c.progress = min(c.progress+c.tickEvery, dur)
		c.p.onTime(c.progress, dur)
	}
	near := dur-c.progress < connectEnd
	if c.asking || (time.Since(c.askedAt) < c.askEvery && !near) {
		return
	}
	c.ask()
}

// changed takes in the device telling that what it plays changed: it is
// asked at once, or once more when it is being asked.
func (c *connectEngine) changed() {
	if c.track == nil {
		return
	}
	if c.asking {
		c.again = true
		return
	}
	c.ask()
}

// ask asks the device what it plays, and follows it.
func (c *connectEngine) ask() {
	track := c.track
	asked := time.Now()
	c.asking, c.again = true, false
	c.askedAt = asked
	await(c.p, c.device.State, func(state *DeviceState, err error) {
		c.asking = false
		// A report asked for before the last command tells what the device
		// did before it.
		if err == nil && c.track == track && !c.sentAt.After(asked) {
			c.follow(state, time.Since(c.sentAt) < c.settle)
		}
		if c.again && c.track != nil {
			c.ask()
		}
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
			c.ended()
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
		c.endedAt = time.Time{}
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
	// The device went on by itself to the track it was given after this
	// one.
	if state != nil && len(c.ahead) > 0 && sameTrack(state.Track, &c.ahead[0].Track) {
		c.followOn(state)
		return
	}
	// Around the end, the device moving on or stopping means the track
	// ended.
	if c.ending || (c.playing && dur-c.progress < connectEnd) {
		c.ended()
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

// ended takes in the device having ended the track. A device given tracks
// to play after it goes on to the first by itself, but may first stop on
// the track a moment, or tell nothing, while it loads that one: Spotify
// does when a seek into the track's last seconds kept it from loading it
// ahead. It has goOn to, before the player moves on itself: told to play
// that track as it went on to it by itself, Spotify started it again,
// late, or went on past it.
func (c *connectEngine) ended() {
	if len(c.ahead) == 0 {
		c.finish()
		return
	}
	if c.endedAt.IsZero() {
		c.endedAt = time.Now()
		// It is asked once more then, unless it tells before.
		track := c.track
		c.p.after(c.goOn, func() {
			if c.track == track && !c.endedAt.IsZero() {
				c.changed()
			}
		})
		return
	}
	if time.Since(c.endedAt) < c.goOn {
		return
	}
	log.Printf("mixmusic: Spotify's player didn't go on by itself after %s", c.track.ID)
	c.finish()
}

func (c *connectEngine) finish() {
	c.stop(false)
	c.p.advance(true)
}

// following is the device having gone on by itself to the entry of uid,
// which it reported as state, with the entries after it it was given.
type following struct {
	uid   string
	state DeviceState
	ahead []Item
}

// followOn takes in the device having gone on by itself to the first entry
// it was given after the current one: the player moves on to it too, as at
// the end of a track, and plays it where the device plays it rather than
// telling the device to (take). When the queue no longer moves on to that
// entry, as one was played next meanwhile, what the device went on to is
// silenced at once, and the player moves on to its own.
func (c *connectEngine) followOn(state *DeviceState) {
	next := c.ahead[0]
	if ahead := c.p.aheadOnDevice(); len(ahead) == 0 || ahead[0].UID != next.UID {
		// Once it is silent, unless the player moved on meanwhile.
		c.stop(false)
		silence := c.device.Pause
		if h, ok := c.device.(husher); ok {
			silence = h.Hush
		}
		token := c.p.loadToken
		await(c.p, func(ctx context.Context) (struct{}, error) { return struct{}{}, silence(ctx) }, func(struct{}, error) {
			if token == c.p.loadToken {
				c.p.advance(true)
			}
		})
		return
	}
	rest := c.ahead[1:]
	c.stop(false)
	c.followed = &following{uid: next.UID, state: *state, ahead: rest}
	c.p.advance(true)
}

// take returns the move the device made by itself to the entry of uid,
// once the player moved on to it, or nil. A move to another one, which the
// player no longer makes, stops on the device.
func (c *connectEngine) take(uid string) *following {
	f := c.followed
	c.followed = nil
	if f != nil && f.uid != uid {
		c.send(c.device.Pause)
		return nil
	}
	return f
}

// follows has the engine follow the device on track, which it went on to
// by itself (f), as if it played it there.
func (c *connectEngine) follows(track media.Track, f *following) {
	c.track = &track
	c.ahead = f.ahead
	c.progress, c.startedAt = f.state.Progress, 0
	c.nudged, c.ending, c.lost, c.strays = false, false, false, 0
	c.sentAt = time.Time{}
	c.playing = f.state.Playing
	c.p.onPlaying(c.playing)
	c.p.onTime(c.progress, c.duration())
	c.poll()
}

// aheadOnDevice lists the entries the queue moves on to by itself after
// the current one, up to maxAhead, that a device which plays them by itself
// gets: the Spotify tracks right after it, up to the first that isn't one,
// that the queue passes over, or whose label a move would check first.
// Repeating one has none.
func (p *Player) aheadOnDevice() []Item {
	if p.s.Repeat == RepeatOne || p.s.Index < 0 {
		return nil
	}
	var ahead []Item
	for i := p.s.Index + 1; i < len(p.s.Queue) && len(ahead) < maxAhead; i++ {
		it := p.s.Queue[i]
		// An episode the player starts itself, from where it was left.
		if it.Skip != "" || it.Track.Source != media.SourceSpotify || spoken(it.Track) {
			break
		}
		if !it.Chosen && p.checks(it.Track) && p.knownLabel(it.Track) != LabelNone {
			break
		}
		ahead = append(ahead, it)
	}
	return ahead
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

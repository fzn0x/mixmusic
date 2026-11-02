package player

// Where the stream engine's audio plays: on the computer, or on a device
// away from it, as a cast device, which the app puts in the engine's place
// behind the player's Stream. The player plays the queue as it does; told
// that the output moved, it loads the stream that plays again there, from
// where it was. A Spotify track that plays on a Spotify device goes on
// there: it isn't the engine's.

import (
	"math"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
)

// Output is where the player's streams play, as the app tells it. The zero
// Output is the computer, whose volume the settings keep.
type Output struct {
	// Remote is set for a device away from the computer, as a cast device.
	// Tracks don't crossfade there, as it plays one stream at a time, and
	// the volume is the device's own, Volume and Muted, which the player
	// shows and sets while it plays there, without keeping it in the
	// settings.
	Remote bool
	Volume float64
	Muted  bool
}

// SetOutput tells the player that its streams play on o from now on: the
// stream of the current track loads again there, where it was, playing or
// paused as it was. A track that loads, or waits for the network, plays
// there once it does. Back on the computer, the volume is the computer's
// again.
func (p *Player) SetOutput(o Output) { p.do(func() { p.setOutput(o) }) }

func (p *Player) setOutput(o Output) {
	if !o.Remote && !p.out.Remote {
		return
	}
	if !p.out.Remote {
		p.home = homeVolume{volume: p.s.Volume, muted: p.s.Muted}
	}
	p.out = o
	p.s.Remote = o.Remote
	if o.Remote {
		p.s.Volume, p.s.Muted = clampVolume(o.Volume, p.s.Volume), o.Muted
	} else {
		p.s.Volume, p.s.Muted = p.home.volume, p.home.muted
		// The engine gets the computer's volume again.
		p.stream.SetVolume(p.heardVolume())
	}
	p.emit(StateChange)
	p.report()
	p.moveStream()
}

// OutputVolume takes in the remote output's volume changing by itself, as
// by its own buttons: the player shows it, and keeps nothing of it. On the
// computer, it changes nothing.
func (p *Player) OutputVolume(v float64, muted bool) {
	p.do(func() {
		if !p.out.Remote {
			return
		}
		p.s.Volume, p.s.Muted = clampVolume(v, p.s.Volume), muted
		p.emit(StateChange)
		p.report()
	})
}

// homeVolume is the computer's volume, as it was before the streams went
// to a remote output.
type homeVolume struct {
	volume float64
	muted  bool
}

// clampVolume returns v from 0 to 1, or else when v isn't a number.
func clampVolume(v, or float64) float64 {
	if math.IsNaN(v) {
		return or
	}
	return min(max(v, 0), 1)
}

// heardVolume is the volume the outputs play at: 0 while muted.
func (p *Player) heardVolume() float64 {
	if p.s.Muted {
		return 0
	}
	return p.s.Volume
}

// keptVolume returns the volume the settings keep of st: the computer's,
// while the streams play on a remote output, whose volume is its own.
func (p *Player) keptVolume(st State) float64 {
	if st.Remote {
		return p.snapHome
	}
	return st.Volume
}

// moveStream loads the current track's stream again, on the output it
// plays on now, where it was: playing, or loading to play, or paused, as
// it was. It sounds there once its time moves past where it started, and
// loads until then.
func (p *Player) moveStream() {
	src := p.s.Source
	if !p.loaded || src == nil || src.Mode != ModeStream || p.s.Error != "" {
		return
	}
	autoplay := !p.paused && (p.s.Playing || p.s.Loading)
	p.paused = !autoplay
	p.retry = retryState{played: p.retry.played, from: p.s.Position}
	p.unsounded, p.soundFrom = autoplay, p.s.Position
	if autoplay {
		p.s.Loading, p.s.Buffering = true, true
	}
	p.emit(StateChange)
	p.report()
	_ = p.stream.Load(p.audioOf(*src), p.cue(p.s.Position, autoplay))
}

// cue is how the stream of the current track starts, from start: at the
// speed the track plays at, with the track, for an output that shows it.
func (p *Player) cue(start time.Duration, autoplay bool) audio.Cue {
	c := audio.Cue{Start: start, Autoplay: autoplay, Speed: p.s.Speed}
	if cur, ok := p.s.Current(); ok {
		t := cur.Track
		c.Track = &t
	}
	return c
}

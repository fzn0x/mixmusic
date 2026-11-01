package player

// Waiting for the network: a track that can't play for want of it keeps
// its place, and plays on from there once the network answers, rather
// than failing, so that the queue doesn't skip on while the network is
// gone.

import (
	"context"
	"errors"
)

// network returns the app's Network, if its services have it.
func (p *Player) network() (Network, bool) {
	n, ok := p.svc.(Network)
	return n, ok
}

// offline has the current track wait for the network when err, why it
// couldn't play, is ErrOffline, and tells whether it does. It plays once
// the network answers when willPlay, from where it is.
func (p *Player) offline(err error, willPlay bool) bool {
	if _, ok := p.network(); !ok || !errors.Is(err, ErrOffline) {
		return false
	}
	p.wait(willPlay)
	return true
}

// radioOffline reports whether err, why the radio couldn't be found, is
// that the network is gone, as the app tells it.
func (p *Player) radioOffline(err error) bool {
	_, ok := p.network()
	return ok && errors.Is(err, ErrOffline)
}

// checkOnline asks whether the network answers, beside the player's
// goroutine, and runs offline or online on it with the answer, unless
// another track loaded meanwhile. Without a Network, it runs online.
func (p *Player) checkOnline(offline, online func()) {
	n, ok := p.network()
	if !ok {
		online()
		return
	}
	token := p.loadToken
	await(p, func(ctx context.Context) (bool, error) { return n.Online(ctx), nil }, func(up bool, _ error) {
		if token != p.loadToken {
			return
		}
		if up {
			online()
		} else {
			offline()
		}
	})
}

// wait has the current track wait for the network where it is, and play
// from there once it answers when willPlay. The engine lets go of it
// meanwhile.
func (p *Player) wait(willPlay bool) {
	p.loaded = false
	p.stream.Unload()
	p.s.Playing, p.s.Loading, p.s.Buffering = false, false, false
	p.waitFor(willPlay, false, func(play bool) { p.playAt(p.s.Index, p.s.Position, play) })
}

// waitRadio has the radio wait for the network at the end of the queue,
// where the queue stopped once its last track ended, and then play.
func (p *Player) waitRadio() {
	p.stopAtEnd(true)
	p.waitFor(true, true, func(play bool) {
		token := p.loadToken
		p.extendWithRadio(func(added bool, err error) {
			switch {
			case token != p.loadToken:
			case added:
				p.playAt(p.s.Index+1, 0, play)
			case errors.Is(err, ErrOffline):
				p.waitRadio() // gone again
			}
		})
	})
}

// waitFor shows the wait, tells the user once that the network is gone,
// and has resume run once it answers, with whether to play, unless
// another track loaded meanwhile.
func (p *Player) waitFor(willPlay, radio bool, resume func(play bool)) {
	n, _ := p.network()
	p.endWait()
	p.s.Waiting, p.s.WillPlay = true, willPlay
	p.s.Error = ""
	p.radioWaits = radio
	p.emit(StateChange)
	p.report()
	if !p.toldOffline {
		p.toldOffline = true
		p.notify(Notice{Message: "No network. MixMusic plays on from here once it's back"})
	}
	token := p.loadToken
	ctx, cancel := context.WithCancel(p.ctx)
	p.stopWait = cancel
	await(p, func(context.Context) (struct{}, error) { return struct{}{}, n.WaitOnline(ctx) }, func(_ struct{}, err error) {
		if err != nil || token != p.loadToken || !p.s.Waiting {
			return
		}
		play := p.s.WillPlay
		p.endWait()
		p.failures = 0
		p.toldOffline = false
		resume(play)
	})
}

// endWait ends the wait for the network, if any.
func (p *Player) endWait() {
	if p.stopWait != nil {
		p.stopWait()
		p.stopWait = nil
	}
	if p.s.Waiting {
		p.s.Waiting, p.s.WillPlay, p.radioWaits = false, false, false
		p.emit(StateChange)
	}
}

// watchStall asks whether the network answers once the stream buffered
// for stallCheck, and has the track wait for it when it doesn't: a
// download that stalled may sit for minutes before it fails.
func (p *Player) watchStall(buffering bool) {
	p.stalls++
	if _, ok := p.network(); !ok || !buffering || !p.loaded {
		return
	}
	token, stall := p.loadToken, p.stalls
	stalled := func() bool { return token == p.loadToken && stall == p.stalls && p.loaded && p.s.Buffering }
	p.after(stallCheck, func() {
		if stalled() {
			p.checkOnline(func() {
				if stalled() {
					p.wait(!p.paused)
				}
			}, func() {})
		}
	})
}

// prefetchOnline prepares the next track again once the network answers:
// it couldn't for want of it.
func (p *Player) prefetchOnline(uid string) {
	n, ok := p.network()
	if !ok {
		return
	}
	await(p, func(ctx context.Context) (struct{}, error) { return struct{}{}, n.WaitOnline(ctx) }, func(struct{}, error) {
		if p.prefetched == uid {
			p.prefetched = ""
		}
	})
}

// Woke takes in the computer waking. A track that waits for the network
// checks it at once. Once the network answers, the current track's stream
// is found again, should its address have expired overnight, and the next
// track prepared again, so that both play at once.
func (p *Player) Woke() {
	p.do(func() {
		p.prefetched = ""
		p.retry.n = 0
		n, ok := p.network()
		if !ok {
			return
		}
		if p.s.Waiting {
			n.KickOnline()
			return
		}
		cur, ok := p.s.Current()
		if !ok || p.s.Source == nil || p.s.Source.Mode != ModeStream {
			return
		}
		await(p, func(ctx context.Context) (Source, error) {
			ctx, cancel := context.WithTimeout(ctx, wakeRefresh)
			defer cancel()
			if err := n.WaitOnline(ctx); err != nil {
				return Source{}, err
			}
			return p.svc.PrepareStream(ctx, cur.Track)
		}, func(Source, error) {})
	})
}

// Sleeping takes in the computer going to sleep. What MixMusic plays
// itself pauses, a track that loads stays paused once loaded, and a track
// that waits for the network doesn't play once it's back: the computer
// may wake somewhere quiet. A Spotify Connect device elsewhere plays on.
func (p *Player) Sleeping() {
	p.do(func() {
		switch {
		case p.s.Waiting:
			p.s.WillPlay = false
			p.emit(StateChange)
		case p.s.Loading:
			p.playAt(p.s.Index, p.s.Position, false)
		case p.s.Playing && (p.s.Source == nil || p.s.Source.Mode != ModeConnect):
			p.pause()
		}
	})
}

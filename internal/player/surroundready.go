package player

// Starting in stereo (#457): a stream whose 5.1 surround audio the
// settings ask for, while that audio isn't ready, plays its stereo audio
// meanwhile. The app makes the 5.1 audio ready beside it: yt-dlp may take
// seconds to find it, and YouTube a few more to serve it. Once it is ready,
// the engine plays it from where the track is then, as pressing 5.1 does.
// A preparation stopped, as the track changed or 5.1 was turned off, or
// whose answer is of another stream, leaves the stereo audio playing; one
// that fails leaves the track in stereo, and tells why once.

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
)

// Surrounds are what the player asks of the app of the 5.1 surround audio
// of streams, when its Services have them too: without, a stream plays its
// 5.1 audio at once, while the settings ask for it.
type Surrounds interface {
	// SurroundReady reports, without waiting, whether the 5.1 surround
	// audio of src, a stream, plays at once: from this computer, or found
	// already, and served by YouTube.
	SurroundReady(src Source) bool
	// PrepareSurround returns once the 5.1 surround audio of src plays at
	// once, as SurroundReady tells, or ctx ends: once it is found, and
	// YouTube serves it. It returns why it can't, as when YouTube has none.
	PrepareSurround(ctx context.Context, src Source) error
}

// readying is a preparation of the 5.1 surround audio of the current
// entry's stream, url, as it loaded with token: id tells it apart from
// those before it.
type readying struct {
	id     int
	url    string
	token  int
	cancel context.CancelFunc
	asked  time.Time
}

// surrounds returns the app's Surrounds, if its services have them.
func (p *Player) surrounds() (Surrounds, bool) {
	s, ok := p.svc.(Surrounds)
	return s, ok
}

// audioOf returns the URL of the audio of the current entry's stream to
// play: its 5.1 audio while the settings ask for it, unless it failed, once
// it is ready, else its stereo audio.
func (p *Player) audioOf(src Source) string {
	if _, ok := p.surrounds(); ok && src.SurroundURL != p.readyURL {
		return src.URL
	}
	return src.audio(p.surround && !p.stereoOnly)
}

// playsSurround reports whether the engine plays the 5.1 audio of the
// current entry's stream.
func (p *Player) playsSurround() bool {
	src := p.s.Source
	return p.inSurround && src != nil && src.Mode == ModeStream
}

// loadAudio has the engine load the audio of src, the current entry's
// stream, that audioOf gives, as cue asks, and keeps which it is. A try
// again asks for it anew.
func (p *Player) loadAudio(src Source, cue audio.Cue, again bool) {
	url := p.audioOf(src)
	p.inSurround = src.SurroundURL != "" && url == src.SurroundURL
	if again {
		url += "?retry=" + strconv.FormatInt(time.Now().UnixMilli(), 10)
	}
	_ = p.stream.Load(url, cue)
}

// knowReady asks the app, without waiting, whether the 5.1 surround audio
// of src, the current entry's stream, plays at once, while the settings ask
// for it and the player doesn't know it does.
func (p *Player) knowReady(src Source) {
	r, ok := p.surrounds()
	if ok && src.SurroundURL != "" && src.SurroundURL != p.readyURL && p.surround && !p.stereoOnly && r.SurroundReady(src) {
		p.readyURL = src.SurroundURL
	}
}

// goSurround has the current entry's stream play its 5.1 surround audio, as
// the settings ask, from where the track is: at once when it is ready, else
// once it is, its stereo audio playing on meanwhile. A stream that plays
// it, or loads, or has none, plays on.
func (p *Player) goSurround() {
	src := p.s.Source
	if src == nil || src.Mode != ModeStream || src.SurroundURL == "" || !p.surround || p.stereoOnly || !p.loaded || p.inSurround {
		return
	}
	p.knowReady(*src)
	if p.audioOf(*src) == src.SurroundURL {
		p.toSurround()
		return
	}
	p.readySurround()
}

// readySurround has the app make the 5.1 surround audio of the current
// entry's stream ready, while the settings ask for it and it isn't, beside
// its stereo audio, which plays meanwhile: once it is, the stream plays it
// from where the track is then (toSurround). One under way goes on.
func (p *Player) readySurround() {
	r, ok := p.surrounds()
	src := p.s.Source
	if !ok || src == nil || src.Mode != ModeStream || src.SurroundURL == "" || !p.surround || p.stereoOnly || p.inSurround ||
		p.readyURL == src.SurroundURL {
		return
	}
	if p.readying.url == src.SurroundURL && p.readying.token == p.loadToken {
		return
	}
	p.stopReadying()
	ctx, cancel := context.WithCancel(p.ctx)
	p.readySeq++
	this := readying{id: p.readySeq, url: src.SurroundURL, token: p.loadToken, cancel: cancel, asked: time.Now()}
	p.readying = this
	s := *src
	await(p, func(context.Context) (struct{}, error) { return struct{}{}, r.PrepareSurround(ctx, s) }, func(_ struct{}, err error) {
		cancel()
		if p.readying.id != this.id {
			return // stopped, or another took over
		}
		p.readying = readying{}
		if this.token != p.loadToken {
			return // of an entry that no longer plays
		}
		cur, _ := p.s.Current()
		if err != nil {
			log.Printf("mixmusic: the 5.1 surround audio of %s can't play: %s; it plays in stereo", cur.Track.ID, err)
			p.stereoFor(err.Error())
			return
		}
		p.readyURL = this.url
		log.Printf("mixmusic: the 5.1 surround audio of %s is ready, %.1fs after it was asked for; it plays from %s",
			cur.Track.ID, time.Since(this.asked).Seconds(), p.s.Position.Round(100*time.Millisecond))
		p.toSurround()
	})
}

// stopReadying stops the preparation of 5.1 surround audio under way, if
// any: what it finds plays no more.
func (p *Player) stopReadying() {
	if p.readying.cancel != nil {
		p.readying.cancel()
	}
	p.readying = readying{}
}

// toSurround has the engine play the 5.1 surround audio of the current
// entry's stream from where the track is, playing or paused as it is,
// while the settings ask for it, it is ready, and the engine plays the
// stereo audio. A crossfade plays out first: its end comes back here.
func (p *Player) toSurround() {
	src := p.s.Source
	if src == nil || src.Mode != ModeStream || src.SurroundURL == "" || !p.loaded || p.inSurround || p.crossfading ||
		p.audioOf(*src) != src.SurroundURL {
		return
	}
	p.loadAudio(*src, p.cue(p.s.Position, !p.paused), false)
}

// stereoFor keeps the current entry in stereo, as its 5.1 surround audio
// couldn't play, for why, and tells why once: until another entry, or
// until 5.1 is turned on again.
func (p *Player) stereoFor(why string) {
	cur, _ := p.s.Current()
	p.tried("its 5.1 surround audio", why)
	p.stereoOnly = true
	p.notify(Notice{Message: fmt.Sprintf("The 5.1 surround audio of %s couldn't play (%s), so it plays in stereo", cur.Track.Title, strings.TrimSuffix(why, "."))})
}

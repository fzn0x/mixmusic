package player

// The next track on the very next sample (#600): with Crossfade off, or
// wherever a track ends without one, the engine plays the next track from
// the sample right after the last one of the track before, with no silence
// and no fade-in. The player preloads the next entry's audio on the
// engine's other deck as before (preload), and arms that deck to follow
// the current track there while the state lets it: after each task, it
// tells the engine again when that changed (arm), so that a queue edit, a
// shuffle or repeat change, an entry newly passed over, Stop, or a move to
// a cast device disarms it, and a speed changed reaches it. As the current
// track ends, the engine makes that deck the one that plays and tells
// Followed in place of Playing(false) and Ended: the player takes the next
// entry as current, as after a crossfade, so that the app, the media
// controls and Discord see one track change. A next track not preloaded
// in time, a live stream or a podcast's episode, which are never
// preloaded, a cast device and a track Spotify plays, start as before.

import "github.com/fzn0x/mixmusic/internal/audio"

// aheadState is the next entry's audio the engine holds on its other deck,
// as preload gave it: the entry of uid, whose stream src plays url there,
// and how the engine was told it follows the current track.
type aheadState struct {
	uid, url string
	src      Source
	told     join
}

// join is how the engine's other deck follows the current track: whether
// it does, at what speed, and whether at the current track's loudness, as
// the songs of one album do.
type join struct {
	on, glide bool
	speed     float64
}

// cue is the engine's cue of j, for Preload.
func (j join) cue() audio.Cue {
	return audio.Cue{Speed: j.speed, Follow: j.on, Glide: j.glide}
}

// Joiner is a Stream that may refuse to start the next track on the sample
// after the current one: as the stream of a room's host while guests
// listen, whose track changes wait for them.
type Joiner interface {
	// Joins reports whether the next track may start so now.
	Joins() bool
}

// holdAhead takes in the engine holding the audio of src, the next entry
// uid's stream, at url, on its other deck: it loads it there, armed to
// follow the current track as the state lets it.
func (p *Player) holdAhead(uid string, src Source, url string) {
	p.ahead = aheadState{uid: uid, url: url, src: src}
	p.ahead.told = p.joinWanted()
	p.stream.Preload(url, p.ahead.told.cue())
}

// arm tells the engine how the next entry's audio it holds follows the
// current track, when the state asks otherwise than it told it last: after
// each task.
func (p *Player) arm() {
	if p.ahead.url == "" {
		return
	}
	if want := p.joinWanted(); want != p.ahead.told {
		p.ahead.told = want
		p.stream.Preload(p.ahead.url, want.cue())
	}
}

// joinWanted is how the next entry's audio the engine holds is to follow
// the current track: on its next sample while the entry is the next one
// and mayFollow lets it, at its own speed, at the current track's loudness
// for a song of the same album, which comes next on it.
func (p *Player) joinWanted() join {
	cur, ok := p.s.Current()
	next, hasNext := p.s.UpNext()
	if !ok || !hasNext || next.UID != p.ahead.uid || !p.mayFollow(cur, next) {
		return join{}
	}
	return join{
		on:    true,
		glide: sameAlbum(cur.Track, next.Track) && follows(cur.Track, next.Track),
		speed: speedOf(p.svc.Settings(), next.Track, false),
	}
}

// mayFollow reports whether next, the entry after cur, the current one,
// may start on the sample after cur's track ends: the engine plays cur's
// stream, here, with no crossfade under way, and next is one the queue
// plays, that may start before its turn.
func (p *Player) mayFollow(cur, next Item) bool {
	if j, ok := p.stream.(Joiner); ok && !j.Joins() {
		return false
	}
	return p.out.Stream == nil && p.loaded && !p.crossfading && p.s.Error == "" && p.s.Source != nil &&
		p.s.Source.Mode == ModeStream && next.UID != cur.UID && next.Skip == "" && p.mayStart(next) &&
		// The last track the sleep timer lets play ends the music (#594).
		!p.sleepLast()
}

// Followed takes in the engine playing the track of url on the sample after
// the current one ended, as audio.FollowEvents.
func (p *Player) Followed(url string) { p.do(func() { p.onFollowed(url) }) }

var _ audio.FollowEvents = (*Player)(nil)

// onFollowed takes the next entry, whose audio at url the engine plays
// now, as the current one, as crossfadeTo does, when the engine was told it
// follows the current track and the state still lets it. When the state
// changed a moment before, as the queue did, it moves on as at the end of
// the track, which loads what the queue goes on to. One that comes after a
// load, or a move to a cast device, which stopped what the engine played,
// changes nothing.
func (p *Player) onFollowed(url string) {
	a := p.ahead
	if a.url == "" || a.url != url || !p.loaded || p.out.Stream != nil {
		return
	}
	p.ahead = aheadState{}
	cur, ok := p.s.Current()
	next, hasNext := p.s.UpNext()
	i := p.indexOf(a.uid)
	if !a.told.on || !ok || !hasNext || next.UID != a.uid || i < 0 || !p.mayFollow(cur, next) {
		p.onEnded()
		return
	}
	p.becomeCurrent(i, next, a.src)
	p.playsAudio(url, a.src)
	p.readySurround()
}

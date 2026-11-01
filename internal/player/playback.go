package player

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/volume"
)

// toggleDebounce is how soon a second toggle is taken as the same press:
// the media keys may reach the player as a global shortcut and through the
// system's media controls.
const toggleDebounce = 400 * time.Millisecond

// PlayAt plays the queue's entry at i from its start, as one the user
// chose: though the queue would pass over it.
func (p *Player) PlayAt(i int) {
	p.do(func() {
		p.choose(i)
		p.playAt(i, 0, true)
	})
}

// PlayID plays the entry of the queue that holds the track id, as a deep
// link asks: a track the queue doesn't hold can't play from its ID alone,
// which the player tells.
func (p *Player) PlayID(id string) {
	p.do(func() {
		for i, it := range p.s.Queue {
			if it.Track.ID == id {
				p.choose(i)
				p.playAt(i, 0, true)
				return
			}
		}
		p.notify(Notice{Message: "Open the track's album or playlist to play it"})
	})
}

// Return plays the entry uid again from at, or holds it there paused
// unless playing, as one the user chose: as undoing a dislike that moved
// on from it. An entry still current only plays on, and one the queue no
// longer holds changes nothing.
func (p *Player) Return(uid string, at time.Duration, playing bool) {
	p.do(func() {
		i := p.indexOf(uid)
		if i < 0 {
			return
		}
		p.choose(i)
		if i != p.s.Index || !p.holds() {
			p.playAt(i, at, playing)
		}
	})
}

// holds reports whether the player has the current entry: plays it,
// holds it paused, loads it, or waits for the network to.
func (p *Player) holds() bool {
	return p.loaded || p.s.Loading || p.s.Waiting || p.connect.active()
}

// choose marks the entry at i as one the user chose to play, which plays
// though the queue would pass over it.
func (p *Player) choose(i int) {
	if i < 0 || i >= len(p.s.Queue) || p.s.Queue[i].Chosen {
		return
	}
	q := slices.Clone(p.s.Queue)
	skipped := q[i].Skip != ""
	q[i].Chosen, q[i].Skip = true, ""
	p.s.Queue = q
	if skipped {
		p.emit(QueueChange)
	}
}

// Disliked takes in that the user disliked the track id, or no longer, as
// they do it: the library may keep it a moment later. While the settings
// skip disliked tracks, the queue passes over its entries from then on,
// those the user chose too, and moves on from the current one, playing or
// paused as it was.
func (p *Player) Disliked(id string, disliked bool) { p.do(func() { p.disliked(id, disliked) }) }

func (p *Player) disliked(id string, disliked bool) {
	q := slices.Clone(p.s.Queue)
	changed := false
	for i, it := range q {
		if it.Track.ID != id {
			continue
		}
		switch {
		case disliked && p.skipDisliked:
			it.Chosen, it.Skip = false, SkipDisliked
		case disliked:
			it.Chosen = false
		case it.Skip == SkipDisliked:
			it.Skip = ""
		default:
			continue
		}
		q[i], changed = it, true
	}
	if !changed {
		return
	}
	p.s.Queue = q
	p.emit(QueueChange)
	if cur, ok := p.s.Current(); ok && cur.Track.ID == id && cur.Skip != "" {
		p.leave()
	}
}

// leave moves on from the current entry, which the queue now passes over,
// to the next it doesn't, playing or paused as the current one was. At the
// end of the queue the current track stops, and the radio autoplay adds,
// from the last entry the queue doesn't pass over, carries on.
func (p *Player) leave() {
	play := p.s.Playing || p.s.Loading || p.s.Waiting && p.s.WillPlay
	if i := p.s.nextIndex(); i >= 0 {
		p.playAt(i, 0, play)
		return
	}
	p.halt()
	p.radioAtEnd(play, false, func() {})
}

// halt stops the current track at its start, as the end of the queue
// does: Play plays it from there.
func (p *Player) halt() {
	p.loadToken++
	p.endWait()
	p.loaded = false
	p.stream.Unload()
	p.connect.stop(true)
	p.s.Loading, p.s.Buffering, p.s.Source = false, false, nil
	p.stopAtEnd(true)
}

// Play plays, or the queue from its start when nothing is current.
func (p *Player) Play() { p.do(p.play) }

// Pause pauses.
func (p *Player) Pause() { p.do(p.pause) }

// Toggle plays or pauses.
func (p *Player) Toggle() { p.do(p.toggle) }

// ToggleOnce plays or pauses, but not again within a moment: for the
// media keys.
func (p *Player) ToggleOnce() {
	p.do(func() {
		if now := time.Now(); now.Sub(p.lastToggle) >= toggleDebounce {
			p.lastToggle = now
			p.toggle()
		}
	})
}

// Seek moves the current track to t.
func (p *Player) Seek(t time.Duration) { p.do(func() { p.seek(t) }) }

// Next skips to the next track.
func (p *Player) Next() { p.do(func() { p.advance(false) }) }

// Previous goes back to the start of the track, or to the one before
// early in it.
func (p *Player) Previous() { p.do(p.previous) }

// SetVolume sets the volume, from 0 to 1, and unmutes. A volume that
// isn't a number changes nothing.
func (p *Player) SetVolume(v float64) { p.do(func() { p.setVolume(v) }) }

// ToggleMute mutes or unmutes.
func (p *Player) ToggleMute() {
	p.do(func() {
		p.s.Muted = !p.s.Muted
		p.apply()
	})
}

// SettingsChanged takes in new settings: a Spotify track goes on from where
// it was in the way of playing Spotify tracks they choose, and the queue
// passes over disliked tracks, or no longer, as they say.
func (p *Player) SettingsChanged(s settings.Settings) {
	p.do(func() {
		if s.SkipDisliked != p.skipDisliked {
			p.skipDisliked = s.SkipDisliked
			p.reflag()
		}
		if s.SpotifyPlayback == p.spotifyMode {
			return
		}
		p.spotifyMode = s.SpotifyPlayback
		if cur, ok := p.s.Current(); ok && cur.Track.Source == media.SourceSpotify && p.s.Source != nil {
			p.playAt(p.s.Index, p.s.Position, p.s.Playing)
		}
	})
}

// reflag finds again which entries the queue passes over, as the settings
// changed: never those the user chose. The current entry plays on, and is
// only ever let off.
func (p *Player) reflag() {
	q := make([]Item, len(p.s.Queue))
	for i, it := range p.s.Queue {
		switch {
		case it.Chosen:
			it.Skip = ""
		case i != p.s.Index || it.Skip != "":
			it.Skip = p.svc.Skip(it.Track)
		}
		q[i] = it
	}
	p.s.Queue = q
	p.emit(QueueChange)
}

// playAt plays the queue's entry at i from start, or holds it there when
// autoplay is false.
func (p *Player) playAt(i int, start time.Duration, autoplay bool) {
	if i < 0 || i >= len(p.s.Queue) {
		return
	}
	entry := p.s.Queue[i]
	p.loadToken++
	token := p.loadToken
	p.retry = retryState{}
	p.endWait()
	p.atEnd = false
	p.markPlayed(entry.UID)
	p.s.Index = i
	p.s.Position = start
	p.s.Duration = ms(entry.Track.DurationMs)
	p.s.Error = ""
	p.s.Buffering = true
	p.s.Loading = autoplay
	p.s.Source = nil
	// What the engine played stops now: finding the track's stream may take
	// a while, or fail.
	p.loaded = false
	p.stream.Unload()
	p.askedAt, p.askedFrom = time.Time{}, start
	if autoplay {
		p.askedAt = time.Now()
	}
	p.emit(TrackChange)
	p.report()
	await(p, func(ctx context.Context) (Source, error) { return p.svc.Prepare(ctx, entry.Track) }, func(src Source, err error) {
		if token != p.loadToken {
			return
		}
		if err != nil {
			if !p.offline(err, start, autoplay) {
				p.fail(err.Error())
			}
			return
		}
		switch {
		case src.Mode == ModeConnect:
			p.playOnDevice(token, entry, src, start, autoplay, p.remote)
		case src.Mode == ModeSpotify && p.here != nil:
			p.playOnDevice(token, entry, src, start, autoplay, p.here)
		case src.Mode == ModeSpotify:
			p.fellBack("Spotify can't play inside MixMusic on this computer")
			p.streamInstead(token, entry, start, autoplay)
		default:
			p.loadStream(src, start, autoplay)
		}
	})
}

// playOnDevice plays a Spotify track on a Spotify device: a Spotify
// Connect device, or MixMusic's own.
func (p *Player) playOnDevice(token int, entry Item, src Source, start time.Duration, autoplay bool, d Device) {
	p.stopStream()
	p.connect.use(d)
	if !autoplay {
		// A device can't wait paused on a track: it starts from here on
		// play.
		p.connect.stop(false)
		p.s.Source = &src
		p.s.Buffering = false
		return
	}
	p.connect.play(entry.Track, start, func(err error) {
		if token != p.loadToken {
			return
		}
		if err == nil {
			p.s.Source = &src
			p.s.Buffering = false
			p.failures = 0
			return
		}
		// A device that can't be reached for want of the network waits for
		// it: YouTube Music needs it too.
		p.checkOnline(func() { p.wait(start, autoplay) }, func() {
			p.fellBack(err.Error())
			p.streamInstead(token, entry, start, autoplay)
		})
	})
}

// streamInstead plays a Spotify track from YouTube Music.
func (p *Player) streamInstead(token int, entry Item, start time.Duration, autoplay bool) {
	await(p, func(ctx context.Context) (Source, error) { return p.svc.PrepareStream(ctx, entry.Track) }, func(src Source, err error) {
		if token != p.loadToken {
			return
		}
		if err != nil {
			if !p.offline(err, start, autoplay) {
				p.fail(err.Error())
			}
			return
		}
		p.loadStream(src, start, autoplay)
	})
}

// loadStream plays a stream in the engine. Its failures come back as its
// Error events.
func (p *Player) loadStream(src Source, start time.Duration, autoplay bool) {
	// A Spotify device left in the middle of its track pauses.
	p.connect.stop(true)
	p.s.Source = &src
	p.loaded = true
	p.paused = !autoplay
	_ = p.stream.Load(src.URL, start, autoplay)
}

func (p *Player) stopStream() {
	p.loaded = false
	p.stream.Stop()
}

// fellBack tells why a Spotify track plays from YouTube Music instead, once
// for each reason.
func (p *Player) fellBack(why string) {
	if p.warned[why] {
		return
	}
	if p.warned == nil {
		p.warned = map[string]bool{}
	}
	p.warned[why] = true
	p.notify(Notice{
		Message:  strings.TrimSuffix(why, ".") + " — playing it from YouTube Music instead",
		Error:    true,
		Settings: true,
	})
}

// fail tells that a track can't play, and moves on after a moment: to the
// next track, or to a radio when it was the last. After maxFailures in a
// row it stops, as nothing seems to play.
func (p *Player) fail(message string) {
	cur, ok := p.s.Current()
	p.askedAt = time.Time{}
	p.s.Error = message
	p.s.Buffering = false
	p.s.Loading = false
	p.s.Playing = false
	// The app last heard of the track as loading: it hears it stopped.
	p.report()
	p.emit(StateChange)
	p.failures++
	stop := p.failures >= maxFailures
	why := message
	if stop {
		why = fmt.Sprintf("%s. Stopped, as %d tracks in a row couldn't play", strings.TrimSuffix(message, "."), maxFailures)
	}
	if ok {
		why = fmt.Sprintf("Couldn't play %s: %s", cur.Track.Title, why)
	}
	p.notify(Notice{Message: why, Error: true})
	if stop {
		return
	}
	token := p.loadToken
	p.after(failDelay, func() {
		if token == p.loadToken {
			p.advance(false)
		}
	})
}

// onStreamError takes in a stream the engine can't play on, and tries it
// again where it stopped.
func (p *Player) onStreamError(message string) {
	if _, ok := p.s.Current(); !ok || !p.loaded {
		return // of a stream a load, or a try, took over from
	}
	// The engine tells that the stream can't play; the stream tells why, as
	// when YouTube refused it.
	why := ""
	if src := p.s.Source; src != nil && src.Mode == ModeStream {
		why = p.svc.StreamFailure(src.Track)
	}
	if why == "" {
		why = message
	}
	// The stream may have broken off for want of the network: the track
	// waits for it then, rather than spending its tries. It loads
	// meanwhile.
	autoplay := !p.paused
	p.loaded = false
	p.s.Loading, p.s.Buffering = autoplay, autoplay
	// The app heard of the track as paused, as the engine paused it.
	p.report()
	p.emit(StateChange)
	p.checkOnline(func() { p.wait(p.s.Position, autoplay) }, func() { p.retryStream(why) })
}

// retryStream plays the current track's stream again from where it was,
// after the next of retryWaits for a stream that played, or at once for
// one that never did; once its tries ran out, it fails with why the last
// one did. A try that finds no stream, as while the network is down,
// counts as one.
func (p *Player) retryStream(why string) {
	entry, _ := p.s.Current()
	r := &p.retry
	tries, wait := 1, time.Duration(0)
	if r.played {
		tries = len(retryWaits)
	}
	if r.n >= tries {
		p.fail(why)
		return
	}
	if r.played {
		wait = retryWaits[r.n]
	}
	r.n++
	token, at, autoplay := p.loadToken, p.s.Position, !p.paused
	r.from = at
	log.Printf("mixmusic: the audio of %s stopped at %s: %s; trying again in %s, %d of %d",
		entry.Track.ID, at.Round(time.Second), why, wait, r.n, tries)
	// Meanwhile the track loads again, as one chosen does: it can't be
	// paused, and a seek, Next or another track takes over.
	p.loaded = false
	p.s.Loading, p.s.Buffering = autoplay, autoplay
	p.emit(StateChange)
	p.after(wait, func() {
		if token != p.loadToken {
			return
		}
		await(p, func(ctx context.Context) (Source, error) { return p.svc.PrepareStream(ctx, entry.Track) }, func(src Source, err error) {
			if token != p.loadToken {
				return
			}
			if err != nil {
				if !p.offline(err, at, autoplay) {
					p.retryStream(err.Error())
				}
				return
			}
			// The engine lets go of the next track it preloaded, which loads
			// again once this stream holds: on a slow network, its download
			// would slow this one's.
			p.s.Source = &src
			p.loaded = true
			p.paused = !autoplay
			_ = p.stream.Load(src.URL+"?retry="+strconv.FormatInt(time.Now().UnixMilli(), 10), at, autoplay)
		})
	})
}

// onProgress counts the play, prepares the next track, and crossfades.
func (p *Player) onProgress() {
	entry, ok := p.s.Current()
	if !ok {
		return
	}
	pos := p.s.Position
	dur := p.s.Duration
	if dur == 0 {
		dur = ms(entry.Track.DurationMs)
	}
	if p.retry.n > 0 && p.loaded && pos >= p.retry.from+retryHold {
		// The stream tried again holds: a later break has all its tries, and
		// the next track, which the engine let go of, is prepared again.
		p.retry.n = 0
		p.prefetched = ""
	}
	if p.counted != entry.UID && pos >= min(countAfter, dur/2) {
		p.counted = entry.UID
		go p.svc.RecordPlay(entry.Track)
	}
	next, hasNext := p.s.UpNext()
	if hasNext && next.UID != entry.UID && p.prepared != next.UID && pos > prepareAfter {
		// Finding the next track's stream costs a request; downloading its
		// audio waits until this track has played a while, so that skipping
		// through a queue doesn't download every track.
		p.prepared = next.UID
		await(p, func(ctx context.Context) (Source, error) { return p.svc.Prepare(ctx, next.Track) }, func(Source, error) {})
	}
	if hasNext && next.UID != entry.UID && p.prefetched != next.UID && pos > prefetchAfter && p.retry.n == 0 {
		p.prefetched = next.UID
		await(p, func(ctx context.Context) (Source, error) { return p.svc.Prepare(ctx, next.Track) }, func(src Source, err error) {
			if err == nil && src.Mode == ModeStream && src.URL != "" {
				p.stream.Preload(src.URL)
			}
			if errors.Is(err, ErrOffline) {
				p.prefetchOnline(next.UID)
			}
		})
	}
	fade := time.Duration(p.svc.Settings().Crossfade) * time.Second
	if fade > 0 && !p.crossfading && p.s.Source != nil && p.s.Source.Mode == ModeStream &&
		hasNext && next.UID != entry.UID && dur > 0 && dur-pos <= fade && dur > 3*fade {
		p.crossfadeTo(next, fade)
	}
	p.reporting.call(p, func() {
		p.svc.Report(p.now())
		p.save()
	})
}

// crossfadeTo plays next while the current track fades out, over fade.
func (p *Player) crossfadeTo(next Item, fade time.Duration) {
	p.crossfading = true
	token := p.loadToken
	await(p, func(ctx context.Context) (Source, error) { return p.svc.Prepare(ctx, next.Track) }, func(src Source, err error) {
		p.after(fade, func() { p.crossfading = false })
		// A track the user chose meanwhile, or a Connect track, starts as
		// the current one ends.
		if err != nil || token != p.loadToken || src.Mode != ModeStream || src.URL == "" {
			return
		}
		i := p.indexOf(next.UID)
		if i < 0 {
			return
		}
		p.loadToken++
		p.retry = retryState{}
		p.endWait()
		p.markPlayed(next.UID)
		p.askedAt = time.Time{}
		p.s.Index = i
		p.s.Position = 0
		p.s.Duration = ms(next.Track.DurationMs)
		p.s.Source = &src
		p.s.Error = ""
		p.emit(TrackChange)
		p.report()
		p.loaded = true
		p.paused = false
		_ = p.stream.Crossfade(src.URL, fade)
	})
}

func (p *Player) play() {
	if p.s.Waiting {
		// It plays once the network answers, which is asked again now.
		p.s.WillPlay = true
		if n, ok := p.network(); ok {
			n.KickOnline()
		}
		p.emit(StateChange)
		return
	}
	if p.s.Loading {
		return // it plays once loaded
	}
	if _, ok := p.s.Current(); !ok {
		if i := p.s.nextIndex(); i >= 0 {
			p.playAt(i, 0, true)
		}
		return
	}
	if p.connect.active() {
		p.connect.resume()
		return
	}
	if !p.loaded || p.s.Error != "" {
		p.playAt(p.s.Index, p.s.Position, true)
		return
	}
	if p.atEnd {
		// The queue ran out at the end of this track: it plays again from
		// its start, where the engine, which ended it, goes once seeked.
		p.atEnd = false
		p.stream.Seek(0)
	}
	p.paused = false
	_ = p.stream.Play()
}

func (p *Player) pause() {
	if p.s.Waiting {
		p.s.WillPlay = false
		p.emit(StateChange)
		return
	}
	if p.s.Loading {
		return // a track loading plays: only another track stops it
	}
	if p.connect.active() {
		p.connect.pause()
		return
	}
	p.paused = true
	p.stream.Pause()
}

func (p *Player) toggle() {
	if p.s.Playing || p.s.Waiting && p.s.WillPlay {
		p.pause()
	} else {
		p.play()
	}
}

func (p *Player) seek(t time.Duration) {
	t = max(t, 0)
	if p.s.Duration > 0 {
		t = min(t, p.s.Duration)
	}
	if p.s.Waiting && !p.radioWaits {
		// The track plays from there once the network answers.
		p.s.Position = t
		p.emit(Progress)
		return
	}
	p.endWait()
	p.s.Position = t
	p.atEnd = false
	p.retry.from = t
	switch {
	case p.connect.active():
		p.connect.seek(t)
	case p.loaded:
		p.stream.Seek(t)
	default:
		if _, ok := p.s.Current(); ok {
			// A track loading goes on to play, from t.
			p.playAt(p.s.Index, t, p.s.Loading)
		}
	}
	p.emit(Progress)
}

// advance moves on from the current track: ended tells an end from a skip.
// It passes over the entries the queue skips.
func (p *Player) advance(ended bool) {
	if len(p.s.Queue) == 0 {
		return
	}
	switch i := p.s.nextIndex(); {
	case ended && p.s.Repeat == RepeatOne:
		p.playAt(p.s.Index, 0, true)
	case i >= 0:
		p.playAt(i, 0, true)
	default:
		p.radioAtEnd(true, ended, func() { p.stopAtEnd(ended) })
	}
}

// radioAtEnd has autoplay carry on at the end of the queue with a radio of
// the last entry it doesn't pass over, which plays, or waits paused unless
// play. Without autoplay, or a radio of tracks it doesn't pass over, stop
// runs. The radio waits for a network gone at the end of a track, as ended
// tells.
func (p *Player) radioAtEnd(play, ended bool, stop func()) {
	seed, ok := p.radioSeed()
	if !ok || !p.svc.Settings().Autoplay {
		stop()
		return
	}
	token := p.loadToken
	p.extendWithRadio(seed, func(added bool, err error) {
		switch i := p.s.nextIndex(); {
		case token != p.loadToken:
		case added && i > p.s.Index:
			p.playAt(i, 0, play)
		case !added && ended && p.radioOffline(err):
			p.waitRadio()
		default:
			stop()
		}
	})
}

// stopAtEnd stops at the end of the queue.
func (p *Player) stopAtEnd(ended bool) {
	if !ended {
		return
	}
	p.s.Playing = false
	p.s.Position = 0
	p.atEnd = true
	p.report()
	p.emit(StateChange)
}

// previous goes back to the start of the track, or to the entry before
// early in it, passing over those the queue skips.
func (p *Player) previous() {
	if p.s.Position <= restartAfter {
		if i := p.s.prevIndex(); i >= 0 {
			p.playAt(i, 0, true)
			return
		}
	}
	p.seek(0)
}

func (p *Player) setVolume(v float64) {
	if math.IsNaN(v) {
		return
	}
	v = min(max(v, 0), 1)
	p.s.Volume = v
	p.s.Muted = false
	p.apply()
	if p.volumeTimer != nil {
		p.volumeTimer.Stop()
	}
	p.volumeTimer = p.after(saveVolumeAfter, func() { p.saveVolume(v) })
}

// saveVolume keeps the volume v in the settings, unless they keep it.
func (p *Player) saveVolume(v float64) {
	if saved := volume.Saved(v); p.svc.Settings().Volume != saved {
		p.svc.SaveVolume(saved)
	}
}

// apply gives the engines the volume.
func (p *Player) apply() {
	v := p.s.Volume
	if p.s.Muted {
		v = 0
	}
	p.stream.SetVolume(v)
	p.connect.setVolume(v)
	p.report()
}

// now is what plays, for the app.
func (p *Player) now() Now {
	n := Now{
		Playing:  p.s.Playing,
		Loading:  p.s.Loading,
		Position: p.s.Position,
		Duration: p.s.Duration,
		Volume:   p.s.Volume,
		Shuffle:  p.s.Shuffle,
		Repeat:   p.s.Repeat,
	}
	if p.s.Muted {
		n.Volume = 0
	}
	if cur, ok := p.s.Current(); ok {
		n.Track = &cur.Track
	}
	return n
}

// report tells the app what plays, and keeps the queue.
func (p *Player) report() {
	p.svc.Report(p.now())
	p.save()
}

// save keeps the queue, every saveEvery at most, as the task publishes it.
func (p *Player) save() {
	p.saveAsked = true
	p.saving.call(p, func() { p.saveDue = true })
}

// writeQueue keeps the state last published, unless a write kept it, or a
// later one, already: a write started earlier never lands over it.
func (p *Player) writeQueue() {
	p.writing.Lock()
	defer p.writing.Unlock()
	p.mu.Lock()
	st, order, played, version := p.snap, p.snapOrder, p.snapPlayed, p.version
	p.mu.Unlock()
	if version <= p.written {
		return
	}
	p.svc.SaveQueue(savedOf(st, order, played))
	p.written = version
}

// Flush keeps the queue and the volume as the player last published them,
// before it returns, for the app to call as it quits, on the main thread.
// It writes the queue again only if the player published since the last
// write. It never waits on the player's goroutine, which may itself wait
// on the main thread, as the app's Report does for the tray: a flush that
// waited for it, or a call of Close in its place, would hang the quit.
func (p *Player) Flush() {
	p.writeQueue()
	p.mu.Lock()
	v := p.snap.Volume
	p.mu.Unlock()
	p.saveVolume(v)
}

// savedOf is the queue of st as it survives a restart: the part of it
// savedWindow keeps, with the current entry's index in that part, which of
// its entries the radio added, which before the current one aren't among
// those played, and their own order, while shuffled, from unshuffled.
func savedOf(st State, unshuffled []Item, played map[string]bool) Saved {
	from, to := savedWindow(len(st.Queue), st.Index)
	kept := st.Queue[from:to]
	saved := Saved{
		Tracks:   make([]media.Track, len(kept)),
		Index:    st.Index - from,
		Position: int(st.Position / time.Millisecond),
		Shuffle:  st.Shuffle,
		Repeat:   st.Repeat,
	}
	at := make(map[string]int, len(kept))
	for i, it := range kept {
		saved.Tracks[i] = it.Track
		at[it.UID] = i
		if it.Radio {
			saved.Radio = append(saved.Radio, i)
		}
		if it.Chosen {
			saved.Chosen = append(saved.Chosen, i)
		}
		if i < saved.Index && !played[it.UID] {
			saved.Unplayed = append(saved.Unplayed, i)
		}
	}
	for _, it := range unshuffled {
		if i, ok := at[it.UID]; ok {
			saved.Order = append(saved.Order, i)
		}
	}
	return saved
}

// The stream engine's events, as audio.Events.

// Time takes in the stream's position and duration.
func (p *Player) Time(position, duration time.Duration) {
	p.do(func() { p.onTime(position, duration) })
}

// Ended takes in the end of the stream.
func (p *Player) Ended() { p.do(func() { p.advance(true) }) }

// Error takes in a stream that failed.
func (p *Player) Error(message string) { p.do(func() { p.onStreamError(message) }) }

// Playing takes in the stream playing or pausing.
func (p *Player) Playing(playing bool) { p.do(func() { p.onPlaying(playing) }) }

// Buffering takes in the stream waiting for its audio.
func (p *Player) Buffering(buffering bool) {
	p.do(func() {
		p.s.Buffering = buffering
		p.emit(StateChange)
		p.watchStall(buffering)
	})
}

func (p *Player) onTime(position, duration time.Duration) {
	if position > p.s.Position {
		// The track plays on, which a Time alone doesn't tell: the engine
		// also tells the time while a stream opens, and just before it tells
		// that one failed.
		p.retry.played = true
		p.failures = 0
	}
	p.s.Position = position
	if duration > 0 {
		p.s.Duration = duration
	}
	// The engine's time moves once it sounds: the log tells how long a
	// stream took, when long, to tell why tracks start late. A time of the
	// track before comes before the engine has this one.
	if !p.askedAt.IsZero() && p.loaded && position > p.askedFrom {
		took := time.Since(p.askedAt)
		if src := p.s.Source; took > slowStart && src != nil && src.Mode == ModeStream {
			log.Printf("mixmusic: %s started %.1fs after it was asked to play", src.Track.ID, took.Seconds())
		}
		p.askedAt = time.Time{}
	}
	p.emit(Progress)
	p.onProgress()
}

func (p *Player) onPlaying(playing bool) {
	p.s.Playing = playing
	if playing {
		p.s.Buffering = false
		p.s.Loading = false
	}
	p.report()
	p.emit(StateChange)
}

func (p *Player) indexOf(uid string) int {
	for i, it := range p.s.Queue {
		if it.UID == uid {
			return i
		}
	}
	return -1
}

package player

// The sleep timer (#594): it stops the music after some minutes, at the
// end of the current track or of a few, or at the end of the queue,
// fading it out first. A timer of minutes runs on the monotonic clock.
// The others count the tracks that end where the queue moves on after a
// track (advance), and as one fades into the next, so that a track that
// repeats, or a song queued twice, ends once each time; the last of them
// plays to its real end, with no crossfade, and the fade falls inside its
// last seconds. At its end the timer pauses as the computer's sleep does,
// a track that loads too, and a Spotify Connect device elsewhere besides.
//
// The fade runs on the engine's gain of its own (Fader). A Spotify device
// and a cast speaker, which the engine doesn't play on, have their own
// volume stepped instead, at most once a second, and put back after the
// pause. The volume the user set, and the settings', never move. The
// computer going to sleep ends the timer.
//
// Bedtime starts a timer of the length chosen last as playback starts
// inside a window of the night the settings set, unless the user ended
// one inside that window.

import (
	"fmt"
	"math"
	"time"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// SleepMode is how a sleep timer stops the music.
type SleepMode string

const (
	// SleepMinutes stops it after some minutes.
	SleepMinutes SleepMode = "minutes"
	// SleepTracks stops it at the end of the current track, or of a few.
	SleepTracks SleepMode = "tracks"
	// SleepQueue stops it at the end of the queue: on repeat all, it
	// doesn't go round, and the radio autoplay adds after it doesn't start.
	SleepQueue SleepMode = "queue"
)

// MaxSleepTracks is the most tracks a timer of SleepTracks waits for.
const MaxSleepTracks = 20

// The minutes the sleep timer's menus offer: the lengths it starts at,
// and the minutes it adds to one that runs.
var (
	SleepLengths = []int{5, 10, 15, 30, 45, 60, 90}
	SleepAdds    = []int{5, 15}
)

// SleepTimer is a sleep timer to start.
type SleepTimer struct {
	Mode SleepMode
	// N is how many minutes a timer of SleepMinutes runs, from 1 to
	// settings.MaxSleepMinutes, or at the end of how many tracks a timer of
	// SleepTracks stops the music, from 1, the current one, to
	// MaxSleepTracks.
	N int
	// Assistant is set for a timer an AI assistant set, as its notice
	// tells.
	Assistant bool
}

// Sleep is the sleep timer that runs, as State tells it. The player never
// changes one in place.
type Sleep struct {
	Mode SleepMode
	// Ends is when a timer of minutes runs out, on the monotonic clock.
	Ends time.Time
	// Tracks counts the tracks still to end before a timer of SleepTracks
	// stops the music, the current one first.
	Tracks int
	// Finishing is set once a timer of minutes ran out while Let the track
	// finish was on: the music stops at the end of the track that plays.
	Finishing bool
	// Fading is set while the music fades out.
	Fading bool
}

// Ending reports whether the timer stops the music at the end of a track:
// a timer of tracks or of the queue, or one of minutes that ran out while
// Let the track finish was on.
func (s *Sleep) Ending() bool { return s != nil && (s.Mode != SleepMinutes || s.Finishing) }

// Fader is a Stream that fades out by itself, apart from its volume, as
// the engine does: the player steps the volume of one that doesn't, at
// most once a second.
type Fader interface {
	// FadeAway fades the stream out to silence over d, and holds it so
	// until FadeBack brings it back over d.
	FadeAway(d time.Duration)
	FadeBack(d time.Duration)
}

// sleepKeeper is Services that keep the length of the timer of minutes
// started last, as the settings' SleepTimer.Minutes.
type sleepKeeper interface {
	KeepSleepMinutes(minutes int)
}

// sleepSecond is the second of the sleep timer: of its minutes, its fade,
// its rest and its steps. Tests shorten it.
var sleepSecond = time.Second

// sleepRest is the silence, at least, between the fade's end and the
// pause.
func sleepRest() time.Duration { return sleepSecond / 2 }

// sleepRun is how long the music may stop, as one track ends and the next
// loads, and still play on: once it stopped longer, a play starts playback
// again, which bedtime takes in.
func sleepRun() time.Duration { return 10 * sleepSecond }

// sleepState is what the player keeps of the sleep timer, on its
// goroutine, beside State.Sleep.
type sleepState struct {
	// gen counts the timers started and ended: a check of one that ended
	// does nothing. check is the next check of a timer of minutes.
	gen   int
	check *time.Timer
	// fadeAt is when the fade under way began, zero while none does, and
	// fadeFor how long it lasts. fadeGen counts the fades, for their steps,
	// and step is the next step of the devices' volume.
	fadeAt  time.Time
	fadeFor time.Duration
	fadeGen int
	step    *time.Timer
	// offUntil holds bedtime off until the end of the window the user ended
	// a timer in.
	offUntil time.Time
	// heardAt is when the music last played on, as its time moved.
	heardAt time.Time
}

// StartSleep starts the sleep timer t, in place of the one that runs, if
// any: a fade under way comes back, and the music plays on. A timer of
// minutes the user started keeps its length as the one chosen last.
func (p *Player) StartSleep(t SleepTimer) { p.do(func() { p.startSleep(t, false) }) }

// AddSleep has the sleep timer that runs stop the music minutes later: a
// timer that stops it at the end of a track runs on from the time it had
// left, as one of minutes. A fade under way comes back. It tells the user
// as assistant asks.
func (p *Player) AddSleep(minutes int, assistant bool) {
	p.do(func() {
		sl := p.s.Sleep
		if sl == nil || minutes <= 0 {
			return
		}
		now := time.Now()
		ends := sl.Ends
		if sl.Ending() {
			left, _ := p.s.SleepLeft(now)
			ends = now.Add(left)
		}
		ends = ends.Add(time.Duration(minutes) * 60 * sleepSecond)
		if limit := now.Add(settings.MaxSleepMinutes * 60 * sleepSecond); ends.After(limit) {
			ends = limit
		}
		p.fadeBack(sleepSecond)
		p.setSleep(&Sleep{Mode: SleepMinutes, Ends: ends})
		p.checkSleep()
		p.tellSleep(locales.SleepInMinutes(minutesLeft(ends.Sub(now))), assistant)
	})
}

// CancelSleep ends the sleep timer that runs, if any: a fade under way
// comes back. Inside a window of bedtime, bedtime starts none until the
// window ends. It tells the user as assistant asks.
func (p *Player) CancelSleep(assistant bool) {
	p.do(func() {
		if p.s.Sleep == nil {
			return
		}
		if end, in := p.svc.Settings().SleepTimer.Window(time.Now()); in {
			p.sleep.offUntil = end
		}
		p.endSleep(false)
		msg := locales.SleepTimerOff()
		if assistant {
			msg = locales.AssistantsToastSleepOff()
		}
		p.notify(Notice{Message: msg})
	})
}

// minutesLeft is d in whole minutes, rounded up, as the notices tell how
// long a timer runs.
func minutesLeft(d time.Duration) int {
	m := sleepSecond * 60
	return int(max((d+m-1)/m, 1))
}

// startSleep starts the timer t, or a timer of the length chosen last at
// bedtime.
func (p *Player) startSleep(t SleepTimer, bedtime bool) {
	sl := &Sleep{Mode: t.Mode}
	var what string
	switch t.Mode {
	case SleepMinutes:
		n := min(max(t.N, 1), settings.MaxSleepMinutes)
		sl.Ends = time.Now().Add(time.Duration(n) * 60 * sleepSecond)
		what = locales.SleepInMinutes(n)
		if k, ok := p.svc.(sleepKeeper); ok && !bedtime && !t.Assistant {
			go k.KeepSleepMinutes(n)
		}
	case SleepTracks:
		sl.Tracks = min(max(t.N, 1), MaxSleepTracks)
		what = locales.SleepAtTrackEnd()
		if sl.Tracks > 1 {
			what = locales.SleepAfterTracks(sl.Tracks)
		}
	case SleepQueue:
		what = locales.SleepAtQueueEnd()
	default:
		return
	}
	p.fadeBack(sleepSecond)
	p.setSleep(sl)
	p.checkSleep()
	if bedtime {
		p.notify(Notice{Message: locales.SleepTimerBedtime(what)})
		return
	}
	p.tellSleep(what, t.Assistant)
}

// tellSleep tells that the timer runs, and what it does.
func (p *Player) tellSleep(what string, assistant bool) {
	msg := locales.SleepTimerOn(what)
	if assistant {
		msg = locales.AssistantsToastSleep(what)
	}
	p.notify(Notice{Message: msg})
}

// setSleep has sl run as the sleep timer, nil for none, in place of the
// one that ran: the checks of that one stop.
func (p *Player) setSleep(sl *Sleep) {
	p.sleep.gen++
	if p.sleep.check != nil {
		p.sleep.check.Stop()
		p.sleep.check = nil
	}
	p.s.Sleep = sl
	p.emit(StateChange)
}

// changeSleep has the timer that runs changed by fn, in a copy.
func (p *Player) changeSleep(fn func(*Sleep)) {
	sl := *p.s.Sleep
	fn(&sl)
	p.s.Sleep = &sl
	p.emit(StateChange)
}

// endSleep ends the timer. The fade under way comes back: at once, once
// the music paused for a moment, when paused, else over a second.
func (p *Player) endSleep(paused bool) {
	p.setSleep(nil)
	if !paused {
		p.fadeBack(sleepSecond)
		return
	}
	if p.sleep.fadeAt.IsZero() {
		return
	}
	// The steps stop, and the engine's pause, which glides, ends first.
	p.stopSteps()
	gen := p.sleep.fadeGen
	p.after(sleepSecond, func() {
		if gen == p.sleep.fadeGen {
			p.fadeBack(0)
		}
	})
}

// checkSleep has a timer of minutes start its fade, and stop the music as
// it runs out, and checks it again when one of them is due.
func (p *Player) checkSleep() {
	if p.sleep.check != nil {
		p.sleep.check.Stop()
		p.sleep.check = nil
	}
	sl := p.s.Sleep
	if sl == nil || sl.Ending() {
		return
	}
	left := time.Until(sl.Ends)
	if left <= 0 {
		p.sleepRanOut()
		return
	}
	wait := left
	if fade, rest := p.sleepFade(), sleepRest(); fade > 0 && !sl.Fading && p.fadesFirst() && p.sleepPlaying() {
		switch at := left - fade - rest; {
		case at > 0:
			wait = at
		case left > rest:
			p.fadeOut(left - rest)
		}
	}
	gen := p.sleep.gen
	p.sleep.check = p.after(wait, func() {
		if gen == p.sleep.gen {
			p.checkSleep()
		}
	})
}

// fadesFirst reports whether a timer of minutes fades before it runs out:
// unless the track that plays then finishes, in whose last seconds it
// fades. A live stream never ends: it fades first.
func (p *Player) fadesFirst() bool {
	return !p.svc.Settings().SleepTimer.Finish || p.s.Live
}

// sleepFade is how long the settings' fade lasts.
func (p *Player) sleepFade() time.Duration {
	return time.Duration(p.svc.Settings().SleepTimer.Fade) * sleepSecond
}

// sleepPlaying reports whether the music plays, or loads or waits to.
func (p *Player) sleepPlaying() bool {
	return p.s.Playing || p.s.Loading || p.s.Waiting && p.s.WillPlay
}

// sleepRanOut takes in a timer of minutes running out: the music stops,
// or with Let the track finish on, plays on to the end of its track. With
// nothing playing, the timer ends.
func (p *Player) sleepRanOut() {
	_, has := p.s.Current()
	switch {
	case !p.sleepPlaying():
		p.endSleep(false)
		p.notify(Notice{Message: locales.SleepTimerEnded()})
	case has && !p.fadesFirst():
		p.changeSleep(func(sl *Sleep) { sl.Finishing = true })
		p.notify(Notice{Message: locales.SleepTimerFinishing()})
		p.sleepProgress()
	default:
		p.sleepStop()
	}
}

// sleepStop pauses the music, as the computer's sleep does: a track that
// loads stays paused once loaded, and one that waits for the network
// doesn't play once it's back. A Spotify Connect device elsewhere pauses
// too. The timer ends.
func (p *Player) sleepStop() {
	switch {
	case p.s.Waiting:
		p.s.WillPlay = false
		p.emit(StateChange)
	case p.s.Loading:
		p.playAt(p.s.Index, p.s.Position, false)
	case p.s.Playing:
		p.pause()
	}
	p.endSleep(true)
	p.notify(Notice{Message: locales.SleepTimerPaused()})
}

// sleepLast reports whether the current track is the last the timer lets
// play: it stops the music at its end.
func (p *Player) sleepLast() bool {
	sl := p.s.Sleep
	switch {
	case !sl.Ending():
		return false
	case sl.Finishing:
		return true
	case sl.Mode == SleepTracks:
		return sl.Tracks <= 1
	}
	return p.s.Repeat == RepeatOne || p.queueEndsHere()
}

// queueEndsHere reports whether the queue ends with the current entry, as
// the sleep timer takes it: no entry after it plays but those of the radio
// autoplay added after it, unless that radio plays already.
func (p *Player) queueEndsHere() bool {
	cur, ok := p.s.Current()
	if !ok {
		return true
	}
	for _, it := range p.s.Queue[p.s.Index+1:] {
		if it.Skip == "" && (!it.Radio || cur.Radio) {
			return false
		}
	}
	return true
}

// sleepEnded takes in the current track ending, as the queue moves on
// after it, and tells whether the timer stopped the music there: at the
// end of the last track it lets play. Another counts toward a timer of
// tracks.
func (p *Player) sleepEnded() bool {
	if !p.s.Sleep.Ending() {
		return false
	}
	if !p.sleepLast() {
		p.sleepPassed()
		return false
	}
	p.sleepAtEnd()
	return true
}

// sleepPassed counts a track that ended, before the last, toward a timer
// of tracks: as the queue moved on after it, or faded into the next.
func (p *Player) sleepPassed() {
	if sl := p.s.Sleep; sl != nil && sl.Mode == SleepTracks && !sl.Finishing && sl.Tracks > 1 {
		p.changeSleep(func(sl *Sleep) { sl.Tracks-- })
	}
}

// sleepAtEnd stops the music at the end of the track that ended: the next
// entry waits at its start, paused, or at the end of the queue, and on
// repeat one, the queue stops as at its end. What a Spotify device went on
// to by itself stops there, as does what Spotify plays by itself after it.
func (p *Player) sleepAtEnd() {
	p.connect.take("")
	if d := p.connect.device; d != nil && p.onDevice() {
		p.connect.send(silencer(d))
	}
	if p.s.Sleep.Mode != SleepQueue && p.s.Repeat != RepeatOne && p.s.nextIndex() >= 0 {
		m, at := p.moveOn(true)
		m.paused = true
		p.step(m, at+1)
	} else {
		p.stopAtEnd(true)
	}
	p.endSleep(true)
	p.notify(Notice{Message: locales.SleepTimerPaused()})
}

// sleepProgress takes in the time of the track that plays: the fade of a
// timer that stops at its end starts in its last seconds, and comes back
// when the track was moved back from there.
func (p *Player) sleepProgress() {
	if p.s.Playing {
		p.sleep.heardAt = time.Now()
	}
	sl := p.s.Sleep
	if !p.sleepLast() || p.s.Live {
		return
	}
	fade, rest, dur := p.sleepFade(), sleepRest(), p.s.Duration
	if fade <= 0 || dur <= 0 {
		return
	}
	left := p.heard(p.endOf(dur) - p.s.Position)
	switch {
	case sl.Fading && left > fade+rest+sleepSecond:
		p.fadeBack(sleepSecond)
		p.changeSleep(func(sl *Sleep) { sl.Fading = false })
	case !sl.Fading && p.s.Playing && left > rest && left <= fade+rest:
		p.fadeOut(left - rest)
	}
}

// sleepSounded takes in the music playing: the volume a fade left is put
// back, a timer of minutes due to fade fades, as it doesn't while nothing
// plays, and playback that starts, rather than plays on, inside a window
// of bedtime starts the timer, at the length chosen last.
func (p *Player) sleepSounded() {
	switch sl := p.s.Sleep; {
	case sl == nil:
		p.fadeBack(0)
	case !sl.Fading:
		p.checkSleep()
	}
	now := time.Now()
	started := p.sleep.heardAt.IsZero() || now.Sub(p.sleep.heardAt) > sleepRun()
	p.sleep.heardAt = now
	if !started || p.s.Sleep != nil {
		return
	}
	st := p.svc.Settings().SleepTimer
	if _, in := st.Window(now); in && !now.Before(p.sleep.offUntil) {
		p.startSleep(SleepTimer{Mode: SleepMinutes, N: st.Minutes}, true)
	}
}

// sleepAsleep ends the timer as the computer goes to sleep.
func (p *Player) sleepAsleep() {
	if p.s.Sleep != nil {
		p.endSleep(false)
		p.notify(Notice{Message: locales.SleepTimerOffAsleep()})
	}
}

// fadeOut starts the fade, over d: of the engine's gain, and of the
// volume of a device or a cast speaker, step by step.
func (p *Player) fadeOut(d time.Duration) {
	p.changeSleep(func(sl *Sleep) { sl.Fading = true })
	p.stopSteps()
	p.sleep.fadeAt, p.sleep.fadeFor = time.Now(), d
	if f, ok := p.stream.(Fader); ok && p.out.Stream == nil {
		f.FadeAway(d)
	}
	p.stepFade(p.sleep.fadeGen)
}

// stopSteps stops the steps of the fade under way.
func (p *Player) stopSteps() {
	p.sleep.fadeGen++
	if p.sleep.step != nil {
		p.sleep.step.Stop()
		p.sleep.step = nil
	}
}

// stepFade steps the volume of the Spotify device that plays, and of a
// cast speaker, along the fade, at most once a second. Neither is kept.
func (p *Player) stepFade(gen int) {
	if gen != p.sleep.fadeGen || p.sleep.fadeAt.IsZero() {
		return
	}
	at := time.Since(p.sleep.fadeAt)
	if !p.s.Muted {
		g := fadeGain(at, p.sleep.fadeFor)
		p.connect.setVolume(p.s.Volume * g)
		if p.out.Stream != nil {
			p.stream.SetVolume(p.s.Volume * g)
		}
	}
	if at < p.sleep.fadeFor {
		p.sleep.step = p.after(min(sleepSecond, p.sleep.fadeFor-at), func() { p.stepFade(gen) })
	}
}

// fadeGain is the gain of a fade over d, at in it: on an equal-power
// curve, from 1 to 0.
func fadeGain(at, d time.Duration) float64 {
	if d <= 0 || at >= d {
		return 0
	}
	return math.Cos(math.Pi / 2 * float64(at) / float64(d))
}

// fadeBack ends the fade, if any: the engine comes back over d, and the
// devices to the volume at once.
func (p *Player) fadeBack(d time.Duration) {
	if p.sleep.fadeAt.IsZero() {
		return
	}
	p.sleep.fadeAt = time.Time{}
	p.stopSteps()
	if f, ok := p.fader(); ok {
		f.FadeBack(d)
	}
	p.connect.setVolume(p.heardVolume())
	if p.out.Stream != nil {
		p.setStreamVolume()
	}
}

// fader is the computer's engine, as a Fader, while the streams play on a
// remote output too.
func (p *Player) fader() (Fader, bool) {
	s := p.stream
	if p.out.Stream != nil {
		s = p.engine
	}
	f, ok := s.(Fader)
	return f, ok
}

// SleepLine says what the sleep timer that runs does, as the first line
// of its menus: how long a timer of minutes has left from now, or at the
// end of what it stops the music. It is "" while none runs.
func (s State) SleepLine(now time.Time) string {
	sl := s.Sleep
	switch {
	case sl == nil:
		return ""
	case !sl.Ending():
		left, _ := s.SleepLeft(now)
		return locales.SleepStopsIn(SleepClock(left))
	case sl.Finishing || sl.Mode == SleepTracks && sl.Tracks <= 1:
		return locales.SleepStopsAtTrack()
	case sl.Mode == SleepTracks:
		return locales.SleepStopsAfterTracks(sl.Tracks)
	}
	return locales.SleepStopsAtQueue()
}

// SleepClock writes how long the sleep timer has left, to the second,
// rounded up: as 0:59, 12:34 or 1:02:03.
func SleepClock(d time.Duration) string {
	total := int(max((d+time.Second-1)/time.Second, 0))
	h, m, sec := total/3600, total/60%60, total%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, sec)
	}
	return fmt.Sprintf("%d:%02d", m, sec)
}

// SleepLeft is how long the sleep timer runs on from now, as heard: until
// a timer of minutes runs out; for one that stops the music at the end of
// a track, what is left of the current track and of those that play
// before it stops, as far as their lengths are known. It tells false
// while none runs.
func (s State) SleepLeft(now time.Time) (time.Duration, bool) {
	sl := s.Sleep
	if sl == nil {
		return 0, false
	}
	if !sl.Ending() {
		return max(sl.Ends.Sub(now), 0), true
	}
	cur, ok := s.Current()
	if !ok {
		return 0, true
	}
	left := max(s.Duration-s.Position, 0)
	switch {
	case sl.Finishing:
	case s.Repeat == RepeatOne && sl.Mode == SleepTracks:
		left += time.Duration(sl.Tracks-1) * ms(cur.Track.DurationMs)
	case sl.Mode == SleepTracks:
		for i, n := s.Index, 1; n < sl.Tracks; n++ {
			if i = s.nextFrom(i); i < 0 {
				break
			}
			left += ms(s.Queue[i].Track.DurationMs)
		}
	case s.Repeat != RepeatOne:
		for _, it := range s.Queue[s.Index+1:] {
			if it.Skip == "" && (!it.Radio || cur.Radio) {
				left += ms(it.Track.DurationMs)
			}
		}
	}
	if s.Speed > 0 {
		left = time.Duration(float64(left) / s.Speed)
	}
	return left, true
}

// nextFrom is the index of the entry the queue moves on to from the one at
// i, past those it passes over and round on repeat all, or -1.
func (s State) nextFrom(i int) int {
	for k := 1; k <= len(s.Queue); k++ {
		j := i + k
		if j >= len(s.Queue) {
			if s.Repeat != RepeatAll {
				return -1
			}
			j -= len(s.Queue)
		}
		if s.Queue[j].Skip == "" {
			return j
		}
	}
	return -1
}

package app

// A guest of a room plays what the host plays, in step (internal/room's
// Follower): each track the host plays, from where it is, and as it plays,
// pauses and seeks. The guest's player holds the host's track alone, as
// one the user chose, so that it moves on as the host does, and not by
// itself. While it follows, the guest's crossfade, autoplay and speeds
// rest, and its queue isn't kept: the queue.json of before the room stays,
// and the user's own queue comes back as the guest leaves. Its volume,
// output device and equalizer stay its own.

import (
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/room"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// cantPlayHere is what a guest shows of a track of the host it has no way
// to play: a file of the host's it has none of.
const cantPlayHere = "Can't play here"

// follower follows the host of a room in the guest's player.
type follower struct {
	a *App

	mu sync.Mutex
	// entry is the host's entry the guest's player holds, of the track
	// trackID here, and ready the track change to tell the host it has
	// ready, once loaded. note tells why the host's track doesn't play
	// here, while it doesn't.
	entry   string
	trackID string
	ready   int64
	note    string
	keep    room.Follower
	stopCh  chan struct{}
	stopped bool
}

func newFollower(a *App) *follower { return &follower{a: a} }

// noteNow tells why the host's track doesn't play here, or "".
func (f *follower) noteNow() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.note
}

// follow follows the host in the session s, until it ends or stop: one
// session at a time.
func (f *follower) follow(s *room.Session) {
	f.mu.Lock()
	if f.stopCh != nil {
		close(f.stopCh)
	}
	stop := make(chan struct{})
	f.stopCh, f.stopped = stop, false
	// The host's track loads again after a drop.
	f.entry, f.ready = "", 0
	f.mu.Unlock()
	f.a.followHost(true)
	go f.run(s, stop)
}

// stop stops following: the guest's settings hold again.
func (f *follower) stop() {
	f.halt()
	f.a.followHost(false)
}

// halt stops following the host, and leaves the player as it is.
func (f *follower) halt() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopCh != nil && !f.stopped {
		close(f.stopCh)
		f.stopped = true
	}
}

// followHost has the player follow a host, or no longer: its settings'
// crossfade, autoplay, speeds and skips rest meanwhile (roomSettings).
func (a *App) followHost(on bool) {
	if a.room.following.Swap(on) == on {
		return
	}
	if p := a.goPlayer.Load(); p != nil {
		p.SettingsChanged(playerServices{a}.Settings())
		if !on {
			// The speed of the track that plays is the settings' again.
			a.roomSetSpeed(p.State().Speed)
		}
	}
}

// roomSettings are the settings s as the player of a guest that follows a
// host takes them: the host decides when tracks change, how fast they
// play, and which parts of them it passes over.
func (a *App) roomSettings(s settings.Settings) settings.Settings {
	if !a.room.following.Load() {
		return s
	}
	s.Crossfade, s.Autoplay = 0, false
	s.SongSpeed, s.PodcastSpeed = 1, 1
	s.SkipDisliked, s.SkipAI = false, false
	// The host's player skips the parts its settings skip, which a seek
	// tells the guests (#548).
	s.SkipParts.On = false
	return s
}

// keepsQueue reports whether the player's queue is kept: not while it
// follows a host, whose queue isn't the user's.
func (a *App) keepsQueue() bool { return !a.room.following.Load() }

// roomSetSpeed plays the engine's track at speed.
func (a *App) roomSetSpeed(speed float64) {
	if o := a.opts.Room; o != nil && o.SetSpeed != nil {
		o.SetSpeed(speed)
		return
	}
	if e := a.nativeEngine(); e != nil {
		e.SetSpeed(speed)
	}
}

func (f *follower) run(s *room.Session, stop chan struct{}) {
	a := f.a
	p := a.goPlayer.Load()
	if p == nil {
		return
	}
	events := make(chan room.Event, 64)
	cancel := s.Watch(func(e room.Event) {
		select {
		case events <- e:
		default:
		}
	})
	defer cancel()
	poke := make(chan struct{}, 1)
	unsub := p.Subscribe(func(player.Event) {
		select {
		case poke <- struct{}{}:
		default:
		}
	})
	defer unsub()
	tick := time.NewTicker(room.FollowEvery)
	defer tick.Stop()
	due := time.NewTimer(time.Hour)
	due.Stop()
	defer due.Stop()
	f.align(s, p)
	for {
		select {
		case <-stop:
			return
		case <-s.Done():
			return
		case e := <-events:
			switch e.Kind {
			case room.Prepare:
				f.prepare(s, p, e)
			case room.StateChanged:
				f.align(s, p)
				f.step(s, p, due)
			}
		case <-poke:
			f.tellReady(s, p)
		case <-tick.C:
			f.tellReady(s, p)
			f.step(s, p, due)
		case <-due.C:
			f.step(s, p, due)
		}
	}
}

// trackOf is the host's track q as this computer plays it: a track of a
// service by its ID, a file of the host's as the same song of this
// computer's files, if it has one.
func (a *App) roomTrack(q remote.QueueTrack) (media.Track, bool) {
	var mine *localSongs
	return trackOfHanded(q, func(q remote.QueueTrack) (media.Track, bool) {
		if mine == nil {
			mine = a.localSongs()
		}
		return mine.find(q)
	})
}

// take has the player hold the host's track q of entry, paused at at: it
// reports false, with why, for a track this computer can't play, or its
// settings hide.
func (f *follower) take(p *player.Player, entry string, q remote.QueueTrack, at time.Duration) bool {
	a := f.a
	t, ok := a.roomTrack(q)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entry, f.trackID, f.note, f.ready = entry, "", "", 0
	f.keep.Reset()
	switch {
	case !ok:
		f.note = cantPlayHere
	case a.settings.Get().HideExplicit && t.ExplicitSong():
		f.note = t.Title + " is explicit, and your settings hide explicit content: it stays silent here"
	}
	if f.note != "" {
		p.Pause()
		return false
	}
	f.trackID = t.ID
	p.TakeOver([]media.Track{t}, 0, at, false, false, player.RepeatOff)
	return true
}

// prepare readies the host's next track, and tells the host once it's
// ready, or at once when it can't play here.
func (f *follower) prepare(s *room.Session, p *player.Player, e room.Event) {
	if !f.take(p, e.Entry, e.Track, 0) {
		_ = s.Ready(e.Seq)
		return
	}
	f.mu.Lock()
	f.ready = e.Seq
	f.mu.Unlock()
	f.tellReady(s, p)
}

// tellReady tells the host that the track of the change it readies is
// ready, once the player loaded it, or couldn't.
func (f *follower) tellReady(s *room.Session, p *player.Player) {
	f.mu.Lock()
	seq, id := f.ready, f.trackID
	f.mu.Unlock()
	if seq == 0 {
		return
	}
	st := p.State()
	cur, ok := st.Current()
	if !ok || cur.Track.ID != id || st.Loading || st.Source == nil && st.Error == "" {
		return
	}
	f.mu.Lock()
	if f.ready == seq {
		f.ready = 0
	}
	f.mu.Unlock()
	_ = s.Ready(seq)
}

// align has the player hold what the host plays, as its state tells: the
// track it plays from where it is, when the host played it without a
// track change, as when the guest joined, and its volume, while the
// settings follow it.
func (f *follower) align(s *room.Session, p *player.Player) {
	st, ok := s.State()
	if !ok {
		return
	}
	if f.a.settings.Get().RoomFollowVolume {
		if v := p.State().Volume; v-st.Volume > 0.005 || st.Volume-v > 0.005 {
			p.SetVolume(st.Volume)
		}
	}
	f.mu.Lock()
	same := st.Entry == f.entry
	f.mu.Unlock()
	if same {
		return
	}
	if st.Entry == "" || st.Track == nil {
		f.mu.Lock()
		f.entry, f.trackID, f.note, f.ready = "", "", "", 0
		f.mu.Unlock()
		p.Pause()
		return
	}
	at := time.Duration(0)
	if off, ok := s.Offset(); ok && !st.Live {
		at = st.Playhead().PositionAt(s.Now() + off)
	}
	f.take(p, st.Entry, *st.Track, at)
}

// step compares the guest's track with the host's, and corrects it; and
// has due fire at the moment a start is due.
func (f *follower) step(s *room.Session, p *player.Player, due *time.Timer) {
	st, ok := s.State()
	off, has := s.Offset()
	if !ok || !has {
		return
	}
	ps := p.State()
	f.mu.Lock()
	entry, id := f.entry, f.trackID
	f.mu.Unlock()
	head := st.Playhead()
	cur, hasCur := ps.Current()
	local := room.Local{}
	if hasCur && entry == st.Entry && id != "" && cur.Track.ID == id && ps.Source != nil && !ps.Loading && ps.Error == "" {
		local = room.Local{Has: true, Playing: ps.Playing, Position: f.a.heardNow(ps),
			OnDevice: ps.Source.Mode != player.ModeStream || ps.Remote}
	}
	if st.Live && local.Has {
		// A live stream can't seek: it plays and pauses as the host's does.
		switch {
		case head.Playing && !ps.Playing:
			p.Play()
		case !head.Playing && ps.Playing:
			p.Pause()
		}
		return
	}
	now := s.Now()
	f.mu.Lock()
	c := f.keep.Step(now, off, head, local)
	f.mu.Unlock()
	if c.Seek {
		p.Seek(c.To)
	}
	switch {
	case c.Pause:
		p.Pause()
	case c.Play:
		p.Play()
	}
	if c.Speed != 0 && local.Has && !local.OnDevice {
		f.a.roomSetSpeed(c.Speed)
	}
	if wait, ok := room.Due(now, off, head); ok {
		due.Reset(wait)
	}
}

// ownQueue is the user's own queue, as it was as they joined a room.
type ownQueue struct {
	tracks   []media.Track
	index    int
	position time.Duration
	shuffle  bool
	repeat   player.Repeat
}

// keepOwnQueue keeps the user's own queue, to give it back as they leave,
// and has queue.json keep it as it is now: it keeps no other while the
// player follows the host, so that the user's comes back at the next start
// too, if MixMusic quits in the room.
func (a *App) keepOwnQueue() *ownQueue {
	p := a.goPlayer.Load()
	if p == nil {
		return &ownQueue{}
	}
	p.Flush()
	st := p.State()
	o := &ownQueue{index: max(st.Index, 0), position: st.Position, shuffle: st.Shuffle, repeat: st.Repeat}
	for _, it := range st.Queue {
		o.tracks = append(o.tracks, it.Track)
	}
	return o
}

// giveOwnQueueBack gives the user their own queue back, paused where it
// was: a user who had none keeps the room's last track, paused.
func (a *App) giveOwnQueueBack(o *ownQueue) {
	p := a.goPlayer.Load()
	if p == nil {
		return
	}
	if len(o.tracks) == 0 {
		p.Pause()
		return
	}
	p.TakeOver(o.tracks, min(o.index, len(o.tracks)-1), o.position, false, o.shuffle, o.repeat)
}

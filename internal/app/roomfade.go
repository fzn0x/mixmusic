package app

// A crossfade on the host of a room (#734): the next track waits for the
// guests, as it does without crossfade (roomStream's Load), rather than
// start at once with the guests catching up during the fade. The track
// that ends plays on meanwhile, half the fade at most, as it ends within
// the fade: the next one loads on the engine's other deck, the guests ready
// it, and the fade starts at the moment they all start it.

import (
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/room"
)

// heldFade is the crossfade into the track of id, from from, whose start,
// seq of the held starts, waits for the guests.
type heldFade struct {
	mu   sync.Mutex
	seq  int
	id   string
	from time.Duration
}

func (s roomStream) Crossfade(url string, d time.Duration, cue audio.Cue) error {
	a := s.a
	held := &a.room.held
	held.release(false)
	h := a.hostedRoom()
	if cue.Track == nil || h == nil || h.server.Listeners() == 0 {
		return s.Stream.Crossfade(url, d, cue)
	}
	s.Stream.Preload(url)
	entry := a.roomEntry(cue.Track.ID)
	asked := time.Now()
	seq := held.hold(func() error { return s.Stream.Crossfade(url, max(d-time.Since(asked), d/4), cue) })
	f := &a.room.fade
	f.mu.Lock()
	f.seq, f.id, f.from = seq, cue.Track.ID, cue.Start
	f.mu.Unlock()
	clock := a.roomClock()
	h.server.PrepareFade(entry, queueTrackOf(*cue.Track), cue.Start, d, func(at time.Duration) {
		time.AfterFunc(max(at-clock.Now(), 0), func() { held.start(seq) })
	})
	return nil
}

// roomEntry names the entry of the player's of the track of id that
// starts: the one that plays, or the next, as the player tells its state
// once it started it; or "".
func (a *App) roomEntry(id string) string {
	p := a.goPlayer.Load()
	if p == nil {
		return ""
	}
	st := p.State()
	if cur, ok := st.Current(); ok && cur.Track.ID == id {
		return cur.UID
	}
	if next, ok := st.UpNext(); ok && next.Track.ID == id {
		return next.UID
	}
	return ""
}

// waits reports whether the start seq waits for the guests yet.
func (h *heldStart) waits(seq int) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.seq == seq && h.play != nil
}

// fadeHeld has st, the host's state, tell the next track waits, at its
// start, while a crossfade into it waits for the guests: the player plays
// it already, but the engine the track that ends.
func (a *App) fadeHeld(st *room.State) {
	f := &a.room.fade
	f.mu.Lock()
	seq, id, from := f.seq, f.id, f.from
	f.mu.Unlock()
	if seq == 0 || st.Track == nil || st.Track.ID != id || !a.room.held.waits(seq) {
		return
	}
	st.Loading, st.Position = true, from
}

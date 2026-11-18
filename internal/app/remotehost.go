package app

// The computer controlled's side of remote control: who may control it,
// the code it shows while another pairs with it, and its player, which the
// server tells of and runs the commands on.

import (
	"context"
	"errors"
	"log"
	"slices"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/locales"
)

// RemoteCode is the code this computer shows while another pairs with it:
// the other's name, as it told it, and the code.
type RemoteCode struct {
	Name, Code string
	// Until is when the code no longer holds.
	Until  time.Time
	cancel func()
}

// Code returns the code this computer shows while another pairs with it,
// and whether one pairs.
func (rc *Remote) Code() (RemoteCode, bool) {
	r := &rc.a.remote
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.code == nil {
		return RemoteCode{}, false
	}
	return *r.code, true
}

// CancelCode ends the pairing whose code shows: the user refused it.
func (rc *Remote) CancelCode() {
	r := &rc.a.remote
	r.mu.Lock()
	code := r.code
	r.mu.Unlock()
	if code != nil {
		code.cancel()
	}
}

// remoteHost is the app as the server's remote.Host.
type remoteHost struct{ a *App }

// Allowed reports whether the computer of fp may control this one: one
// paired as such, while remote control is on.
func (h remoteHost) Allowed(fp remote.Fingerprint) bool {
	kept := h.a.remoteKept()
	if !kept.On {
		return false
	}
	for _, c := range kept.Computers {
		if c.Key == fp.String() && c.ControlsThis {
			return true
		}
	}
	return false
}

// Pairing shows the code of req, one pairing at a time, while remote
// control is on.
func (h remoteHost) Pairing(req remote.PairRequest) (remote.PairEnd, error) {
	a := h.a
	if !a.remoteKept().On {
		return remote.PairEnd{}, remote.ErrOff
	}
	r := &a.remote
	r.mu.Lock()
	if r.code != nil {
		r.mu.Unlock()
		return remote.PairEnd{}, remote.ErrBusy
	}
	code := &RemoteCode{Name: req.Name, Code: req.Code, Until: time.Now().Add(remoteCodeHolds(a)), cancel: req.Cancel}
	r.code = code
	r.mu.Unlock()
	log.Printf("mixmusic: remote control: %s asks to pair", req.Name)
	a.notifyRemote("")
	// The code shows no more once both proved it, so that the next pairing
	// can show its own as soon as the other computer hears how this went.
	unshow := func() {
		r.mu.Lock()
		if r.code == code {
			r.code = nil
		}
		r.mu.Unlock()
	}
	// What Keep did, which Done reads: the server calls one after the
	// other.
	var (
		keepErr error
		unkeep  func()
	)
	return remote.PairEnd{
		Keep: func() error {
			unshow()
			was := a.remoteKept().Computers
			if keepErr = a.keepPaired(req.Peer, req.Name, true); keepErr != nil {
				return keepErr
			}
			unkeep = func() { a.unkeepPaired(req.Peer, was) }
			return nil
		},
		Done: func(err error) {
			unshow()
			note := ""
			switch {
			case err == nil:
				log.Printf("mixmusic: remote control: paired with %s, which may control this computer", req.Name)
				note = locales.RemoteNotePairedHere(req.Name)
			case keepErr != nil:
				log.Printf("mixmusic: remote control: keeping %s: %v", req.Name, keepErr)
				note = locales.RemoteNoteNotKept(req.Name, keepErr.Error())
			case unkeep != nil:
				// Kept, but the other computer didn't hear it, and keeps
				// nothing: neither does this one.
				unkeep()
				log.Printf("mixmusic: remote control: the pairing with %s didn't finish: %v", req.Name, err)
				note = locales.RemoteNoteUnfinished(req.Name)
				if errors.Is(err, remote.ErrNotKept) {
					note = locales.RemoteNoteUnfinishedSlow(req.Name)
				}
			case errors.Is(err, remote.ErrCanceled):
			case errors.Is(err, remote.ErrWrongCode):
				log.Printf("mixmusic: remote control: %s typed another code", req.Name)
				note = locales.RemoteNoteWrongCode(req.Name)
			default:
				log.Printf("mixmusic: remote control: pairing with %s: %v", req.Name, err)
				note = locales.RemoteNoteStopped(req.Name)
			}
			a.relistRemote()
			a.notifyRemote(note)
		},
	}, nil
}

// remoteCodeHolds is how long a code holds.
func remoteCodeHolds(a *App) time.Duration {
	if o := a.opts.Remote; o != nil && o.Timing.Code > 0 {
		return o.Timing.Code
	}
	return 2 * time.Minute
}

// player is the player the others control.
func (h remoteHost) player() *player.Player {
	if o := h.a.opts.Remote; o != nil && o.Player != nil {
		return o.Player()
	}
	return h.a.goPlayer.Load()
}

// State is what the player does now, whether the track that plays is
// liked, and where this computer can play.
func (h remoteHost) State() remote.State {
	p := h.player()
	if p == nil {
		return remote.State{}
	}
	st := p.State()
	out := remoteStateOf(st)
	if cur, ok := st.Current(); ok {
		out.Liked = h.a.IsLiked(cur.Track.ID)
	}
	out.Outputs, out.Output = h.a.remoteOutputs()
	return out
}

// remoteStateOf is the state the player's st tells the computer that
// controls it: the track, where it is, the parts it passes over, the
// volume, and the tracks it moves on to, each with its entry's UID, which
// the commands on the queue name.
func remoteStateOf(st player.State) remote.State {
	out := remote.State{Playing: st.Playing || st.Waiting && st.WillPlay, Loading: st.Loading, Live: st.Live,
		PositionMs: st.Position.Milliseconds(), DurationMs: st.Duration.Milliseconds(), Volume: st.Volume, Muted: st.Muted,
		Shuffle: st.Shuffle, Repeat: repeatOf(st.Repeat)}
	cur, ok := st.Current()
	if !ok {
		return out
	}
	t := remoteTrackOf(cur.Track)
	t.Entry = cur.UID
	out.Track = &t
	if out.DurationMs <= 0 && !st.Live {
		out.DurationMs = int64(cur.Track.DurationMs)
	}
	out.Parts = remotePartsOf(st)
	for i := st.Index + 1; i < len(st.Queue) && len(out.Next) < remote.MaxNext; i++ {
		if st.Queue[i].Skip == "" {
			next := remoteTrackOf(st.Queue[i].Track)
			next.Entry = st.Queue[i].UID
			out.Next = append(out.Next, next)
		}
	}
	return out
}

// remoteTrackOf is t as a state tells it: its cover only where it has one
// online, which the other computer can fetch.
func remoteTrackOf(t media.Track) remote.Track {
	out := remote.Track{Title: t.Title, Artwork: largeArtwork(t.Artwork)}
	for _, ar := range t.Artists {
		out.Artists = append(out.Artists, ar.Name)
	}
	if t.Album != nil {
		out.Album = t.Album.Name
	}
	return out
}

// Do runs a command on the player.
func (h remoteHost) Do(c remote.Command) {
	p := h.player()
	if p == nil {
		return
	}
	switch c.Op {
	case remote.OpPlay:
		p.Play()
	case remote.OpPause:
		p.Pause()
	case remote.OpNext:
		p.Next()
	case remote.OpPrevious:
		p.Previous()
	case remote.OpSeek:
		p.Seek(time.Duration(c.PositionMs) * time.Millisecond)
	case remote.OpVolume:
		p.SetVolume(c.Volume)
	case remote.OpPlayEntry:
		p.PlayEntry(c.Entry)
	case remote.OpRemove:
		p.RemoveEntry(c.Entry)
	case remote.OpMove:
		p.MoveEntry(c.Entry, c.To)
	case remote.OpClear:
		p.ClearUpcoming()
	case remote.OpShuffle:
		p.SetShuffle(c.Shuffle)
	case remote.OpRepeat:
		p.SetRepeat(repeatNamed(c.Repeat))
	case remote.OpMute:
		p.SetMuted(c.Muted)
	case remote.OpLike:
		h.like(p, c.Entry, c.Liked)
	case remote.OpOutput:
		h.a.playOn(c.Output)
	}
}

// like likes the track of the queue's entry, or the one that plays for
// none, or no longer, as the windows do: a like takes a dislike back.
func (h remoteHost) like(p *player.Player, entry string, liked bool) {
	a := h.a
	st := p.State()
	i := st.Index
	if entry != "" {
		i = slices.IndexFunc(st.Queue, func(it player.Item) bool { return it.UID == entry })
	}
	if i < 0 || i >= len(st.Queue) {
		return
	}
	t := st.Queue[i].Track
	if liked && a.IsDisliked(t.ID) {
		p.Disliked(t.ID, false)
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := (&Collection{a}).SetLiked(ctx, t, liked); err != nil {
			log.Printf("mixmusic: remote control: liking a track as asked: %v", err)
		}
		notify()
		a.pokeRemote()
	}()
}

// Watch has fn told as the player changes, and as what the computers that
// control this one see changes otherwise (pokeRemote).
func (h remoteHost) Watch(fn func()) func() {
	p := h.player()
	if p == nil {
		return func() {}
	}
	r := &h.a.remote
	f := &fn
	r.mu.Lock()
	r.pokes = append(slices.Clone(r.pokes), f)
	r.mu.Unlock()
	stop := p.Subscribe(func(player.Event) { fn() })
	return func() {
		stop()
		r.mu.Lock()
		r.pokes = slices.DeleteFunc(slices.Clone(r.pokes), func(g *func()) bool { return g == f })
		r.mu.Unlock()
	}
}

// pokeRemote tells the sessions of the computers that control this one
// that what they see changed, as a track liked.
func (a *App) pokeRemote() {
	r := &a.remote
	r.mu.Lock()
	fns := r.pokes
	r.mu.Unlock()
	for _, fn := range fns {
		(*fn)()
	}
}

// repeatNamed is the repeat a command or a hand-over names: "all", "one",
// or neither (repeatOf).
func repeatNamed(word string) player.Repeat {
	switch word {
	case "all":
		return player.RepeatAll
	case "one":
		return player.RepeatOne
	}
	return player.RepeatOff
}

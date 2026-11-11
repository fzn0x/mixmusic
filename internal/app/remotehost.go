package app

// The computer controlled's side of remote control: who may control it,
// the code it shows while another pairs with it, and its player, which the
// server tells of and runs the commands on.

import (
	"errors"
	"log"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/remote"
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
func (h remoteHost) Pairing(req remote.PairRequest) (func(error) error, error) {
	a := h.a
	if !a.remoteKept().On {
		return nil, remote.ErrOff
	}
	r := &a.remote
	r.mu.Lock()
	if r.code != nil {
		r.mu.Unlock()
		return nil, remote.ErrBusy
	}
	code := &RemoteCode{Name: req.Name, Code: req.Code, Until: time.Now().Add(remoteCodeHolds(a)), cancel: req.Cancel}
	r.code = code
	r.mu.Unlock()
	log.Printf("mixmusic: remote control: %s asks to pair", req.Name)
	a.notifyRemote("")
	return func(err error) error {
		r.mu.Lock()
		if r.code == code {
			r.code = nil
		}
		r.mu.Unlock()
		note := ""
		switch {
		case err == nil:
			if err = a.keepPaired(req.Peer, req.Name, true); err != nil {
				log.Printf("mixmusic: remote control: keeping %s: %v", req.Name, err)
				note = "Couldn't keep " + req.Name + " as paired: " + err.Error()
				break
			}
			log.Printf("mixmusic: remote control: paired with %s, which may control this computer", req.Name)
			note = "Paired with " + req.Name + ": it can control MixMusic on this computer"
		case errors.Is(err, remote.ErrCanceled):
		case errors.Is(err, remote.ErrWrongCode):
			log.Printf("mixmusic: remote control: %s typed another code", req.Name)
			note = "Didn't pair with " + req.Name + ": the code typed there wasn't the one shown"
		default:
			log.Printf("mixmusic: remote control: pairing with %s: %v", req.Name, err)
			note = "Didn't pair with " + req.Name + ": it stopped answering"
		}
		a.relistRemote()
		a.notifyRemote(note)
		return err
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

// State is what the player does now.
func (h remoteHost) State() remote.State {
	p := h.player()
	if p == nil {
		return remote.State{}
	}
	return remoteStateOf(p.State())
}

// remoteStateOf is the state the player's st tells the computer that
// controls it: the track, where it is, the volume, and the tracks it moves
// on to, each with its entry's UID, which the commands on the queue name.
func remoteStateOf(st player.State) remote.State {
	out := remote.State{Playing: st.Playing || st.Waiting && st.WillPlay, Loading: st.Loading, Live: st.Live,
		PositionMs: st.Position.Milliseconds(), DurationMs: st.Duration.Milliseconds(), Volume: st.Volume, Muted: st.Muted}
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
	}
}

// Watch has fn told as the player changes.
func (h remoteHost) Watch(fn func()) func() {
	p := h.player()
	if p == nil {
		return func() {}
	}
	return p.Subscribe(func(player.Event) { fn() })
}

package app

// Programs on this computer controlling MixMusic (#591): the token they
// carry, kept in control.json with the port so a script finds both, and
// the player as the routes of /control run commands on it.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/overlay"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// ControlFileName is the file in the data folder that keeps the token of
// /control and its port, which only the user may read.
const ControlFileName = "control.json"

// controlHold is what the app keeps of the token, and the file it is in.
type controlHold struct {
	mu    sync.Mutex
	token string
}

// controlFile is what control.json keeps: the port the server listens on,
// and the token every request of /control must carry.
type controlFile struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
}

// newControlToken is a token no one can guess: 256 random bits.
func newControlToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// readControlFile reads control.json in dir.
func readControlFile(dir string) (controlFile, error) {
	var c controlFile
	data, err := os.ReadFile(filepath.Join(dir, ControlFileName))
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, err
	}
	if len(c.Token) != 64 || c.Port < 1024 || c.Port > 65535 {
		return c, errors.New("it names no token or no port")
	}
	return c, nil
}

// writeControlFile writes control.json in dir, which the user alone may
// read.
func writeControlFile(dir string, c controlFile) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ControlFileName+".*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(append(data, '\n'))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp, 0o600)
	}
	if err == nil {
		err = os.Rename(tmp, filepath.Join(dir, ControlFileName))
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

// controlToken is the token /control takes: the one control.json keeps,
// or a new one, and then the same across restarts.
func (a *App) controlToken() (string, error) {
	c := &a.control
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" {
		return c.token, nil
	}
	if f, err := readControlFile(a.opts.DataDir); err == nil {
		c.token = f.Token
		return c.token, nil
	}
	token, err := newControlToken()
	if err != nil {
		return "", err
	}
	c.token = token
	return token, nil
}

// NewControlToken makes another token, which takes the place of the one
// the programs carry at once, and keeps it in control.json. The listener
// stays open.
func (a *App) NewControlToken() error {
	token, err := newControlToken()
	if err != nil {
		return err
	}
	a.control.mu.Lock()
	a.control.token = token
	a.control.mu.Unlock()
	if srv := a.overlay.server.Load(); srv != nil {
		srv.SetToken(token)
	}
	if err := a.keepControlFile(a.settings.Get().Clean()); err != nil {
		return err
	}
	log.Printf("mixmusic: control: a new token")
	return nil
}

// keepControlFile writes control.json of the settings, with the token, so
// that a script finds both.
func (a *App) keepControlFile(s settings.Settings) error {
	token, err := a.controlToken()
	if err != nil {
		return err
	}
	if err := writeControlFile(a.opts.DataDir, controlFile{Port: s.OverlayPort, Token: token}); err != nil {
		log.Printf("mixmusic: control: keeping %s: %v", ControlFileName, err)
		return err
	}
	return nil
}

// controlPlayer is the player the programs control.
func (a *App) controlPlayer() *player.Player {
	if o := a.opts.Remote; o != nil && o.Player != nil {
		return o.Player()
	}
	return a.goPlayer.Load()
}

// controlHost is the app as the routes of /control run commands on it and
// tell of it.
type controlHost struct{ a *App }

// Extra is what /control/state adds to /now-playing.
func (h controlHost) Extra() overlay.Extra {
	p := h.a.controlPlayer()
	if p == nil {
		return overlay.Extra{}
	}
	st := p.State()
	out := overlay.Extra{Volume: int(math.Round(st.Volume * 100)), Muted: st.Muted, Shuffle: st.Shuffle,
		Repeat: controlRepeat(st.Repeat)}
	out.Volume = min(max(out.Volume, 0), 100)
	if cur, ok := st.Current(); ok {
		out.Liked = h.a.IsLiked(cur.Track.ID)
		out.CanPrevious = st.Index > 0 || st.Position > 0
		for i := st.Index + 1; i < len(st.Queue) && len(out.Queue) < remote.MaxNext; i++ {
			if st.Queue[i].Skip != "" {
				continue
			}
			out.Queue = append(out.Queue, controlTrackOf(st.Queue[i].Track))
			out.CanNext = true
		}
	}
	return out
}

// controlRepeat is the repeat the state tells, which is "off" for none.
func controlRepeat(r player.Repeat) string {
	if name := repeatOf(r); name != "" {
		return name
	}
	return "off"
}

// controlTrackOf is t as the queue tells it.
func controlTrackOf(t media.Track) overlay.Track {
	out := overlay.Track{ID: t.ID, Title: oneLine(t.Title), Artists: []string{}}
	for _, ar := range t.Artists {
		if name := oneLine(ar.Name); name != "" {
			out.Artists = append(out.Artists, name)
		}
	}
	return out
}

// Do runs a command on the player.
func (h controlHost) Do(c overlay.Command) error {
	p := h.a.controlPlayer()
	if p == nil {
		return overlay.ErrNoPlayer
	}
	st := p.State()
	cur, hasTrack := st.Current()
	switch c.Op {
	case "toggle", "play", "pause", "next", "previous", "seek", "like", "dislike":
		if !hasTrack {
			return overlay.ErrNothingLoaded
		}
	}
	switch c.Op {
	case "toggle":
		p.ToggleOnce()
	case "play":
		p.Play()
	case "pause":
		p.Pause()
	case "next":
		p.Next()
	case "previous":
		p.Previous()
	case "mute":
		p.ToggleMute()
	case "shuffle":
		p.ToggleShuffle()
	case "repeat":
		if c.Repeat == "" {
			p.CycleRepeat()
		} else {
			p.SetRepeat(repeatNamed(c.Repeat))
		}
	case "track":
		if !slices.ContainsFunc(st.Queue, func(it player.Item) bool { return it.Track.ID == c.ID }) {
			return overlay.ErrNoTrack
		}
		p.PlayID(c.ID)
	case "seek":
		at := st.Position
		if c.Position != nil {
			at = time.Duration(*c.Position * float64(time.Second))
		} else {
			at += time.Duration(*c.By * float64(time.Second))
		}
		p.Seek(max(at, 0))
	case "volume":
		if c.Percent != nil {
			p.SetVolume(min(max(*c.Percent/100, 0), 1))
		} else {
			p.ChangeVolume(*c.ByPercent / 100)
		}
	case "like":
		h.like(p, cur.Track, !h.a.IsLiked(cur.Track.ID))
	case "dislike":
		h.dislike(p, cur.Track, !h.a.IsDisliked(cur.Track.ID))
	}
	// The state answered is the one the command left.
	p.Await()
	return nil
}

// like likes the track that plays, or takes the like back, as the media
// keys do: a like takes a dislike back.
func (h controlHost) like(p *player.Player, t media.Track, on bool) {
	a := h.a
	if on && a.IsDisliked(t.ID) {
		p.Disliked(t.ID, false)
		go func() {
			if err := (&Collection{a}).SetDisliked(context.Background(), t, false); err != nil {
				log.Printf("mixmusic: control: taking back the dislike of %s: %v", t.ID, err)
			}
		}()
	}
	go func() {
		if err := (&Collection{a}).SetLiked(context.Background(), t, on); err != nil {
			log.Printf("mixmusic: control: liking %s: %v", t.ID, err)
			return
		}
		p.Tell(player.Notice{Message: controlLiked(t, on)})
	}()
}

// dislike dislikes the track that plays, or takes the dislike back, as the
// media keys do: a dislike takes a like back.
func (h controlHost) dislike(p *player.Player, t media.Track, on bool) {
	a := h.a
	if on && a.IsLiked(t.ID) {
		go func() {
			if err := (&Collection{a}).SetLiked(context.Background(), t, false); err != nil {
				log.Printf("mixmusic: control: unliking %s: %v", t.ID, err)
			}
		}()
	}
	a.dislikeGoPlayer(p, t, on)
	p.Tell(player.Notice{Message: controlDisliked(t, on)})
}

// controlLiked is what the toast of a track liked, or no longer, says.
func controlLiked(t media.Track, liked bool) string {
	if liked {
		return locales.ControlToastLiked(noteBody(t))
	}
	return locales.ControlToastUnliked(noteBody(t))
}

// controlDisliked is what the toast of a track disliked, or no longer,
// says.
func controlDisliked(t media.Track, disliked bool) string {
	if disliked {
		return locales.ControlToastDisliked(noteBody(t))
	}
	return locales.ControlToastUndisliked(noteBody(t))
}

// Watch has fn told as the player changes, and as what the programs
// control changes otherwise, as the computers that control this one hear.
func (h controlHost) Watch(fn func()) func() {
	return remoteHost{h.a}.Watch(fn)
}

var _ overlay.Control = controlHost{}

// ControlStatus is what the server does, for the settings page: the same
// listener of overlays, which now takes commands too.
func (s *System) ControlStatus() overlay.Status { return s.OverlayStatus() }

// NewControlToken makes another token for the programs, which cuts the old
// one off at once.
func (s *System) NewControlToken() error { return s.a.NewControlToken() }

// ControlExample is a command to copy, with the token: "curl" or
// "powershell".
func (s *System) ControlExample(kind string) (string, error) {
	token, err := s.a.controlToken()
	if err != nil {
		return "", err
	}
	st := s.OverlayStatus()
	base := "http://" + st.Addr
	if kind == "powershell" {
		return "Invoke-RestMethod -Method Post -Headers @{Authorization='Bearer " + token +
			"'} " + base + "/control/next", nil
	}
	return "curl -X POST -H 'Authorization: Bearer " + token + "' " + base + "/control/next", nil
}

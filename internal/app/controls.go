package app

// The system's media controls show what MixMusic's own audio engine plays,
// which they control: a Spotify device shows there what it plays itself.

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/nowplaying"
)

// controls shows what MixMusic's own audio engine plays in the system's
// media controls, and passes on what they ask, as the media keys do.
type controls struct {
	mu sync.Mutex
	c  *nowplaying.Controls
	// tried is set once MixMusic reached for the system's controls.
	tried bool
	// native is set while the engine has the player's track.
	native bool
	np     NowPlaying
	at     time.Time // when np came
}

// controlCommands are the player's commands for what the system's
// controls ask without a value.
var controlCommands = map[nowplaying.Command]Command{
	nowplaying.Play:     CommandPlay,
	nowplaying.Pause:    CommandPause,
	nowplaying.Toggle:   CommandToggle,
	nowplaying.Next:     CommandNext,
	nowplaying.Previous: CommandPrevious,
}

// setNative tells whether MixMusic's own audio engine has the player's
// track, from its load to its stop.
func (a *App) setNative(on bool) {
	a.controls.mu.Lock()
	a.controls.native = on
	a.controls.mu.Unlock()
	a.showControls()
}

// reportControls takes what the player plays.
func (a *App) reportControls(np NowPlaying) {
	a.controls.mu.Lock()
	a.controls.np, a.controls.at = np, time.Now()
	a.controls.mu.Unlock()
	a.showControls()
}

// showControls shows the engine's track in the system's controls, or
// nothing when the engine has none. It reaches for the controls the first
// time it has one.
func (a *App) showControls() {
	c := &a.controls
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.c == nil && !c.tried && c.native && c.np.Track != nil {
		c.tried = true
		c.mu.Unlock()
		ctl, err := nowplaying.Open(a.controlsAsked, mygo.RunOnMain)
		c.mu.Lock()
		switch {
		case err == nil:
			c.c = ctl
		case !errors.Is(err, nowplaying.ErrUnsupported):
			log.Printf("mixmusic: the system's media controls: %v", err)
		}
	}
	if c.c == nil {
		return
	}
	if !c.native || c.np.Track == nil {
		c.c.Show(nil)
		return
	}
	c.c.Show(controlsInfo(c.np, c.at))
}

// controlsInfo is what np shows in the system's controls, reported at at.
func controlsInfo(np NowPlaying, at time.Time) *nowplaying.Info {
	t := np.Track
	info := &nowplaying.Info{
		ID:       t.ID,
		Title:    t.Title,
		Artwork:  largeArtwork(t.Artwork),
		Duration: time.Duration(np.DurationMs) * time.Millisecond,
		Position: time.Duration(np.PositionMs) * time.Millisecond,
		At:       at,
		Playing:  np.Playing,
		Volume:   np.Volume,
	}
	switch {
	case np.Live:
		// A live stream has no length, and no seeking: the controls show
		// how long it played.
		info.Duration = 0
	case info.Duration <= 0:
		info.Duration = time.Duration(t.DurationMs) * time.Millisecond
	}
	for _, artist := range t.Artists {
		info.Artists = append(info.Artists, artist.Name)
	}
	if t.Album != nil {
		info.Album = t.Album.Name
	}
	return info
}

// controlsAsked does what the system's controls ask.
func (a *App) controlsAsked(r nowplaying.Request) {
	switch r.Command {
	case nowplaying.Raise:
		a.showMain()
	case nowplaying.Seek:
		a.command(PlayerCommand{Command: CommandSeek, Value: ms(r.Position)})
	case nowplaying.SetVolume:
		a.command(PlayerCommand{Command: CommandVolume, Value: r.Volume})
	default:
		if c, ok := controlCommands[r.Command]; ok {
			a.command(PlayerCommand{Command: c})
		}
	}
}

package app

import (
	"slices"
	"sync"

	"github.com/fzn0x/mixmusic/internal/media"
)

// Repeat is the repeat mode of the queue.
type Repeat string

const (
	RepeatOff Repeat = "off"
	RepeatAll Repeat = "all"
	RepeatOne Repeat = "one"
)

// NowPlaying is what the player plays, as it reports it to the tray,
// notifications, the system's media controls, Discord and Last.fm.
type NowPlaying struct {
	Track   *media.Track `json:"track"`
	Playing bool         `json:"playing"`
	// Loading is set while the track is asked to play and doesn't yet.
	Loading    bool    `json:"loading"`
	PositionMs int     `json:"positionMs"`
	DurationMs int     `json:"durationMs"`
	Volume     float64 `json:"volume"`
	Shuffle    bool    `json:"shuffle"`
	Repeat     Repeat  `json:"repeat"`
	Liked      bool    `json:"liked"`
	// Live is set for a live stream, which has no length: PositionMs is
	// how long it played.
	Live bool `json:"live,omitempty"`
}

// Command is something the player is asked to do from outside its
// window: the tray, media keys, the system's media controls, a deep link.
type Command string

const (
	CommandToggle   Command = "toggle"
	CommandPlay     Command = "play"
	CommandPause    Command = "pause"
	CommandNext     Command = "next"
	CommandPrevious Command = "previous"
	CommandSeek     Command = "seek"
	CommandVolume   Command = "volume"
	CommandLike     Command = "like"
	CommandShuffle  Command = "shuffle"
	CommandRepeat   Command = "repeat"
	// CommandPlayTrack plays the track of TrackID.
	CommandPlayTrack Command = "playTrack"
)

// PlayerCommand is a Command with its argument: the position in
// milliseconds to seek to, or the volume from 0 to 1.
type PlayerCommand struct {
	Command Command `json:"command"`
	Value   float64 `json:"value,omitempty"`
	TrackID string  `json:"trackId,omitempty"`
}

// watchers are told when the settings, the accounts, the add-ons or the
// updates change: the windows read them again then.
var watchers struct {
	mu  sync.Mutex
	fns []*func()
}

// notify tells the watchers the app's state changed.
func notify() {
	watchers.mu.Lock()
	fns := watchers.fns
	watchers.mu.Unlock()
	for _, fn := range fns {
		(*fn)()
	}
}

// Watch has fn told, from any goroutine, when the settings, the accounts,
// the add-ons or the updates change, until cancel.
func (a *App) Watch(fn func()) (cancel func()) {
	f := &fn
	watchers.mu.Lock()
	watchers.fns = append(slices.Clone(watchers.fns), f)
	watchers.mu.Unlock()
	return func() {
		watchers.mu.Lock()
		watchers.fns = slices.DeleteFunc(slices.Clone(watchers.fns), func(g *func()) bool { return g == f })
		watchers.mu.Unlock()
	}
}

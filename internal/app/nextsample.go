package app

// The next track on the sample after the last one of the track before
// (#600): the engine tells the player that the next track it preloaded
// followed the one that ended, and a room's host joins no tracks so while
// guests listen, as its track changes wait for them (roomStream).

import (
	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/player"
)

// Followed tells the player that the track of url followed the one that
// ended, as audio.FollowEvents.
func (e engineEvents) Followed(url string) {
	if f, ok := e.to().(audio.FollowEvents); ok {
		f.Followed(url)
	}
}

var _ audio.FollowEvents = engineEvents{}

// Joins reports whether the next track may start on the sample after the
// one that plays: not while guests of the room this computer hosts listen,
// whose track changes wait for them, as Load's do.
func (s roomStream) Joins() bool {
	h := s.a.hostedRoom()
	return h == nil || h.server.Listeners() == 0
}

var _ player.Joiner = roomStream{}

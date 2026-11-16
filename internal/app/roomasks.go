package app

// The guest's side of the host's rules (#734): what the host lets its
// guests do, the songs this computer adds to the room and what became of
// them, its controls of the host's player, and its wait to be let in.

import (
	"errors"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/room"
)

// errNotInRoom: this computer is in no room it could add to, or control.
var errNotInRoom = errors.New("You're in no room now")

// inRoom is the session of the room this computer is in, and the name of
// its host, while it is.
func (a *App) inRoom() (*room.Session, string, bool) {
	r := &a.room
	r.mu.Lock()
	defer r.mu.Unlock()
	g := r.guest
	if g == nil || g.step != RoomIn || g.session == nil {
		return nil, "", false
	}
	return g.session, g.name, true
}

// RoomRules returns what the host of the room this computer is in lets its
// guests do, while it is in one.
func (rs *Rooms) RoomRules() (room.Rules, bool) {
	s, _, ok := rs.a.inRoom()
	if !ok {
		return room.Rules{}, false
	}
	return s.Rules()
}

// Asking reports whether the host of the room this computer joins asks its
// user to let it in.
func (rs *Rooms) Asking() bool {
	r := &rs.a.room
	r.mu.Lock()
	g := r.guest
	var s *room.Session
	if g != nil && g.step == RoomJoining {
		s = g.session
	}
	r.mu.Unlock()
	return s != nil && s.Asking()
}

// AddSong adds t to the queue of the room this computer is in, or asks
// its host to: what became of it comes as a note.
func (rs *Rooms) AddSong(t media.Track) error {
	s, _, ok := rs.a.inRoom()
	if !ok {
		return errNotInRoom
	}
	_, err := s.AddSong(queueTrackOf(t))
	return err
}

// Control has the host's player of the room this computer is in do do:
// room.Play, room.Pause, room.Next or room.Previous.
func (rs *Rooms) Control(do string) error {
	s, _, ok := rs.a.inRoom()
	if !ok {
		return errNotInRoom
	}
	return s.Control(do)
}

// watchRoomAsks tells the windows what the host of the room in the
// session s answers of this computer's songs and controls, and as its
// rules change, until the session ends.
func (a *App) watchRoomAsks(s *room.Session, host string) {
	s.Watch(func(e room.Event) {
		switch e.Kind {
		case room.RulesChanged:
			a.notifyRooms("")
		case room.SongAnswered:
			a.notifyRooms(songAnswer(host, e.Track.Title, e.Reason))
		case room.ControlRefused:
			a.notifyRooms(controlRefusal(host, e.Reason))
		}
	})
}

// songAnswer tells what became of the song title this computer added to
// the room of host, as the host's reason says.
func songAnswer(host, title, reason string) string {
	switch reason {
	case "":
		return host + " added " + title + " to the room"
	case room.SongWaits:
		return title + " waits for " + host + " to accept it"
	case room.SongDeclined:
		return host + " declined " + title
	case room.SongsAreOff:
		return host + " takes no songs from guests"
	case room.TooMany:
		return "You added many songs: wait a moment, or for " + host + " to answer those waiting"
	}
	return host + " can't play " + title
}

// controlRefusal tells why the host refused a control.
func controlRefusal(host, reason string) string {
	if reason == room.TooMany {
		return "Too many at once: try again in a moment"
	}
	return host + " doesn't let guests control playback"
}

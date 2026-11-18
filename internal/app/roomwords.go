package app

import (
	"errors"
	"strings"

	"github.com/fzn0x/mixmusic/internal/room"
	"github.com/fzn0x/mixmusic/locales"
)

// The errors of the rooms and of remote control keep their English for
// the logs; the windows show each in the interface's words (#800). An
// error of no message keeps its own words, after the message that leads
// it, where there is one.

// RoomErrorText is err, which a room's method returned, in the
// interface's words.
func RoomErrorText(err error) string {
	switch {
	case errors.Is(err, errInRoom):
		return locales.RoomErrorInRoom()
	case errors.Is(err, errNotInRoom):
		return locales.RoomErrorNotInRoom()
	case errors.Is(err, errCantHost), errors.Is(err, room.ErrCantHost):
		return locales.RoomErrorCantHost()
	case errors.Is(err, errMoving), errors.Is(err, room.ErrMoving):
		return locales.RoomErrorMoving()
	case errors.Is(err, room.ErrMoved):
		return locales.RoomErrorMoved()
	case errors.Is(err, room.ErrDropped):
		return locales.RoomErrorDropped()
	case errors.Is(err, room.ErrLeft):
		return locales.RoomErrorLeft()
	case errors.Is(err, errCantPlaySong):
		return locales.RoomErrorCantPlaySong()
	case errors.Is(err, errRoomListen):
		return locales.RoomErrorListen(causeOf(err, errRoomListen))
	case errors.Is(err, errRoomAnnounce):
		return locales.RoomErrorAnnounce(causeOf(err, errRoomAnnounce))
	case errors.Is(err, errKeyBusy):
		return locales.RemoteErrorKeyBusy()
	case err == nil:
		return ""
	}
	return ErrorText(err)
}

// peerError is an error of the computer named name, which remote control
// reached: the windows tell it by name.
type peerError struct {
	name string
	err  error
}

func (e *peerError) Error() string { return e.err.Error() }

func (e *peerError) Unwrap() error { return e.err }

// errHandOverLate: the other computer didn't answer a hand-over in time.
var errHandOverLate = errors.New("it didn't answer in time: what plays here plays on")

// RemoteErrorText is err, which remote control returned, in the
// interface's words.
func RemoteErrorText(err error) string {
	var peer *peerError
	switch {
	case errors.Is(err, errHandOverLate) && errors.As(err, &peer):
		return locales.RemoteErrorHandOverLate(peer.name)
	case errors.As(err, &peer):
		return remoteProblem(peer.name, peer.err)
	case errors.Is(err, errNotConnected):
		return locales.RemoteErrorNotConnected()
	case errors.Is(err, errNothingPlays):
		return locales.RemoteErrorNothingPlays()
	case errors.Is(err, errKeyBusy):
		return locales.RemoteErrorKeyBusy()
	case errors.Is(err, errRemoteListen):
		return locales.RemoteErrorListen(causeOf(err, errRemoteListen))
	case err == nil:
		return ""
	}
	return ErrorText(err)
}

// causeOf is the words of err after those of lead, which it wraps first:
// the system's own.
func causeOf(err, lead error) string {
	msg := err.Error()
	if rest, ok := strings.CutPrefix(msg, lead.Error()+": "); ok {
		return rest
	}
	return msg
}

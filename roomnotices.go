package main

import (
	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/player"
)

// roomNotice tells err, of a room, in the interface's words.
func roomNotice(err error) player.Notice {
	return player.Notice{Message: app.RoomErrorText(err), Error: true}
}

// remoteNotice tells err, of remote control, in the interface's words.
func remoteNotice(err error) player.Notice {
	return player.Notice{Message: app.RemoteErrorText(err), Error: true}
}

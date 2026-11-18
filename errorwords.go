package main

import (
	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/player"
)

// wordsNotice tells err, of an account's flow or the desktop, in the
// interface's words.
func wordsNotice(err error) player.Notice {
	return player.Notice{Message: app.ErrorWords(err), Error: true}
}

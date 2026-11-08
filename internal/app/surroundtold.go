package app

import (
	"strings"

	"github.com/fzn0x/mixmusic/internal/stream"
)

// surroundTold has the track that plays a video offer its 5.1 surround
// audio, which YouTube told of only after the video's stream was found, as
// the track started without waiting for that answer: its 5.1 button shows
// then, and it plays its 5.1 audio while 5.1 surround is on.
func (a *App) surroundTold(videoID string) {
	if p := a.goPlayer.Load(); p != nil {
		p.SurroundFound(streamURL(videoID), surroundStreamURL(videoID))
	}
}

// surroundStreamURL is the address of a video's 5.1 surround audio, which
// the engine opens.
func surroundStreamURL(videoID string) string {
	return schemeURL(streamScheme) + strings.TrimPrefix(stream.SurroundPath(videoID), "/")
}

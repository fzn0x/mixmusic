package app

import (
	"context"

	"github.com/fzn0x/mixmusic/internal/player"
)

// The player asks the proxy whether a stream's 5.1 surround audio plays at
// once, and has it made ready while the stream's stereo audio plays: yt-dlp
// may take seconds to find it, and YouTube a few more to serve it (#457).
var _ player.Surrounds = playerServices{}

// SurroundReady reports whether the proxy serves the 5.1 surround audio of
// src at once: from the cache, or as found, once YouTube serves it. Audio
// not of a video's has nothing to make ready.
func (s playerServices) SurroundReady(src player.Source) bool {
	videoID, surround, err := videoOf(src.SurroundURL)
	if err != nil || !surround || s.a.proxy == nil {
		return true
	}
	return s.a.proxy.SurroundReady(videoID)
}

// PrepareSurround returns once the proxy serves the 5.1 surround audio of
// src at once, or why it can't.
func (s playerServices) PrepareSurround(ctx context.Context, src player.Source) error {
	videoID, surround, err := videoOf(src.SurroundURL)
	if err != nil || !surround || s.a.proxy == nil {
		return nil
	}
	return s.a.proxy.PrepareSurround(ctx, videoID)
}

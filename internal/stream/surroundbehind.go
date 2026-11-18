package stream

import (
	"context"
	"log"
	"strings"
	"time"
)

// SetSurround tells the resolver whether 5.1 surround is on: while it is,
// yt-dlp's full run looks for the 5.1 surround audio of a video whose
// stream its quick run found (lookBehind), which lists none. Off, the
// quick run's stream is all there is, as before.
func (r *Resolver) SetSurround(on bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.surround = on
}

// lookBehind has yt-dlp's full run look for the 5.1 surround audio of a
// video whose stream its quick run found, the one the resolver holds,
// while 5.1 is on: once a stream, on a goroutine of its own. The quick run
// asks as the visionOS client alone, which YouTube lists no 5.1 audio to,
// so its stream starts at once in stereo, and the 5.1 audio found comes
// after it, as the TV app's answer does (keepTold): the track then offers
// it. A stream found with 5.1 off gets the look as it is asked for once
// 5.1 is on.
func (r *Resolver) lookBehind(videoID string) {
	r.mu.Lock()
	res, quality := r.calls[videoID], r.quality
	look := r.surround && res != nil && res.ended() && !res.lookedBehind && res.err == nil &&
		res.stream.Route == RouteQuick && !res.stream.Live && res.stream.Surround == nil && res.surround == nil
	if look {
		res.lookedBehind = true
	}
	r.mu.Unlock()
	if !look {
		return
	}
	go func() {
		if sur := r.findBehind(videoID, quality); sur != nil {
			r.keepTold(videoID, res, sur)
		}
	}()
}

// findBehind has yt-dlp's full run find the 5.1 surround audio of a video,
// as Resolver.Surround does, or nil when it finds none, fails, or can't
// find it: without yt-dlp and a JavaScript runtime, which the 5.1 audio's
// URL takes. The log tells how it went, never the URL.
func (r *Resolver) findBehind(videoID string, quality Quality) *Stream {
	if !r.findsSurround() {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
	defer cancel()
	start := time.Now()
	s, err := r.ytdlp(ctx, videoID, quality)
	took := time.Since(start).Round(10 * time.Millisecond)
	switch {
	case err != nil:
		log.Printf("mixmusic: yt-dlp's full run, behind its quick run, found no 5.1 surround audio of %s in %s (%s)",
			videoID, took, strings.TrimPrefix(err.Error(), "stream: "))
		return nil
	case s.Surround == nil || s.Live:
		log.Printf("mixmusic: yt-dlp's full run, behind its quick run, found no 5.1 surround audio of %s in %s: it has none", videoID, took)
		return nil
	}
	log.Printf("mixmusic: found the 5.1 surround audio of %s in %s with yt-dlp's full run, behind its quick run", videoID, took)
	return s.Surround
}

package stream

import (
	"log"
	"time"
)

// surroundTimeout bounds the TV client's answer of whether a video has 5.1
// surround audio, which goes on after the video's stream was found.
const surroundTimeout = 10 * time.Second

// toldLater is the TV client's answer of whether a video has 5.1 surround
// audio, which may come after the video's stream was found: no stream
// waits for it. done is closed once it came, or the request ended without
// it; surround is the 5.1 audio, nil when the video has none, or when the
// answer didn't come.
type toldLater struct {
	done     chan struct{}
	surround *Stream
}

// now returns the answer's 5.1 audio, and whether it came, without waiting.
func (t *toldLater) now() (*Stream, bool) {
	select {
	case <-t.done:
		return t.surround, true
	default:
		return nil, false
	}
}

// hear waits for the TV client's answer of a video's 5.1 surround audio,
// later, which came after res found the video's stream, keeps it with
// that stream, and tells Told. When yt-dlp couldn't find that audio,
// without a JavaScript runtime, it isn't kept, as resolve does with an
// answer that came in time.
func (r *Resolver) hear(videoID string, res *resolution, later *toldLater) {
	<-later.done
	s := later.surround
	if s == nil {
		return
	}
	if s.URL == "" && !r.findsSurround() {
		log.Printf("mixmusic: YouTube has 5.1 surround audio of %s, which takes yt-dlp and a JavaScript runtime to play", videoID)
		return
	}
	r.mu.Lock()
	res.surround = s
	r.mu.Unlock()
	if r.Told != nil {
		r.Told(videoID)
	}
}

// result is what res found, once it ended: its stream, with the 5.1
// surround audio the TV client told of after it, if any.
func (r *Resolver) result(res *resolution) (Stream, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := res.stream
	if s.Surround == nil {
		s.Surround = res.surround
	}
	return s, res.err
}

package stream

// Known returns the stream of a video the resolver holds, fresh, as
// Resolve would return it, without finding one or waiting for one: what
// Track info tells of a track MixMusic found the stream of.
func (r *Resolver) Known(videoID string) (Stream, bool) { return r.known(videoID, false) }

// KnownSurround is Known of a video's 5.1 surround audio: the stream of it
// yt-dlp found, or the one Resolve's stream holds, when its URL came with
// it.
func (r *Resolver) KnownSurround(videoID string) (Stream, bool) {
	if s, ok := r.known(videoID, true); ok {
		return s, true
	}
	if s, ok := r.known(videoID, false); ok && s.Surround != nil && s.Surround.URL != "" {
		return *s.Surround, true
	}
	return Stream{}, false
}

// known returns what the resolver found of a video, in calls, or in
// surrounds for its 5.1 surround audio, when that ended with a stream
// that isn't about to expire.
func (r *Resolver) known(videoID string, surround bool) (Stream, bool) {
	r.mu.Lock()
	calls := r.calls
	if surround {
		calls = r.surrounds
	}
	res := calls[videoID]
	r.mu.Unlock()
	if res == nil {
		return Stream{}, false
	}
	select {
	case <-res.done:
		if !res.fresh() {
			return Stream{}, false
		}
		return res.stream, true
	default:
		return Stream{}, false
	}
}

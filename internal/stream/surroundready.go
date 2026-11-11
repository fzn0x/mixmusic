package stream

import (
	"context"
	"time"
)

// SurroundReady reports, without waiting, whether the proxy serves the 5.1
// surround audio of a video at once: from the cache, or as found before,
// once YouTube serves it. The player plays the video's stereo audio until
// it does (PrepareSurround), as yt-dlp may take seconds to find it, and
// YouTube a few more to serve it.
func (p *Proxy) SurroundReady(videoID string) bool {
	if p.Cache != nil && p.Cache.Has(SurroundKey(videoID)) {
		return true
	}
	s, ok := p.Resolver.heldSurround(videoID)
	return ok && !time.Now().Before(s.Available)
}

// PrepareSurround returns once the proxy serves the 5.1 surround audio of a
// video at once, as SurroundReady tells: once yt-dlp found it, when it
// must, and YouTube serves it, or ctx ends. It returns why it can't, as
// errNoSurround when YouTube has none. What yt-dlp finds is kept, also
// when ctx ends first.
func (p *Proxy) PrepareSurround(ctx context.Context, videoID string) error {
	if p.Cache != nil && p.Cache.Has(SurroundKey(videoID)) {
		return nil
	}
	_, err := p.resolveSurround(ctx, videoID)
	return err
}

// heldSurround returns the 5.1 surround audio of a video the resolver holds,
// as Surround would return it without yt-dlp or a wait: false when it holds
// none, or would have yt-dlp find it.
func (r *Resolver) heldSurround(videoID string) (Stream, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	res := r.calls[videoID]
	if res == nil || !res.ended() || !res.fresh() {
		return Stream{}, false
	}
	s := res.stream.Surround
	if s == nil {
		s = res.surround
	}
	switch {
	case s == nil:
		return Stream{}, false
	case s.URL != "":
		return *s, true
	}
	found := r.surrounds[videoID]
	if found == nil || !found.ended() || !found.fresh() {
		return Stream{}, false
	}
	return found.stream, true
}

// ended reports whether res ended, found or failed.
func (res *resolution) ended() bool {
	select {
	case <-res.done:
		return true
	default:
		return false
	}
}

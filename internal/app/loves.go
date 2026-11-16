package app

import (
	"context"
	"log"

	"github.com/fzn0x/mixmusic/internal/listenbrainz"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/scrobble"
)

// sendLoves sends a like of track to each scrobbling service whose Send
// likes as loves is on, as a love, and an unlike as the love taken back:
// a track the services count, as scrobbleTrack tells, of a connection the
// service accepts.
func (a *App) sendLoves(ctx context.Context, track media.Track, liked bool) {
	a.mu.Lock()
	lf, lb := a.lastfm, a.listenbrainz.conn
	a.mu.Unlock()
	id, t := scrobbleTrack(NowPlaying{Track: &track})
	if lf == nil && lb == nil || id == "" {
		return
	}
	s := a.settings.Get()
	a.mu.Lock()
	var lfOn, lbOn bool
	var token string
	if lf != nil {
		lfOn = s.LastfmLoves && !lf.expired
	}
	if lb != nil {
		lbOn, token = s.ListenBrainzLoves && !lb.expired, lb.creds.Token
	}
	a.mu.Unlock()
	if lfOn {
		c, err := a.lastfmStore.Load()
		if err == nil {
			err = lf.client.Love(ctx, c.Session.Key, t, liked)
		}
		if err != nil {
			log.Printf("mixmusic: sending a love to Last.fm: %v", err)
		}
	}
	if lbOn {
		if err := lb.love(ctx, token, t, liked); err != nil {
			log.Printf("mixmusic: sending a love to ListenBrainz: %v", err)
		}
	}
}

// love sends the love of t, or takes it back, by the MessyBrainz ID its
// recording got when it played, or else by the MusicBrainz ID the
// server's lookup finds: a track it finds none of is left out.
func (l *brainzScrobbles) love(ctx context.Context, token string, t scrobble.Track, loved bool) error {
	r := listenbrainz.Recording{MSID: l.recordings.msid(t.ID)}
	if r.MSID == "" {
		mbid, err := l.client.Lookup(ctx, token, t)
		if err != nil || mbid == "" {
			return err
		}
		r.MBID = mbid
	}
	return l.client.Love(ctx, token, r, loved)
}

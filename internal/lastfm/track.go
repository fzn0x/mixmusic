package lastfm

import (
	"context"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/fzn0x/mixmusic/internal/scrobble"
)

// maxBatch is the most plays one track.scrobble call takes.
const maxBatch = 50

// Track is a song as Last.fm knows it, as every scrobbling service does.
type Track = scrobble.Track

// Play is a listen to a track: the track and when it started playing.
type Play = scrobble.Play

var _ scrobble.Service = (*Client)(nil)

// NowPlaying tells Last.fm that the user of sessionKey started listening to
// t, which their profile shows until the track ends or another one starts.
func (c *Client) NowPlaying(ctx context.Context, sessionKey string, t Track) error {
	params := url.Values{"sk": {sessionKey}, "artist": {t.Artist}, "track": {t.Title}}
	if t.Album != "" {
		params.Set("album", t.Album)
	}
	if s := seconds(t.Duration); s > 0 {
		params.Set("duration", strconv.Itoa(s))
	}
	return c.call(ctx, "track.updateNowPlaying", params, nil)
}

// Scrobble adds plays to the listening history of the user of sessionKey.
// It sends them in order, up to 50 per call. When a call fails, Scrobble
// returns its error without making the rest; the plays of the calls before
// were scrobbled.
func (c *Client) Scrobble(ctx context.Context, sessionKey string, plays []Play) error {
	for batch := range slices.Chunk(plays, maxBatch) {
		params := url.Values{"sk": {sessionKey}}
		for i, p := range batch {
			n := "[" + strconv.Itoa(i) + "]"
			params.Set("artist"+n, p.Artist)
			params.Set("track"+n, p.Title)
			params.Set("timestamp"+n, strconv.FormatInt(p.Start.Unix(), 10))
			if p.Album != "" {
				params.Set("album"+n, p.Album)
			}
			if s := seconds(p.Duration); s > 0 {
				params.Set("duration"+n, strconv.Itoa(s))
			}
		}
		if err := c.call(ctx, "track.scrobble", params, nil); err != nil {
			return err
		}
	}
	return nil
}

// seconds returns d in whole seconds, rounded.
func seconds(d time.Duration) int {
	return int(d.Round(time.Second) / time.Second)
}

// Love loves t for the user of sessionKey, which their profile shows, or
// takes the love back: track.love, or track.unlove.
func (c *Client) Love(ctx context.Context, sessionKey string, t Track, loved bool) error {
	method := "track.unlove"
	if loved {
		method = "track.love"
	}
	return c.call(ctx, method, url.Values{"sk": {sessionKey}, "artist": {t.Artist}, "track": {t.Title}}, nil)
}

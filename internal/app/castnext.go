package app

// The next track handed to the cast device ahead, as the player preloads
// it, where the device takes one: it plays on without a gap then, and
// the next track's load takes in what the device moved on to by itself.

import (
	"context"
	"log"
	"net/url"
	"time"

	"github.com/fzn0x/mixmusic/internal/cast"
	"github.com/fzn0x/mixmusic/internal/media"
)

// castQueue is a session that takes the media that plays next ahead, as
// a renderer's SetNextAVTransportURI: one step, which any cast device
// that queues can take.
type castQueue interface {
	Next(ctx context.Context, m cast.Media) error
}

// preload hands the device the stream of rawURL, which plays next, with
// what shows of the track up next, where its session queues.
func (o *castOut) preload(rawURL string) {
	q, ok := o.session.(castQueue)
	if !ok {
		return
	}
	path, contentType, err := o.pick(rawURL)
	if err != nil {
		return
	}
	u, _ := url.Parse(rawURL)
	m := cast.Media{URL: o.served(path) + query(u), ContentType: contentType, Autoplay: true}
	o.do(func(ctx context.Context) {
		o.mu.Lock()
		gone := o.closed || o.idle || o.loading || o.loadedURL == rawURL
		o.mu.Unlock()
		if gone {
			return
		}
		if o.upNext != nil {
			if t := o.upNext(); t != nil {
				describeMedia(&m, *t)
			}
		}
		if err := q.Next(ctx, m); err != nil {
			log.Printf("mixmusic: the cast device didn't take the next track ahead: %v", err)
		}
	})
}

// describeMedia gives m what the device's screen shows of t.
func describeMedia(m *cast.Media, t media.Track) {
	m.Title, m.Artist, m.Image = t.Title, t.ArtistNames(), coverOf(t)
	if t.Album != nil {
		m.Album = t.Album.Name
	}
	m.Duration = time.Duration(t.DurationMs) * time.Millisecond
}

// pick returns the path a cast device fetches a stream's audio at, past
// its server's token, and its type: by the device's own formats (source),
// or, on Google Cast, the stream's own.
func (o *castOut) pick(rawURL string) (string, string, error) {
	if o.source != nil {
		return o.source(rawURL)
	}
	videoID, surround, err := videoOf(rawURL)
	if err != nil {
		return "", "", err
	}
	u, _ := url.Parse(rawURL)
	return u.Path, o.contentType(videoID, surround), nil
}

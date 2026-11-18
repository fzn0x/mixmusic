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
	o.mu.Lock()
	if o.loadedURL != rawURL {
		o.nextURL = rawURL
	}
	o.mu.Unlock()
	o.serveOnly()
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
				o.describe(&m, *t, rawURL)
			}
		}
		if err := q.Next(ctx, m); err != nil {
			log.Printf("mixmusic: the cast device didn't take the next track ahead: %v", err)
		}
	})
}

// describe gives m what the device's screen shows of t, the track of the
// stream of: its cover too, online, or served by MixMusic for a song on
// this computer (art).
func (o *castOut) describe(m *cast.Media, t media.Track, of string) {
	m.Title, m.Artist, m.Image = t.Title, t.ArtistNames(), coverOf(t)
	if m.Image == "" && o.art != nil {
		m.Image = o.art(t, of)
	}
	if t.Album != nil {
		m.Album = t.Album.Name
	}
	m.Duration = time.Duration(t.DurationMs) * time.Millisecond
}

// serveOnly has the server serve the files of the stream cast and of the
// one queued after it alone.
func (o *castOut) serveOnly() {
	if o.keep == nil {
		return
	}
	o.mu.Lock()
	of := []string{o.loadedURL, o.nextURL}
	o.mu.Unlock()
	o.keep(of...)
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

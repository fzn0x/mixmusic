package media

import (
	"context"
	"slices"
	"time"
)

// MaxTracks bounds how many tracks of a playlist MixMusic reads: of one
// that has more, it reads the first MaxTracks, and the playlist is Cut.
const MaxTracks = 20000

// PageFunc reads the page of a long list of tracks that token names: its
// tracks, and the token of the page after it, "" after the last.
type PageFunc func(ctx context.Context, token string) ([]Track, string, error)

// PageWaits are how long a page of tracks that failed waits before each
// new try: it's tried once, then once after each wait. Tests shorten them.
var PageWaits = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}

// maxEmptyPages is how many pages in a row without a track end a list: a
// service that gave the token of another page with each, forever, would
// be read forever.
const maxEmptyPages = 10

// ReadPage reads the page of token with read, and while it fails, tries
// again after each of PageWaits, unless ctx ends.
func ReadPage(ctx context.Context, read PageFunc, token string) ([]Track, string, error) {
	for try := 0; ; try++ {
		tracks, next, err := read(ctx, token)
		if err == nil || try == len(PageWaits) || ctx.Err() != nil {
			return tracks, next, err
		}
		select {
		case <-time.After(PageWaits[try]):
		case <-ctx.Done():
			return nil, "", err
		}
	}
}

// ReadOn reads the tracks of p past those it holds, a page at a time with
// read, from the page of More on, and adds them to p: until there are no
// more, or until p holds MaxTracks of a playlist that has more, which Cut
// tells. After each page, onPage, if set, is given p as it is then. A page
// that fails after its tries stops it with the error: p holds the tracks
// of the pages before, and More names the page that failed, to read on
// from. A playlist read whole that didn't say how many tracks it has
// counts those it holds.
func (p *Playlist) ReadOn(ctx context.Context, read PageFunc, onPage func(Playlist)) error {
	// The tracks p holds may be shared with whoever gave it, and with
	// what onPage was given: the pages go after them, never into their
	// room.
	p.Tracks = slices.Clip(p.Tracks)
	p.bound()
	for empty := 0; p.More != ""; {
		tracks, next, err := ReadPage(ctx, read, p.More)
		if err != nil {
			return err
		}
		if len(tracks) == 0 {
			empty++
		} else {
			empty = 0
		}
		if empty == maxEmptyPages || next == p.More {
			next = ""
		}
		p.Tracks, p.More = append(p.Tracks, tracks...), next
		p.bound()
		if onPage != nil {
			now := *p
			now.Tracks = slices.Clip(p.Tracks)
			onPage(now)
		}
	}
	if p.TrackCount == 0 && !p.Cut {
		p.TrackCount = len(p.Tracks)
	}
	return nil
}

// bound cuts p at MaxTracks, when it holds more, or holds that many and
// has more to read.
func (p *Playlist) bound() {
	if len(p.Tracks) > MaxTracks || len(p.Tracks) == MaxTracks && p.More != "" {
		p.Tracks, p.More, p.Cut = slices.Clip(p.Tracks[:MaxTracks]), "", true
	}
}

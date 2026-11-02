package main

// Long playlists: a playlist's page shows its first page of tracks at once,
// and reads the rest beside it, a page at a time, telling how it goes. Its
// Play and Shuffle take them all: asked while they come, they wait for
// them.

import (
	"context"
	"errors"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
)

// wholeWait is how long what takes a whole playlist waits for it to be
// read, as a download: one of thousands of tracks takes minutes.
const wholeWait = 10 * time.Minute

// readPlaylistOn reads the tracks of a playlist past those it holds, from
// its service; tests answer for the services.
var readPlaylistOn = func(s *shell, ctx context.Context, p media.Playlist, onPage func(media.Playlist)) (media.Playlist, error) {
	return s.svc.Catalog.ReadOn(ctx, p, onPage)
}

// playlistRead is a playlist as a page reads it: from the first page its
// fetch got, on. The loads keep it under its key, beside the fetch's.
type playlistRead struct {
	key string
	// of is the fetch of the first page, and at the answer of it read on
	// from.
	of *loaded[media.Playlist]
	at int
	// p is the playlist as read so far; reading is set while the rest
	// comes, and err tells why the reading stopped before the end.
	p       media.Playlist
	reading bool
	err     error
	// length is how long the tracks last, summed once they're all read.
	length time.Duration
	// then plays the tracks once they're all read, as the user asked while
	// they came, unless another track started meanwhile: from is the entry
	// that was current as they asked, and does tells what then does.
	then   func([]media.Track)
	from   string
	does   string
	cancel context.CancelFunc
}

// readOn returns the playlist of of, the fetch of key, as read so far. Its
// first answer, and each answer of a refresh, start reading the rest
// beside the page.
func (s *shell) readOn(key string, of *loaded[media.Playlist]) *playlistRead {
	key += " read"
	r, _ := s.loads[key].(*playlistRead)
	if r != nil && r.of == of && r.at == of.answers {
		return r
	}
	if r != nil && r.cancel != nil {
		r.cancel()
	}
	r = &playlistRead{key: key, of: of, at: of.answers, p: of.value}
	s.loads[key] = r
	if r.p.More == "" {
		r.length = total(r.p.Tracks)
	} else {
		s.readRest(r)
	}
	return r
}

// readRest reads the rest of r, from where it is, beside the page. A read
// that the loads no longer keep, after another account signed in, stops.
func (s *shell) readRest(r *playlistRead) {
	ctx, cancel := context.WithCancel(context.Background())
	r.reading, r.err, r.cancel = true, nil, cancel
	from := r.p
	go func() {
		defer cancel()
		p, err := readPlaylistOn(s, ctx, from, func(p media.Playlist) {
			s.update(func() {
				if s.loads[r.key] != r {
					cancel()
					return
				}
				r.p = p
			})
		})
		s.update(func() {
			if s.loads[r.key] == r {
				s.readDone(r, p, err)
			}
		})
	}()
}

// readDone takes the end of the reading of r: the playlist read, and why
// it stopped short, if it did. What the user asked to play meanwhile plays
// what was read.
func (s *shell) readDone(r *playlistRead, p media.Playlist, err error) {
	r.p, r.err, r.reading, r.cancel = p, err, false, nil
	if err == nil {
		r.length = total(p.Tracks)
	}
	if then := r.then; then != nil {
		r.then = nil
		if cur, _ := s.p.State().Current(); cur.UID == r.from {
			then(p.Tracks)
		}
	}
}

// whenRead gives do the tracks of r, the whole playlist: at once when
// they're read, else once they are, which the page tells as does says.
func (s *shell) whenRead(r *playlistRead, does string, do func([]media.Track)) {
	if !r.reading {
		do(r.p.Tracks)
		return
	}
	cur, _ := s.st.Current()
	r.then, r.from, r.does = do, cur.UID, does
}

// readLine tells how the reading of r, a list that noun names, goes: how
// many of its tracks came while the rest come, and what plays once they're
// here; how many could be read when a page failed, with a way to try
// again; and where a list too long was cut.
func (s *shell) readLine(c *ui.Context, r *playlistRead, noun string) {
	k := &s.lk
	n, count := len(r.p.Tracks), r.p.TrackCount
	switch {
	case r.reading:
		text := "Loading tracks · " + thousands(n)
		if count > n {
			text += " of " + thousands(count)
		}
		if r.then != nil {
			text += " · " + r.does + " once they're all here"
		}
		ui.Row(c).Gap(8).Margin(0, 0, 16).Children(func() {
			ui.Spinner(c).Size(16, 16).Label("Loading tracks")
			ui.Text(c, text).FontSize(k.smallSize()).TextColor(k.muted).FontFeatures("tnum")
		})
	case r.err != nil:
		what := "Only the first " + thousands(n) + " tracks of this " + noun
		if count > n {
			what = "Only " + thousands(n) + " of this " + noun + "'s " + thousands(count) + " tracks"
		}
		ui.Column(c).Margin(0, 0, 16).Children(func() {
			s.problem(c, errors.New(what+" could be loaded: "+r.err.Error()), func() { s.readRest(r) })
		})
	case r.p.Cut:
		text := "MixMusic shows and plays the first " + thousands(media.MaxTracks) + " tracks of a " + noun + ": this one has more."
		if count > n {
			text = "This " + noun + " has " + thousands(count) + " tracks: MixMusic shows and plays its first " + thousands(n) + "."
		}
		ui.Text(c, text).FontSize(k.smallSize()).TextColor(k.muted).Margin(0, 0, 16).MaxWidth(640)
	}
}

package app

// All of an artist's songs (#543). YouTube Music lists them in a playlist,
// which the title of an artist's Top songs opens (media.Artist.SongsID).
// Their Play and Shuffle play its first page, about 100 songs, as soon as
// it comes, and the rest join the queue as they're read, a page at a time.

import (
	"context"
	"errors"
	"fmt"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// PlayArtist plays all the songs of a, an artist's page, on p, in their
// order or shuffled: the first page of the list of them as soon as it
// comes, and the rest as they're read, up to media.MaxTracks, each song
// once and none the settings hide. Replacing the queue stops the reading;
// a page that still fails after its tries stops it too, and p tells how
// many songs came. An artist without a list plays their top songs, which
// for the music on this computer are all theirs, as one whose list's
// first page still fails after its tries does; p tells why, but for the
// music on this computer and Spotify, whose artists have no list. Nothing
// plays when another track started meanwhile. It returns once it asked p
// to play the songs, or fails when a has none.
func (c *Catalog) PlayArtist(ctx context.Context, p *player.Player, a media.Artist, shuffled bool) error {
	from, _ := p.State().Current()
	moved := func() bool {
		cur, _ := p.State().Current()
		return cur.UID != from.UID
	}
	var failed error
	if a.SongsID != "" {
		var list media.Playlist
		failed = media.TryPage(ctx, func(ctx context.Context) error {
			var err error
			list, err = c.a.playlist(ctx, a.SongsID)
			return err
		})
		// A list whose first page holds none that play here, as one of
		// explicit songs the settings hide, leaves the top songs.
		keep := c.a.songsKept()
		if tracks := keep(list.Tracks); failed == nil && len(tracks) > 0 {
			if moved() {
				return nil
			}
			l := p.PlayList(tracks, shuffled)
			if list.More != "" {
				go c.a.readArtistSongs(p, l, list, keep, a.Name)
			}
			return nil
		}
	}
	top := c.a.songsKept()(a.TopTracks)
	switch {
	case len(top) == 0 && failed != nil:
		return fmt.Errorf("%s's songs couldn't be loaded: %w", a.Name, failed)
	case len(top) == 0:
		return errors.New("No songs listed for " + a.Name)
	case moved():
		return nil
	}
	p.PlayAll(top, shuffled)
	switch {
	case failed != nil:
		p.Tell(player.Notice{Message: a.Name + "'s top songs play, as the list of all their songs couldn't be loaded: " + failed.Error(),
			Error: true})
	case a.SongsID == "" && a.Source == media.SourceYouTube:
		p.Tell(player.Notice{Message: a.Name + "'s top songs play: YouTube Music lists no more of their songs"})
	}
	return nil
}

// readArtistSongs reads the songs of list, an artist's whose first page
// plays as l, past that page, and adds those keep keeps to l as they come,
// until l is no longer in the queue. A page that still fails after its
// tries stops it, and p tells how many of the artist's songs came.
func (a *App) readArtistSongs(p *player.Player, l *player.List, list media.Playlist, keep func([]media.Track) []media.Track, name string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	n := len(list.Tracks)
	read, err := a.readOn(ctx, list, func(now media.Playlist) {
		more := keep(now.Tracks[n:])
		n = len(now.Tracks)
		if !l.Add(more) {
			cancel()
		}
	})
	if err == nil || ctx.Err() != nil || !l.Add(nil) {
		return
	}
	got := len(read.Tracks)
	what := fmt.Sprintf("Only the first %d of %s's songs", got, name)
	if read.TrackCount > got {
		what = fmt.Sprintf("Only %d of %s's %d songs", got, name, read.TrackCount)
	}
	p.Tell(player.Notice{Message: what + " could be loaded: " + err.Error(), Error: true})
}

// songsKept returns what keeps, of each part of a list of songs in turn,
// those to play: each song once, by its ID, and none the settings hide.
func (a *App) songsKept() func([]media.Track) []media.Track {
	hide := a.settings.Get().HideExplicit
	seen := map[string]bool{}
	return func(tracks []media.Track) []media.Track {
		out := make([]media.Track, 0, len(tracks))
		for _, t := range tracks {
			if seen[t.ID] || hide && t.ExplicitSong() {
				continue
			}
			seen[t.ID] = true
			out = append(out, t)
		}
		return out
	}
}

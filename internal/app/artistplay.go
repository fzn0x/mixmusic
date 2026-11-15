package app

// Playing all of an artist's songs (#543). YouTube Music lists them in a
// playlist, which the title of an artist's Top songs opens
// (media.Artist.SongsID). Their Play and Shuffle play its first page,
// about 100 songs, as soon as it comes, and the rest join the queue as
// they're read, a page at a time.

import (
	"context"
	"errors"
	"fmt"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// songsRead reads all the songs of an artist, giving each the new ones as
// they come, as Catalog.YouTubeArtistSongs does.
type songsRead func(ctx context.Context, each func([]media.Track)) ([]media.Track, error)

// PlayArtist plays all the songs of a, an artist's page, on p, in their
// order or shuffled, as the page's Play and Shuffle do: of YouTube Music,
// the list of them (youtube.Client.SongList), whose first page plays as
// soon as it comes and the rest as they're read. An artist without a list
// plays their top songs, which for the music on this computer are all
// theirs. It returns once the reading ended (playArtistSongs).
func (c *Catalog) PlayArtist(ctx context.Context, p *player.Player, a media.Artist, shuffled bool) error {
	var read songsRead
	if source, list, err := media.ParseID(a.SongsID); err == nil && source == media.SourceYouTube {
		read = func(ctx context.Context, each func([]media.Track)) ([]media.Track, error) {
			return c.a.youtube.SongList(ctx, list, each)
		}
	}
	return c.a.playArtistSongs(ctx, p, a, shuffled, read)
}

// playArtistSongs plays the songs of the artist a that read reads on p, in
// their order or shuffled: the first that come, and the rest as they
// come, each song once and none the settings hide. Replacing the queue
// stops the reading; one that came in part says how many songs came.
// Nothing plays when another track started before the first came. When
// read gives none, or is nil, a's top songs play, and p tells why: the
// reading's error, or that YouTube Music lists no more of a's songs. It
// returns once the reading ended, or fails when a has no songs to play.
func (a *App) playArtistSongs(ctx context.Context, p *player.Player, artist media.Artist, shuffled bool, read songsRead) error {
	from, _ := p.State().Current()
	moved := func() bool {
		cur, _ := p.State().Current()
		return cur.UID != from.UID
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	keep := a.songsKept()
	var (
		l    *player.List
		gone bool
		err  error
	)
	if read != nil {
		_, err = read(ctx, func(songs []media.Track) {
			songs = keep(songs)
			switch {
			case gone || len(songs) == 0:
			case l == nil && moved():
				gone = true
				cancel()
			case l == nil:
				l = p.PlayList(songs, shuffled)
			case !l.Add(songs):
				gone = true
				cancel()
			}
		})
	}
	switch {
	case gone:
		return nil
	case l != nil:
		if short, ok := media.Short(err); ok && l.Add(nil) {
			what := fmt.Sprintf("Only the first %d of %s's songs", short.Got, artist.Name)
			if short.Total > short.Got {
				what = fmt.Sprintf("Only %d of %s's %d songs", short.Got, artist.Name, short.Total)
			}
			p.Tell(player.Notice{Message: what + " could be loaded: " + short.Err.Error(), Error: true})
		}
		return nil
	}
	// The list held none to play here, as one of explicit songs the
	// settings hide, or none came: the top songs play.
	top := a.songsKept()(artist.TopTracks)
	switch {
	case len(top) == 0 && err != nil:
		return fmt.Errorf("%s's songs couldn't be loaded: %w", artist.Name, err)
	case len(top) == 0:
		return errors.New("No songs listed for " + artist.Name)
	case moved():
		return nil
	}
	p.PlayAll(top, shuffled)
	switch {
	case err != nil:
		p.Tell(player.Notice{Message: artist.Name + "'s top songs play, as the list of all their songs couldn't be loaded: " + err.Error(),
			Error: true})
	case read == nil && artist.Source == media.SourceYouTube:
		p.Tell(player.Notice{Message: artist.Name + "'s top songs play: YouTube Music lists no more of their songs"})
	}
	return nil
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

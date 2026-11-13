package main

import (
	"context"
	"errors"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/spotify"
)

// hiddenTracks reports whether err is Spotify's refusal to show apps the
// tracks of p, a playlist of others, which it told the rest of, or to show
// apps anything of p, one of its own (#530).
func hiddenTracks(err error, p media.Playlist) bool {
	return errors.Is(err, spotify.ErrPlaylistUnavailable) && p.Title != "" || errors.Is(err, spotify.ErrPlaylistNotShown)
}

// tracksHidden reports whether p is a Spotify playlist of others, as the
// user's library lists those they follow, which Spotify shows apps no
// track of: its tile neither plays nor downloads it, and Home doesn't
// offer it as a shelf (#530).
func tracksHidden(p media.Playlist) bool {
	return p.Source == media.SourceSpotify && !p.Editable
}

// hiddenTracksPage is the page of a Spotify playlist whose tracks Spotify
// shows apps none of: what it tells of the playlist, or for one of its
// own, which it tells nothing of, what the user's library does, with a way
// to open it on Spotify and to save it, rather than an error.
func (s *shell) hiddenTracksPage(c *ui.Context, key string, res *loaded[media.Playlist]) {
	p := res.value
	why := "Spotify shows apps the tracks of the playlists you own or collaborate on only: open this one on Spotify to play it."
	if errors.Is(res.err, spotify.ErrPlaylistNotShown) {
		why = "Spotify shows apps nothing of its own playlists, nor of those deleted or made private: open this one on Spotify to play it."
		if l, ok := s.loads["spotify:playlists"].(*loaded[[]media.Playlist]); ok {
			for _, known := range l.value {
				if known.ID == p.ID {
					p.Title, p.Owner, p.Artwork, p.Description = known.Title, known.Owner, known.Artwork, known.Description
				}
			}
		}
		if p.Title == "" {
			p.Title = "Spotify playlist"
		}
	}
	s.pageScroll(c, func() {
		s.pageHeader(c, p.Title, nil, nil)
		facts := texts(p.Owner, collaborativeFact(p))
		if p.TrackCount > 0 {
			facts = append(facts, fact{text: plural(p.TrackCount, "track")})
		}
		h := headProps{title: p.Title, art: p.Artwork, source: p.Source, facts: facts, description: p.Description,
			menu: s.collectionMenu(p.Title, nil, p.URL, p.Source, "")}
		if p.URL != "" {
			h.playLabel = "open on " + sourceName(p.Source)
			h.play = func() { go s.svc.System.OpenExternal(p.URL) }
		}
		if s.signedIn(p.Source) {
			h.keep, h.keepLabel, h.keptLabel, h.keeping = &res.value.Saved, "save", "saved", s.keeping[key]
			h.onKeep = func(on bool) {
				s.keep(key, string(p.Source)+":playlists", p.Title, "saved", &res.value.Saved, on, func(ctx context.Context) error {
					return s.svc.Collection.SetSaved(ctx, "playlist", p.ID, "", on)
				})
			}
		}
		s.collectionHead(c, h)
		s.emptyLine(c, why)
	})
}

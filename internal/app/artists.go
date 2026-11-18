package app

import (
	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
)

// FollowedArtists returns the artists the user follows in MixMusic, the
// last followed first.
func (c *Collection) FollowedArtists() []media.Artist { return c.a.library.Artists() }

// FollowsArtist reports whether the user follows the artist id in
// MixMusic.
func (c *Collection) FollowsArtist(id string) bool { return c.a.library.FollowsArtist(id) }

// SetArtistFollowed follows an artist in MixMusic, or stops following
// them there. Nothing of it reaches their service, signed in or not:
// SetFollowing follows there. Signed out of it, MixMusic's follows are the
// whole list of the artists the user follows, and an artist they stop
// following loses their pin.
func (c *Collection) SetArtistFollowed(a media.Artist, on bool) error {
	if err := c.a.library.SetArtistFollowed(a, on); err != nil {
		return err
	}
	if !on {
		c.a.releases.forget(c.a, a.ID)
		if !c.a.signedInTo(a.Source) && c.a.library.IsPinned(a.ID) {
			return c.a.library.SetPinned(library.Mark{ID: a.ID}, false)
		}
	}
	return nil
}

// signedInTo reports whether the user is signed in to source.
func (a *App) signedInTo(source media.Source) bool {
	switch source {
	case media.SourceSpotify:
		return a.spotify.SignedIn()
	case media.SourceYouTube:
		return a.youtube.SignedIn()
	}
	return false
}

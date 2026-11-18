package app

// Links: what a link to YouTube, YouTube Music or Spotify, pasted in the
// search box or the command palette, opens (#614).

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/youtube"
	"github.com/fzn0x/mixmusic/locales"
)

// errSpotifyLinks is why a Spotify link doesn't open signed out; it says
// itself in the window's words, as worded errors do.
var errSpotifyLinks = wordedLater("sign in to Spotify in Settings › Accounts to open its links", locales.TopbarLinkSpotifySignIn, spotify.ErrNotSignedIn)

// errNoShow is why a Spotify episode's link doesn't open: Spotify named no
// show it is of.
var errNoShow = errors.New("spotify named no show of the episode")

// Linked is what a link names, as Link reads it: in Item, a song, which
// plays, or the album, playlist, artist or podcast whose page opens.
type Linked struct {
	Item media.Item
	// Radio is what plays after the song of a radio link: its radio,
	// without what the settings leave out of radios.
	Radio []media.Track
}

// Link reads what l names, as media.ParseLink read it from a link:
//
//   - a YouTube song from its radio, whose first track it is, and the
//     song of a radio link, with that radio after it;
//   - a Spotify track with the Web API's Get Track;
//   - an album, a playlist or an artist, as its page shows it, and the
//     podcast of a show or of an episode. The link of the playlist of an
//     album's tracks names the album.
//
// A Spotify link needs the user signed in to Spotify. A Spotify playlist
// of others, whose tracks Spotify shows apps none of, reads, as its page
// says why.
func (c *Catalog) Link(ctx context.Context, l media.Link) (Linked, error) {
	if l.Source == media.SourceSpotify && !c.a.spotify.SignedIn() {
		return Linked{}, errSpotifyLinks
	}
	v, err := c.link(ctx, l)
	if err != nil {
		log.Printf("mixmusic: opening %s: %v", l.URL(), err)
	}
	return v, err
}

func (c *Catalog) link(ctx context.Context, l media.Link) (Linked, error) {
	_, native, err := media.ParseID(l.ID)
	if err != nil {
		return Linked{}, err
	}
	switch l.Kind {
	case media.LinkSong:
		var t media.Track
		if l.Source == media.SourceSpotify {
			t, err = c.a.spotify.Track(ctx, native)
		} else {
			tracks, rerr := c.a.youtube.Radio(ctx, native)
			t, _, err = startOf(l.ID, tracks, rerr)
		}
		return Linked{Item: media.Item{Kind: media.ItemTrack, Track: &t}}, err
	case media.LinkRadio:
		_, list, err := media.ParseID(l.Radio)
		if err != nil {
			return Linked{}, err
		}
		tracks, err := c.a.youtube.RadioOf(ctx, native, list)
		t, radio, err := startOf(l.ID, tracks, err)
		return Linked{Item: media.Item{Kind: media.ItemTrack, Track: &t}, Radio: c.keptOfRadio(radio)}, err
	case media.LinkAlbum:
		a, err := c.Album(ctx, l.ID)
		return Linked{Item: media.Item{Kind: media.ItemAlbum, Album: &a}}, err
	case media.LinkArtist:
		a, err := c.Artist(ctx, l.ID)
		return Linked{Item: media.Item{Kind: media.ItemArtist, Artist: &a}}, err
	case media.LinkPlaylist:
		// The playlist of an album's tracks (OLAK5uy_…) names the album,
		// which YouTube Music shows as one.
		if l.Source == media.SourceYouTube && strings.HasPrefix(native, "OLAK5uy_") {
			album, err := c.a.youtube.AlbumOf(ctx, native)
			if err == nil {
				return c.link(ctx, media.Link{Kind: media.LinkAlbum, Source: media.SourceYouTube, ID: media.ID(media.SourceYouTube, album)})
			}
			if !errors.Is(err, youtube.ErrNotFound) {
				return Linked{}, err
			}
		}
		p, err := c.a.playlist(ctx, l.ID)
		if errors.Is(err, spotify.ErrPlaylistUnavailable) || errors.Is(err, spotify.ErrPlaylistNotShown) {
			p.ID, p.Source, err = l.ID, media.SourceSpotify, nil
		}
		return Linked{Item: media.Item{Kind: media.ItemPlaylist, Playlist: &p}}, err
	case media.LinkPodcast, media.LinkEpisode:
		id := l.ID
		if l.Kind == media.LinkEpisode {
			if id, err = c.showOf(ctx, native); err != nil {
				return Linked{}, err
			}
		}
		p, err := c.Podcast(ctx, id)
		return Linked{Item: media.Item{Kind: media.ItemPodcast, Podcast: &p}}, err
	}
	return Linked{}, fmt.Errorf("no link of %s", l.Kind)
}

// startOf splits tracks, a YouTube radio read from the track id, which
// it starts with, into that track and the rest; err is the read's.
func startOf(id string, tracks []media.Track, err error) (media.Track, []media.Track, error) {
	switch {
	case err != nil:
		return media.Track{}, nil, err
	case len(tracks) == 0 || tracks[0].ID != id:
		return media.Track{}, nil, youtube.ErrNotFound
	}
	return tracks[0], tracks[1:], nil
}

// showOf returns the ID of the podcast of the Spotify episode native.
func (c *Catalog) showOf(ctx context.Context, native string) (string, error) {
	d, err := c.a.spotify.EpisodeDetails(ctx, native)
	if err == nil && d.Show.ID == "" {
		err = errNoShow
	}
	return d.Show.ID, err
}

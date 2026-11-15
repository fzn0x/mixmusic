package youtube

import (
	"context"
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
)

// Artist returns an artist's page: their top songs, albums, singles and
// related artists, by their channel ID (UC…).
func (c *Client) Artist(ctx context.Context, channelID string) (media.Artist, error) {
	n, err := c.browse(ctx, channelID)
	if err != nil {
		return media.Artist{}, err
	}
	return parseArtist(n, channelID)
}

func parseArtist(n node, channelID string) (media.Artist, error) {
	var h node
	for _, name := range []string{"musicImmersiveHeaderRenderer", "musicVisualHeaderRenderer", "musicHeaderRenderer"} {
		if h = n.get("header", name); h.exists() {
			break
		}
	}
	if !h.exists() {
		return media.Artist{}, ErrNotFound
	}
	a := media.Artist{
		ID:          media.ID(media.SourceYouTube, channelID),
		Source:      media.SourceYouTube,
		Name:        h.get("title").text(),
		Artwork:     artwork(thumbnailList(h)),
		Description: strings.TrimSpace(h.get("description").text()),
		Subtitle:    h.get("subscriptionButton", "subscribeButtonRenderer", "subscriberCountText").text(),
		TopTracks:   []media.Track{},
		Albums:      []media.Album{},
		Singles:     []media.Album{},
		Related:     []media.Ref{},
		URL:         "https://music.youtube.com/channel/" + channelID,
	}
	a.Following = h.get("subscriptionButton", "subscribeButtonRenderer", "subscribed").boolean()
	if a.Subtitle != "" {
		a.Subtitle += " subscribers"
	}
	self := []media.Ref{{ID: a.ID, Name: a.Name}}
	sections := n.get("contents", "singleColumnBrowseResultsRenderer", "tabs", 0, "tabRenderer", "content", "sectionListRenderer", "contents")
	for _, s := range sections.list() {
		if shelf := s.get("musicShelfRenderer"); shelf.exists() && len(a.TopTracks) == 0 {
			a.TopTracks = tracksOf(parseItems(shelf.get("contents").list()))
			a.SongsID = songsList(shelf)
			continue
		}
		carousel := s.get("musicCarouselShelfRenderer")
		if !carousel.exists() {
			continue
		}
		title := carousel.get("header", "musicCarouselShelfBasicHeaderRenderer", "title").text()
		items := parseItems(carousel.get("contents").list())
		switch {
		case title == "Albums":
			a.Albums = append(a.Albums, albumsOf(items, media.AlbumKindAlbum, self)...)
		case strings.HasPrefix(title, "Singles"):
			a.Singles = append(a.Singles, albumsOf(items, media.AlbumKindSingle, self)...)
		case strings.HasPrefix(title, "Fans might also like"):
			for _, item := range items {
				if item.Kind == media.ItemArtist {
					a.Related = append(a.Related, media.Ref{ID: item.Artist.ID, Name: item.Artist.Name})
				}
			}
		}
	}
	return a, nil
}

// albumsOf returns the albums among items, completing what an artist's
// page leaves out of its cards.
func albumsOf(items []media.Item, kind media.AlbumKind, artists []media.Ref) []media.Album {
	albums := []media.Album{}
	for _, item := range items {
		if item.Kind != media.ItemAlbum {
			continue
		}
		al := *item.Album
		if al.Kind == "" {
			al.Kind = kind
		}
		if len(al.Artists) == 0 {
			al.Artists = artists
		}
		albums = append(albums, al)
	}
	return albums
}

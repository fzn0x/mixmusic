package youtube

import (
	"context"

	"github.com/fzn0x/mixmusic/internal/media"
)

// Album returns an album with its tracks, by its browse ID (MPREb_…).
func (c *Client) Album(ctx context.Context, browseID string) (media.Album, error) {
	n, err := c.browse(ctx, browseID)
	if err != nil {
		return media.Album{}, err
	}
	return parseAlbum(n, browseID)
}

func parseAlbum(n node, browseID string) (media.Album, error) {
	h := responsiveHeader(n)
	if !h.exists() {
		return media.Album{}, ErrNotFound
	}
	sub := parseByline(h.get("subtitle").runs())
	artists := parseByline(h.get("straplineTextOne").runs()).fallbackArtists()
	album := media.Album{
		ID:         media.ID(media.SourceYouTube, browseID),
		Source:     media.SourceYouTube,
		Title:      h.get("title").text(),
		Artists:    artists,
		Kind:       albumKind(sub.label),
		Year:       sub.year,
		Artwork:    artwork(thumbnailList(h)),
		TrackCount: trackCount(h.get("secondSubtitle").text()),
		URL:        "https://music.youtube.com/browse/" + browseID,
	}
	_, shelf := secondaryShelf(n)
	album.Tracks = tracksOf(parseItems(shelf.get("contents").list()))
	for _, item := range shelf.get("contents").list() {
		watch := flexRuns(item.get("musicResponsiveListItemRenderer"), 0)
		if len(watch) > 0 {
			if id := watch[0].get("navigationEndpoint", "watchEndpoint", "playlistId").str(); id != "" {
				album.PlaylistID = id
				break
			}
		}
	}
	album.Saved = savedInLibrary(h)
	ref := &media.Ref{ID: album.ID, Name: album.Title}
	for i := range album.Tracks {
		t := &album.Tracks[i]
		if len(t.Artists) == 0 {
			t.Artists = artists
		}
		t.Album = ref
		if len(t.Artwork) == 0 {
			t.Artwork = album.Artwork
		}
		album.Explicit = album.Explicit || t.Explicit
	}
	if album.TrackCount == 0 {
		album.TrackCount = len(album.Tracks)
	}
	return album, nil
}

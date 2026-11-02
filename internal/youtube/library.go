package youtube

import (
	"context"
	"errors"
	"strings"

	"github.com/fzn0x/mixmusic/internal/media"
)

// ErrSignedOut is returned for what needs the user signed in.
var ErrSignedOut = errors.New("youtube music: sign in to YouTube Music first")

// ErrSessionRejected is returned for what needs the account once YouTube
// stopped accepting the session.
var ErrSessionRejected = errors.New("YouTube Music no longer accepts this sign-in")

// maxLibraryPages bounds how many pages of a library list are read, of
// about two dozen playlists, albums or artists each.
const maxLibraryPages = 40

// likedSongs is the playlist ID of the user's liked songs.
const likedSongs = "LM"

// Account is the YouTube account the client is signed in as.
type Account struct {
	Name   string `json:"name"`
	Handle string `json:"handle,omitempty"`
	Photo  string `json:"photo,omitempty"`
}

// Account returns the account the client is signed in as.
func (c *Client) Account(ctx context.Context) (Account, error) {
	if err := c.session(); err != nil {
		return Account{}, err
	}
	n, err := c.call(ctx, "account/account_menu", map[string]any{}, nil)
	if err == nil {
		err = c.session()
	}
	if err != nil {
		return Account{}, err
	}
	a := parseAccount(n)
	if a.Name == "" {
		return Account{}, ErrSignedOut
	}
	return a, nil
}

func parseAccount(n node) Account {
	h := n.get("actions", 0, "openPopupAction", "popup", "multiPageMenuRenderer", "header", "activeAccountHeaderRenderer")
	a := Account{Name: h.get("accountName").text(), Handle: h.get("channelHandle").text()}
	if thumbs := h.get("accountPhoto", "thumbnails").list(); len(thumbs) > 0 {
		a.Photo = thumbs[len(thumbs)-1].get("url").str()
	}
	return a
}

// LikedSongs returns the user's liked songs on YouTube Music.
func (c *Client) LikedSongs(ctx context.Context) (media.Playlist, error) {
	if err := c.session(); err != nil {
		return media.Playlist{}, err
	}
	p, err := c.Playlist(ctx, likedSongs)
	if err == nil {
		err = c.session()
	}
	return p, err
}

// LibraryPlaylists returns the playlists the user made or saved.
func (c *Client) LibraryPlaylists(ctx context.Context) ([]media.Playlist, error) {
	items, err := c.libraryItems(ctx, "FEmusic_liked_playlists")
	playlists := []media.Playlist{}
	for _, it := range items {
		// Liked songs have a place of their own.
		if it.Kind == media.ItemPlaylist && it.Playlist.ID != media.ID(media.SourceYouTube, likedSongs) {
			playlists = append(playlists, *it.Playlist)
		}
	}
	return playlists, err
}

// LibraryAlbums returns the albums the user saved.
func (c *Client) LibraryAlbums(ctx context.Context) ([]media.Album, error) {
	items, err := c.libraryItems(ctx, "FEmusic_liked_albums")
	albums := []media.Album{}
	for _, it := range items {
		if it.Kind == media.ItemAlbum {
			albums = append(albums, *it.Album)
		}
	}
	return albums, err
}

// LibraryArtists returns the artists the user subscribes to.
func (c *Client) LibraryArtists(ctx context.Context) ([]media.Artist, error) {
	items, err := c.libraryItems(ctx, "FEmusic_library_corpus_artists")
	artists := []media.Artist{}
	for _, it := range items {
		if it.Kind == media.ItemArtist {
			artists = append(artists, *it.Artist)
		}
	}
	return artists, err
}

// libraryItems returns the items of a list of the user's library, page
// after page.
func (c *Client) libraryItems(ctx context.Context, browseID string) ([]media.Item, error) {
	if err := c.session(); err != nil {
		return nil, err
	}
	n, err := c.browse(ctx, browseID)
	if err == nil {
		err = c.session()
	}
	if err != nil {
		return nil, err
	}
	var items []media.Item
	for _, s := range parseShelves(n) {
		items = append(items, s.Items...)
	}
	token := listToken(n)
	for page := 1; token != "" && page < maxLibraryPages; page++ {
		more, err := c.call(ctx, "browse", map[string]any{"continuation": token}, nil)
		if err != nil {
			break // keep the items read so far
		}
		var contents []node
		contents, token = listContinuation(more)
		items = append(items, parseItems(contents)...)
	}
	return items, nil
}

// listToken returns the token of the next page of the grid or the list of
// a browse page, if it has more.
func listToken(n node) string {
	var find func(sections node) string
	find = func(sections node) string {
		for _, s := range sections.list() {
			if inner := s.get("itemSectionRenderer", "contents"); inner.exists() {
				if token := find(inner); token != "" {
					return token
				}
				continue
			}
			if grid := s.get("gridRenderer"); grid.exists() {
				return shelfToken(grid, grid.get("items").list())
			}
			if list := s.get("musicShelfRenderer"); list.exists() {
				return shelfToken(list, list.get("contents").list())
			}
		}
		return ""
	}
	return find(n.get("contents", "singleColumnBrowseResultsRenderer", "tabs", 0, "tabRenderer", "content", "sectionListRenderer", "contents"))
}

// listContinuation returns the items of the next page of a grid or a list,
// and the token of the page after it.
func listContinuation(n node) ([]node, string) {
	if items := n.get("onResponseReceivedActions", 0, "appendContinuationItemsAction", "continuationItems").list(); len(items) > 0 {
		return items, continuationToken(items[len(items)-1])
	}
	if grid := n.get("continuationContents", "gridContinuation"); grid.exists() {
		items := grid.get("items").list()
		return items, shelfToken(grid, items)
	}
	list := n.get("continuationContents", "musicShelfContinuation")
	contents := list.get("contents").list()
	return contents, shelfToken(list, contents)
}

// shelfToken returns the token of a grid's or a list's next page: in its
// continuations, or in the continuation item that closes its items.
func shelfToken(shelf node, items []node) string {
	if token := shelf.get("continuations", 0, "nextContinuationData", "continuation").str(); token != "" {
		return token
	}
	if len(items) > 0 {
		return continuationToken(items[len(items)-1])
	}
	return ""
}

// SetLiked likes a track on YouTube Music, by its video ID, or takes the
// like back.
func (c *Client) SetLiked(ctx context.Context, videoID string, liked bool) error {
	return c.rate(ctx, map[string]any{"videoId": videoID}, liked)
}

// SetSaved saves a playlist, or an album by its playlist (OLAK5uy_…), to
// the user's library, or takes it out.
func (c *Client) SetSaved(ctx context.Context, playlistID string, saved bool) error {
	return c.rate(ctx, map[string]any{"playlistId": playlistID}, saved)
}

// SetDisliked dislikes a track on YouTube Music, by its video ID, or takes
// the dislike back, which leaves it neither liked nor disliked.
func (c *Client) SetDisliked(ctx context.Context, videoID string, disliked bool) error {
	endpoint := "like/removelike"
	if disliked {
		endpoint = "like/dislike"
	}
	return c.rateAt(ctx, endpoint, map[string]any{"videoId": videoID})
}

// rate likes target, a video or a playlist, or takes the like back.
func (c *Client) rate(ctx context.Context, target map[string]any, on bool) error {
	endpoint := "like/removelike"
	if on {
		endpoint = "like/like"
	}
	return c.rateAt(ctx, endpoint, target)
}

// rateAt rates target at one of the endpoints that rate: like/like,
// like/dislike, or like/removelike, which takes either back. They take the
// same target, and answer 401 to a visitor.
func (c *Client) rateAt(ctx context.Context, endpoint string, target map[string]any) error {
	if err := c.session(); err != nil {
		return err
	}
	_, err := c.call(ctx, endpoint, map[string]any{"target": target}, nil)
	return err
}

// SetSubscribed subscribes the user to an artist, by its channel ID, or
// unsubscribes them.
func (c *Client) SetSubscribed(ctx context.Context, channelID string, on bool) error {
	if err := c.session(); err != nil {
		return err
	}
	endpoint := "subscription/unsubscribe"
	if on {
		endpoint = "subscription/subscribe"
	}
	// Artist pages are browsed as MPLAUC…; subscriptions take the channel.
	_, err := c.call(ctx, endpoint, map[string]any{"channelIds": []string{strings.TrimPrefix(channelID, "MPLA")}}, nil)
	return err
}

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

// maxLibraryPages bounds how many pages of a list of the user's library
// are read, of about two dozen playlists, albums, artists, songs or
// podcasts each: 1,000 pages, some 24,000 items. A list that goes on past
// it comes Cut. Tests lower it.
var maxLibraryPages = 1000

// errLibraryBound is why a list of the library stopped at maxLibraryPages.
var errLibraryBound = errors.New("youtube music: MixMusic reads no more pages of a list")

// errPagesGoRound is why a list of the library stopped at a page whose
// token names a page read before: read on, it would be read forever.
var errPagesGoRound = errors.New("youtube music: the pages of the list came round to one read before")

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

// LikedSongs returns the user's liked songs on YouTube Music, as the
// playlist of their first page: More reads on, as for any playlist.
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

// LibraryPlaylists returns the playlists the user made or saved, all of
// them: those of a list that stopped short come with a media.ShortList.
func (c *Client) LibraryPlaylists(ctx context.Context) ([]media.Playlist, error) {
	return libraryList(ctx, c, "FEmusic_liked_playlists", func(it media.Item) (media.Playlist, string, bool) {
		// Liked songs have a place of their own.
		if it.Kind != media.ItemPlaylist || it.Playlist.ID == media.ID(media.SourceYouTube, likedSongs) {
			return media.Playlist{}, "", false
		}
		return *it.Playlist, it.Playlist.ID, true
	})
}

// LibraryAlbums returns the albums the user saved, all of them: those of
// a list that stopped short come with a media.ShortList.
func (c *Client) LibraryAlbums(ctx context.Context) ([]media.Album, error) {
	return libraryList(ctx, c, "FEmusic_liked_albums", func(it media.Item) (media.Album, string, bool) {
		if it.Kind != media.ItemAlbum {
			return media.Album{}, "", false
		}
		return *it.Album, it.Album.ID, true
	})
}

// LibraryArtists returns the artists the user subscribes to, all of them:
// those of a list that stopped short come with a media.ShortList.
func (c *Client) LibraryArtists(ctx context.Context) ([]media.Artist, error) {
	return libraryList(ctx, c, "FEmusic_library_corpus_artists", func(it media.Item) (media.Artist, string, bool) {
		if it.Kind != media.ItemArtist {
			return media.Artist{}, "", false
		}
		return *it.Artist, it.Artist.ID, true
	})
}

// libraryList returns what pick takes of the items of a list of the user's
// library, browseID's, page after page, each once by the ID pick tells
// ("" keeps an item without telling it from the others). A page past the
// first that fails is tried again on media.PageWaits. A list that stops
// short, at a page that still fails, at a page whose token names one read
// before, or at maxLibraryPages, which Cut tells, comes in part, with a
// media.ShortList: YouTube Music tells no count of the list, so it has no
// Total. One whose first page failed, as signed out, comes with the error
// alone.
func libraryList[T any](ctx context.Context, c *Client, browseID string, pick func(media.Item) (T, string, bool)) ([]T, error) {
	out := []T{}
	if err := c.session(); err != nil {
		return out, err
	}
	n, err := c.browse(ctx, browseID)
	if err == nil {
		err = c.session()
	}
	if err != nil {
		return out, err
	}
	seen := map[string]bool{}
	take := func(items []media.Item) {
		for _, it := range items {
			v, id, ok := pick(it)
			if !ok || id != "" && seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, v)
		}
	}
	// What came so far goes to a reader that asked (media.WithPartial),
	// after each page. out only grows past what was handed over, so a
	// reader may keep what it got.
	handOver := func() {
		if partial := media.Partial[[]T](ctx); partial != nil && len(out) > 0 {
			partial(out[:len(out):len(out)])
		}
	}
	for _, s := range parseShelves(n) {
		take(s.Items)
	}
	handOver()
	token, asked := listToken(n), map[string]bool{}
	var end error
	for pages := 1; token != ""; pages++ {
		if pages >= maxLibraryPages {
			end = errLibraryBound
			break
		}
		if asked[token] {
			end = errPagesGoRound
			break
		}
		asked[token] = true
		var contents []node
		next := ""
		end = media.TryPage(ctx, func(ctx context.Context) error {
			more, err := c.call(ctx, "browse", map[string]any{"continuation": token}, nil)
			if err == nil {
				contents, next = listContinuation(more)
			}
			return err
		})
		if end == nil {
			// An answer as to a visitor ended the session: what it holds
			// isn't the user's.
			end = c.session()
		}
		if end != nil {
			break
		}
		before := len(out)
		take(parseItems(contents))
		if len(out) > before {
			handOver()
		}
		token = next
	}
	if end == nil {
		return out, nil
	}
	return out, &media.ShortList{Got: len(out), Cut: end == errLibraryBound, Err: end}
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

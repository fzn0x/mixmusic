package spotify

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/fzn0x/mixmusic/internal/media"
)

const (
	// searchLimit is the most results of each kind one search returns.
	searchLimit = 10
	// pageLimit is the most items of a page of most lists.
	pageLimit = 50
	// maxTracks bounds how many tracks of an album or playlist are read.
	maxTracks = 1000
)

// ErrPlaylistUnavailable is returned for playlists whose tracks Spotify
// does not show to apps: those the user neither owns nor collaborates on.
var ErrPlaylistUnavailable = errors.New("spotify only lets apps read the tracks of playlists you own or collaborate on")

// Profile is the signed-in user.
type Profile struct {
	ID    string        `json:"id"`
	Name  string        `json:"name"`
	Image []media.Image `json:"image"`
}

// Me returns the signed-in user.
func (c *Client) Me(ctx context.Context) (Profile, error) {
	var me struct {
		ID          string  `json:"id"`
		DisplayName string  `json:"display_name"`
		Images      []image `json:"images"`
	}
	if err := c.get(ctx, "/me", nil, &me); err != nil {
		return Profile{}, err
	}
	name := me.DisplayName
	if name == "" {
		name = me.ID
	}
	return Profile{ID: me.ID, Name: name, Image: images(me.Images)}, nil
}

// Search returns the tracks, albums, artists and playlists Spotify finds for
// query.
func (c *Client) Search(ctx context.Context, query string) (media.SearchResults, error) {
	var res struct {
		Tracks    paging[*track]    `json:"tracks"`
		Albums    paging[*album]    `json:"albums"`
		Artists   paging[*artist]   `json:"artists"`
		Playlists paging[*playlist] `json:"playlists"`
	}
	q := url.Values{"q": {query}, "type": {"track,album,artist,playlist"}, "limit": {strconv.Itoa(searchLimit)}}
	if err := c.get(ctx, "/search", q, &res); err != nil {
		return media.SearchResults{}, err
	}
	out := media.SearchResults{
		Tracks:    toTracks(res.Tracks.Items, nil),
		Albums:    []media.Album{},
		Artists:   []media.Artist{},
		Playlists: []media.Playlist{},
	}
	for _, a := range res.Albums.Items {
		if a != nil {
			out.Albums = append(out.Albums, toAlbum(a))
		}
	}
	for _, a := range res.Artists.Items {
		if a != nil {
			out.Artists = append(out.Artists, toArtist(a))
		}
	}
	for _, p := range res.Playlists.Items {
		if p != nil {
			out.Playlists = append(out.Playlists, toPlaylist(p, ""))
		}
	}
	return out, nil
}

// Album returns an album with its tracks.
func (c *Client) Album(ctx context.Context, id string) (media.Album, error) {
	var a album
	if err := c.get(ctx, "/albums/"+url.PathEscape(id), nil, &a); err != nil {
		return media.Album{}, err
	}
	out := toAlbum(&a)
	if a.Tracks == nil {
		return out, nil
	}
	for next := a.Tracks.Next; next != "" && len(out.Tracks) < maxTracks; {
		var page paging[*track]
		if err := c.get(ctx, next, nil, &page); err != nil {
			break // keep the tracks read so far
		}
		out.Tracks = append(out.Tracks, toTracks(page.Items, &a)...)
		next = page.Next
	}
	return out, nil
}

// Artist returns an artist with their albums and singles, and their most
// popular tracks as search finds them.
func (c *Client) Artist(ctx context.Context, id string) (media.Artist, error) {
	var a artist
	if err := c.get(ctx, "/artists/"+url.PathEscape(id), nil, &a); err != nil {
		return media.Artist{}, err
	}
	out := toArtist(&a)
	var wg sync.WaitGroup
	wg.Go(func() {
		var albums paging[*album]
		q := url.Values{"include_groups": {"album,single"}, "limit": {strconv.Itoa(pageLimit)}}
		if c.get(ctx, "/artists/"+url.PathEscape(id)+"/albums", q, &albums) != nil {
			return
		}
		for _, al := range albums.Items {
			if al == nil {
				continue
			}
			if converted := toAlbum(al); converted.Kind == media.AlbumKindAlbum {
				out.Albums = append(out.Albums, converted)
			} else {
				out.Singles = append(out.Singles, converted)
			}
		}
	})
	wg.Go(func() {
		var res struct {
			Tracks paging[*track] `json:"tracks"`
		}
		q := url.Values{"q": {`artist:"` + a.Name + `"`}, "type": {"track"}, "limit": {strconv.Itoa(searchLimit)}}
		if c.get(ctx, "/search", q, &res) != nil {
			return
		}
		for _, t := range toTracks(res.Tracks.Items, nil) {
			for _, r := range t.Artists {
				if r.ID == out.ID {
					out.TopTracks = append(out.TopTracks, t)
					break
				}
			}
		}
	})
	wg.Wait()
	return out, nil
}

// Playlist returns a playlist with its tracks. Spotify only shows the
// tracks of playlists the user owns or collaborates on.
func (c *Client) Playlist(ctx context.Context, id, userID string) (media.Playlist, error) {
	var p playlist
	if err := c.get(ctx, "/playlists/"+url.PathEscape(id), nil, &p); err != nil {
		return media.Playlist{}, err
	}
	out := toPlaylist(&p, userID)
	entries := p.entries()
	if entries == nil {
		if !out.Editable {
			return out, ErrPlaylistUnavailable
		}
		entries = &paging[playlistItem]{Next: c.BaseURL + "/playlists/" + url.PathEscape(id) + "/items?limit=" + strconv.Itoa(pageLimit)}
	}
	for next := entries.Next; next != "" && len(out.Tracks) < maxTracks; {
		var page paging[playlistItem]
		if err := c.get(ctx, next, nil, &page); err != nil {
			break
		}
		out.Tracks = append(out.Tracks, playlistTracks(page.Items)...)
		out.TrackCount = max(out.TrackCount, page.Total)
		next = page.Next
	}
	return out, nil
}

// MyPlaylists returns the playlists the user made or follows.
func (c *Client) MyPlaylists(ctx context.Context, userID string) ([]media.Playlist, error) {
	out := []media.Playlist{}
	next := "/me/playlists?limit=" + strconv.Itoa(pageLimit)
	for page := 0; next != "" && page < 4; page++ {
		var res paging[*playlist]
		if err := c.get(ctx, next, nil, &res); err != nil {
			if page == 0 {
				return nil, err
			}
			break
		}
		for _, p := range res.Items {
			if p != nil {
				out = append(out, toPlaylist(p, userID))
			}
		}
		next = res.Next
	}
	return out, nil
}

// SavedTracks returns a page of the user's liked songs, from offset, and
// how many they have.
func (c *Client) SavedTracks(ctx context.Context, offset int) ([]media.Track, int, error) {
	var res paging[struct {
		Track *track `json:"track"`
	}]
	q := url.Values{"limit": {strconv.Itoa(pageLimit)}, "offset": {strconv.Itoa(offset)}}
	if err := c.get(ctx, "/me/tracks", q, &res); err != nil {
		return nil, 0, err
	}
	tracks := make([]*track, len(res.Items))
	for i, it := range res.Items {
		tracks[i] = it.Track
	}
	return toTracks(tracks, nil), res.Total, nil
}

// SavedAlbums returns the albums the user saved.
func (c *Client) SavedAlbums(ctx context.Context) ([]media.Album, error) {
	var res paging[struct {
		Album *album `json:"album"`
	}]
	if err := c.get(ctx, "/me/albums", url.Values{"limit": {strconv.Itoa(pageLimit)}}, &res); err != nil {
		return nil, err
	}
	out := []media.Album{}
	for _, it := range res.Items {
		if it.Album != nil {
			al := toAlbum(it.Album)
			al.Tracks = []media.Track{}
			out = append(out, al)
		}
	}
	return out, nil
}

// FollowedArtists returns the artists the user follows.
func (c *Client) FollowedArtists(ctx context.Context) ([]media.Artist, error) {
	out := []media.Artist{}
	after := ""
	for page := 0; page < 4; page++ {
		var res struct {
			Artists paging[*artist] `json:"artists"`
		}
		q := url.Values{"type": {"artist"}, "limit": {strconv.Itoa(pageLimit)}}
		if after != "" {
			q.Set("after", after)
		}
		if err := c.get(ctx, "/me/following", q, &res); err != nil {
			if page == 0 {
				return nil, err
			}
			break
		}
		for _, a := range res.Artists.Items {
			if a != nil {
				out = append(out, toArtist(a))
			}
		}
		if after = res.Artists.Cursors.After; after == "" || res.Artists.Next == "" {
			break
		}
	}
	return out, nil
}

// RecentlyPlayed returns the tracks the user played last on Spotify.
func (c *Client) RecentlyPlayed(ctx context.Context) ([]media.Track, error) {
	var res paging[struct {
		Track *track `json:"track"`
	}]
	if err := c.get(ctx, "/me/player/recently-played", url.Values{"limit": {strconv.Itoa(pageLimit)}}, &res); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var tracks []*track
	for _, it := range res.Items {
		if it.Track != nil && !seen[it.Track.ID] {
			seen[it.Track.ID] = true
			tracks = append(tracks, it.Track)
		}
	}
	return toTracks(tracks, nil), nil
}

// TopTracks returns the tracks the user played most lately.
func (c *Client) TopTracks(ctx context.Context) ([]media.Track, error) {
	var res paging[*track]
	q := url.Values{"limit": {"20"}, "time_range": {"short_term"}}
	if err := c.get(ctx, "/me/top/tracks", q, &res); err != nil {
		return nil, err
	}
	return toTracks(res.Items, nil), nil
}

// Home returns shelves made of the user's Spotify library: their
// playlists, recent and top tracks, and saved albums. Shelves whose
// requests fail are left out; once all of them failed, the error tells
// why.
func (c *Client) Home(ctx context.Context, userID string) ([]media.Shelf, error) {
	var (
		playlists   []media.Playlist
		recent, top []media.Track
		albums      []media.Album
		errs        [4]error
		wg          sync.WaitGroup
	)
	wg.Go(func() { playlists, errs[0] = c.MyPlaylists(ctx, userID) })
	wg.Go(func() { recent, errs[1] = c.RecentlyPlayed(ctx) })
	wg.Go(func() { top, errs[2] = c.TopTracks(ctx) })
	wg.Go(func() { albums, errs[3] = c.SavedAlbums(ctx) })
	wg.Wait()
	if errs[0] != nil && errs[1] != nil && errs[2] != nil && errs[3] != nil {
		return nil, errs[0]
	}

	shelves := []media.Shelf{}
	add := func(title string, items []media.Item) {
		if len(items) > 0 {
			shelves = append(shelves, media.Shelf{Title: title, Source: media.SourceSpotify, Items: items})
		}
	}
	add("Recently played on Spotify", trackItems(recent))
	add("Your top tracks", trackItems(top))
	var items []media.Item
	for i := range playlists {
		items = append(items, media.Item{Kind: media.ItemPlaylist, Playlist: &playlists[i]})
	}
	add("Your Spotify playlists", items)
	items = nil
	for i := range albums {
		items = append(items, media.Item{Kind: media.ItemAlbum, Album: &albums[i]})
	}
	add("Saved albums", items)
	return shelves, nil
}

func trackItems(tracks []media.Track) []media.Item {
	items := make([]media.Item, len(tracks))
	for i := range tracks {
		items[i] = media.Item{Kind: media.ItemTrack, Track: &tracks[i]}
	}
	return items
}

// URI returns the Spotify URI of an item of a kind ("track", "album",
// "artist" or "playlist") from its ID in the API.
func URI(kind, id string) string { return "spotify:" + kind + ":" + id }

// joinURIs joins at most 40 URIs, the most the library endpoints take.
func joinURIs(uris []string) string {
	if len(uris) > 40 {
		uris = uris[:40]
	}
	return strings.Join(uris, ",")
}

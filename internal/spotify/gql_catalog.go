package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

const gqlSearchLimit = 20

// Shared Pathfinder JSON types.

type gqlImageSource struct {
	URL       string `json:"url"`
	Width     *int   `json:"width"`
	Height    *int   `json:"height"`
	MaxWidth  *int   `json:"maxWidth"`
	MaxHeight *int   `json:"maxHeight"`
}

type gqlImageList struct {
	Sources []gqlImageSource `json:"sources"`
}

type gqlPlaylistImages struct {
	Items []gqlImageList `json:"items"`
}

type gqlSimpleArtist struct {
	URI     string `json:"uri"`
	ID      string `json:"id"`
	Profile struct {
		Name string `json:"name"`
	} `json:"profile"`
}

type gqlDuration struct {
	TotalMilliseconds int `json:"totalMilliseconds"`
	TotalMs           int `json:"totalMs"`
	Milliseconds      int `json:"milliseconds"`
}

func (d *gqlDuration) ms() int {
	if d == nil {
		return 0
	}
	if d.TotalMilliseconds > 0 {
		return d.TotalMilliseconds
	}
	if d.TotalMs > 0 {
		return d.TotalMs
	}
	return d.Milliseconds
}

type gqlAlbumRef struct {
	URI      string       `json:"uri"`
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	CoverArt gqlImageList `json:"coverArt"`
}

type gqlTrack struct {
	TypeName      string       `json:"__typename"`
	URI           string       `json:"uri"`
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	TrackNumber   int          `json:"trackNumber"`
	DiscNumber    int          `json:"discNumber"`
	Duration      *gqlDuration `json:"duration"`
	TrackDuration *gqlDuration `json:"trackDuration"`
	DurationMs    int          `json:"durationMs"`
	ContentRating struct {
		Label string `json:"label"`
	} `json:"contentRating"`
	Artists struct {
		Items []gqlSimpleArtist `json:"items"`
	} `json:"artists"`
	FirstArtist struct {
		Items []gqlSimpleArtist `json:"items"`
	} `json:"firstArtist"`
	OtherArtists struct {
		Items []gqlSimpleArtist `json:"items"`
	} `json:"otherArtists"`
	AlbumOfTrack *gqlAlbumRef `json:"albumOfTrack"`
}

type gqlAlbumData struct {
	TypeName string       `json:"__typename"`
	URI      string       `json:"uri"`
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Type     string       `json:"type"`
	CoverArt gqlImageList `json:"coverArt"`
	Date     struct {
		Year      int    `json:"year"`
		Month     int    `json:"month"`
		Day       int    `json:"day"`
		Precision string `json:"precision"`
		ISOString string `json:"isoString"`
	} `json:"date"`
	Artists struct {
		Items []gqlSimpleArtist `json:"items"`
	} `json:"artists"`
	Tracks *struct {
		TotalCount int `json:"totalCount"`
	} `json:"tracks"`
	TracksV2 *struct {
		TotalCount int `json:"totalCount"`
		Items      []struct {
			Track *gqlTrack `json:"track"`
		} `json:"items"`
	} `json:"tracksV2"`
}

type gqlArtistData struct {
	TypeName string `json:"__typename"`
	URI      string `json:"uri"`
	ID       string `json:"id"`
	Profile  struct {
		Name string `json:"name"`
	} `json:"profile"`
	Visuals struct {
		AvatarImage gqlImageList `json:"avatarImage"`
	} `json:"visuals"`
}

type gqlPlaylistOwner struct {
	Data struct {
		TypeName string `json:"__typename"`
		URI      string `json:"uri"`
		ID       string `json:"id"`
		Username string `json:"username"`
		Name     string `json:"name"`
	} `json:"data"`
}

type gqlPlaylistData struct {
	TypeName       string            `json:"__typename"`
	URI            string            `json:"uri"`
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	Images         gqlPlaylistImages `json:"images"`
	OwnerV2        gqlPlaylistOwner  `json:"ownerV2"`
	Following      bool              `json:"following"`
	Collaborative  bool              `json:"collaborative"`
	BasePermission string            `json:"basePermission"`
	Members        struct {
		TotalCount int `json:"totalCount"`
		Items      []struct {
			IsOwner         bool   `json:"isOwner"`
			PermissionLevel string `json:"permissionLevel"`
		} `json:"items"`
	} `json:"members"`
	CurrentUserCapabilities struct {
		CanEditItems bool `json:"canEditItems"`
	} `json:"currentUserCapabilities"`
	Content *struct {
		TotalCount int               `json:"totalCount"`
		Items      []gqlPlaylistItem `json:"items"`
	} `json:"content"`
	Attributes []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"attributes"`
}

type gqlPlaylistItem struct {
	UID    string `json:"uid"`
	ItemV2 struct {
		TypeName string          `json:"__typename"`
		URI      string          `json:"_uri"`
		Data     json.RawMessage `json:"data"`
	} `json:"itemV2"`
}

func uriID(uri string) string {
	if uri == "" {
		return ""
	}
	if i := strings.LastIndex(uri, ":"); i >= 0 {
		return uri[i+1:]
	}
	return uri
}

func gqlImages(sources []gqlImageSource) []media.Image {
	out := make([]media.Image, 0, len(sources))
	for _, s := range sources {
		if s.URL == "" {
			continue
		}
		w, h := 0, 0
		if s.Width != nil {
			w = *s.Width
		} else if s.MaxWidth != nil {
			w = *s.MaxWidth
		}
		if s.Height != nil {
			h = *s.Height
		} else if s.MaxHeight != nil {
			h = *s.MaxHeight
		}
		out = append(out, media.Image{URL: s.URL, Width: w, Height: h})
	}
	slices.SortStableFunc(out, func(a, b media.Image) int { return a.Width - b.Width })
	return out
}

func gqlPlaylistCover(images gqlPlaylistImages) []media.Image {
	var all []gqlImageSource
	for _, group := range images.Items {
		all = append(all, group.Sources...)
	}
	return gqlImages(all)
}

func gqlArtistRefs(items []gqlSimpleArtist) []media.Ref {
	out := make([]media.Ref, 0, len(items))
	for _, a := range items {
		name := strings.TrimSpace(a.Profile.Name)
		if name == "" {
			continue
		}
		id := a.ID
		if id == "" {
			id = uriID(a.URI)
		}
		r := media.Ref{Name: name}
		if id != "" {
			r.ID = media.ID(media.SourceSpotify, id)
		}
		out = append(out, r)
	}
	return out
}

func gqlTrackToMedia(t *gqlTrack, uriOverride string, fromAlbum *media.Album) (media.Track, bool) {
	if t == nil {
		return media.Track{}, false
	}
	if t.TypeName != "" && t.TypeName != "Track" {
		return media.Track{}, false
	}
	id := t.ID
	if id == "" {
		id = uriID(t.URI)
	}
	if id == "" {
		id = uriID(uriOverride)
	}
	if id == "" {
		return media.Track{}, false
	}
	dur := t.Duration.ms()
	if dur == 0 {
		dur = t.TrackDuration.ms()
	}
	if dur == 0 {
		dur = t.DurationMs
	}
	artists := gqlArtistRefs(t.Artists.Items)
	if len(artists) == 0 && (len(t.FirstArtist.Items) > 0 || len(t.OtherArtists.Items) > 0) {
		combined := make([]gqlSimpleArtist, 0, len(t.FirstArtist.Items)+len(t.OtherArtists.Items))
		combined = append(combined, t.FirstArtist.Items...)
		combined = append(combined, t.OtherArtists.Items...)
		artists = gqlArtistRefs(combined)
	}
	mt := media.Track{
		ID:          media.ID(media.SourceSpotify, id),
		Source:      media.SourceSpotify,
		Title:       t.Name,
		Artists:     artists,
		DurationMs:  dur,
		Explicit:    strings.EqualFold(t.ContentRating.Label, "EXPLICIT"),
		Artwork:     []media.Image{},
		URL:         "https://open.spotify.com/track/" + url.PathEscape(id),
		TrackNumber: t.TrackNumber,
		DiscNumber:  t.DiscNumber,
	}
	if t.AlbumOfTrack != nil {
		alID := t.AlbumOfTrack.ID
		if alID == "" {
			alID = uriID(t.AlbumOfTrack.URI)
		}
		if alID != "" || t.AlbumOfTrack.Name != "" {
			var fullID string
			if alID != "" {
				fullID = media.ID(media.SourceSpotify, alID)
			}
			mt.Album = &media.Ref{ID: fullID, Name: t.AlbumOfTrack.Name}
		}
		mt.Artwork = gqlImages(t.AlbumOfTrack.CoverArt.Sources)
	} else if fromAlbum != nil {
		mt.Album = &media.Ref{ID: fromAlbum.ID, Name: fromAlbum.Title}
		mt.Artwork = fromAlbum.Artwork
	}
	return mt, true
}

func gqlAlbumKind(raw string) media.AlbumKind {
	k := media.AlbumKind(strings.ToLower(strings.TrimSpace(raw)))
	if k == "compilation" || k == "" {
		return media.AlbumKindAlbum
	}
	return k
}

func gqlAlbumYear(a *gqlAlbumData) string {
	if a.Date.Year > 0 {
		return strconv.Itoa(a.Date.Year)
	}
	if a.Date.ISOString != "" {
		y, _, _ := strings.Cut(a.Date.ISOString, "-")
		return y
	}
	return ""
}

func gqlAlbumReleaseDate(a *gqlAlbumData) string {
	if a.Date.ISOString != "" {
		date, _, _ := strings.Cut(a.Date.ISOString, "T")
		return releaseDate(date, strings.ToLower(a.Date.Precision))
	}
	switch {
	case a.Date.Year > 0 && a.Date.Month > 0 && a.Date.Day > 0:
		return fmt.Sprintf("%04d-%02d-%02d", a.Date.Year, a.Date.Month, a.Date.Day)
	case a.Date.Year > 0 && a.Date.Month > 0:
		return fmt.Sprintf("%04d-%02d", a.Date.Year, a.Date.Month)
	case a.Date.Year > 0:
		return strconv.Itoa(a.Date.Year)
	}
	return ""
}

func gqlAlbumToMedia(a *gqlAlbumData) media.Album {
	id := a.ID
	if id == "" {
		id = uriID(a.URI)
	}
	total := 0
	if a.TracksV2 != nil {
		total = a.TracksV2.TotalCount
	} else if a.Tracks != nil {
		total = a.Tracks.TotalCount
	}
	out := media.Album{
		ID:         media.ID(media.SourceSpotify, id),
		Source:     media.SourceSpotify,
		Title:      a.Name,
		Artists:    gqlArtistRefs(a.Artists.Items),
		Kind:       gqlAlbumKind(a.Type),
		Year:       gqlAlbumYear(a),
		Artwork:    gqlImages(a.CoverArt.Sources),
		TrackCount: total,
		Tracks:     []media.Track{},
		URL:        "https://open.spotify.com/album/" + url.PathEscape(id),
	}
	if a.TracksV2 != nil {
		for _, item := range a.TracksV2.Items {
			if mt, ok := gqlTrackToMedia(item.Track, "", &out); ok {
				out.Tracks = append(out.Tracks, mt)
			}
		}
		out.Explicit = slices.ContainsFunc(out.Tracks, func(t media.Track) bool { return t.Explicit })
	}
	return out
}

func gqlArtistToMedia(a *gqlArtistData) media.Artist {
	id := a.ID
	if id == "" {
		id = uriID(a.URI)
	}
	return media.Artist{
		ID:        media.ID(media.SourceSpotify, id),
		Source:    media.SourceSpotify,
		Name:      a.Profile.Name,
		Artwork:   gqlImages(a.Visuals.AvatarImage.Sources),
		TopTracks: []media.Track{},
		Albums:    []media.Album{},
		Singles:   []media.Album{},
		Related:   []media.Ref{},
		URL:       "https://open.spotify.com/artist/" + url.PathEscape(id),
	}
}

func gqlPlaylistOwnerID(o gqlPlaylistOwner) string {
	if o.Data.ID != "" {
		return o.Data.ID
	}
	if id := uriID(o.Data.URI); id != "" {
		return id
	}
	return o.Data.Username
}

func gqlPlaylistCollaborative(p *gqlPlaylistData) bool {
	if p.Collaborative || strings.EqualFold(p.BasePermission, "CONTRIBUTOR") {
		return true
	}
	for _, m := range p.Members.Items {
		if !m.IsOwner && strings.EqualFold(m.PermissionLevel, "CONTRIBUTOR") {
			return true
		}
	}
	return false
}

func gqlPlaylistToMedia(p *gqlPlaylistData, fallbackID, userID string) media.Playlist {
	id := p.ID
	if id == "" {
		id = uriID(p.URI)
	}
	if id == "" {
		id = fallbackID
	}
	ownerID := gqlPlaylistOwnerID(p.OwnerV2)
	ownerName := p.OwnerV2.Data.Name
	if ownerName == "" {
		ownerName = ownerID
	}
	collab := gqlPlaylistCollaborative(p)
	mine := userID != "" && ownerID == userID
	editable := p.CurrentUserCapabilities.CanEditItems || mine
	out := media.Playlist{
		ID:            media.ID(media.SourceSpotify, id),
		Source:        media.SourceSpotify,
		Title:         p.Name,
		Description:   p.Description,
		Owner:         ownerName,
		Artwork:       gqlPlaylistCover(p.Images),
		Tracks:        []media.Track{},
		Mine:          mine,
		Collaborative: collab,
		Editable:      editable,
		Saved:         p.Following,
		URL:           "https://open.spotify.com/playlist/" + url.PathEscape(id),
	}
	if p.Content != nil {
		out.TrackCount = p.Content.TotalCount
		out.Tracks, out.Unplayable = gqlPlaylistTracks(p.Content.Items)
	}
	return out
}

func gqlPlaylistTracks(items []gqlPlaylistItem) ([]media.Track, media.Unplayable) {
	out := []media.Track{}
	var left media.Unplayable
	for _, it := range items {
		raw := bytesTrimSpace(it.ItemV2.Data)
		if len(raw) == 0 {
			left.Gone++
			continue
		}
		var kind struct {
			TypeName string `json:"__typename"`
		}
		if json.Unmarshal(raw, &kind) != nil {
			left.Gone++
			continue
		}
		switch kind.TypeName {
		case "LocalTrack":
			left.Local++
		case "Episode":
			left.Episodes++
		case "Track", "":
			var tr gqlTrack
			if json.Unmarshal(raw, &tr) == nil {
				if mt, ok := gqlTrackToMedia(&tr, it.ItemV2.URI, nil); ok {
					out = append(out, mt)
					continue
				}
			}
			left.Gone++
		default:
			if strings.Contains(strings.ToLower(kind.TypeName), "episode") {
				left.Episodes++
			} else {
				left.Gone++
			}
		}
	}
	return out, left
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

// ── Profile ─────────────────────────────────────────────────────────────

func (c *Client) gqlMe(ctx context.Context) (Profile, error) {
	var res struct {
		Me *struct {
			Profile *struct {
				URI      string       `json:"uri"`
				Name     string       `json:"name"`
				Username string       `json:"username"`
				Avatar   gqlImageList `json:"avatar"`
			} `json:"profile"`
		} `json:"me"`
	}
	if err := c.gql(ctx, "profileAttributes", map[string]any{}, &res); err != nil {
		return Profile{}, err
	}
	if res.Me == nil || res.Me.Profile == nil {
		return Profile{}, ErrSignInExpired
	}
	p := res.Me.Profile
	id := uriID(p.URI)
	if id == "" {
		id = p.Username
	}
	name := p.Name
	if name == "" {
		name = id
	}
	return Profile{ID: id, Name: name, Image: gqlImages(p.Avatar.Sources)}, nil
}

// ── Search ──────────────────────────────────────────────────────────────

type gqlSearchResponse struct {
	SearchV2 *struct {
		TracksV2 struct {
			Items []struct {
				Item struct {
					TypeName string    `json:"__typename"`
					URI      string    `json:"_uri"`
					Data     *gqlTrack `json:"data"`
				} `json:"item"`
			} `json:"items"`
		} `json:"tracksV2"`
		AlbumsV2 struct {
			Items []struct {
				TypeName string        `json:"__typename"`
				Data     *gqlAlbumData `json:"data"`
			} `json:"items"`
		} `json:"albumsV2"`
		Artists struct {
			Items []struct {
				TypeName string         `json:"__typename"`
				Data     *gqlArtistData `json:"data"`
			} `json:"items"`
		} `json:"artists"`
		Playlists struct {
			Items []struct {
				TypeName string           `json:"__typename"`
				Data     *gqlPlaylistData `json:"data"`
			} `json:"items"`
		} `json:"playlists"`
		Podcasts struct {
			Items []struct {
				TypeName string   `json:"__typename"`
				Data     *gqlShow `json:"data"`
			} `json:"items"`
		} `json:"podcasts"`
		Episodes struct {
			Items []struct {
				TypeName string      `json:"__typename"`
				Data     *gqlEpisode `json:"data"`
			} `json:"items"`
		} `json:"episodes"`
	} `json:"searchV2"`
}

func (c *Client) gqlSearchDesktop(ctx context.Context, query string, limit int) (gqlSearchResponse, error) {
	vars := map[string]any{
		"searchTerm":                    query,
		"offset":                        0,
		"limit":                         limit,
		"numberOfTopResults":            5,
		"includeAudiobooks":             false,
		"includeArtistHasConcertsField": false,
		"includePreReleases":            false,
		"includeLocalConcertsField":     false,
		"includeAuthors":                false,
	}
	var res gqlSearchResponse
	err := c.gql(ctx, "searchDesktop", vars, &res)
	return res, err
}

func (c *Client) gqlSearch(ctx context.Context, query, userID string) (media.SearchResults, error) {
	res, err := c.gqlSearchDesktop(ctx, query, gqlSearchLimit)
	if err != nil {
		return media.SearchResults{}, err
	}
	out := media.SearchResults{
		Tracks:    []media.Track{},
		Videos:    []media.Track{},
		Albums:    []media.Album{},
		Artists:   []media.Artist{},
		Playlists: []media.Playlist{},
	}
	if res.SearchV2 == nil {
		return out, nil
	}
	for _, it := range res.SearchV2.TracksV2.Items {
		if mt, ok := gqlTrackToMedia(it.Item.Data, it.Item.URI, nil); ok {
			out.Tracks = append(out.Tracks, mt)
		}
	}
	for _, it := range res.SearchV2.AlbumsV2.Items {
		if it.Data != nil && (it.Data.URI != "" || it.Data.ID != "") {
			out.Albums = append(out.Albums, gqlAlbumToMedia(it.Data))
		}
	}
	for _, it := range res.SearchV2.Artists.Items {
		if it.Data != nil && (it.Data.URI != "" || it.Data.ID != "") {
			out.Artists = append(out.Artists, gqlArtistToMedia(it.Data))
		}
	}
	for _, it := range res.SearchV2.Playlists.Items {
		if it.Data != nil && (it.Data.URI != "" || it.Data.ID != "") {
			out.Playlists = append(out.Playlists, gqlPlaylistToMedia(it.Data, "", userID))
		}
	}
	return out, nil
}

// ── Album & Track ───────────────────────────────────────────────────────

func (c *Client) gqlAlbum(ctx context.Context, id string) (media.Album, error) {
	vars := map[string]any{
		"uri":    URI("album", id),
		"locale": "",
		"offset": 0,
		"limit":  pageLimit,
	}
	var res struct {
		AlbumUnion *gqlAlbumData `json:"albumUnion"`
	}
	if err := c.gql(ctx, "getAlbum", vars, &res); err != nil {
		return media.Album{}, err
	}
	if res.AlbumUnion == nil || (res.AlbumUnion.Name == "" && res.AlbumUnion.URI == "") {
		return media.Album{}, &APIError{Status: http.StatusNotFound, Message: "album not found"}
	}
	if res.AlbumUnion.ID == "" {
		res.AlbumUnion.ID = id
	}
	out := gqlAlbumToMedia(res.AlbumUnion)
	total := out.TrackCount
	read := func(ctx context.Context, next string) ([]media.Track, string, error) {
		off, _ := strconv.Atoi(next)
		var pageRes struct {
			AlbumUnion *struct {
				TracksV2 *struct {
					TotalCount int `json:"totalCount"`
					Items      []struct {
						Track *gqlTrack `json:"track"`
					} `json:"items"`
				} `json:"tracksV2"`
			} `json:"albumUnion"`
		}
		pv := map[string]any{"uri": URI("album", id), "locale": "", "offset": off, "limit": pageLimit}
		if err := c.gql(ctx, "queryAlbumTracks", pv, &pageRes); err != nil {
			return nil, "", err
		}
		if pageRes.AlbumUnion == nil || pageRes.AlbumUnion.TracksV2 == nil || len(pageRes.AlbumUnion.TracksV2.Items) == 0 {
			return nil, "", nil
		}
		var tracks []media.Track
		for _, it := range pageRes.AlbumUnion.TracksV2.Items {
			if mt, ok := gqlTrackToMedia(it.Track, "", &out); ok {
				tracks = append(tracks, mt)
			}
		}
		nextOff := off + len(pageRes.AlbumUnion.TracksV2.Items)
		after := ""
		if nextOff < pageRes.AlbumUnion.TracksV2.TotalCount {
			after = strconv.Itoa(nextOff)
		}
		return tracks, after, nil
	}
	for next := ""; total > len(out.Tracks) && len(out.Tracks) < media.MaxTracks && len(out.Tracks) > 0; {
		if next == "" {
			next = strconv.Itoa(len(out.Tracks))
		}
		tracks, after, err := media.ReadPage(ctx, read, next)
		if err != nil {
			return media.Album{}, err
		}
		if len(tracks) == 0 {
			break
		}
		out.Tracks, next = append(out.Tracks, tracks...), after
		if next == "" {
			break
		}
	}
	if len(out.Tracks) > media.MaxTracks {
		out.Tracks = out.Tracks[:media.MaxTracks]
	}
	out.Explicit = slices.ContainsFunc(out.Tracks, func(t media.Track) bool { return t.Explicit })
	return out, nil
}

func (c *Client) gqlTrack(ctx context.Context, id string) (media.Track, error) {
	var res struct {
		TrackUnion *gqlTrack `json:"trackUnion"`
	}
	if err := c.gql(ctx, "getTrack", map[string]any{"uri": URI("track", id)}, &res); err != nil {
		return media.Track{}, err
	}
	mt, ok := gqlTrackToMedia(res.TrackUnion, "", nil)
	if !ok || mt.ID != media.ID(media.SourceSpotify, id) {
		return media.Track{}, errNotATrack
	}
	return mt, nil
}

// ── Artist & Discography ────────────────────────────────────────────────

type gqlDiscographyReleases struct {
	Items []struct {
		Releases struct {
			Items []*gqlAlbumData `json:"items"`
		} `json:"releases"`
	} `json:"items"`
	TotalCount int `json:"totalCount"`
}

func (c *Client) gqlArtist(ctx context.Context, id string) (media.Artist, error) {
	var res struct {
		ArtistUnion *struct {
			TypeName string `json:"__typename"`
			URI      string `json:"uri"`
			ID       string `json:"id"`
			Profile  struct {
				Name string `json:"name"`
			} `json:"profile"`
			Visuals struct {
				AvatarImage gqlImageList `json:"avatarImage"`
			} `json:"visuals"`
			Discography struct {
				TopTracks struct {
					Items []struct {
						Track *gqlTrack `json:"track"`
					} `json:"items"`
				} `json:"topTracks"`
				Albums       gqlDiscographyReleases `json:"albums"`
				Singles      gqlDiscographyReleases `json:"singles"`
				Compilations gqlDiscographyReleases `json:"compilations"`
			} `json:"discography"`
			RelatedContent struct {
				RelatedArtists struct {
					Items []gqlArtistData `json:"items"`
				} `json:"relatedArtists"`
			} `json:"relatedContent"`
		} `json:"artistUnion"`
	}
	if err := c.gql(ctx, "queryArtistOverview", map[string]any{"uri": URI("artist", id), "locale": ""}, &res); err != nil {
		return media.Artist{}, err
	}
	u := res.ArtistUnion
	if u == nil || (u.Profile.Name == "" && u.URI == "") {
		return media.Artist{}, &APIError{Status: http.StatusNotFound, Message: "artist not found"}
	}
	out := gqlArtistToMedia(&gqlArtistData{
		URI:     u.URI,
		ID:      id,
		Profile: u.Profile,
		Visuals: u.Visuals,
	})
	for _, tt := range u.Discography.TopTracks.Items {
		if mt, ok := gqlTrackToMedia(tt.Track, "", nil); ok {
			out.TopTracks = append(out.TopTracks, mt)
		}
	}
	for _, group := range u.Discography.Albums.Items {
		for _, al := range group.Releases.Items {
			if al != nil {
				out.Albums = append(out.Albums, gqlAlbumToMedia(al))
			}
		}
	}
	for _, group := range u.Discography.Singles.Items {
		for _, al := range group.Releases.Items {
			if al != nil {
				out.Singles = append(out.Singles, gqlAlbumToMedia(al))
			}
		}
	}
	for _, rel := range u.RelatedContent.RelatedArtists.Items {
		rid := rel.ID
		if rid == "" {
			rid = uriID(rel.URI)
		}
		if rid != "" && rel.Profile.Name != "" {
			out.Related = append(out.Related, media.Ref{
				ID:   media.ID(media.SourceSpotify, rid),
				Name: rel.Profile.Name,
			})
		}
	}
	return out, nil
}

func (c *Client) gqlDiscographyPage(ctx context.Context, id string, offset, limit int) ([]*gqlAlbumData, int, error) {
	var res struct {
		ArtistUnion *struct {
			Discography struct {
				All gqlDiscographyReleases `json:"all"`
			} `json:"discography"`
		} `json:"artistUnion"`
	}
	vars := map[string]any{
		"uri":    URI("artist", id),
		"offset": offset,
		"limit":  limit,
		"order":  "DATE_DESC",
	}
	if err := c.gql(ctx, "queryArtistDiscographyAll", vars, &res); err != nil {
		return nil, 0, err
	}
	if res.ArtistUnion == nil {
		return nil, 0, nil
	}
	var out []*gqlAlbumData
	for _, item := range res.ArtistUnion.Discography.All.Items {
		for _, r := range item.Releases.Items {
			if r != nil && (r.ID != "" || r.URI != "") {
				out = append(out, r)
			}
		}
	}
	return out, res.ArtistUnion.Discography.All.TotalCount, nil
}

func (c *Client) gqlArtistReleasesList(ctx context.Context, id string) ([]media.Release, error) {
	items, _, err := c.gqlDiscographyPage(ctx, id, 0, 50)
	if err != nil {
		return nil, err
	}
	out := make([]media.Release, 0, len(items))
	for _, r := range items {
		out = append(out, media.Release{
			Album:    gqlAlbumToMedia(r),
			Released: gqlAlbumReleaseDate(r),
		})
	}
	return out, nil
}

func (c *Client) gqlArtistReleasesForSongs(ctx context.Context, id string) (releases []*album, missed int, err error) {
	first, total, err := c.gqlDiscographyPage(ctx, id, 0, pageLimit)
	if err != nil {
		return nil, 0, err
	}
	pages := [][]*gqlAlbumData{first}
	for offset := pageLimit; offset < total && offset <= maxListOffset; offset += pageLimit {
		pages = append(pages, nil)
	}
	errs := make([]error, len(pages))
	slots := make(chan struct{}, listPagesAtOnce)
	var wg sync.WaitGroup
	for i := 1; i < len(pages); i++ {
		slots <- struct{}{}
		wg.Go(func() {
			defer func() { <-slots }()
			errs[i] = media.TryPage(ctx, func(ctx context.Context) error {
				var perr error
				pages[i], _, perr = c.gqlDiscographyPage(ctx, id, i*pageLimit, pageLimit)
				return perr
			})
		})
	}
	wg.Wait()
	for i, page := range pages {
		if errs[i] != nil {
			missed += min(pageLimit, total-i*pageLimit)
			if err == nil {
				err = errs[i]
			}
			continue
		}
		for _, al := range page {
			aid := al.ID
			if aid == "" {
				aid = uriID(al.URI)
			}
			if aid != "" {
				releases = append(releases, &album{ID: aid, Name: al.Name})
			}
		}
	}
	return releases, missed, err
}

// ── Playlists ───────────────────────────────────────────────────────────

const gqlPlaylistCursorPrefix = "gql:playlist:"

func gqlPlaylistCursor(id string, offset int) string {
	return gqlPlaylistCursorPrefix + id + ":" + strconv.Itoa(offset)
}

func parseGQLPlaylistCursor(next string) (id string, offset int, ok bool) {
	rest, found := strings.CutPrefix(next, gqlPlaylistCursorPrefix)
	if !found {
		return "", 0, false
	}
	id, offStr, found := strings.Cut(rest, ":")
	if !found || id == "" {
		return "", 0, false
	}
	off, err := strconv.Atoi(offStr)
	if err != nil || off < 0 {
		return "", 0, false
	}
	return id, off, true
}

func (c *Client) gqlFetchPlaylist(ctx context.Context, id string, offset, limit int) (*gqlPlaylistData, error) {
	var res struct {
		PlaylistV2 *gqlPlaylistData `json:"playlistV2"`
	}
	vars := map[string]any{
		"uri":                       URI("playlist", id),
		"offset":                    offset,
		"limit":                     limit,
		"enableWatchFeedEntrypoint": false,
	}
	if err := c.gql(ctx, "fetchPlaylist", vars, &res); err != nil {
		return nil, err
	}
	return res.PlaylistV2, nil
}

func (c *Client) gqlPlaylist(ctx context.Context, id, userID string) (media.Playlist, error) {
	const limit = 100
	p, err := c.gqlFetchPlaylist(ctx, id, 0, limit)
	if err != nil {
		var api *APIError
		if errors.As(err, &api) && api.Status == http.StatusNotFound {
			return media.Playlist{
				ID:     media.ID(media.SourceSpotify, id),
				Source: media.SourceSpotify,
				URL:    "https://open.spotify.com/playlist/" + url.PathEscape(id),
			}, ErrPlaylistNotShown
		}
		return media.Playlist{}, err
	}
	if p == nil || p.TypeName == "NotFound" || p.TypeName == "GenericError" || (p.URI == "" && p.Name == "") {
		return media.Playlist{
			ID:     media.ID(media.SourceSpotify, id),
			Source: media.SourceSpotify,
			URL:    "https://open.spotify.com/playlist/" + url.PathEscape(id),
		}, ErrPlaylistNotShown
	}
	out := gqlPlaylistToMedia(p, id, userID)
	if p.Content != nil && len(p.Content.Items) > 0 && len(p.Content.Items) < p.Content.TotalCount {
		out.More = gqlPlaylistCursor(id, len(p.Content.Items))
	}
	return out, nil
}

func (c *Client) gqlPlaylistPage(ctx context.Context, id string, offset int) ([]media.Track, string, media.Unplayable, error) {
	const limit = 100
	p, err := c.gqlFetchPlaylist(ctx, id, offset, limit)
	if err != nil {
		return nil, "", media.Unplayable{}, err
	}
	if p == nil || p.Content == nil || len(p.Content.Items) == 0 {
		return nil, "", media.Unplayable{}, nil
	}
	tracks, left := gqlPlaylistTracks(p.Content.Items)
	nextOffset := offset + len(p.Content.Items)
	next := ""
	if nextOffset < p.Content.TotalCount {
		next = gqlPlaylistCursor(id, nextOffset)
	}
	return tracks, next, left, nil
}

// ── Library (libraryV3 & fetchLibraryTracks) ────────────────────────────

type gqlLibraryV3Response struct {
	Me *struct {
		LibraryV3 *struct {
			TotalCount int `json:"totalCount"`
			PagingInfo struct {
				Limit  int `json:"limit"`
				Offset int `json:"offset"`
			} `json:"pagingInfo"`
			Items []struct {
				Item struct {
					TypeName string          `json:"__typename"`
					URI      string          `json:"_uri"`
					Data     json.RawMessage `json:"data"`
				} `json:"item"`
			} `json:"items"`
		} `json:"libraryV3"`
	} `json:"me"`
}

func (c *Client) gqlLibraryPage(ctx context.Context, filter string, offset, limit int) (gqlLibraryV3Response, error) {
	vars := map[string]any{
		"filters":                      []string{filter},
		"order":                        nil,
		"textFilter":                   "",
		"features":                     []string{"LIKED_SONGS", "YOUR_EPISODES_V2", "PRERELEASES", "EVENTS"},
		"limit":                        limit,
		"offset":                       offset,
		"flatten":                      true,
		"expandedFolders":              []string{},
		"folderUri":                    nil,
		"includeFoldersWhenFlattening": false,
	}
	var res gqlLibraryV3Response
	err := c.gql(ctx, "libraryV3", vars, &res)
	return res, err
}

func readGQLLibrary[T any](ctx context.Context, c *Client, filter string, take func(typeName, uri string, data json.RawMessage) (id string, val T, ok bool)) ([]T, error) {
	first, err := c.gqlLibraryPage(ctx, filter, 0, pageLimit)
	if err != nil {
		return nil, err
	}
	if first.Me == nil || first.Me.LibraryV3 == nil {
		return []T{}, nil
	}
	total := first.Me.LibraryV3.TotalCount
	seen := map[string]bool{}
	var items []T
	consume := func(r gqlLibraryV3Response) (bool, error) {
		if r.Me == nil || r.Me.LibraryV3 == nil || len(r.Me.LibraryV3.Items) == 0 {
			return false, nil
		}
		known, fresh := 0, 0
		for _, raw := range r.Me.LibraryV3.Items {
			id, val, ok := take(raw.Item.TypeName, raw.Item.URI, raw.Item.Data)
			if !ok || id == "" {
				continue
			}
			if seen[id] {
				known++
				continue
			}
			seen[id] = true
			items = append(items, val)
			fresh++
		}
		if len(r.Me.LibraryV3.Items) < pageLimit {
			return false, nil
		}
		if known > 0 && fresh == 0 {
			return false, errListEnded
		}
		return true, nil
	}

	more, end := consume(first)
	for offset := pageLimit; more; {
		if offset > maxListOffset {
			end = errListBound
			break
		}
		n := min(max((total-offset+pageLimit-1)/pageLimit, 1), listPagesAtOnce, (maxListOffset-offset)/pageLimit+1)
		pages, errs := make([]gqlLibraryV3Response, n), make([]error, n)
		var wg sync.WaitGroup
		for i := range n {
			wg.Go(func() {
				errs[i] = media.TryPage(ctx, func(ctx context.Context) error {
					var perr error
					pages[i], perr = c.gqlLibraryPage(ctx, filter, offset+i*pageLimit, pageLimit)
					return perr
				})
			})
		}
		wg.Wait()
		for i := 0; i < n && more; i++ {
			if errs[i] != nil {
				more, end = false, errs[i]
				break
			}
			more, end = consume(pages[i])
		}
		offset += n * pageLimit
	}
	return items, shortList(len(items), total, end)
}

func parseLibraryPlaylist(typeName, uri string, raw json.RawMessage, userID string) (string, media.Playlist, bool) {
	if !strings.Contains(strings.ToLower(typeName), "playlist") {
		return "", media.Playlist{}, false
	}
	var p gqlPlaylistData
	if json.Unmarshal(raw, &p) != nil {
		return "", media.Playlist{}, false
	}
	if p.TypeName != "" && p.TypeName != "Playlist" {
		return "", media.Playlist{}, false
	}
	if p.URI == "" {
		p.URI = uri
	}
	id := p.ID
	if id == "" {
		id = uriID(p.URI)
	}
	if id == "" {
		return "", media.Playlist{}, false
	}
	return id, gqlPlaylistToMedia(&p, id, userID), true
}

func parseLibraryAlbum(typeName, uri string, raw json.RawMessage) (string, media.Album, bool) {
	if !strings.Contains(strings.ToLower(typeName), "album") {
		return "", media.Album{}, false
	}
	var a gqlAlbumData
	if json.Unmarshal(raw, &a) != nil {
		return "", media.Album{}, false
	}
	if a.URI == "" {
		a.URI = uri
	}
	id := a.ID
	if id == "" {
		id = uriID(a.URI)
	}
	if id == "" {
		return "", media.Album{}, false
	}
	al := gqlAlbumToMedia(&a)
	al.Tracks = []media.Track{}
	return id, al, true
}

func parseLibraryArtist(typeName, uri string, raw json.RawMessage) (string, media.Artist, bool) {
	if !strings.Contains(strings.ToLower(typeName), "artist") {
		return "", media.Artist{}, false
	}
	var a gqlArtistData
	if json.Unmarshal(raw, &a) != nil {
		return "", media.Artist{}, false
	}
	if a.URI == "" {
		a.URI = uri
	}
	id := a.ID
	if id == "" {
		id = uriID(a.URI)
	}
	if id == "" {
		return "", media.Artist{}, false
	}
	return id, gqlArtistToMedia(&a), true
}

func (c *Client) gqlMyPlaylists(ctx context.Context, userID string) ([]media.Playlist, error) {
	items, err := readGQLLibrary(ctx, c, "Playlists", func(tn, uri string, raw json.RawMessage) (string, media.Playlist, bool) {
		return parseLibraryPlaylist(tn, uri, raw, userID)
	})
	if !came(err) {
		return nil, err
	}
	if items == nil {
		items = []media.Playlist{}
	}
	return items, err
}

func (c *Client) gqlSavedAlbums(ctx context.Context) ([]media.Album, error) {
	items, err := readGQLLibrary(ctx, c, "Albums", parseLibraryAlbum)
	if !came(err) {
		return nil, err
	}
	if items == nil {
		items = []media.Album{}
	}
	return items, err
}

func (c *Client) gqlFollowedArtists(ctx context.Context) ([]media.Artist, error) {
	items, err := readGQLLibrary(ctx, c, "Artists", parseLibraryArtist)
	if !came(err) {
		return nil, err
	}
	if items == nil {
		items = []media.Artist{}
	}
	return items, err
}

func (c *Client) gqlSavedTracks(ctx context.Context, offset int) ([]media.Track, int, error) {
	var res struct {
		Me *struct {
			Library *struct {
				Tracks *struct {
					TotalCount int `json:"totalCount"`
					Items      []struct {
						Track struct {
							URI  string    `json:"_uri"`
							Data *gqlTrack `json:"data"`
						} `json:"track"`
					} `json:"items"`
				} `json:"tracks"`
			} `json:"library"`
		} `json:"me"`
	}
	if err := c.gql(ctx, "fetchLibraryTracks", map[string]any{"offset": offset, "limit": pageLimit}, &res); err != nil {
		return nil, 0, err
	}
	out := []media.Track{}
	total := 0
	if res.Me != nil && res.Me.Library != nil && res.Me.Library.Tracks != nil {
		total = res.Me.Library.Tracks.TotalCount
		for _, it := range res.Me.Library.Tracks.Items {
			if mt, ok := gqlTrackToMedia(it.Track.Data, it.Track.URI, nil); ok {
				out = append(out, mt)
			}
		}
	}
	return out, total, nil
}

// ── Home ────────────────────────────────────────────────────────────────

func (c *Client) gqlHome(ctx context.Context, userID string) ([]media.Shelf, error) {
	tz := time.Now().Location().String()
	if tz == "" || tz == "Local" {
		tz = "UTC"
	}
	vars := map[string]any{
		"homeEndUserIntegration":         "INTEGRATION_WEB_PLAYER",
		"timeZone":                       tz,
		"sp_t":                           c.spT(),
		"facet":                          "",
		"sectionItemsLimit":              20,
		"includeEpisodeContentRatingsV2": false,
	}
	var res struct {
		Home *struct {
			SectionContainer *struct {
				Sections struct {
					Items []struct {
						URI  string `json:"uri"`
						Data struct {
							TypeName string `json:"__typename"`
							Title    struct {
								TransformedLabel   string `json:"transformedLabel"`
								TranslatedBaseText string `json:"translatedBaseText"`
								Text               string `json:"text"`
							} `json:"title"`
						} `json:"data"`
						SectionItems struct {
							Items []struct {
								Content struct {
									TypeName string          `json:"__typename"`
									URI      string          `json:"_uri"`
									Data     json.RawMessage `json:"data"`
								} `json:"content"`
							} `json:"items"`
						} `json:"sectionItems"`
					} `json:"items"`
				} `json:"sections"`
			} `json:"sectionContainer"`
		} `json:"home"`
	}
	homeErr := c.gql(ctx, "home", vars, &res)
	shelves := []media.Shelf{}
	if homeErr == nil && res.Home != nil && res.Home.SectionContainer != nil {
		for idx, sec := range res.Home.SectionContainer.Sections.Items {
			title := sec.Data.Title.TransformedLabel
			if title == "" {
				title = sec.Data.Title.TranslatedBaseText
			}
			if title == "" {
				title = sec.Data.Title.Text
			}
			var items []media.Item
			for _, rawItem := range sec.SectionItems.Items {
				rawData := bytesTrimSpace(rawItem.Content.Data)
				if len(rawData) == 0 {
					continue
				}
				switch rawItem.Content.TypeName {
				case "PlaylistResponseWrapper":
					var p gqlPlaylistData
					if json.Unmarshal(rawData, &p) == nil && (p.URI != "" || p.ID != "") {
						pl := gqlPlaylistToMedia(&p, "", userID)
						items = append(items, media.Item{Kind: media.ItemPlaylist, Playlist: &pl})
					}
				case "AlbumResponseWrapper":
					var a gqlAlbumData
					if json.Unmarshal(rawData, &a) == nil && (a.URI != "" || a.ID != "") {
						al := gqlAlbumToMedia(&a)
						items = append(items, media.Item{Kind: media.ItemAlbum, Album: &al})
					}
				case "ArtistResponseWrapper":
					var a gqlArtistData
					if json.Unmarshal(rawData, &a) == nil && (a.URI != "" || a.ID != "") {
						ar := gqlArtistToMedia(&a)
						items = append(items, media.Item{Kind: media.ItemArtist, Artist: &ar})
					}
				case "TrackResponseWrapper":
					var t gqlTrack
					if json.Unmarshal(rawData, &t) == nil {
						if mt, ok := gqlTrackToMedia(&t, rawItem.Content.URI, nil); ok {
							items = append(items, media.Item{Kind: media.ItemTrack, Track: &mt})
						}
					}
				}
			}
			if len(items) == 0 || title == "" {
				continue
			}
			key := strings.TrimPrefix(sec.URI, "spotify:section:")
			if key == "" {
				if sec.Data.TypeName == "HomeShortsSectionData" {
					key = "shortcuts"
				} else {
					key = "home-" + strconv.Itoa(idx)
				}
			}
			shelves = append(shelves, media.Shelf{
				Title:  title,
				Source: media.SourceSpotify,
				Items:  items,
				Key:    key,
			})
		}
	}
	if len(shelves) > 0 {
		return shelves, nil
	}
	// Fallback to the user's library shelves if Home feed returned nothing.
	var (
		playlists []media.Playlist
		albums    []media.Album
		errs      [2]error
		wg        sync.WaitGroup
	)
	wg.Go(func() {
		r, err := c.gqlLibraryPage(ctx, "Playlists", 0, pageLimit)
		if err != nil {
			errs[0] = err
			return
		}
		if r.Me != nil && r.Me.LibraryV3 != nil {
			for _, it := range r.Me.LibraryV3.Items {
				if _, pl, ok := parseLibraryPlaylist(it.Item.TypeName, it.Item.URI, it.Item.Data, userID); ok {
					playlists = append(playlists, pl)
				}
			}
		}
	})
	wg.Go(func() {
		r, err := c.gqlLibraryPage(ctx, "Albums", 0, pageLimit)
		if err != nil {
			errs[1] = err
			return
		}
		if r.Me != nil && r.Me.LibraryV3 != nil {
			for _, it := range r.Me.LibraryV3.Items {
				if _, al, ok := parseLibraryAlbum(it.Item.TypeName, it.Item.URI, it.Item.Data); ok {
					albums = append(albums, al)
				}
			}
		}
	})
	wg.Wait()
	if errs[0] != nil && errs[1] != nil {
		if homeErr != nil {
			return nil, homeErr
		}
		return nil, errs[0]
	}
	if len(playlists) > 0 {
		items := make([]media.Item, len(playlists))
		for i := range playlists {
			items[i] = media.Item{Kind: media.ItemPlaylist, Playlist: &playlists[i]}
		}
		shelves = append(shelves, media.Shelf{Title: "Your Spotify playlists", Source: media.SourceSpotify, Items: items, Key: "playlists"})
	}
	if len(albums) > 0 {
		items := make([]media.Item, len(albums))
		for i := range albums {
			items[i] = media.Item{Kind: media.ItemAlbum, Album: &albums[i]}
		}
		shelves = append(shelves, media.Shelf{Title: "Saved albums", Source: media.SourceSpotify, Items: items, Key: "albums"})
	}
	return shelves, nil
}

// ── Library & Playlist Mutations ────────────────────────────────────────

func (c *Client) gqlSave(ctx context.Context, add bool, uris ...string) error {
	if len(uris) > 40 {
		uris = uris[:40]
	}
	op := "removeFromLibrary"
	if add {
		op = "addToLibrary"
	}
	return c.gql(ctx, op, map[string]any{"libraryItemUris": uris}, nil)
}

func (c *Client) gqlFollowPlaylist(ctx context.Context, playlistID string, on bool) error {
	uri := URI("playlist", playlistID)
	op := "removeItemsFromRootlist"
	if on {
		op = "addItemsToRootlist"
	}
	err := c.gql(ctx, op, map[string]any{"uris": []string{uri}}, nil)
	if err == nil {
		return nil
	}
	return c.gqlSave(ctx, on, uri)
}

func (c *Client) gqlContains(ctx context.Context, uris ...string) ([]bool, error) {
	if len(uris) > 40 {
		uris = uris[:40]
	}
	var res struct {
		Lookup []struct {
			TypeName string `json:"__typename"`
			Data     *struct {
				Saved     *bool `json:"saved"`
				IsCurated *bool `json:"isCurated"`
			} `json:"data"`
		} `json:"lookup"`
	}
	if err := c.gql(ctx, "areEntitiesInLibrary", map[string]any{"uris": uris}, &res); err != nil {
		return nil, err
	}
	out := make([]bool, len(uris))
	for i := range uris {
		if i < len(res.Lookup) && res.Lookup[i].Data != nil {
			d := res.Lookup[i].Data
			if d.Saved != nil {
				out[i] = *d.Saved
				continue
			}
			if d.IsCurated != nil {
				out[i] = *d.IsCurated
				continue
			}
		}
		if strings.HasPrefix(uris[i], "spotify:playlist:") {
			if pl, err := c.gqlFetchPlaylist(ctx, uriID(uris[i]), 0, 1); err == nil && pl != nil {
				out[i] = pl.Following
			}
		}
	}
	return out, nil
}

func (c *Client) gqlAddToPlaylist(ctx context.Context, playlistID string, uris ...string) error {
	vars := map[string]any{
		"playlistUri":      URI("playlist", playlistID),
		"playlistItemUris": uris,
		"newPosition": map[string]any{
			"moveType": "BOTTOM_OF_PLAYLIST",
			"fromUid":  nil,
		},
	}
	return refusedChange(c.gql(ctx, "addToPlaylist", vars, nil))
}

func (c *Client) gqlMoveInPlaylist(ctx context.Context, playlistID string, from, to int) error {
	if from < 0 || to < 0 {
		return errors.New("spotify: no such place in the playlist")
	}
	if from == to {
		return nil
	}
	fromPage, err := c.gqlFetchPlaylist(ctx, playlistID, from, 1)
	if err != nil {
		return refusedChange(err)
	}
	if fromPage == nil || fromPage.Content == nil || len(fromPage.Content.Items) == 0 || fromPage.Content.Items[0].UID == "" {
		return errors.New("spotify: no such place in the playlist")
	}
	fromUID := fromPage.Content.Items[0].UID
	targetIdx := to
	if to > from {
		targetIdx = to + 1
	}
	newPos := map[string]any{"moveType": "BOTTOM_OF_PLAYLIST", "fromUid": nil}
	if targetIdx < fromPage.Content.TotalCount {
		toPage, err := c.gqlFetchPlaylist(ctx, playlistID, targetIdx, 1)
		if err != nil {
			return refusedChange(err)
		}
		if toPage != nil && toPage.Content != nil && len(toPage.Content.Items) > 0 && toPage.Content.Items[0].UID != "" {
			newPos = map[string]any{
				"moveType": "BEFORE_UID",
				"fromUid":  toPage.Content.Items[0].UID,
			}
		}
	}
	vars := map[string]any{
		"playlistUri": URI("playlist", playlistID),
		"uids":        []string{fromUID},
		"newPosition": newPos,
	}
	return refusedChange(c.gql(ctx, "moveItemsInPlaylist", vars, nil))
}

// ── Podcasts & Episodes ─────────────────────────────────────────────────

type gqlShow struct {
	TypeName    string       `json:"__typename"`
	URI         string       `json:"uri"`
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	CoverArt    gqlImageList `json:"coverArt"`
	Publisher   struct {
		Name string `json:"name"`
	} `json:"publisher"`
	EpisodesV2 *struct {
		TotalCount int `json:"totalCount"`
	} `json:"episodesV2"`
}

type gqlEpisode struct {
	TypeName        string       `json:"__typename"`
	URI             string       `json:"uri"`
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Description     string       `json:"description"`
	HTMLDescription string       `json:"htmlDescription"`
	Duration        *gqlDuration `json:"duration"`
	CoverArt        gqlImageList `json:"coverArt"`
	ContentRating   struct {
		Label string `json:"label"`
	} `json:"contentRating"`
	ReleaseDate struct {
		ISOString string `json:"isoString"`
		Precision string `json:"precision"`
	} `json:"releaseDate"`
	PlayedState *struct {
		PlayPositionMilliseconds int    `json:"playPositionMilliseconds"`
		State                    string `json:"state"`
	} `json:"playedState"`
	PodcastV2 *struct {
		Data *gqlShow `json:"data"`
	} `json:"podcastV2"`
}

func gqlShowToPodcast(s *gqlShow) media.Podcast {
	id := s.ID
	if id == "" {
		id = uriID(s.URI)
	}
	p := media.Podcast{
		ID:          media.ID(media.SourceSpotify, id),
		Source:      media.SourceSpotify,
		Title:       s.Name,
		Authors:     []media.Ref{},
		Description: s.Description,
		Artwork:     gqlImages(s.CoverArt.Sources),
		Episodes:    []media.Track{},
		URL:         "https://open.spotify.com/show/" + url.PathEscape(id),
	}
	if s.Publisher.Name != "" {
		p.Authors = []media.Ref{{Name: s.Publisher.Name}}
	}
	if s.EpisodesV2 != nil {
		p.EpisodeCount = s.EpisodesV2.TotalCount
	}
	return p
}

func gqlEpisodeToMedia(e *gqlEpisode, uriOverride string, of media.Ref) (media.Track, bool) {
	if e == nil {
		return media.Track{}, false
	}
	id := e.ID
	if id == "" {
		id = uriID(e.URI)
	}
	if id == "" {
		id = uriID(uriOverride)
	}
	if id == "" {
		return media.Track{}, false
	}
	if e.PodcastV2 != nil && e.PodcastV2.Data != nil {
		sid := e.PodcastV2.Data.ID
		if sid == "" {
			sid = uriID(e.PodcastV2.Data.URI)
		}
		if sid != "" {
			of = media.Ref{ID: media.ID(media.SourceSpotify, sid), Name: e.PodcastV2.Data.Name}
		}
	}
	date, _, _ := strings.Cut(e.ReleaseDate.ISOString, "T")
	ep := &media.Episode{Show: of, Released: date}
	if e.PlayedState != nil {
		ep.ResumeMs = e.PlayedState.PlayPositionMilliseconds
		ep.Played = strings.EqualFold(e.PlayedState.State, "PLAYED")
	}
	t := media.Track{
		ID:         media.ID(media.SourceSpotify, id),
		Source:     media.SourceSpotify,
		Title:      e.Name,
		Artists:    []media.Ref{},
		DurationMs: e.Duration.ms(),
		Explicit:   strings.EqualFold(e.ContentRating.Label, "EXPLICIT"),
		Artwork:    gqlImages(e.CoverArt.Sources),
		URL:        "https://open.spotify.com/episode/" + url.PathEscape(id),
		Episode:    ep,
	}
	if of.Name != "" {
		t.Artists = []media.Ref{{Name: of.Name}}
	}
	return t, true
}

func (c *Client) gqlSearchPodcasts(ctx context.Context, query string) (media.PodcastResults, error) {
	res, err := c.gqlSearchDesktop(ctx, query, gqlSearchLimit)
	if err != nil {
		return media.PodcastResults{}, err
	}
	out := media.EmptyPodcastResults()
	if res.SearchV2 == nil {
		return out, nil
	}
	for _, it := range res.SearchV2.Podcasts.Items {
		if it.Data != nil && (it.Data.URI != "" || it.Data.ID != "") {
			out.Podcasts = append(out.Podcasts, gqlShowToPodcast(it.Data).Summary())
		}
	}
	for _, it := range res.SearchV2.Episodes.Items {
		if mt, ok := gqlEpisodeToMedia(it.Data, "", media.Ref{}); ok {
			out.Episodes = append(out.Episodes, mt)
		}
	}
	return out, nil
}

func (c *Client) gqlShow(ctx context.Context, id string) (media.Podcast, error) {
	var meta struct {
		PodcastUnionV2 *gqlShow `json:"podcastUnionV2"`
	}
	if err := c.gql(ctx, "queryShowMetadataV2", map[string]any{"uri": URI("show", id)}, &meta); err != nil {
		return media.Podcast{}, err
	}
	if meta.PodcastUnionV2 == nil || (meta.PodcastUnionV2.Name == "" && meta.PodcastUnionV2.URI == "") {
		return media.Podcast{}, &APIError{Status: http.StatusNotFound, Message: "show not found"}
	}
	if meta.PodcastUnionV2.ID == "" {
		meta.PodcastUnionV2.ID = id
	}
	p := gqlShowToPodcast(meta.PodcastUnionV2)
	page, err := c.gqlShowEpisodesPage(ctx, id, media.Ref{ID: p.ID, Name: p.Title}, 0)
	if err == nil {
		p.Episodes, p.More = page.Episodes, page.More
	}
	return p, nil
}

func (c *Client) gqlShowEpisodes(ctx context.Context, of media.Ref, more string) (media.EpisodePage, error) {
	_, id, err := media.ParseID(of.ID)
	if err != nil {
		return media.EpisodePage{}, err
	}
	offset, _ := strconv.Atoi(more)
	return c.gqlShowEpisodesPage(ctx, id, of, offset)
}

func (c *Client) gqlShowEpisodesPage(ctx context.Context, showID string, of media.Ref, offset int) (media.EpisodePage, error) {
	var res struct {
		PodcastUnionV2 *struct {
			EpisodesV2 *struct {
				TotalCount int `json:"totalCount"`
				Items      []struct {
					Entity struct {
						URI  string      `json:"_uri"`
						Data *gqlEpisode `json:"data"`
					} `json:"entity"`
				} `json:"items"`
			} `json:"episodesV2"`
		} `json:"podcastUnionV2"`
	}
	vars := map[string]any{"uri": URI("show", showID), "offset": offset, "limit": episodeLimit}
	if err := c.gql(ctx, "queryPodcastEpisodes", vars, &res); err != nil {
		return media.EpisodePage{}, err
	}
	page := media.EpisodePage{Episodes: []media.Track{}}
	if res.PodcastUnionV2 == nil || res.PodcastUnionV2.EpisodesV2 == nil {
		return page, nil
	}
	epV2 := res.PodcastUnionV2.EpisodesV2
	for _, it := range epV2.Items {
		if mt, ok := gqlEpisodeToMedia(it.Entity.Data, it.Entity.URI, of); ok {
			page.Episodes = append(page.Episodes, mt)
		}
	}
	nextOff := offset + len(epV2.Items)
	if len(epV2.Items) > 0 && nextOff < epV2.TotalCount {
		page.More = strconv.Itoa(nextOff)
	}
	return page, nil
}

func (c *Client) gqlEpisodeDetails(ctx context.Context, id string) (media.EpisodeDetails, error) {
	var res struct {
		EpisodeUnionV2 *gqlEpisode `json:"episodeUnionV2"`
	}
	if err := c.gql(ctx, "getEpisodeOrChapter", map[string]any{"uri": URI("episode", id)}, &res); err != nil {
		return media.EpisodeDetails{}, err
	}
	e := res.EpisodeUnionV2
	if e == nil {
		return media.EpisodeDetails{}, &APIError{Status: http.StatusNotFound, Message: "episode not found"}
	}
	text := e.Description
	if strings.TrimSpace(text) == "" {
		text = media.PlainText(e.HTMLDescription)
	}
	released, _, _ := strings.Cut(e.ReleaseDate.ISOString, "T")
	d := media.EpisodeDetails{
		Released:    released,
		DurationMs:  e.Duration.ms(),
		Description: media.ParseDescription(text),
	}
	if e.PodcastV2 != nil && e.PodcastV2.Data != nil {
		sid := e.PodcastV2.Data.ID
		if sid == "" {
			sid = uriID(e.PodcastV2.Data.URI)
		}
		if sid != "" {
			d.Show = media.Ref{ID: media.ID(media.SourceSpotify, sid), Name: e.PodcastV2.Data.Name}
		}
	}
	return d, nil
}

func (c *Client) gqlSavedShows(ctx context.Context) ([]media.Podcast, error) {
	items, err := readGQLLibrary(ctx, c, "Podcasts & Shows", func(typeName, uri string, raw json.RawMessage) (string, media.Podcast, bool) {
		lower := strings.ToLower(typeName)
		if !strings.Contains(lower, "podcast") && !strings.Contains(lower, "show") {
			return "", media.Podcast{}, false
		}
		var s gqlShow
		if json.Unmarshal(raw, &s) != nil {
			return "", media.Podcast{}, false
		}
		if s.URI == "" {
			s.URI = uri
		}
		id := s.ID
		if id == "" {
			id = uriID(s.URI)
		}
		if id == "" {
			return "", media.Podcast{}, false
		}
		return id, gqlShowToPodcast(&s).Summary(), true
	})
	if !came(err) {
		return nil, err
	}
	if items == nil {
		items = []media.Podcast{}
	}
	return items, err
}

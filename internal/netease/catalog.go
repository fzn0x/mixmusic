package netease

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// The objects of the weapi, as far as MixMusic reads them. Songs come in
// two shapes: ar/al/dt from the newer endpoints, artists/album/duration
// from the older ones, as similar songs.

type ref struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type album struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	PicURL      string `json:"picUrl"`
	Artist      *ref   `json:"artist"`
	Artists     []ref  `json:"artists"`
	Size        int    `json:"size"`
	PublishTime int64  `json:"publishTime"`
	Type        string `json:"type"`
}

type song struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Ar       []ref  `json:"ar"`
	Artists  []ref  `json:"artists"`
	Al       *album `json:"al"`
	Album    *album `json:"album"`
	Dt       int    `json:"dt"`
	Duration int    `json:"duration"`
}

type artist struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	PicURL    string `json:"picUrl"`
	Img1v1URL string `json:"img1v1Url"`
	BriefDesc string `json:"briefDesc"`
}

type playlist struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	CoverImgURL string `json:"coverImgUrl"`
	PicURL      string `json:"picUrl"` // of recommended playlists
	Description string `json:"description"`
	TrackCount  int    `json:"trackCount"`
	Creator     *struct {
		UserID   int64  `json:"userId"`
		Nickname string `json:"nickname"`
	} `json:"creator"`
	Subscribed bool   `json:"subscribed"`
	Tracks     []song `json:"tracks"`
	TrackIDs   []struct {
		ID int64 `json:"id"`
	} `json:"trackIds"`
}

func id(n int64) string { return media.ID(media.SourceNetEase, strconv.FormatInt(n, 10)) }

func artwork(pic string) []media.Image {
	if pic == "" {
		return []media.Image{}
	}
	return []media.Image{
		{URL: pic + "?param=120y120", Width: 120, Height: 120},
		{URL: pic + "?param=544y544", Width: 544, Height: 544},
	}
}

func refs(in []ref) []media.Ref {
	out := make([]media.Ref, 0, len(in))
	for _, a := range in {
		if a.Name == "" {
			continue
		}
		r := media.Ref{Name: a.Name}
		if a.ID != 0 {
			r.ID = id(a.ID)
		}
		out = append(out, r)
	}
	return out
}

func toTrack(s song) media.Track {
	ar, al, dt := s.Ar, s.Al, s.Dt
	if len(ar) == 0 {
		ar = s.Artists
	}
	if al == nil {
		al = s.Album
	}
	if dt == 0 {
		dt = s.Duration
	}
	t := media.Track{
		ID:         id(s.ID),
		Source:     media.SourceNetEase,
		Title:      s.Name,
		Artists:    refs(ar),
		DurationMs: dt,
		Artwork:    []media.Image{},
		URL:        origin + "/#/song?id=" + strconv.FormatInt(s.ID, 10),
	}
	if al != nil {
		t.Album = &media.Ref{ID: id(al.ID), Name: al.Name}
		t.Artwork = artwork(al.PicURL)
	}
	return t
}

func toTracks(in []song) []media.Track {
	out := make([]media.Track, 0, len(in))
	for _, s := range in {
		if s.ID != 0 {
			out = append(out, toTrack(s))
		}
	}
	return out
}

func toAlbum(a album) media.Album {
	artists := a.Artists
	if len(artists) == 0 && a.Artist != nil {
		artists = []ref{*a.Artist}
	}
	out := media.Album{
		ID:         id(a.ID),
		Source:     media.SourceNetEase,
		Title:      a.Name,
		Artists:    refs(artists),
		Kind:       media.AlbumKindAlbum,
		Artwork:    artwork(a.PicURL),
		TrackCount: a.Size,
		Tracks:     []media.Track{},
		URL:        origin + "/#/album?id=" + strconv.FormatInt(a.ID, 10),
	}
	switch a.Type {
	case "Single":
		out.Kind = media.AlbumKindSingle
	case "EP/Single", "EP":
		out.Kind = media.AlbumKindEP
	}
	if a.PublishTime > 0 {
		out.Year = strconv.Itoa(time.UnixMilli(a.PublishTime).Year())
	}
	return out
}

func toArtist(a artist) media.Artist {
	pic := a.PicURL
	if pic == "" {
		pic = a.Img1v1URL
	}
	return media.Artist{
		ID:          id(a.ID),
		Source:      media.SourceNetEase,
		Name:        a.Name,
		Artwork:     artwork(pic),
		Description: a.BriefDesc,
		TopTracks:   []media.Track{},
		Albums:      []media.Album{},
		Singles:     []media.Album{},
		Related:     []media.Ref{},
		URL:         origin + "/#/artist?id=" + strconv.FormatInt(a.ID, 10),
	}
}

// toPlaylist converts a playlist; userID is the signed-in user's, whose
// own playlists are theirs.
func toPlaylist(p playlist, userID string) media.Playlist {
	pic := p.CoverImgURL
	if pic == "" {
		pic = p.PicURL
	}
	out := media.Playlist{
		ID:          id(p.ID),
		Source:      media.SourceNetEase,
		Title:       p.Name,
		Description: p.Description,
		Artwork:     artwork(pic),
		TrackCount:  p.TrackCount,
		Tracks:      []media.Track{},
		Saved:       p.Subscribed,
		URL:         origin + "/#/playlist?id=" + strconv.FormatInt(p.ID, 10),
	}
	if p.Creator != nil {
		out.Owner = p.Creator.Nickname
		out.Mine = userID != "" && strconv.FormatInt(p.Creator.UserID, 10) == userID
	}
	return out
}

type searchResult struct {
	Result struct {
		Songs     []song     `json:"songs"`
		Albums    []album    `json:"albums"`
		Artists   []artist   `json:"artists"`
		Playlists []playlist `json:"playlists"`
	} `json:"result"`
}

// search searches for one kind: 1 songs, 10 albums, 100 artists, 1000
// playlists. NetEase refuses cloudsearch to who isn't signed in, whom the
// older search answers, with songs without their covers: those are read
// again whole.
func (c *Client) search(ctx context.Context, query string, kind, limit int) (searchResult, error) {
	var r searchResult
	params := map[string]any{"s": query, "type": kind, "limit": limit, "offset": 0}
	if err := c.call(ctx, "/cloudsearch/get/web", params, &r); err == nil {
		return r, nil
	}
	if err := c.call(ctx, "/search/get", params, &r); err != nil {
		return r, err
	}
	return r, nil
}

// Search finds songs, albums, artists and playlists.
func (c *Client) Search(ctx context.Context, query string) (media.SearchResults, error) {
	res := media.SearchResults{Tracks: []media.Track{}, Videos: []media.Track{}, Albums: []media.Album{}, Artists: []media.Artist{}, Playlists: []media.Playlist{}}
	// Songs first: an error there fails the search; the other kinds are
	// left out when they fail.
	tracks, err := c.SearchSongs(ctx, query)
	if err != nil {
		return res, err
	}
	res.Tracks = tracks
	for _, kind := range []int{10, 100, 1000} {
		r, err := c.search(ctx, query, kind, 12)
		if err != nil {
			continue
		}
		for _, a := range r.Result.Albums {
			res.Albums = append(res.Albums, toAlbum(a))
		}
		for _, a := range r.Result.Artists {
			res.Artists = append(res.Artists, toArtist(a))
		}
		for _, p := range r.Result.Playlists {
			res.Playlists = append(res.Playlists, toPlaylist(p, ""))
		}
	}
	return res, nil
}

// SearchSongs finds songs alone, as matching a track of another service
// needs.
func (c *Client) SearchSongs(ctx context.Context, query string) ([]media.Track, error) {
	r, err := c.search(ctx, query, 1, 30)
	if err != nil {
		return nil, err
	}
	if len(r.Result.Songs) > 0 && r.Result.Songs[0].Al == nil { // the older search's
		ids := make([]int64, len(r.Result.Songs))
		for i, s := range r.Result.Songs {
			ids[i] = s.ID
		}
		if full, err := c.songs(ctx, ids); err == nil {
			return full, nil
		}
	}
	return toTracks(r.Result.Songs), nil
}

// songs reads the songs of ids, 500 at a time as the weapi takes them.
func (c *Client) songs(ctx context.Context, ids []int64) ([]media.Track, error) {
	out := []media.Track{}
	for start := 0; start < len(ids); start += 500 {
		batch := ids[start:min(start+500, len(ids))]
		list := make([]map[string]int64, len(batch))
		for i, n := range batch {
			list[i] = map[string]int64{"id": n}
		}
		c2, _ := json.Marshal(list)
		var r struct {
			Songs []song `json:"songs"`
		}
		if err := c.call(ctx, "/v3/song/detail", map[string]any{"c": string(c2)}, &r); err != nil {
			return nil, err
		}
		out = append(out, toTracks(r.Songs)...)
	}
	return out, nil
}

// maxTracks bounds the tracks read of a playlist.
const maxTracks = 5000

// Playlist returns a playlist with its tracks; userID is the signed-in
// user's, or "".
func (c *Client) Playlist(ctx context.Context, native, userID string) (media.Playlist, error) {
	var r struct {
		Playlist playlist `json:"playlist"`
	}
	if err := c.call(ctx, "/v6/playlist/detail", map[string]any{"id": native, "n": 100000, "s": 8}, &r); err != nil {
		return media.Playlist{}, err
	}
	p := toPlaylist(r.Playlist, userID)
	ids := make([]int64, 0, len(r.Playlist.TrackIDs))
	for _, t := range r.Playlist.TrackIDs {
		ids = append(ids, t.ID)
	}
	if len(ids) > maxTracks {
		ids, p.Cut = ids[:maxTracks], true
	}
	if len(ids) == 0 {
		p.Tracks = toTracks(r.Playlist.Tracks)
		return p, nil
	}
	tracks, err := c.songs(ctx, ids)
	if err != nil {
		return media.Playlist{}, err
	}
	p.Tracks = tracks
	return p, nil
}

// UserPlaylists returns the playlists the user made and saved; the first
// of those they made is their liked songs.
func (c *Client) UserPlaylists(ctx context.Context, userID string) ([]media.Playlist, error) {
	var r struct {
		Playlist []playlist `json:"playlist"`
	}
	if err := c.call(ctx, "/user/playlist", map[string]any{"uid": userID, "limit": 1000, "offset": 0, "includeVideo": true}, &r); err != nil {
		return nil, err
	}
	out := make([]media.Playlist, 0, len(r.Playlist))
	for _, p := range r.Playlist {
		mp := toPlaylist(p, userID)
		mp.Saved = mp.Saved || !mp.Mine
		out = append(out, mp)
	}
	return out, nil
}

// Album returns an album with its tracks.
func (c *Client) Album(ctx context.Context, native string) (media.Album, error) {
	var r struct {
		Album album  `json:"album"`
		Songs []song `json:"songs"`
	}
	if err := c.call(ctx, "/v1/album/"+native, nil, &r); err != nil {
		return media.Album{}, err
	}
	a := toAlbum(r.Album)
	a.Tracks = toTracks(r.Songs)
	for i := range a.Tracks {
		a.Tracks[i].Artwork = a.Artwork // the album's songs leave their cover out
	}
	return a, nil
}

// Artist returns an artist with their popular songs and albums.
func (c *Client) Artist(ctx context.Context, native string) (media.Artist, error) {
	var r struct {
		Artist   artist `json:"artist"`
		HotSongs []song `json:"hotSongs"`
	}
	if err := c.call(ctx, "/v1/artist/"+native, nil, &r); err != nil {
		return media.Artist{}, err
	}
	a := toArtist(r.Artist)
	a.TopTracks = toTracks(r.HotSongs)
	var al struct {
		HotAlbums []album `json:"hotAlbums"`
	}
	if c.call(ctx, "/artist/albums/"+native, map[string]any{"limit": 50, "offset": 0, "total": true}, &al) == nil {
		for _, x := range al.HotAlbums {
			if alb := toAlbum(x); alb.Kind == media.AlbumKindAlbum {
				a.Albums = append(a.Albums, alb)
			} else {
				a.Singles = append(a.Singles, alb)
			}
		}
	}
	return a, nil
}

// Similar returns songs like a song, for its radio.
func (c *Client) Similar(ctx context.Context, native string) ([]media.Track, error) {
	var r struct {
		Songs []song `json:"songs"`
	}
	if err := c.call(ctx, "/v1/discovery/simiSong", map[string]any{"songid": native, "limit": 50, "offset": 0}, &r); err != nil {
		return nil, err
	}
	return toTracks(r.Songs), nil
}

// Home returns the shelves of the home page: the user's daily songs and
// recommended playlists once signed in, and the charts.
func (c *Client) Home(ctx context.Context, userID string) ([]media.Shelf, error) {
	var shelves []media.Shelf
	if userID != "" {
		var daily struct {
			Data struct {
				DailySongs []song `json:"dailySongs"`
			} `json:"data"`
		}
		if c.call(ctx, "/v3/discovery/recommend/songs", nil, &daily) == nil && len(daily.Data.DailySongs) > 0 {
			shelves = append(shelves, trackShelf("每日推荐", "daily", toTracks(daily.Data.DailySongs)))
		}
		var rec struct {
			Recommend []playlist `json:"recommend"`
		}
		if c.call(ctx, "/v1/discovery/recommend/resource", nil, &rec) == nil && len(rec.Recommend) > 0 {
			shelves = append(shelves, playlistShelf("推荐歌单", "recommend", rec.Recommend))
		}
		if mine, err := c.UserPlaylists(ctx, userID); err == nil && len(mine) > 0 {
			items := make([]media.Item, len(mine))
			for i := range mine {
				items[i] = media.Item{Kind: media.ItemPlaylist, Playlist: &mine[i]}
			}
			shelves = append(shelves, media.Shelf{Title: "我的歌单", Source: media.SourceNetEase, Items: items, Key: "mine"})
		}
	}
	var top struct {
		List []playlist `json:"list"`
	}
	err := c.call(ctx, "/toplist", nil, &top)
	if err == nil && len(top.List) > 0 {
		shelves = append(shelves, playlistShelf("排行榜", "charts", top.List))
	}
	if len(shelves) == 0 {
		return nil, err
	}
	return shelves, nil
}

func trackShelf(title, key string, tracks []media.Track) media.Shelf {
	items := make([]media.Item, len(tracks))
	for i := range tracks {
		items[i] = media.Item{Kind: media.ItemTrack, Track: &tracks[i]}
	}
	return media.Shelf{Title: title, Source: media.SourceNetEase, Items: items, Key: key}
}

func playlistShelf(title, key string, in []playlist) media.Shelf {
	items := make([]media.Item, len(in))
	for i := range in {
		p := toPlaylist(in[i], "")
		items[i] = media.Item{Kind: media.ItemPlaylist, Playlist: &p}
	}
	return media.Shelf{Title: title, Source: media.SourceNetEase, Items: items, Key: key}
}

// Lyrics returns a song's lyrics as LRC, and their Chinese translation
// as LRC when NetEase has one.
func (c *Client) Lyrics(ctx context.Context, native string) (lrc, translation string, err error) {
	var r struct {
		Lrc struct {
			Lyric string `json:"lyric"`
		} `json:"lrc"`
		Tlyric struct {
			Lyric string `json:"lyric"`
		} `json:"tlyric"`
	}
	if err := c.call(ctx, "/song/lyric", map[string]any{"id": native, "lv": -1, "tv": -1}, &r); err != nil {
		return "", "", err
	}
	return r.Lrc.Lyric, r.Tlyric.Lyric, nil
}

// Audio is where a song's audio is.
type Audio struct {
	URL  string
	Size int64
	// Type is the file's kind, as "mp3" or "flac".
	Type string
	// Expires is when the URL stops working.
	Expires time.Time
}

// SongAudio returns where the audio of a song is, at level: "standard",
// "exhigh" (320 kbit/s) or "lossless". NetEase gives only a sample of a
// song for members to one who isn't: ErrUnavailable then, as for a song
// it has no rights to.
func (c *Client) SongAudio(ctx context.Context, native, level string) (Audio, error) {
	var r struct {
		Data []struct {
			URL           string          `json:"url"`
			Size          int64           `json:"size"`
			Type          string          `json:"type"`
			Expi          int             `json:"expi"`
			FreeTrialInfo json.RawMessage `json:"freeTrialInfo"`
		} `json:"data"`
	}
	if err := c.call(ctx, "/song/enhance/player/url/v1", map[string]any{"ids": "[" + native + "]", "level": level, "encodeType": "flac"}, &r); err != nil {
		return Audio{}, err
	}
	if len(r.Data) == 0 || r.Data[0].URL == "" {
		return Audio{}, ErrUnavailable
	}
	d := r.Data[0]
	if len(d.FreeTrialInfo) > 0 && string(d.FreeTrialInfo) != "null" {
		return Audio{}, ErrUnavailable
	}
	expi := d.Expi
	if expi <= 0 {
		expi = 1200
	}
	return Audio{URL: d.URL, Size: d.Size, Type: d.Type, Expires: time.Now().Add(time.Duration(expi) * time.Second)}, nil
}

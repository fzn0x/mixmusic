package app

// What the tools of AI assistants (#477) find, and play: the results of
// search, and the user's playlists, kept by ID, as MCP keeps no state
// between requests, for play_result and play_playlist, which play them as
// the windows do.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/fzn0x/mixmusic/internal/mcp"
	"github.com/fzn0x/mixmusic/internal/media"
)

// assistantCatalog is where the tools find music: the services, and the
// user's library. Tests give a fake.
type assistantCatalog interface {
	// Search searches the services, and doesn't keep the query among the
	// user's recent searches.
	Search(ctx context.Context, query string) (SearchResponse, error)
	Album(ctx context.Context, id string) (media.Album, error)
	Artist(ctx context.Context, id string) (media.Artist, error)
	// Playlist returns a playlist with all its tracks.
	Playlist(ctx context.Context, id string) (media.Playlist, error)
	// Playlists are the user's: their mixes, then their playlists of the
	// services they signed in to, and why a service's didn't come.
	Playlists(ctx context.Context) ([]media.Playlist, []error)
	// Liked are the user's liked songs.
	Liked() []media.Track
}

// appCatalog is the app as the tools' catalog.
type appCatalog struct{ a *App }

func (c appCatalog) Search(ctx context.Context, q string) (SearchResponse, error) {
	return (&Catalog{c.a}).search(ctx, q)
}

func (c appCatalog) Album(ctx context.Context, id string) (media.Album, error) {
	return (&Catalog{c.a}).Album(ctx, id)
}

func (c appCatalog) Artist(ctx context.Context, id string) (media.Artist, error) {
	return (&Catalog{c.a}).Artist(ctx, id)
}

func (c appCatalog) Playlist(ctx context.Context, id string) (media.Playlist, error) {
	return (&Catalog{c.a}).Playlist(ctx, id)
}

func (c appCatalog) Playlists(ctx context.Context) ([]media.Playlist, []error) {
	col := &Collection{c.a}
	out := col.Playlists()
	var errs []error
	if c.a.youtube.SignedIn() {
		ps, err := col.YouTubePlaylists(ctx)
		out = append(out, ps...)
		if err != nil {
			errs = append(errs, fmt.Errorf("YouTube Music: %w", err))
		}
	}
	if c.a.spotify.SignedIn() {
		ps, err := col.SpotifyPlaylists(ctx)
		out = append(out, ps...)
		if err != nil {
			errs = append(errs, fmt.Errorf("Spotify: %w", err))
		}
	}
	return out, errs
}

func (c appCatalog) Liked() []media.Track { return c.a.library.Liked() }

// toolCatalog returns where the tools find music.
func (a *App) toolCatalog() assistantCatalog {
	if c := a.assistants.catalog; c != nil {
		return c
	}
	return appCatalog{a}
}

// The kinds of what the tools tell of, which the tools that play take.
const (
	kindSong     = "song"
	kindVideo    = "video"
	kindAlbum    = "album"
	kindArtist   = "artist"
	kindPlaylist = "playlist"
)

// likedID is the ID of the user's liked songs among their playlists.
const likedID = "liked"

// assistantFound is what a tool told of: its kind, its title, and a
// song's track.
type assistantFound struct {
	Kind  string
	Title string
	Track media.Track
}

// foundItems keeps what the tools told of, by ID, for the tools that play.
func (a *App) foundItems() *memo[assistantFound] {
	h := &a.assistants
	h.foundOnce.Do(func() { h.found = newMemo[assistantFound](1000) })
	return h.found
}

// assistantResult is a result of search, or a playlist of
// list_playlists.
type assistantResult struct {
	ID              string   `json:"id"`
	Kind            string   `json:"kind"`
	Title           string   `json:"title"`
	Artists         []string `json:"artists,omitempty"`
	Album           string   `json:"album,omitempty"`
	Year            string   `json:"year,omitempty"`
	Owner           string   `json:"owner,omitempty"`
	Tracks          int      `json:"tracks,omitempty"`
	Service         string   `json:"service"`
	DurationSeconds float64  `json:"durationSeconds,omitempty"`
}

// keepFound keeps r as found, for the tools that play, and returns it.
func (a *App) keepFound(r assistantResult, t media.Track) assistantResult {
	a.foundItems().put(r.ID, assistantFound{Kind: r.Kind, Title: r.Title, Track: t})
	return r
}

func (a *App) songResult(t media.Track, kind string) assistantResult {
	at := assistantTrackOf(t)
	return a.keepFound(assistantResult{ID: t.ID, Kind: kind, Title: at.Title, Artists: at.Artists, Album: at.Album,
		Service: at.Service, DurationSeconds: at.DurationSeconds}, t)
}

func (a *App) albumResult(al media.Album) assistantResult {
	r := assistantResult{ID: al.ID, Kind: kindAlbum, Title: al.Title, Year: al.Year, Tracks: al.TrackCount,
		Service: serviceName(al.Source)}
	for _, ar := range al.Artists {
		r.Artists = append(r.Artists, ar.Name)
	}
	return a.keepFound(r, media.Track{})
}

func (a *App) artistResult(ar media.Artist) assistantResult {
	return a.keepFound(assistantResult{ID: ar.ID, Kind: kindArtist, Title: ar.Name, Service: serviceName(ar.Source)}, media.Track{})
}

func (a *App) playlistResult(pl media.Playlist) assistantResult {
	return a.keepFound(assistantResult{ID: pl.ID, Kind: kindPlaylist, Title: pl.Title, Owner: pl.Owner,
		Tracks: max(pl.TrackCount, len(pl.Tracks)), Service: serviceName(pl.Source)}, media.Track{})
}

// assistantSearch is what search found.
type assistantSearch struct {
	Results []assistantResult `json:"results"`
	// Problems are the services that didn't answer.
	Problems []string `json:"problems,omitempty"`
}

// interleave takes up to n of each list in turn, YouTube Music's first.
func interleave[T any](n int, lists ...[]T) []T {
	var out []T
	for i := 0; len(out) < n; i++ {
		took := false
		for _, l := range lists {
			if i < len(l) && len(out) < n {
				out = append(out, l[i])
				took = true
			}
		}
		if !took {
			break
		}
	}
	return out
}

func (a *App) toolSearch(ctx context.Context, args json.RawMessage) (mcp.Result, error) {
	in, err := mcp.Args[struct {
		Query string `json:"query"`
		Kind  string `json:"kind"`
		Limit *int   `json:"limit"`
	}](args)
	if err != nil {
		return mcp.Result{}, err
	}
	query := strings.TrimSpace(in.Query)
	if query == "" || len(query) > 200 {
		return mcp.Result{}, errors.New("query is from 1 to 200 characters")
	}
	kind := in.Kind
	if kind == "" {
		kind = "any"
	}
	if !slices.Contains([]string{"any", kindSong, kindVideo, kindAlbum, kindArtist, kindPlaylist}, kind) {
		return mcp.Result{}, errors.New("kind is any, song, video, album, artist or playlist")
	}
	limit := 5
	if in.Limit != nil {
		limit = *in.Limit
	}
	if limit < 1 || limit > 10 {
		return mcp.Result{}, errors.New("limit is from 1 to 10")
	}
	res, err := a.toolCatalog().Search(ctx, query)
	if err != nil {
		return mcp.Result{}, fmt.Errorf("the search failed: %w", err)
	}
	hide := a.settings.Get().HideExplicit
	songs := func(ts []media.Track) []media.Track {
		return slices.DeleteFunc(slices.Clone(ts), func(t media.Track) bool { return hide && t.ExplicitSong() })
	}
	albums := func(as []media.Album) []media.Album {
		return slices.DeleteFunc(slices.Clone(as), func(al media.Album) bool { return hide && al.ExplicitAlbum() })
	}
	yt, sp := res.YouTube, res.Spotify
	out := assistantSearch{Results: []assistantResult{}, Problems: res.Errors}
	wants := func(k string) bool { return kind == k || kind == "any" && k != kindVideo }
	if wants(kindSong) {
		for _, t := range interleave(limit, songs(yt.Tracks), songs(sp.Tracks)) {
			out.Results = append(out.Results, a.songResult(t, kindSong))
		}
	}
	if wants(kindVideo) {
		for _, t := range interleave(limit, songs(yt.Videos), songs(sp.Videos)) {
			out.Results = append(out.Results, a.songResult(t, kindVideo))
		}
	}
	if wants(kindAlbum) {
		for _, al := range interleave(limit, albums(yt.Albums), albums(sp.Albums)) {
			out.Results = append(out.Results, a.albumResult(al))
		}
	}
	if wants(kindArtist) {
		for _, ar := range interleave(limit, yt.Artists, sp.Artists) {
			out.Results = append(out.Results, a.artistResult(ar))
		}
	}
	if wants(kindPlaylist) {
		for _, pl := range interleave(limit, yt.Playlists, sp.Playlists) {
			out.Results = append(out.Results, a.playlistResult(pl))
		}
	}
	return jsonResult(out)
}

// foundTracks finds the tracks of what a tool told of: a song's own, an
// album's, an artist's top tracks, a playlist's, the liked songs.
func (a *App) foundTracks(ctx context.Context, id string, f assistantFound) ([]media.Track, string, error) {
	cat := a.toolCatalog()
	switch f.Kind {
	case kindSong, kindVideo:
		return []media.Track{f.Track}, named(f.Track), nil
	case kindAlbum:
		al, err := cat.Album(ctx, id)
		return al.Tracks, "the album “" + oneLine(f.Title) + "”", err
	case kindArtist:
		ar, err := cat.Artist(ctx, id)
		return ar.TopTracks, "the top tracks of " + oneLine(f.Title), err
	}
	if id == likedID {
		return cat.Liked(), "your liked songs", nil
	}
	pl, err := cat.Playlist(ctx, id)
	return pl.Tracks, "“" + oneLine(f.Title) + "”", err
}

// playFound plays what a tool told of, now, next or last.
func (a *App) playFound(ctx context.Context, id string, f assistantFound, when string, shuffle bool) (mcp.Result, error) {
	if when == "" {
		when = "now"
	}
	if !slices.Contains([]string{"now", "next", "last"}, when) {
		return mcp.Result{}, errors.New("when is now, next or last")
	}
	if shuffle && when != "now" {
		return mcp.Result{}, errors.New("shuffle plays it now: leave when out, or shuffle")
	}
	p, err := a.assistantPlayer()
	if err != nil {
		return mcp.Result{}, err
	}
	tracks, what, err := a.foundTracks(ctx, id, f)
	if err != nil {
		return mcp.Result{}, fmt.Errorf("%s didn't open: %w", what, err)
	}
	if len(tracks) == 0 {
		return mcp.Result{}, fmt.Errorf("%s has no tracks", what)
	}
	song := f.Kind == kindSong || f.Kind == kindVideo
	switch {
	case when == "next":
		p.PlayNext(tracks)
		assistantDid(p, "put "+what+" next")
		return mcp.Result{Text: "Put " + what + " next."}, nil
	case when == "last":
		p.Enqueue(tracks)
		assistantDid(p, "added "+what+" to the queue")
		return mcp.Result{Text: "Added " + what + " to the queue."}, nil
	case song:
		// As a song played from the search of the windows: its radio
		// follows it, while autoplay is on.
		p.PlayWithRadio(tracks[0])
	default:
		p.PlayAll(tracks, shuffle)
	}
	if shuffle {
		assistantDid(p, "shuffled "+what)
		return mcp.Result{Text: "Shuffling " + what + "."}, nil
	}
	assistantDid(p, "played "+what)
	return mcp.Result{Text: "Playing " + what + "."}, nil
}

func (a *App) toolPlayResult(ctx context.Context, args json.RawMessage) (mcp.Result, error) {
	in, err := mcp.Args[struct {
		ID      string `json:"id"`
		When    string `json:"when"`
		Shuffle bool   `json:"shuffle"`
	}](args)
	if err != nil {
		return mcp.Result{}, err
	}
	f, ok := a.foundItems().get(in.ID)
	if !ok {
		return mcp.Result{}, fmt.Errorf("no result has the id %q: search, or list the playlists, and give an id they tell", in.ID)
	}
	return a.playFound(ctx, in.ID, f, in.When, in.Shuffle)
}

// assistantPlaylists are the user's playlists, as list_playlists tells
// them.
type assistantPlaylists struct {
	Playlists []assistantResult `json:"playlists"`
	// Problems are the services that didn't answer.
	Problems []string `json:"problems,omitempty"`
}

// userPlaylists lists the user's playlists, the liked songs first, and keeps
// them as found.
func (a *App) userPlaylists(ctx context.Context) assistantPlaylists {
	cat := a.toolCatalog()
	out := assistantPlaylists{Playlists: []assistantResult{}}
	if liked := cat.Liked(); len(liked) > 0 {
		out.Playlists = append(out.Playlists, a.keepFound(assistantResult{ID: likedID, Kind: kindPlaylist, Title: "Liked songs",
			Owner: "You", Tracks: len(liked), Service: serviceName(media.SourceLocal)}, media.Track{}))
	}
	pls, errs := cat.Playlists(ctx)
	for _, pl := range pls {
		out.Playlists = append(out.Playlists, a.playlistResult(pl))
	}
	for _, err := range errs {
		out.Problems = append(out.Problems, err.Error())
	}
	return out
}

func (a *App) toolPlaylists(ctx context.Context, _ json.RawMessage) (mcp.Result, error) {
	return jsonResult(a.userPlaylists(ctx))
}

func (a *App) toolPlayPlaylist(ctx context.Context, args json.RawMessage) (mcp.Result, error) {
	in, err := mcp.Args[struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Shuffle bool   `json:"shuffle"`
	}](args)
	if err != nil {
		return mcp.Result{}, err
	}
	id, name := strings.TrimSpace(in.ID), strings.TrimSpace(in.Name)
	if (id == "") == (name == "") {
		return mcp.Result{}, errors.New("give id or name, one of them")
	}
	if id != "" {
		f, ok := a.foundItems().get(id)
		if !ok || f.Kind != kindPlaylist {
			// Not told of yet: the list tells it.
			a.userPlaylists(ctx)
			if f, ok = a.foundItems().get(id); !ok || f.Kind != kindPlaylist {
				return mcp.Result{}, fmt.Errorf("no playlist has the id %q: list_playlists tells their ids", id)
			}
		}
		return a.playFound(ctx, id, f, "now", in.Shuffle)
	}
	list := a.userPlaylists(ctx).Playlists
	match := matchPlaylists(list, name)
	switch len(match) {
	case 0:
		return mcp.Result{}, fmt.Errorf("no playlist of the user is named %q: list_playlists tells them", name)
	case 1:
		f, _ := a.foundItems().get(match[0].ID)
		return a.playFound(ctx, match[0].ID, f, "now", in.Shuffle)
	}
	var names []string
	for _, r := range match {
		names = append(names, fmt.Sprintf("%q (id %s)", r.Title, r.ID))
	}
	return mcp.Result{}, fmt.Errorf("several playlists match %q: %s; give the id of one", name, strings.Join(names, ", "))
}

// matchPlaylists are the playlists of list named name: those whose title
// is name, without regard to case; or else those whose title holds it; or
// else those whose title name holds, as "my liked songs" holds "Liked
// songs".
func matchPlaylists(list []assistantResult, name string) []assistantResult {
	name = strings.ToLower(name)
	var exact, part, whole []assistantResult
	for _, r := range list {
		title := strings.ToLower(strings.TrimSpace(r.Title))
		switch {
		case title == "":
		case title == name:
			exact = append(exact, r)
		case strings.Contains(title, name):
			part = append(part, r)
		case strings.Contains(name, title):
			whole = append(whole, r)
		}
	}
	switch {
	case len(exact) > 0:
		return exact
	case len(part) > 0:
		return part
	}
	return whole
}

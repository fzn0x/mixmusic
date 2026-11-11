package app

// The tools of AI assistants (#477). What plays and the queue are told as
// remote control tells them (remoteStateOf), and play, pause, next,
// previous, seeking and the volume go through remote control's commands
// (remoteHost.Do); what an assistant finds plays as the windows play it.
// Each change shows a toast, through the player's notices.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/mcp"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/remote"
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

// errNoPlayer is why a tool can't reach the player.
var errNoPlayer = errors.New("MixMusic's player isn't ready yet, as MixMusic is still starting: try again in a moment")

// assistantPlayer is the player, as remote control has it.
func (a *App) assistantPlayer() (*player.Player, error) {
	if p := (remoteHost{a}).player(); p != nil {
		return p, nil
	}
	return nil, errNoPlayer
}

// assistantDid shows the user what an assistant did, as a toast, and logs it.
func assistantDid(p *player.Player, what string) {
	log.Printf("mixmusic: AI assistants: %s", what)
	p.Tell(player.Notice{Message: "An AI assistant " + what})
}

// assistantTrack is a track as the tools tell it.
type assistantTrack struct {
	Title           string   `json:"title"`
	Artists         []string `json:"artists,omitempty"`
	Album           string   `json:"album,omitempty"`
	Service         string   `json:"service"`
	DurationSeconds float64  `json:"durationSeconds,omitempty"`
}

// assistantTrackOf is t as the tools tell it: as remote control does, and
// its service.
func assistantTrackOf(t media.Track) assistantTrack {
	r := remoteTrackOf(t)
	return assistantTrack{Title: r.Title, Artists: r.Artists, Album: r.Album, Service: serviceName(discordSource(t)),
		DurationSeconds: seconds(int64(t.DurationMs))}
}

// serviceName names a service for the assistant.
func serviceName(s media.Source) string {
	switch s {
	case media.SourceYouTube:
		return "YouTube Music"
	case media.SourceSpotify:
		return "Spotify"
	case media.SourceFile:
		return "this computer"
	case media.SourceLocal:
		return "MixMusic"
	}
	return string(s)
}

// seconds is ms in seconds, to a tenth.
func seconds(ms int64) float64 { return math.Round(float64(ms)/100) / 10 }

// minutesOf is ms as minutes and seconds, as 3:07, or hours too.
func minutesOf(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	h, m, s := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// named is a track as a toast names it: its title in quotes, and its
// artists.
func named(t media.Track) string {
	r := remoteTrackOf(t)
	s := "“" + oneLine(r.Title) + "”"
	if len(r.Artists) > 0 {
		s += " by " + oneLine(strings.Join(r.Artists, ", "))
	}
	return s
}

// jsonResult is a result whose text is its data, as JSON.
func jsonResult(v any) (mcp.Result, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return mcp.Result{}, err
	}
	return mcp.Result{Text: string(data), Data: v}, nil
}

// noArgs is the schema of a tool without arguments.
const noArgs = `{"type":"object","properties":{},"additionalProperties":false}`

// assistantTools are what the AI assistants may call, in the order
// tools/list tells them.
func (a *App) assistantTools() []mcp.Tool {
	return []mcp.Tool{
		{Name: "now_playing", Title: "Now playing", ReadOnly: true, Input: json.RawMessage(noArgs),
			Description: "Tells what MixMusic plays: the track, with its artists, album and service, whether it plays or is " +
				"paused, where it is and how long it is in seconds, the volume, shuffle and repeat. The track is null " +
				"when nothing is in the queue.",
			Call: a.toolNowPlaying},
		{Name: "get_queue", Title: "Queue", ReadOnly: true, Input: json.RawMessage(noArgs),
			Description: "Lists the tracks that play after the current one, in the order they play, up to 20, and how " +
				"many more come after those.",
			Call: a.toolQueue},
		{Name: "play", Title: "Play", Idempotent: true, Input: json.RawMessage(noArgs),
			Description: "Plays the current track of the queue, from where it was paused. To play something else, use " +
				"play_result or play_playlist.",
			Call: a.toolPlay},
		{Name: "pause", Title: "Pause", Idempotent: true, Input: json.RawMessage(noArgs),
			Description: "Pauses the music where it is: play goes on from there.",
			Call:        a.toolPause},
		{Name: "next_track", Title: "Next track",
			Input: json.RawMessage(`{"type":"object","properties":{"count":{"type":"integer","minimum":1,"maximum":20,` +
				`"description":"How many tracks to skip: 1, the default, plays the next one; 3 skips the next two and plays the third."}},` +
				`"additionalProperties":false}`),
			Description: "Skips to the next track of the queue, or past several.",
			Call:        a.toolNext},
		{Name: "previous_track", Title: "Previous track", Input: json.RawMessage(noArgs),
			Description: "Goes back to the previous track, as the previous button does: a track that played for more " +
				"than 3 seconds starts again instead.",
			Call: a.toolPrevious},
		{Name: "seek", Title: "Seek", Idempotent: true,
			Input: json.RawMessage(`{"type":"object","properties":{` +
				`"position_seconds":{"type":"number","minimum":0,"description":"Where to move to, in seconds from the track's start."},` +
				`"by_seconds":{"type":"number","description":"How far to move from where the track is, in seconds: negative goes back."}},` +
				`"additionalProperties":false}`),
			Description: "Moves the current track to a place in it: give position_seconds, or by_seconds, not both. A " +
				"live stream can't move.",
			Call: a.toolSeek},
		{Name: "set_volume", Title: "Volume", Idempotent: true,
			Input: json.RawMessage(`{"type":"object","properties":{` +
				`"percent":{"type":"integer","minimum":0,"maximum":100,"description":"The volume, from 0 to 100."},` +
				`"by_percent":{"type":"integer","minimum":-100,"maximum":100,"description":"How much to turn the volume up, or down when negative."}},` +
				`"additionalProperties":false}`),
			Description: "Sets MixMusic's volume: give percent, or by_percent, not both. It unmutes.",
			Call:        a.toolVolume},
		{Name: "search", Title: "Search", ReadOnly: true, OpenWorld: true,
			Input: json.RawMessage(`{"type":"object","properties":{` +
				`"query":{"type":"string","minLength":1,"maxLength":200,"description":"What to look for: a title, an artist, an album, words of a song."},` +
				`"kind":{"type":"string","enum":["any","song","video","album","artist","playlist"],"description":"What to find: any, the default, finds songs, albums, artists and playlists; video finds music videos."},` +
				`"limit":{"type":"integer","minimum":1,"maximum":10,"description":"How many results of each kind, 5 by default."}},` +
				`"required":["query"],"additionalProperties":false}`),
			Description: "Searches YouTube Music, and Spotify when the user signed in to it. Each result has an id, which " +
				"play_result plays, puts next or adds to the queue.",
			Call: a.toolSearch},
		{Name: "play_result", Title: "Play a result", OpenWorld: true,
			Input: json.RawMessage(`{"type":"object","properties":{` +
				`"id":{"type":"string","description":"The id of a result of search, or of a playlist of list_playlists."},` +
				`"when":{"type":"string","enum":["now","next","last"],"description":"now, the default, plays it at once, in place of the queue; next puts it after the current track; last adds it at the end of the queue."},` +
				`"shuffle":{"type":"boolean","description":"Plays an album, an artist's top tracks or a playlist shuffled, when now."}},` +
				`"required":["id"],"additionalProperties":false}`),
			Description: "Plays a song, a video, an album, an artist's top tracks or a playlist that search or " +
				"list_playlists found, or puts it in the queue. A song played now is followed by songs like it, " +
				"when the user's autoplay is on.",
			Call: a.toolPlayResult},
		{Name: "list_playlists", Title: "Your playlists", ReadOnly: true, OpenWorld: true, Input: json.RawMessage(noArgs),
			Description: "Lists the user's own: their liked songs, their mixes made in MixMusic, and their playlists on " +
				"YouTube Music and Spotify, when they signed in to those. Each has an id, which play_playlist and " +
				"play_result take.",
			Call: a.toolPlaylists},
		{Name: "play_playlist", Title: "Play a playlist", OpenWorld: true,
			Input: json.RawMessage(`{"type":"object","properties":{` +
				`"id":{"type":"string","description":"The id list_playlists gives."},` +
				`"name":{"type":"string","description":"The playlist's name, or a part of it, in place of the id: liked songs plays the liked songs."},` +
				`"shuffle":{"type":"boolean","description":"Plays it shuffled."}},` +
				`"additionalProperties":false}`),
			Description: "Plays one of the user's playlists, mixes or their liked songs, by its id or its name, in place " +
				"of the queue.",
			Call: a.toolPlayPlaylist},
	}
}

// assistantNow is what plays, as now_playing tells it.
type assistantNow struct {
	Playing         bool            `json:"playing"`
	Loading         bool            `json:"loading,omitempty"`
	Track           *assistantTrack `json:"track"`
	PositionSeconds float64         `json:"positionSeconds"`
	DurationSeconds float64         `json:"durationSeconds,omitempty"`
	Live            bool            `json:"live,omitempty"`
	VolumePercent   int             `json:"volumePercent"`
	Muted           bool            `json:"muted,omitempty"`
	Shuffle         bool            `json:"shuffle"`
	Repeat          string          `json:"repeat"`
}

func (a *App) toolNowPlaying(context.Context, json.RawMessage) (mcp.Result, error) {
	p, err := a.assistantPlayer()
	if err != nil {
		return mcp.Result{}, err
	}
	st := p.State()
	rs := remoteStateOf(st)
	now := assistantNow{Playing: rs.Playing, Loading: rs.Loading, Live: rs.Live, Muted: rs.Muted,
		PositionSeconds: seconds(rs.PositionMs), DurationSeconds: seconds(rs.DurationMs),
		VolumePercent: int(math.Round(rs.Volume * 100)), Shuffle: st.Shuffle, Repeat: string(st.Repeat)}
	if cur, ok := st.Current(); ok {
		t := assistantTrackOf(cur.Track)
		now.Track = &t
	}
	return jsonResult(now)
}

// assistantQueue is what comes next, as get_queue tells it.
type assistantQueue struct {
	Next []assistantTrack `json:"next"`
	// More counts the tracks after those.
	More    int    `json:"more,omitempty"`
	Shuffle bool   `json:"shuffle"`
	Repeat  string `json:"repeat"`
}

func (a *App) toolQueue(context.Context, json.RawMessage) (mcp.Result, error) {
	p, err := a.assistantPlayer()
	if err != nil {
		return mcp.Result{}, err
	}
	st := p.State()
	q := assistantQueue{Next: []assistantTrack{}, Shuffle: st.Shuffle, Repeat: string(st.Repeat)}
	coming := 0
	for i := st.Index + 1; i < len(st.Queue); i++ {
		if st.Queue[i].Skip == "" {
			coming++
		}
	}
	for _, next := range remoteStateOf(st).Next {
		if i := slices.IndexFunc(st.Queue, func(it player.Item) bool { return it.UID == next.Entry }); i >= 0 {
			q.Next = append(q.Next, assistantTrackOf(st.Queue[i].Track))
		}
	}
	q.More = coming - len(q.Next)
	return jsonResult(q)
}

func (a *App) toolPlay(context.Context, json.RawMessage) (mcp.Result, error) {
	p, err := a.assistantPlayer()
	if err != nil {
		return mcp.Result{}, err
	}
	st := p.State()
	cur, ok := st.Current()
	switch {
	case !ok:
		return mcp.Result{}, errors.New("nothing is in the queue: find something with search and play it with play_result, or play a playlist with play_playlist")
	case remoteStateOf(st).Playing:
		return mcp.Result{Text: "It plays already."}, nil
	}
	remoteHost{a}.Do(remote.Command{Op: remote.OpPlay})
	assistantDid(p, "played "+named(cur.Track))
	return mcp.Result{Text: "Playing " + named(cur.Track) + "."}, nil
}

func (a *App) toolPause(context.Context, json.RawMessage) (mcp.Result, error) {
	p, err := a.assistantPlayer()
	if err != nil {
		return mcp.Result{}, err
	}
	st := p.State()
	cur, ok := st.Current()
	if !ok || !remoteStateOf(st).Playing {
		return mcp.Result{Text: "Nothing plays: it was paused already."}, nil
	}
	remoteHost{a}.Do(remote.Command{Op: remote.OpPause})
	assistantDid(p, "paused "+named(cur.Track))
	return mcp.Result{Text: "Paused."}, nil
}

func (a *App) toolNext(_ context.Context, args json.RawMessage) (mcp.Result, error) {
	in, err := mcp.Args[struct {
		Count *int `json:"count"`
	}](args)
	if err != nil {
		return mcp.Result{}, err
	}
	count := 1
	if in.Count != nil {
		count = *in.Count
	}
	if count < 1 || count > remote.MaxNext {
		return mcp.Result{}, fmt.Errorf("count is from 1 to %d", remote.MaxNext)
	}
	p, err := a.assistantPlayer()
	if err != nil {
		return mcp.Result{}, err
	}
	st := p.State()
	if _, ok := st.Current(); !ok {
		return mcp.Result{}, errors.New("nothing is in the queue")
	}
	if count == 1 {
		remoteHost{a}.Do(remote.Command{Op: remote.OpNext})
		assistantDid(p, "skipped to the next track")
		return mcp.Result{Text: "Skipped to the next track."}, nil
	}
	next := remoteStateOf(st).Next
	if count > len(next) {
		return mcp.Result{}, fmt.Errorf("only %d tracks come next", len(next))
	}
	i := slices.IndexFunc(st.Queue, func(it player.Item) bool { return it.UID == next[count-1].Entry })
	if i < 0 {
		return mcp.Result{}, errors.New("the queue changed: try again")
	}
	remoteHost{a}.Do(remote.Command{Op: remote.OpPlayEntry, Entry: next[count-1].Entry})
	assistantDid(p, "skipped ahead to "+named(st.Queue[i].Track))
	return mcp.Result{Text: "Skipped ahead: " + named(st.Queue[i].Track) + " plays."}, nil
}

func (a *App) toolPrevious(context.Context, json.RawMessage) (mcp.Result, error) {
	p, err := a.assistantPlayer()
	if err != nil {
		return mcp.Result{}, err
	}
	if _, ok := p.State().Current(); !ok {
		return mcp.Result{}, errors.New("nothing is in the queue")
	}
	remoteHost{a}.Do(remote.Command{Op: remote.OpPrevious})
	assistantDid(p, "went back")
	return mcp.Result{Text: "Went back."}, nil
}

func (a *App) toolSeek(_ context.Context, args json.RawMessage) (mcp.Result, error) {
	in, err := mcp.Args[struct {
		Position *float64 `json:"position_seconds"`
		By       *float64 `json:"by_seconds"`
	}](args)
	if err != nil {
		return mcp.Result{}, err
	}
	if (in.Position == nil) == (in.By == nil) {
		return mcp.Result{}, errors.New("give position_seconds or by_seconds, one of them")
	}
	p, err := a.assistantPlayer()
	if err != nil {
		return mcp.Result{}, err
	}
	st := p.State()
	cur, ok := st.Current()
	rs := remoteStateOf(st)
	switch {
	case !ok:
		return mcp.Result{}, errors.New("nothing is in the queue")
	case rs.Live:
		return mcp.Result{}, errors.New("a live stream can't move")
	case rs.Loading:
		return mcp.Result{}, errors.New("the track is loading: try again in a moment")
	}
	var to float64
	if in.Position != nil {
		to = *in.Position * 1000
	} else {
		to = float64(rs.PositionMs) + *in.By*1000
	}
	if math.IsNaN(to) || math.IsInf(to, 0) {
		return mcp.Result{}, errors.New("that isn't a place in the track")
	}
	to = max(to, 0)
	if rs.DurationMs > 0 {
		to = min(to, float64(rs.DurationMs))
	}
	ms := int64(to)
	remoteHost{a}.Do(remote.Command{Op: remote.OpSeek, PositionMs: ms})
	assistantDid(p, "moved "+named(cur.Track)+" to "+minutesOf(ms))
	return mcp.Result{Text: "Moved to " + minutesOf(ms) + "."}, nil
}

func (a *App) toolVolume(_ context.Context, args json.RawMessage) (mcp.Result, error) {
	in, err := mcp.Args[struct {
		Percent *int `json:"percent"`
		By      *int `json:"by_percent"`
	}](args)
	if err != nil {
		return mcp.Result{}, err
	}
	if (in.Percent == nil) == (in.By == nil) {
		return mcp.Result{}, errors.New("give percent or by_percent, one of them")
	}
	p, err := a.assistantPlayer()
	if err != nil {
		return mcp.Result{}, err
	}
	var to int
	if in.Percent != nil {
		to = *in.Percent
	} else {
		to = int(math.Round(p.State().Volume*100)) + *in.By
	}
	to = min(max(to, 0), 100)
	remoteHost{a}.Do(remote.Command{Op: remote.OpVolume, Volume: float64(to) / 100})
	assistantDid(p, fmt.Sprintf("set the volume to %d%%", to))
	return mcp.Result{Text: fmt.Sprintf("The volume is %d%%.", to)}, nil
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

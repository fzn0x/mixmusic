package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/match"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/youtube"
	"github.com/fzn0x/mixmusic/lyrics"
)

// shelfTimeout bounds how long the home page waits for each service.
const shelfTimeout = 15 * time.Second

// Catalog searches and browses every service.
type Catalog struct{ a *App }

// SearchResponse holds what each service found.
type SearchResponse struct {
	YouTube media.SearchResults `json:"youtube"`
	Spotify media.SearchResults `json:"spotify"`
	// SpotifySignedIn tells whether Spotify was searched.
	SpotifySignedIn bool `json:"spotifySignedIn"`
	// Errors are the services' errors, for those that failed.
	Errors []string `json:"errors"`
}

func emptyResults() media.SearchResults {
	return media.SearchResults{Tracks: []media.Track{}, Albums: []media.Album{}, Artists: []media.Artist{}, Playlists: []media.Playlist{}}
}

// Search searches YouTube Music, and Spotify when the user signed in.
func (c *Catalog) Search(ctx context.Context, query string) (SearchResponse, error) {
	query = strings.TrimSpace(query)
	res := SearchResponse{YouTube: emptyResults(), Spotify: emptyResults(), SpotifySignedIn: c.a.spotify.SignedIn(), Errors: []string{}}
	if query == "" {
		return res, nil
	}
	var (
		wg           sync.WaitGroup
		ytErr, spErr error
	)
	wg.Go(func() { res.YouTube, ytErr = c.a.youtube.SearchAll(ctx, query) })
	if res.SpotifySignedIn {
		wg.Go(func() { res.Spotify, spErr = c.a.spotify.Search(ctx, query) })
	}
	wg.Wait()
	if ytErr != nil {
		res.YouTube = emptyResults()
		res.Errors = append(res.Errors, ytErr.Error())
	}
	if spErr != nil {
		res.Spotify = emptyResults()
		res.Errors = append(res.Errors, spErr.Error())
	}
	if ytErr != nil && (spErr != nil || !res.SpotifySignedIn) {
		return res, ytErr
	}
	c.a.library.AddSearch(query)
	return res, nil
}

// Suggestions returns searches to suggest for what the user typed.
func (c *Catalog) Suggestions(ctx context.Context, input string) ([]string, error) {
	if strings.TrimSpace(input) == "" {
		return c.a.library.Searches(), nil
	}
	return c.a.youtube.Suggestions(ctx, input)
}

// Home returns the shelves of the home page: the user's own, then
// Spotify's when they signed in, then YouTube Music's.
func (c *Catalog) Home(ctx context.Context) ([]media.Shelf, error) {
	ctx, cancel := context.WithTimeout(ctx, shelfTimeout)
	defer cancel()
	var (
		wg      sync.WaitGroup
		sp, yt  []media.Shelf
		ytErr   error
		userID  = c.a.spotifyUserID(ctx)
		shelves = c.localShelves()
	)
	if userID != "" {
		wg.Go(func() { sp = c.a.spotify.Home(ctx, userID) })
	}
	wg.Go(func() { yt, ytErr = c.a.youtube.Home(ctx) })
	wg.Wait()
	shelves = append(shelves, sp...)
	shelves = append(shelves, yt...)
	if len(shelves) == 0 && ytErr != nil {
		return nil, ytErr
	}
	return shelves, nil
}

// localShelves returns the shelves of what the user played and made.
func (c *Catalog) localShelves() []media.Shelf {
	var shelves []media.Shelf
	if recent := c.a.library.RecentTracks(16); len(recent) > 0 {
		items := make([]media.Item, len(recent))
		for i := range recent {
			items[i] = media.Item{Kind: media.ItemTrack, Track: &recent[i]}
		}
		shelves = append(shelves, media.Shelf{Title: "Recently played", Source: media.SourceLocal, Items: items})
	}
	if playlists := c.a.library.Playlists(); len(playlists) > 0 {
		items := make([]media.Item, len(playlists))
		for i := range playlists {
			items[i] = media.Item{Kind: media.ItemPlaylist, Playlist: &playlists[i]}
		}
		shelves = append(shelves, media.Shelf{Title: "Your mixes", Source: media.SourceLocal, Items: items})
	}
	return shelves
}

// Album returns an album with its tracks.
func (c *Catalog) Album(ctx context.Context, id string) (media.Album, error) {
	source, native, err := media.ParseID(id)
	if err != nil {
		return media.Album{}, err
	}
	switch source {
	case media.SourceYouTube:
		return c.a.youtube.Album(ctx, native)
	case media.SourceSpotify:
		a, err := c.a.spotify.Album(ctx, native)
		if err == nil {
			a.Saved = c.a.spotifyHas(ctx, spotify.URI("album", native))
		}
		return a, err
	}
	return media.Album{}, fmt.Errorf("no album %s", id)
}

// Artist returns an artist's page.
func (c *Catalog) Artist(ctx context.Context, id string) (media.Artist, error) {
	source, native, err := media.ParseID(id)
	if err != nil {
		return media.Artist{}, err
	}
	switch source {
	case media.SourceYouTube:
		return c.a.youtube.Artist(ctx, native)
	case media.SourceSpotify:
		a, err := c.a.spotify.Artist(ctx, native)
		if err == nil {
			a.Following = c.a.spotifyHas(ctx, spotify.URI("artist", native))
		}
		return a, err
	}
	return media.Artist{}, fmt.Errorf("no artist %s", id)
}

// Playlist returns a playlist with its tracks, of any service or made in
// MixMusic.
func (c *Catalog) Playlist(ctx context.Context, id string) (media.Playlist, error) {
	return c.a.playlist(ctx, id)
}

func (a *App) playlist(ctx context.Context, id string) (media.Playlist, error) {
	source, native, err := media.ParseID(id)
	if err != nil {
		return media.Playlist{}, err
	}
	switch source {
	case media.SourceYouTube:
		return a.youtube.Playlist(ctx, native)
	case media.SourceSpotify:
		p, err := a.spotify.Playlist(ctx, native, a.spotifyUserID(ctx))
		if err == nil && !p.Editable {
			p.Saved = a.spotifyHas(ctx, spotify.URI("playlist", native))
		}
		return p, err
	case media.SourceLocal:
		return a.library.Playlist(native)
	}
	return media.Playlist{}, fmt.Errorf("no playlist %s", id)
}

// Radio returns tracks like the one given, to play after it. Spotify
// tracks start the radio of their YouTube Music recording. While the
// settings skip the songs the user disliked, the radio leaves them out.
func (c *Catalog) Radio(ctx context.Context, track media.Track) ([]media.Track, error) {
	tracks, err := c.radioOf(ctx, track)
	if err != nil || !c.a.settings.Get().SkipDisliked {
		return tracks, err
	}
	return withoutDisliked(tracks, c.a.library.Disliked()), nil
}

// withoutDisliked leaves out of tracks the songs of disliked: the same
// uploads, and the other uploads of the same songs, which radios take for
// one song.
func withoutDisliked(tracks, disliked []media.Track) []media.Track {
	if len(disliked) == 0 {
		return tracks
	}
	ids := make(map[string]bool, len(disliked))
	songs := make(map[string]bool, len(disliked))
	for _, t := range disliked {
		ids[t.ID] = true
		songs[player.SongKey(t)] = true
	}
	out := make([]media.Track, 0, len(tracks))
	for _, t := range tracks {
		if !ids[t.ID] && !songs[player.SongKey(t)] {
			out = append(out, t)
		}
	}
	return out
}

// radioOf returns YouTube Music's radio of track, or of its recording
// there.
func (c *Catalog) radioOf(ctx context.Context, track media.Track) ([]media.Track, error) {
	seed, err := c.a.matcher.Match(ctx, track)
	if err != nil {
		return nil, err
	}
	_, videoID, err := media.ParseID(seed.ID)
	if err != nil {
		return nil, err
	}
	tracks, err := c.a.youtube.Radio(ctx, videoID)
	if err != nil {
		return nil, err
	}
	out := make([]media.Track, 0, len(tracks))
	for _, t := range tracks {
		if t.ID != seed.ID && t.ID != track.ID {
			out = append(out, t)
		}
	}
	return out, nil
}

// Lyrics returns the lyrics of a track: synchronized ones from LRCLIB
// when it has them, else YouTube Music's.
func (c *Catalog) Lyrics(ctx context.Context, track media.Track) (lyrics.Lyrics, error) {
	if l, ok := c.a.lyricsMemo.get(track.ID); ok {
		return l, nil
	}
	l, err := c.a.lyrics.Get(ctx, lyricsQuery(track))
	// YouTube Music fills in when LRCLIB has nothing, or is down, and
	// when its plain lyrics are longer than LRCLIB's.
	if err != nil || (!l.Synced && !l.Instrumental) {
		if yt, ytErr := c.youtubeLyrics(ctx, track); ytErr == nil && (err != nil || len(yt.Lines) > len(l.Lines)) {
			l, err = yt, nil
		}
	}
	if err != nil {
		// A track LRCLIB was too busy to look everywhere for reads the
		// same, but the lyrics panel offers to try again.
		if errors.Is(err, lyrics.ErrNotFound) || errors.Is(err, lyrics.ErrNotFoundYet) {
			return lyrics.Lyrics{}, lyricsError{"No lyrics found for this track.", err}
		}
		log.Printf("mixmusic: the lyrics of %s: %v", track.ID, err)
		return lyrics.Lyrics{}, lyricsError{"Lyrics can't be loaded right now. Check your connection, or try again in a moment.", err}
	}
	c.a.lyricsMemo.put(track.ID, l)
	return l, nil
}

// lyricsQuery asks LRCLIB for a track: its title without what YouTube
// adds, as "(Official Video)", and its first artist.
func lyricsQuery(t media.Track) lyrics.Query {
	q := lyrics.Query{Title: match.CleanTitle(t.Title), Duration: time.Duration(t.DurationMs) * time.Millisecond}
	if len(t.Artists) > 0 {
		q.Artist = t.Artists[0].Name
	}
	if t.Album != nil {
		q.Album = t.Album.Name
	}
	return q
}

// lyricsError is why a track's lyrics don't show, in the words the lyrics
// panel shows; what it wraps tells a track without lyrics from a failure
// worth trying again.
type lyricsError struct {
	text string
	err  error
}

func (e lyricsError) Error() string { return e.text }
func (e lyricsError) Unwrap() error { return e.err }

// youtubeLyrics returns YouTube Music's lyrics of a track, or of its
// recording there.
func (c *Catalog) youtubeLyrics(ctx context.Context, track media.Track) (lyrics.Lyrics, error) {
	yt := track
	if track.Source != media.SourceYouTube {
		var err error
		if yt, err = c.a.matcher.Match(ctx, track); err != nil {
			return lyrics.Lyrics{}, err
		}
	}
	_, videoID, err := media.ParseID(yt.ID)
	if err != nil {
		return lyrics.Lyrics{}, err
	}
	text, source, err := c.a.youtube.Lyrics(ctx, videoID)
	if err != nil {
		return lyrics.Lyrics{}, err
	}
	return lyrics.Plain(text, strings.TrimPrefix(source, "Source: ")), nil
}

// Explore returns YouTube Music's new albums and singles, trending songs
// and new music videos.
func (c *Catalog) Explore(ctx context.Context) ([]media.Shelf, error) {
	return c.a.youtube.Explore(ctx)
}

// Charts returns YouTube Music's charts: top songs, videos and artists.
func (c *Catalog) Charts(ctx context.Context) ([]media.Shelf, error) {
	return c.a.youtube.Charts(ctx)
}

// Moods returns the moods and the genres to browse YouTube Music by.
func (c *Catalog) Moods(ctx context.Context) ([]youtube.MoodGroup, error) {
	return c.a.youtube.Moods(ctx)
}

// MoodPage is a mood or a genre, with its shelves of playlists.
type MoodPage struct {
	Title   string        `json:"title"`
	Shelves []media.Shelf `json:"shelves"`
}

// Mood returns the page of a mood or a genre, by the params of Moods.
func (c *Catalog) Mood(ctx context.Context, params string) (MoodPage, error) {
	title, shelves, err := c.a.youtube.Mood(ctx, params)
	return MoodPage{Title: title, Shelves: shelves}, err
}

// Related returns what YouTube Music relates to a track: songs, playlists,
// other performances and similar artists. A Spotify track shows what
// relates to its recording on YouTube Music.
func (c *Catalog) Related(ctx context.Context, track media.Track) ([]media.Shelf, error) {
	seed, err := c.a.matcher.Match(ctx, track)
	if err != nil {
		return nil, err
	}
	if shelves, ok := c.a.related.get(seed.ID); ok {
		return shelves, nil
	}
	_, videoID, err := media.ParseID(seed.ID)
	if err != nil {
		return nil, err
	}
	shelves, err := c.a.youtube.Related(ctx, videoID)
	if err != nil {
		return nil, err
	}
	c.a.related.put(seed.ID, shelves)
	return shelves, nil
}

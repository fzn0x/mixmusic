package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/match"
	"github.com/fzn0x/mixmusic/internal/media"
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

// songWait is how long a radio waits for YouTube Music to find the song of
// a music video, before it goes on without it.
var songWait = 5 * time.Second

// relayTries is how many of the videos in the radio of an upload without
// a song are looked up for a song to carry on from.
const relayTries = 3

// Radio returns tracks like the one given, to play after it. Spotify
// tracks start the radio of their YouTube Music recording.
//
// YouTube Music makes the radio of a music video out of videos, so a video
// starts the radio of its song. An upload without a song, such as a
// cover, starts the radio of the first song its own radio suggests, or
// when it suggests none, of the song of one of its first videos. When
// there is no such song, or YouTube Music doesn't answer, the video's own
// radio stands. The song of the video that plays is left out, so that it
// isn't heard twice.
func (c *Catalog) Radio(ctx context.Context, track media.Track) ([]media.Track, error) {
	seed, err := c.a.matcher.Match(ctx, track)
	if err != nil {
		return nil, err
	}
	skip := []string{track.ID, seed.ID}
	if seed.Video {
		song, err := c.songOf(ctx, seed)
		switch {
		case err == nil:
			tracks, err := c.radio(ctx, song, append(skip, song.ID))
			if err != nil || len(tracks) > 0 {
				return tracks, err
			}
		case errors.Is(err, match.ErrNoMatch):
			tracks, err := c.radio(ctx, seed, skip)
			if err != nil {
				return nil, err
			}
			return c.relay(ctx, tracks, skip), nil
		}
		// The song's radio is empty, or YouTube Music didn't find the
		// song in time: the video's own radio, as before.
	}
	return c.radio(ctx, seed, skip)
}

// radio returns YouTube Music's radio of seed, without the tracks whose
// IDs are in skip.
func (c *Catalog) radio(ctx context.Context, seed media.Track, skip []string) ([]media.Track, error) {
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
		if !slices.Contains(skip, t.ID) {
			out = append(out, t)
		}
	}
	return out, nil
}

// relay carries on from tracks, the radio of an upload without a song,
// with a radio of songs: the radio of the first song in it, or when it
// has none, of the song of one of its first videos. It returns tracks
// when there is no such song, or that radio is empty, or YouTube Music
// doesn't answer.
func (c *Catalog) relay(ctx context.Context, tracks []media.Track, skip []string) []media.Track {
	if i := slices.IndexFunc(tracks, isSong); i >= 0 {
		if found, err := c.radio(ctx, tracks[i], skip); err == nil && len(found) > 0 {
			return found
		}
		return tracks
	}
	for _, v := range tracks[:min(len(tracks), relayTries)] {
		song, err := c.songOf(ctx, v)
		if errors.Is(err, match.ErrNoMatch) {
			continue
		}
		if err != nil {
			break // no more waits on a YouTube Music that doesn't answer
		}
		found, err := c.radio(ctx, song, skip)
		if err != nil {
			break
		}
		if len(found) > 0 {
			return found
		}
	}
	return tracks
}

// songOf returns the song of a music video, as Matcher.Song finds it,
// waiting at most songWait.
func (c *Catalog) songOf(ctx context.Context, video media.Track) (media.Track, error) {
	ctx, cancel := context.WithTimeout(ctx, songWait)
	defer cancel()
	song, err := c.a.matcher.Song(ctx, video)
	if err != nil && !errors.Is(err, match.ErrNoMatch) && !errors.Is(err, context.Canceled) {
		log.Printf("mixmusic: the song of %s: %v", video.ID, err)
	}
	return song, err
}

func isSong(t media.Track) bool { return !t.Video }

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

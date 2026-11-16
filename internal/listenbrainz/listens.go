package listenbrainz

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/fzn0x/mixmusic/internal/scrobble"
)

// MaxListens is the most listens one submission takes, as ListenBrainz's
// MAX_LISTENS_PER_REQUEST says.
const MaxListens = 1000

// Track is a song as the scrobbling services know it.
type Track = scrobble.Track

// Play is a listen to a track: the track and when it started playing.
type Play = scrobble.Play

var _ scrobble.Service = (*Client)(nil)

// client is how MixMusic names itself, as the client that submits and as
// the player that plays.
const client = "MixMusic"

// submission is the JSON of POST /1/submit-listens.
type submission struct {
	// ListenType is "playing_now", "single" or "import".
	ListenType string   `json:"listen_type"`
	Payload    []listen `json:"payload"`
}

type listen struct {
	// ListenedAt is when the track started playing, in Unix seconds; a
	// now-playing update has none.
	ListenedAt int64         `json:"listened_at,omitempty"`
	Track      trackMetadata `json:"track_metadata"`
}

type trackMetadata struct {
	ArtistName  string         `json:"artist_name"`
	TrackName   string         `json:"track_name"`
	ReleaseName string         `json:"release_name,omitempty"`
	Info        additionalInfo `json:"additional_info"`
}

// additionalInfo is what ListenBrainz's docs ask clients to tell of a
// listen, where they know it.
type additionalInfo struct {
	SubmissionClient        string `json:"submission_client"`
	SubmissionClientVersion string `json:"submission_client_version,omitempty"`
	MediaPlayer             string `json:"media_player"`
	MediaPlayerVersion      string `json:"media_player_version,omitempty"`
	// MusicService is the domain of the service, and OriginURL the
	// track's page there.
	MusicService string `json:"music_service,omitempty"`
	OriginURL    string `json:"origin_url,omitempty"`
	DurationMs   int64  `json:"duration_ms,omitempty"`
	// SpotifyID is the track's address on Spotify, as the docs ask.
	SpotifyID string `json:"spotify_id,omitempty"`
	ISRC      string `json:"isrc,omitempty"`
}

// listen is the listen of t that started at start, zero for a track that
// starts playing.
func (c *Client) listen(t Track, start time.Time) listen {
	l := listen{Track: trackMetadata{ArtistName: t.Artist, TrackName: t.Title, ReleaseName: t.Album, Info: additionalInfo{
		SubmissionClient: client, SubmissionClientVersion: c.Version, MediaPlayer: client, MediaPlayerVersion: c.Version,
		MusicService: t.Service, OriginURL: t.URL, DurationMs: t.Duration.Milliseconds(), SpotifyID: t.SpotifyURL, ISRC: t.ISRC,
	}}}
	if !start.IsZero() {
		l.ListenedAt = start.Unix()
	}
	return l
}

// NowPlaying tells the server that the user of token started listening to
// t, and hands OnRecording the MessyBrainz ID of its recording, which the
// server keeps for loves.
func (c *Client) NowPlaying(ctx context.Context, token string, t Track) error {
	var answer struct {
		RecordingMSID string `json:"recording_msid"`
	}
	body := submission{ListenType: "playing_now", Payload: []listen{c.listen(t, time.Time{})}}
	if err := c.call(ctx, http.MethodPost, "submit-listens", token, url.Values{"return_msid": {"true"}}, body, &answer); err != nil {
		return err
	}
	if answer.RecordingMSID != "" && c.OnRecording != nil {
		c.OnRecording(t, answer.RecordingMSID)
	}
	return nil
}

// Scrobble adds plays to the listens of the user of token: one as a
// single listen, more as an import of a backlog, up to MaxListens per
// submission. When a submission fails, Scrobble returns its error without
// making the rest; the listens of the submissions before were taken.
func (c *Client) Scrobble(ctx context.Context, token string, plays []Play) error {
	kind := "import"
	if len(plays) == 1 {
		kind = "single"
	}
	for batch := range slices.Chunk(plays, MaxListens) {
		body := submission{ListenType: kind}
		for _, p := range batch {
			body.Payload = append(body.Payload, c.listen(p.Track, p.Start))
		}
		if err := c.call(ctx, http.MethodPost, "submit-listens", token, nil, body, nil); err != nil {
			return err
		}
	}
	return nil
}

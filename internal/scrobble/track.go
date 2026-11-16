package scrobble

import (
	"strings"
	"time"
)

// Track is a song as the scrobbling services know it.
type Track struct {
	// Artist and Title name the track. Tracks without both are neither
	// announced nor scrobbled.
	Artist string `json:"artist"`
	Title  string `json:"title"`
	// Album is empty when it is not known.
	Album string `json:"album,omitempty"`
	// Duration is the length of the track, or 0 when it is not known. In
	// JSON it is a number of nanoseconds.
	Duration time.Duration `json:"duration,omitempty"`
	// ID is MixMusic's ID of the track, as "yt:…", which no service hears:
	// what the app keeps of a service's answers about the track goes by it.
	ID string `json:"id,omitempty"`
	// Service is the domain of the service the track plays from, as
	// music.youtube.com, and URL its page there; both are empty for the
	// music on this computer. Services that keep them, as ListenBrainz,
	// hear them.
	Service string `json:"service,omitempty"`
	URL     string `json:"url,omitempty"`
	// SpotifyURL is the track's address on Spotify, and ISRC its
	// recording's code, where they are known.
	SpotifyURL string `json:"spotifyUrl,omitempty"`
	ISRC       string `json:"isrc,omitempty"`
}

// complete reports whether t has the artist and title the services need.
func (t Track) complete() bool {
	return strings.TrimSpace(t.Artist) != "" && strings.TrimSpace(t.Title) != ""
}

// Play is a listen to a track: the track and when it started playing.
type Play struct {
	Track
	// Start is when the track started playing.
	Start time.Time `json:"start"`
}

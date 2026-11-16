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

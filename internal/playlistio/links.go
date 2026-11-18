package playlistio

import (
	"net/url"
	"regexp"
	"strings"
)

var (
	videoIDPattern   = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	spotifyIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{22}$`)
)

// link reads a link to a song or its ID: a YouTube video's watch URL or
// ID, a Spotify track's URL or URI. bare takes an ID alone, as a column
// of IDs holds them; else only one that can't be a word is taken.
func link(s string, bare bool) (videoID, spotifyID string) {
	s = strings.TrimSpace(s)
	if id, ok := strings.CutPrefix(s, "spotify:track:"); ok && spotifyIDPattern.MatchString(id) {
		return "", id
	}
	if videoIDPattern.MatchString(s) && (bare || idLike(s)) {
		return s, ""
	}
	if spotifyIDPattern.MatchString(s) && bare {
		return "", s
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", ""
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch {
	case host == "open.spotify.com" || host == "play.spotify.com":
		// /track/ID, and /intl-de/track/ID of the links of other
		// languages.
		if len(parts) > 0 && strings.HasPrefix(parts[0], "intl-") {
			parts = parts[1:]
		}
		if len(parts) == 2 && parts[0] == "track" && spotifyIDPattern.MatchString(parts[1]) {
			return "", parts[1]
		}
	case host == "youtu.be":
		if len(parts) == 1 && videoIDPattern.MatchString(parts[0]) {
			return parts[0], ""
		}
	case host == "youtube.com" || strings.HasSuffix(host, ".youtube.com") || host == "youtube-nocookie.com":
		if v := u.Query().Get("v"); videoIDPattern.MatchString(v) {
			return v, ""
		}
		if len(parts) == 2 && (parts[0] == "shorts" || parts[0] == "embed" || parts[0] == "live" || parts[0] == "v") &&
			videoIDPattern.MatchString(parts[1]) {
			return parts[1], ""
		}
	}
	return "", ""
}

// idLike reports whether an 11-character word is rather a video's ID than
// a word: it has a digit, a dash or an underscore, or a capital past its
// first letter.
func idLike(s string) bool {
	if strings.ContainsAny(s, "0123456789-_") {
		return true
	}
	return strings.ToLower(s[1:]) != s[1:]
}

// isLink reports whether s is a link of the web, which names no file.
func isLink(s string) bool {
	u, err := url.Parse(strings.TrimSpace(s))
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

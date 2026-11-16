package settings

import (
	"regexp"
	"strings"
)

// pageIDRe matches the page ID of a YouTube brand channel, a number.
var pageIDRe = regexp.MustCompile(`^[0-9]{1,32}$`)

// Accounts is how MixMusic uses the user's accounts: Settings › Accounts.
type Accounts struct {
	// SpotifyClientID is the Client ID of the user's Spotify app, which
	// they sign in to Spotify with; "" until they paste one.
	SpotifyClientID string `json:"spotifyClientId"`
	// Scrobbling sends what plays to Last.fm, once the user connected it.
	Scrobbling bool `json:"scrobbling"`
	// LastfmLoves sends the user's likes to Last.fm as loves, and their
	// unlikes as unloves, once they connected it.
	LastfmLoves bool `json:"lastfmLoves"`
	// ListenBrainzScrobbling sends what plays to ListenBrainz, or to the
	// user's own server of its API, once they connected it.
	ListenBrainzScrobbling bool `json:"listenbrainzScrobbling"`
	// ListenBrainzLoves sends the user's likes to ListenBrainz as loves,
	// and their unlikes as the loves taken back, once they connected it.
	ListenBrainzLoves bool `json:"listenbrainzLoves"`
	// YouTubeChannel is the brand channel the user uses YouTube Music as,
	// by its page ID; "" is the account itself.
	YouTubeChannel string `json:"youtubeChannel"`
	// YouTubeHistory adds what the user plays to their history on YouTube
	// Music, once they signed in, for its recommendations, and where they
	// left episodes, which YouTube Music resumes them from.
	YouTubeHistory bool `json:"youtubeHistory"`
}

// defaultAccounts is how a new install uses the accounts.
func defaultAccounts() Accounts {
	var a Accounts
	a.Scrobbling = true
	a.ListenBrainzScrobbling = true
	a.YouTubeHistory = true
	return a
}

// clean is a without the spaces around its IDs, and without a channel
// that isn't a page ID.
func (a Accounts) clean() Accounts {
	a.SpotifyClientID = strings.TrimSpace(a.SpotifyClientID)
	if a.YouTubeChannel = strings.TrimSpace(a.YouTubeChannel); !pageIDRe.MatchString(a.YouTubeChannel) {
		a.YouTubeChannel = ""
	}
	return a
}

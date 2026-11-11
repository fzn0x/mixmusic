package app

import "net/http"

// audioAsAccountSession returns the cookies of the user's YouTube Music
// session for the resolver to ask for streams as, while "Play audio as
// your account" is on (stream.Resolver.SetAsAccount): nil signed out,
// once YouTube ended the session, and while MixMusic uses another Google
// account of the session than its first. yt-dlp asks as the first one,
// as YouTube serves the watch page it reads to the first account of a
// session, which may not be the account the user chose, nor one that
// has Premium.
func (a *App) audioAsAccountSession() []*http.Cookie {
	if !a.youtube.SignedIn() || a.youtube.Rejected() || a.youtube.AuthUser() != 0 {
		return nil
	}
	return a.youtube.Cookies()
}

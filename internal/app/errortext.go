package app

import (
	"iter"
	"net/http"
	"strings"

	"github.com/fzn0x/mixmusic/internal/addons"
	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/colorfile"
	"github.com/fzn0x/mixmusic/internal/firewall"
	"github.com/fzn0x/mixmusic/internal/lastfm"
	"github.com/fzn0x/mixmusic/internal/librefm"
	"github.com/fzn0x/mixmusic/internal/listenbrainz"
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/internal/tagwrite"
	"github.com/fzn0x/mixmusic/internal/tools"
	"github.com/fzn0x/mixmusic/internal/youtube"
	"github.com/fzn0x/mixmusic/locales"
)

// The errors of the services and of the system keep their English, for
// the logs and the failure details' Copy; the windows show each they know
// as a message of the catalog, in their language (#800). An error of no
// message keeps its own words: the service's or the system's, which the
// message around it, where there is one, leads.

// ErrorText is err as the windows show it: the error it wraps that the
// windows know, the first of its chain, as its message, in the words
// around it, which MixMusic or the service added; or err's own words.
// MixMusic's own errors, which WordsOf says, count first.
func ErrorText(err error) string {
	if err == nil {
		return ""
	}
	for e := range errorChain(err) {
		said, ok := WordsOf(e)
		if !ok {
			said, ok = serviceErrorText(e)
		}
		if !ok {
			said, ok = systemErrorText(e)
		}
		if !ok {
			continue
		}
		whole, part := err.Error(), e.Error()
		if i := strings.Index(whole, part); i >= 0 && whole != part {
			return whole[:i] + said + whole[i+len(part):]
		}
		return said
	}
	return err.Error()
}

// errorChain yields err, then the errors it wraps, in turn, the
// shallowest first.
func errorChain(err error) iter.Seq[error] {
	return func(yield func(error) bool) {
		queue := []error{err}
		for len(queue) > 0 {
			e := queue[0]
			queue = queue[1:]
			if e == nil {
				continue
			}
			if !yield(e) {
				return
			}
			switch u := e.(type) {
			case interface{ Unwrap() error }:
				queue = append(queue, u.Unwrap())
			case interface{ Unwrap() []error }:
				queue = append(queue, u.Unwrap()...)
			}
		}
	}
}

// asSpotify tells whether err is an error of Spotify's API, itself.
func asSpotify(err error, sp **spotify.APIError) bool {
	e, ok := err.(*spotify.APIError)
	*sp = e
	return ok
}

// serviceErrorText is err itself, of a service, not the errors it wraps,
// as the windows show it, when they know it.
func serviceErrorText(err error) (string, bool) {
	var sp *spotify.APIError
	switch {
	case err == spotify.ErrNoClientID:
		return locales.ErrorSpotifyNoClientID(), true
	case err == spotify.ErrSignInExpired:
		return locales.ErrorSpotifySignInAgain(), true
	case err == spotify.ErrNotSignedIn:
		return locales.ErrorSpotifyNotSignedIn(), true
	case err == spotify.ErrPlaylistUnavailable:
		return locales.ErrorSpotifyPlaylistUnavailable(), true
	case err == spotify.ErrPlaylistNotShown:
		return locales.ErrorSpotifyPlaylistNotShown(), true
	case err == spotify.ErrPlaylistRefused:
		return locales.ErrorSpotifyPlaylistRefused(), true
	case asSpotify(err, &sp) && sp.Reason == "PREMIUM_REQUIRED":
		return locales.ErrorSpotifyPremium(), true
	case asSpotify(err, &sp) && sp.Status == http.StatusTooManyRequests:
		return locales.ErrorSpotifyTooMany(), true
	case err == youtube.ErrSignedOut:
		return locales.ErrorYouTubeSignedOut(), true
	case err == youtube.ErrSessionRejected:
		return locales.ErrorYouTubeSessionRejected(), true
	case err == youtube.ErrNotFound:
		return locales.ErrorYouTubeNotFound(), true
	case err == youtube.ErrAlreadyInPlaylist:
		return locales.ErrorYouTubeAlreadyInPlaylist(), true
	case err == youtube.ErrNoSongList:
		return locales.ErrorYouTubeNoSongList(), true
	case err == lastfm.ErrNotAuthorized:
		return locales.ErrorLastfmNotAuthorized(), true
	case err == listenbrainz.ErrNotServer:
		return locales.ErrorListenBrainzNotServer(), true
	case err == librefm.ErrNotServer:
		return locales.ErrorLibrefmNotServer(), true
	}
	if txt, ok := streamAccountText(stream.Reason(err), stream.Account(err)); ok {
		return txt, true
	}
	if txt, ok := streamReasonText(stream.Reason(err)); ok {
		return txt, true
	}
	switch {
	case err == stream.ErrThrottled:
		return locales.ErrorStreamThrottled(), true
	case err == stream.ErrNoAudio:
		return locales.ErrorStreamNoAudio(), true
	case err == stream.ErrLive:
		return locales.ErrorStreamLive(), true
	case err == stream.ErrNoCaptions:
		return locales.ErrorStreamNoCaptions(), true
	case err == stream.ErrBadVideoID:
		return locales.ErrorStreamBadVideoID(), true
	case err == stream.ErrUnavailable:
		return locales.ErrorStreamUnavailable(), true
	case err == stream.ErrNetwork:
		return locales.ErrorStreamNetwork(), true
	}
	return "", false
}

// streamReasonText is a reason of stream.Reason as the windows say it.
func streamReasonText(reason string) (string, bool) {
	switch reason {
	case stream.ReasonBot:
		return locales.ErrorStreamBot(), true
	case stream.ReasonThrottled:
		return locales.ErrorStreamThrottled(), true
	case stream.ReasonAdults:
		return locales.ErrorStreamAdults(), true
	case stream.ReasonSignedIn:
		return locales.ErrorStreamSignedInOnly(), true
	case stream.ReasonCountry:
		return locales.ErrorStreamCountry(), true
	case stream.ReasonPrivate:
		return locales.ErrorStreamPrivate(), true
	case stream.ReasonMembers:
		return locales.ErrorStreamMembers(), true
	case stream.ReasonPremium:
		return locales.ErrorStreamPremium(), true
	case stream.ReasonNoFormat:
		return locales.ErrorStreamNoFormat(), true
	case stream.ReasonGone:
		return locales.ErrorStreamGone(), true
	case stream.ReasonRefused:
		return locales.ErrorStreamRefused(), true
	case stream.ReasonStalled:
		return locales.ErrorStreamStalled(), true
	}
	return "", false
}

// streamAccountText is why a track YouTube plays only to some signed-in
// viewers didn't play, of the reason, as the windows say it, with how
// asking as the user's account went (stream.Account): what to change for
// the account to be asked, or that it was too (#577).
func streamAccountText(reason, account string) (string, bool) {
	why, ok := streamReasonText(reason)
	if !ok {
		return "", false
	}
	switch account {
	case stream.AccountOff:
		if reason == stream.ReasonSignedIn {
			return locales.ErrorStreamSignedIn(), true
		}
		return locales.ErrorStreamAccountOff(why), true
	case stream.AccountSignIn:
		return locales.ErrorStreamAccountSignIn(why), true
	case stream.AccountSignedOut:
		return locales.ErrorStreamAccountSignedOut(why), true
	case stream.AccountEnded:
		return locales.ErrorStreamAccountEnded(why), true
	case stream.AccountNotFirst:
		return locales.ErrorStreamAccountNotFirst(why), true
	case stream.AccountSaver:
		return locales.ErrorStreamAccountSaver(why), true
	case stream.AccountNoTools:
		return locales.ErrorStreamAccountNoTools(why), true
	case stream.AccountRefused:
		return locales.ErrorStreamAccountRefused(why), true
	}
	return "", false
}

// systemErrorText is err itself, of the system or of MixMusic's tools and
// files, not the errors it wraps, as the windows show it, when they know
// it.
func systemErrorText(err error) (string, bool) {
	switch {
	case err == tools.ErrNoYTDLP:
		return locales.ErrorToolsNoYTDLP(), true
	case err == tools.ErrBrowserOpen:
		return locales.ErrorToolsBrowserOpen(), true
	case err == tools.ErrCookiesSealed:
		return locales.ErrorToolsCookiesSealed(), true
	case err == tools.ErrNoBrowserCookies:
		return locales.ErrorToolsNoBrowserCookies(), true
	case err == tools.ErrCookiesDenied:
		return locales.ErrorToolsCookiesDenied(), true
	case err == audio.ErrNoOutput:
		return locales.ErrorAudioNoOutput(), true
	case err == audio.ErrCantPlay:
		return locales.ErrorAudioCantPlay(), true
	case err == firewall.ErrUnsupported:
		return locales.ErrorFirewallUnsupported(), true
	case err == firewall.ErrDeclined:
		return locales.ErrorFirewallDeclined(), true
	case err == colorfile.ErrEmpty:
		return locales.ErrorColorFileEmpty(), true
	case err == colorfile.ErrPartial:
		return locales.ErrorColorFilePartial(), true
	case err == colorfile.ErrUnknown:
		return locales.ErrorColorFileUnknown(), true
	case err == colorfile.ErrFolder:
		return locales.ErrorColorFileFolder(), true
	case err == colorfile.ErrTooBig:
		return locales.ErrorColorFileTooBig(), true
	case err == localfiles.ErrNetworkDrive:
		return locales.ErrorLocalFilesNetworkDrive(), true
	case err == localfiles.ErrRemovableDrive:
		return locales.ErrorLocalFilesRemovableDrive(), true
	case err == localfiles.ErrWatchLimit:
		return locales.ErrorLocalFilesWatchLimit(), true
	case err == localfiles.ErrCantWatch:
		return locales.ErrorLocalFilesCantWatch(), true
	case err == tagwrite.ErrUnsupported:
		return locales.ErrorTagsUnsupported(), true
	case err == tagwrite.ErrFormat:
		return locales.ErrorTagsFormat(), true
	case err == tagwrite.ErrReadOnly:
		return locales.ErrorTagsReadOnly(), true
	case err == tagwrite.ErrFolderReadOnly:
		return locales.ErrorTagsFolderReadOnly(), true
	case err == tagwrite.ErrCover:
		return locales.ErrorTagsCover(), true
	case err == addons.ErrBuiltin:
		return locales.ErrorAddonsBuiltin(), true
	}
	return "", false
}

package main

import (
	"errors"
	"net/http"

	"github.com/fzn0x/mixmusic/internal/addons"
	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/colorfile"
	"github.com/fzn0x/mixmusic/internal/firewall"
	"github.com/fzn0x/mixmusic/internal/lastfm"
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

// errorText is err as the windows show it.
func errorText(err error) string {
	if err == nil {
		return ""
	}
	if txt, ok := serviceErrorText(err); ok {
		return txt
	}
	if txt, ok := systemErrorText(err); ok {
		return txt
	}
	return err.Error()
}

// serviceErrorText is err, of a service, as the windows show it, when
// they know it.
func serviceErrorText(err error) (string, bool) {
	var sp *spotify.APIError
	switch {
	case errors.Is(err, spotify.ErrNoClientID):
		return locales.ErrorSpotifyNoClientID(), true
	case errors.Is(err, spotify.ErrSignInExpired):
		return locales.ErrorSpotifySignInAgain(), true
	case errors.Is(err, spotify.ErrNotSignedIn):
		return locales.ErrorSpotifyNotSignedIn(), true
	case errors.Is(err, spotify.ErrPlaylistUnavailable):
		return locales.ErrorSpotifyPlaylistUnavailable(), true
	case errors.Is(err, spotify.ErrPlaylistNotShown):
		return locales.ErrorSpotifyPlaylistNotShown(), true
	case errors.Is(err, spotify.ErrPlaylistRefused):
		return locales.ErrorSpotifyPlaylistRefused(), true
	case errors.As(err, &sp) && sp.Reason == "PREMIUM_REQUIRED":
		return locales.ErrorSpotifyPremium(), true
	case errors.As(err, &sp) && sp.Status == http.StatusTooManyRequests:
		return locales.ErrorSpotifyTooMany(), true
	case errors.Is(err, youtube.ErrSignedOut):
		return locales.ErrorYouTubeSignedOut(), true
	case errors.Is(err, youtube.ErrSessionRejected):
		return locales.ErrorYouTubeSessionRejected(), true
	case errors.Is(err, youtube.ErrNotFound):
		return locales.ErrorYouTubeNotFound(), true
	case errors.Is(err, youtube.ErrAlreadyInPlaylist):
		return locales.ErrorYouTubeAlreadyInPlaylist(), true
	case errors.Is(err, youtube.ErrNoSongList):
		return locales.ErrorYouTubeNoSongList(), true
	case errors.Is(err, lastfm.ErrNotAuthorized):
		return locales.ErrorLastfmNotAuthorized(), true
	case errors.Is(err, listenbrainz.ErrNotServer):
		return locales.ErrorListenBrainzNotServer(), true
	}
	if txt, ok := streamReasonText(stream.Reason(err)); ok {
		return txt, true
	}
	switch {
	case errors.Is(err, stream.ErrThrottled):
		return locales.ErrorStreamThrottled(), true
	case errors.Is(err, stream.ErrNoAudio):
		return locales.ErrorStreamNoAudio(), true
	case errors.Is(err, stream.ErrLive):
		return locales.ErrorStreamLive(), true
	case errors.Is(err, stream.ErrNoCaptions):
		return locales.ErrorStreamNoCaptions(), true
	case errors.Is(err, stream.ErrBadVideoID):
		return locales.ErrorStreamBadVideoID(), true
	case errors.Is(err, stream.ErrUnavailable):
		return locales.ErrorStreamUnavailable(), true
	case errors.Is(err, stream.ErrNetwork):
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
		return locales.ErrorStreamSignedIn(), true
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
	case stream.ReasonUnreachable:
		return locales.ErrorStreamUnreachable(), true
	}
	return "", false
}

// systemErrorText is err, of the system or of MixMusic's tools and files,
// as the windows show it, when they know it.
func systemErrorText(err error) (string, bool) {
	switch {
	case errors.Is(err, tools.ErrNoYTDLP):
		return locales.ErrorToolsNoYTDLP(), true
	case errors.Is(err, tools.ErrBrowserOpen):
		return locales.ErrorToolsBrowserOpen(), true
	case errors.Is(err, tools.ErrCookiesSealed):
		return locales.ErrorToolsCookiesSealed(), true
	case errors.Is(err, tools.ErrNoBrowserCookies):
		return locales.ErrorToolsNoBrowserCookies(), true
	case errors.Is(err, tools.ErrCookiesDenied):
		return locales.ErrorToolsCookiesDenied(), true
	case errors.Is(err, audio.ErrNoOutput):
		return locales.ErrorAudioNoOutput(), true
	case errors.Is(err, audio.ErrCantPlay):
		return locales.ErrorAudioCantPlay(), true
	case errors.Is(err, firewall.ErrUnsupported):
		return locales.ErrorFirewallUnsupported(), true
	case errors.Is(err, firewall.ErrDeclined):
		return locales.ErrorFirewallDeclined(), true
	case errors.Is(err, colorfile.ErrEmpty):
		return locales.ErrorColorFileEmpty(), true
	case errors.Is(err, colorfile.ErrPartial):
		return locales.ErrorColorFilePartial(), true
	case errors.Is(err, colorfile.ErrUnknown):
		return locales.ErrorColorFileUnknown(), true
	case errors.Is(err, colorfile.ErrFolder):
		return locales.ErrorColorFileFolder(), true
	case errors.Is(err, colorfile.ErrTooBig):
		return locales.ErrorColorFileTooBig(), true
	case errors.Is(err, localfiles.ErrNetworkDrive):
		return locales.ErrorLocalFilesNetworkDrive(), true
	case errors.Is(err, localfiles.ErrRemovableDrive):
		return locales.ErrorLocalFilesRemovableDrive(), true
	case errors.Is(err, localfiles.ErrWatchLimit):
		return locales.ErrorLocalFilesWatchLimit(), true
	case errors.Is(err, localfiles.ErrCantWatch):
		return locales.ErrorLocalFilesCantWatch(), true
	case errors.Is(err, tagwrite.ErrUnsupported):
		return locales.ErrorTagsUnsupported(), true
	case errors.Is(err, tagwrite.ErrFormat):
		return locales.ErrorTagsFormat(), true
	case errors.Is(err, tagwrite.ErrReadOnly):
		return locales.ErrorTagsReadOnly(), true
	case errors.Is(err, tagwrite.ErrFolderReadOnly):
		return locales.ErrorTagsFolderReadOnly(), true
	case errors.Is(err, tagwrite.ErrCover):
		return locales.ErrorTagsCover(), true
	case errors.Is(err, addons.ErrBuiltin):
		return locales.ErrorAddonsBuiltin(), true
	}
	return "", false
}

// colorFileFault is why the colour file of st couldn't be read, as the
// windows show it.
func colorFileFault(st app.ColorFileStatus) string {
	if st.Fault != nil {
		return errorText(st.Fault)
	}
	return st.Err
}

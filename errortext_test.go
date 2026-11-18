package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/addons"
	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/colorfile"
	"github.com/fzn0x/mixmusic/internal/firewall"
	"github.com/fzn0x/mixmusic/internal/lastfm"
	"github.com/fzn0x/mixmusic/internal/listenbrainz"
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/internal/tagwrite"
	"github.com/fzn0x/mixmusic/internal/tools"
	"github.com/fzn0x/mixmusic/internal/youtube"
	"github.com/fzn0x/mixmusic/locales"
)

// TestErrorText says each error the windows know in the language, wrapped
// as the services wrap them, and another in its own words.
func TestErrorText(t *testing.T) {
	usePseudoLocale(t)
	for _, err := range []error{
		spotify.ErrNoClientID, spotify.ErrSignInExpired, spotify.ErrNotSignedIn, spotify.ErrPlaylistUnavailable,
		spotify.ErrPlaylistNotShown, spotify.ErrPlaylistRefused, &spotify.APIError{Status: http.StatusForbidden, Reason: "PREMIUM_REQUIRED"},
		&spotify.APIError{Status: http.StatusTooManyRequests}, youtube.ErrSignedOut, youtube.ErrSessionRejected, youtube.ErrNotFound,
		youtube.ErrAlreadyInPlaylist, youtube.ErrNoSongList, lastfm.ErrNotAuthorized, listenbrainz.ErrNotServer,
		stream.ErrThrottled, stream.ErrNoAudio, stream.ErrLive, stream.ErrNoCaptions, stream.ErrBadVideoID, stream.ErrUnavailable,
		stream.ErrNetwork, tools.ErrNoYTDLP, tools.ErrBrowserOpen, tools.ErrCookiesSealed, tools.ErrNoBrowserCookies,
		tools.ErrCookiesDenied, audio.ErrNoOutput, audio.ErrCantPlay, firewall.ErrUnsupported, firewall.ErrDeclined,
		colorfile.ErrEmpty, colorfile.ErrPartial, colorfile.ErrUnknown, colorfile.ErrFolder, colorfile.ErrTooBig,
		localfiles.ErrNetworkDrive, localfiles.ErrRemovableDrive, localfiles.ErrWatchLimit, localfiles.ErrCantWatch,
		tagwrite.ErrUnsupported, tagwrite.ErrFormat, tagwrite.ErrReadOnly, tagwrite.ErrFolderReadOnly, tagwrite.ErrCover,
		addons.ErrBuiltin,
	} {
		if got := errorText(err); !strings.HasPrefix(got, "[") {
			t.Errorf("%v says %q, which no catalog translates", err, got)
		}
	}
	// The words around an error the windows know stay, as they are.
	wrapped := fmt.Errorf("MixMusic can't change memo.wav: %w", tagwrite.ErrUnsupported)
	if got, want := errorText(wrapped), "MixMusic can't change memo.wav: "+locales.ErrorTagsUnsupported(); got != want {
		t.Errorf("a wrapped error says %q, want %q", got, want)
	}
	if got := errorText(errors.New("spotify: HTTP 502")); got != "spotify: HTTP 502" {
		t.Errorf("an error of no message says %q, not its own words", got)
	}
	st := app.ColorFileStatus{Err: colorfile.ErrEmpty.Error(), Fault: colorfile.ErrEmpty}
	if got := colorFileFault(st); got != locales.ErrorColorFileEmpty() {
		t.Errorf("the colour file's fault: %q", got)
	}
}

// ytdlpMissingPlayer is the test player, whose tracks' streams need
// yt-dlp, which isn't installed.
type ytdlpMissingPlayer struct{ testPlayer }

func (ytdlpMissingPlayer) Prepare(context.Context, media.Track) (player.Source, error) {
	return player.Source{}, tools.ErrNoYTDLP
}

// TestPseudoLocalePlaybackError shows the toast of a track that couldn't
// play, for an error the windows know, in en-XA: its reason is the
// language's too.
func TestPseudoLocalePlaybackError(t *testing.T) {
	usePseudoLocale(t)
	s := newTestShellWith(t, nil, func(a *app.App) player.Services { return ytdlpMissingPlayer{testPlayer{a: a}} }, player.Saved{})
	s.main = true
	t.Cleanup(s.p.Notices(func(n player.Notice) { s.update(func() { s.notes = append(s.notes, n) }) }))
	tt := tester(t, s)
	tt.SetLocale("en-XA")
	s.p.PlayTracks([]media.Track{brokenTrack}, 0)
	want := locales.PlayerCouldntPlay(brokenTrack.Title, locales.ErrorToolsNoYTDLP())
	for deadline := time.Now().Add(5 * time.Second); !tt.HasText(want); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("no toast says %q: %q", want, tt.Texts())
		}
		s.drain()
		tt.Frame()
	}
	checkPseudo(t, "The toast of a track that couldn't play", tt.Texts(), brokenTrack.Title, "The Band")
}

package player

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/i18n"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// TestErrorWords says why a track couldn't play in the words set, as the
// windows do in their language, while its details keep the error's
// English.
func TestErrorWords(t *testing.T) {
	refused := errors.New("YouTube refused it")
	h := newHarness(t, Saved{}, func(f *fakeServices) {
		f.prepare = func(media.Track) (Source, error) { return Source{}, refused }
	})
	h.p.SetErrorWords(func(err error) string {
		if errors.Is(err, refused) {
			return "YouTube a refusé"
		}
		return err.Error()
	})
	h.p.PlayTracks(tracks("a"), 0)
	f := h.failedOnce(1)
	if !slices.Equal(f[0].Steps, []Step{{"finding its stream", "YouTube refused it", "YouTube a refusé", 1}}) {
		t.Errorf("steps %+v", f[0].Steps)
	}
	if n := h.notices(); len(n) != 1 || !strings.Contains(n[0].Message, "YouTube a refusé") {
		t.Errorf("notices %+v", n)
	}
}

// TestStreamErrorWords says why a stream the engine couldn't play failed,
// as the stream tells it, in the words set: in its notice, and in each
// step of its details, which keeps the error's English too.
func TestStreamErrorWords(t *testing.T) {
	h := newHarness(t, Saved{}, func(f *fakeServices) { f.failure = "YouTube refused it" })
	h.p.SetErrorWords(func(err error) string {
		if err.Error() == "YouTube refused it" {
			return "YouTube a refusé"
		}
		return err.Error()
	})
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(42*time.Second, 200*time.Second)
	h.stream.silence("a")
	useRetries(t, h, "a", 0)
	h.p.Error(errors.New("the stream failed"))
	h.wait("b playing", playing("b"))
	f := h.failedOnce(1)
	for _, st := range f[0].Steps {
		if st.Why != "YouTube refused it" || st.Said != "YouTube a refusé" {
			t.Errorf("steps %+v", f[0].Steps)
		}
	}
	if n := h.notices(); len(n) != 1 || n[0].Message != "Couldn't play Song a: YouTube a refusé" {
		t.Errorf("notices %+v", n)
	}
}

// TestSpotifyNotHereWords tells why a Spotify track plays from YouTube
// Music, where MixMusic can't play Spotify itself, in the app's language:
// in its notice, and in the step of its details, which keeps the English
// too. Under the pseudo locale, en-XA, the words are in brackets.
func TestSpotifyNotHereWords(t *testing.T) {
	was := i18n.Locale()
	i18n.SetLocaleFunc(func() string { return "en-XA" })
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return was }) })
	h := newHarness(t, Saved{}, func(f *fakeServices) {
		f.prepare = here
		f.streamErr = errors.New("not on YouTube Music")
	})
	// Spotify in MixMusic, which some systems' defaults aren't.
	h.p.SettingsChanged(settings.Settings{Playback: settings.Playback{SpotifyPlayback: settings.SpotifyPlaybackHere}})
	h.p.PlayTracks([]media.Track{spotifyTrack("1")}, 0)
	f := h.failedOnce(1)
	said := locales.PlayerSpotifyNotHere()
	if n := h.notices(); len(n) == 0 || n[0].Message != locales.PlayerFromYouTubeInstead(said) || !strings.HasPrefix(n[0].Message, "[") {
		t.Errorf("notices %+v", n)
	}
	if st := f[0].Steps; len(st) == 0 || st[0].What != "Spotify in MixMusic" || st[0].Why != errSpotifyNotHere.Error() ||
		st[0].Said != said || !strings.HasPrefix(said, "[") {
		t.Errorf("steps %+v", st)
	}
}

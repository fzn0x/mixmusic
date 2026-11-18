package player

import (
	"errors"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestSettingsErrors offers the playback settings in the notice of a track
// that couldn't play for an error they change, as one YouTube plays only
// to signed-in viewers (#577), beside its details; and not for another
// error.
func TestSettingsErrors(t *testing.T) {
	needs := errors.New("YouTube plays this track only to signed-in viewers")
	for _, c := range []struct {
		err      error
		settings bool
	}{
		{needs, true},
		{errors.New("this track is private"), false},
	} {
		h := newHarness(t, Saved{}, func(f *fakeServices) {
			f.prepare = func(media.Track) (Source, error) { return Source{}, c.err }
		})
		h.p.SetSettingsErrors(func(err error) bool { return errors.Is(err, needs) })
		h.p.PlayTracks(tracks("a"), 0)
		h.failedOnce(1)
		n := h.notices()
		if len(n) != 1 || n[0].Failure == 0 || !n[0].Error || n[0].Settings != c.settings {
			t.Errorf("%v: notices %+v, want one of a failure, offering the settings %v", c.err, n, c.settings)
		}
	}
}

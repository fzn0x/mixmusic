package player

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
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
	if !slices.Equal(f[0].Steps, []Step{{"finding its stream", "YouTube refused it", 1}}) {
		t.Errorf("steps %+v", f[0].Steps)
	}
	if n := h.notices(); len(n) != 1 || !strings.Contains(n[0].Message, "YouTube a refusé") {
		t.Errorf("notices %+v", n)
	}
}

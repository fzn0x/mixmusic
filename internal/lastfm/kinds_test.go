package lastfm

import (
	"errors"
	"fmt"
	"testing"

	"github.com/fzn0x/mixmusic/internal/scrobble"
)

// TestErrorKinds tells the shared scrobbler what becomes of the plays: a
// refused session keeps them for the next, a temporary error or the
// network keeps them for later, and any other answer drops them.
func TestErrorKinds(t *testing.T) {
	if !errors.Is(fmt.Errorf("sending: %w", ErrSessionExpired), scrobble.ErrKeyRefused) {
		t.Error("an expired session isn't a refused key")
	}
	for _, c := range []struct {
		err      error
		rejected bool
	}{
		{&APIError{Code: 6, Message: "Invalid parameters", Status: 400}, true},
		{&APIError{Status: 404}, true},
		{&APIError{Code: codeTemporary, Status: 200}, false},
		{&APIError{Code: codeRateLimited, Status: 429}, false},
		{&APIError{Status: 503}, false},
		{&netError{errors.New("connection reset")}, false},
		{ErrSessionExpired, false},
	} {
		if got := scrobble.IsRejected(c.err); got != c.rejected {
			t.Errorf("IsRejected(%v) = %v", c.err, got)
		}
	}
}

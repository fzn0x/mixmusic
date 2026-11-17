package stream

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

// TestReasons names the reason of each failure of plainErrors, so that a
// new one gets words in the windows' language too, and of the stream's
// own failures.
func TestReasons(t *testing.T) {
	for _, e := range plainErrors {
		if plainReasons[e.plain] == "" {
			t.Errorf("%q has no reason", e.plain)
		}
		if got := Reason(fmt.Errorf("resolving: %w", failure{e.plain, e.kind})); got != plainReasons[e.plain] {
			t.Errorf("Reason of %q = %q", e.plain, got)
		}
	}
	for err, want := range map[error]string{
		errExpired: ReasonRefused, statusError(http.StatusTooManyRequests): ReasonThrottled, errStalled: ReasonStalled,
		&url.Error{Op: "Get", URL: "https://example.com", Err: errors.New("refused")}: ReasonUnreachable,
		statusError(http.StatusInternalServerError):                                   "", failure{"yt-dlp: something new", nil}: "", errors.New("other"): "",
	} {
		if got := Reason(err); got != want {
			t.Errorf("Reason(%v) = %q, want %q", err, got, want)
		}
	}
}

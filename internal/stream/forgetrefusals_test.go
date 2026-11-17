package stream

import (
	"context"
	"sync/atomic"
	"testing"
)

// TestForgetRefusals asks as the account again at once for a video YouTube
// refused the stream of found so, once the app forgets the refusals, as it
// does when it signs in or uses another Google account or channel: also
// when the session the resolver is given stays the same, as after a
// switch away and back before the next video.
func TestForgetRefusals(t *testing.T) {
	var runs atomic.Int32
	signedOut := fakeYTDLP("https://example.com/audio", &runs)
	y := &accountYTDLP{
		asAccount: func(context.Context) ([]byte, error) { return premiumAnswer(t), nil },
		signedOut: func(ctx context.Context) ([]byte, error) { return signedOut(ctx, "--") },
	}
	r := accountResolver(t, y)
	resolve := func(step string, asAccount bool) {
		t.Helper()
		if s, err := r.Resolve(context.Background(), songID); err != nil || s.AsAccount != asAccount {
			t.Errorf("%s: %+v, %v; want as the account %v", step, s, err, asAccount)
		}
		r.Forget(songID) // as after YouTube refused its URL
	}
	resolve("first", true)
	resolve("refused, the same session", false)
	r.ForgetRefusals()
	r.mu.Lock()
	marks := len(r.refusedAsAccount)
	r.mu.Unlock()
	if marks != 0 {
		t.Errorf("%d marks stay", marks)
	}
	resolve("the refusals forgotten", true)
	if n := asAccountRuns(y); n != 2 {
		t.Errorf("yt-dlp ran %d times as the account, want 2", n)
	}
	// The refusal of the stream found as the account just now marks the
	// video again.
	resolve("refused again", false)
}

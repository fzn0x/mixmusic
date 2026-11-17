package stream

import (
	"context"
	"slices"
	"strings"
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

// TestForgetRefusalsDropsStreamsPastAMark drops, as the refusals go, the
// stream the resolver keeps of a video it found signed out only because
// YouTube refused the stream of it found as the account, so the next play
// asks as the account at once. It keeps every other stream: one found as
// the account, and one found signed out after the account found nothing
// that plays. The end of a mark drops none.
func TestForgetRefusalsDropsStreamsPastAMark(t *testing.T) {
	const noAudio = "ccccccccccc" // the account finds nothing that plays
	var runs atomic.Int32
	signedOut := fakeYTDLP("https://example.com/audio", &runs)
	y := &accountYTDLP{
		asAccount: func(context.Context) ([]byte, error) { return premiumAnswer(t), nil },
		signedOut: func(ctx context.Context) ([]byte, error) { return signedOut(ctx, "--") },
	}
	r := accountResolver(t, y)
	r.Run = func(ctx context.Context, args ...string) ([]byte, error) {
		if slices.Contains(args, "--cookies") && strings.HasSuffix(args[len(args)-1], noAudio) {
			y.run(ctx, args...) // counted
			return []byte(`{"formats": []}`), nil
		}
		return y.run(ctx, args...)
	}
	resolve := func(step, videoID string, asAccount bool) {
		t.Helper()
		if s, err := r.Resolve(context.Background(), videoID); err != nil || s.AsAccount != asAccount {
			t.Errorf("%s: %+v, %v; want as the account %v", step, s, err, asAccount)
		}
	}
	resolve("first", songID, true)
	r.Forget(songID) // as after YouTube refused its URL
	resolve("past the mark", songID, false)
	resolve("another, as the account", testID, true)
	resolve("no audio as the account", noAudio, false)
	backdate(t, r, songID, refusalMark)
	resolve("the mark ended", songID, false)
	before := len(y.runs())

	r.ForgetRefusals()
	resolve("the refusals forgotten", songID, true)
	resolve("another, as the account, kept", testID, true)
	resolve("no audio as the account, kept", noAudio, false)
	if runs := y.runs()[before:]; len(runs) != 1 || !slices.Contains(runs[0], "--cookies") {
		t.Errorf("yt-dlp ran %q after the refusals went; want one run, as the account", runs)
	}
}

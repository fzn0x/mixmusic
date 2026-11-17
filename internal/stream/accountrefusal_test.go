package stream

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

// asAccountRuns counts the runs of y as the account.
func asAccountRuns(y *accountYTDLP) int {
	n := 0
	for _, args := range y.runs() {
		if slices.Contains(args, "--cookies") {
			n++
		}
	}
	return n
}

// backdate makes the mark of a video's refusal as the account as old as
// age.
func backdate(t *testing.T, r *Resolver, videoID string, age time.Duration) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.refusedAsAccount[videoID]; !ok {
		t.Fatalf("%s has no mark", videoID)
	}
	r.refusedAsAccount[videoID] = time.Now().Add(-age)
}

// TestAccountRefusalLastsTenMinutes finds a video signed out for 10
// minutes once YouTube refused its stream found as the account, and as
// the account again after that.
func TestAccountRefusalLastsTenMinutes(t *testing.T) {
	var runs atomic.Int32
	signedOut := fakeYTDLP("https://example.com/audio", &runs)
	y := &accountYTDLP{
		asAccount: func(context.Context) ([]byte, error) { return premiumAnswer(t), nil },
		signedOut: func(ctx context.Context) ([]byte, error) { return signedOut(ctx, "--") },
	}
	r := accountResolver(t, y)
	if s, err := r.Resolve(context.Background(), songID); err != nil || !s.AsAccount {
		t.Fatalf("first: %+v, %v", s, err)
	}
	r.Forget(songID) // as after YouTube refused its URL
	backdate(t, r, songID, refusalMark-time.Minute)
	if s, err := r.Resolve(context.Background(), songID); err != nil || s.AsAccount {
		t.Errorf("9 minutes after the refusal: %+v, %v", s, err)
	}
	r.Forget(songID) // its stream found signed out, which marks nothing
	backdate(t, r, songID, refusalMark)
	if s, err := r.Resolve(context.Background(), songID); err != nil || !s.AsAccount {
		t.Errorf("10 minutes after the refusal: %+v, %v", s, err)
	}
	if n := asAccountRuns(y); n != 2 {
		t.Errorf("yt-dlp ran %d times as the account, want 2", n)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.refusedAsAccount[songID]; ok {
		t.Error("the mark that ended stays")
	}
}

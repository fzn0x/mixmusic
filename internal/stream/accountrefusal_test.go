package stream

import (
	"context"
	"net/http"
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

// signInRequired is what yt-dlp prints, signed out, of a video YouTube
// plays only to signed-in viewers, as #511 quotes it, turned into the
// error runYTDLP returns.
func signInRequired() error {
	return ytdlpFailure("ERROR: [youtube] " + testID + ": Please sign in. This helps protect our community. " +
		"Use --cookies-from-browser or --cookies for the authentication. See https://github.com/yt-dlp/yt-dlp/wiki/FAQ\n")
}

// TestSetAsAccountClearsRefusals asks as the account again for a video
// YouTube refused the stream of found so, once the switch was turned off
// and on: a track it plays only to signed-in viewers plays again (#578).
func TestSetAsAccountClearsRefusals(t *testing.T) {
	y := &accountYTDLP{
		asAccount: func(context.Context) ([]byte, error) { return premiumAnswer(t), nil },
		signedOut: func(context.Context) ([]byte, error) { return nil, signInRequired() },
	}
	r := accountResolver(t, y)
	if s, err := r.Resolve(context.Background(), testID); err != nil || !s.AsAccount {
		t.Fatalf("first: %+v, %v", s, err)
	}
	r.Forget(testID) // as after YouTube refused its URL
	r.SetAsAccount(false)
	r.SetAsAccount(true)
	s, err := r.Resolve(context.Background(), testID)
	if err != nil || !s.AsAccount {
		t.Errorf("after a refusal and a toggle: %+v, %v", s, err)
	}
	if n, all := asAccountRuns(y), len(y.runs()); n != 2 || all != 2 {
		t.Errorf("yt-dlp ran %d times, %d of them as the account; want 2, both as the account", all, n)
	}
}

// TestSessionChangeClearsRefusals asks as the account again for a video
// YouTube refused the stream of found so, once the session changed:
// signed in again, or signed out and in, as YouTube ending the session or
// another Google account of it are to the resolver too. A cookie YouTube
// refreshes as it goes keeps the mark.
func TestSessionChangeClearsRefusals(t *testing.T) {
	var runs atomic.Int32
	signedOut := fakeYTDLP("https://example.com/audio", &runs)
	y := &accountYTDLP{
		asAccount: func(context.Context) ([]byte, error) { return premiumAnswer(t), nil },
		signedOut: func(ctx context.Context) ([]byte, error) { return signedOut(ctx, "--") },
	}
	r := accountResolver(t, y)
	var session atomic.Pointer[[]*http.Cookie]
	use := func(cookies []*http.Cookie) { session.Store(&cookies) }
	r.Session = func() []*http.Cookie { return *session.Load() }
	resolve := func(step string, asAccount bool) {
		t.Helper()
		if s, err := r.Resolve(context.Background(), songID); err != nil || s.AsAccount != asAccount {
			t.Errorf("%s: %+v, %v; want as the account %v", step, s, err, asAccount)
		}
		r.Forget(songID) // as after YouTube refused its URL
	}
	use(testSession())
	resolve("first", true)
	use(append(testSession(), &http.Cookie{Name: "__Secure-1PSIDTS", Value: "refreshed"}))
	resolve("a cookie refreshed", false)
	again := testSession()
	again[0] = &http.Cookie{Name: "LOGIN_INFO", Value: "madeup-login-again"}
	use(again)
	resolve("signed in again", true)
	use(nil)
	resolve("signed out", false)
	use(again)
	resolve("signed in after", true)
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

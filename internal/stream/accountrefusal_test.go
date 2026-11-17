package stream

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
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

// TestRefusedSignedInOnlyAsksAgain plays again a track YouTube plays only
// to signed-in viewers once YouTube refused its stream found as the
// account, as after a move to another network: signed out, it meets
// YouTube's sign-in wall, and the account is asked once more, for a
// stream that plays, with no restart (#578).
func TestRefusedSignedInOnlyAsksAgain(t *testing.T) {
	var minGen atomic.Int32
	srv, data := upstream(t, 1000, &minGen)
	var gen atomic.Int32
	y := &accountYTDLP{
		asAccount: func(context.Context) ([]byte, error) {
			g := gen.Add(1)
			return fmt.Appendf(nil, `{"url": "%s/a?gen=%d&itag=774&expire=%d", "ext": "webm", "acodec": "opus", "filesize": 1000}`,
				srv.URL, g, time.Now().Add(6*time.Hour).Unix()), nil
		},
		signedOut: func(context.Context) ([]byte, error) { return nil, signInRequired() },
	}
	r := accountResolver(t, y)
	p := NewProxy(r)
	if rec := get(t, p, ""); rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), data) {
		t.Fatalf("1st play, as the account: HTTP %d, %d bytes", rec.Code, rec.Body.Len())
	}
	minGen.Store(2) // YouTube refuses the URL found as the account (403)
	if rec := get(t, p, "bytes=0-99"); rec.Code != 206 || !bytes.Equal(rec.Body.Bytes(), data[:100]) {
		t.Fatalf("2nd play, after a 403 of that URL: HTTP %d, failure shown: %q", rec.Code, p.Failure(testID))
	}
	if s, ok := r.Known(testID); !ok || !s.AsAccount || s.Route != RouteAccount || s.Itag() != 774 {
		t.Errorf("after the refusal, the resolver knows %+v", s)
	}
	if rec := get(t, p, ""); rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), data) {
		t.Errorf("3rd play: HTTP %d, failure shown: %q", rec.Code, p.Failure(testID))
	}
	// The 2nd play: the quick and the full run signed out, then the
	// account once more.
	if n, all := asAccountRuns(y), len(y.runs()); n != 2 || all != 4 {
		t.Errorf("yt-dlp ran %d times, %d of them as the account; want 4, 2 as the account", all, n)
	}
}

// TestRefusedAsksAgainOnlyAtTheSignInWall asks as the account once more
// only for a video whose stream found so YouTube refused, when the
// signed-out ways meet YouTube's sign-in wall: "Please sign in", its check
// of age, or a channel's members only. Never with data saver, and never
// for another failure, which stays the signed-out ways'.
func TestRefusedAsksAgainOnlyAtTheSignInWall(t *testing.T) {
	says := func(msg string) func(context.Context) ([]byte, error) {
		return func(context.Context) ([]byte, error) {
			return nil, ytdlpFailure("ERROR: [youtube] " + songID + ": " + msg + "\n")
		}
	}
	for _, c := range []struct {
		name      string
		signedOut func(context.Context) ([]byte, error)
		saver     bool
		again     bool
	}{
		{"please sign in", says("Please sign in. This helps protect our community."), false, true},
		{"age", says("Sign in to confirm your age. This video may be inappropriate for some users."), false, true},
		{"members only", says("Join this channel to get access to members-only content like this video, and other exclusive perks."), false, true},
		{"bot check", says("Sign in to confirm you’re not a bot. Use --cookies-from-browser or --cookies for the authentication."), false, false},
		{"country", says("The uploader has not made this video available in your country"), false, false},
		{"a crash", func(context.Context) ([]byte, error) { return nil, errors.New("yt-dlp: exit status 1") }, false, false},
		{"data saver", says("Please sign in. This helps protect our community."), true, false},
	} {
		y := &accountYTDLP{
			asAccount: func(context.Context) ([]byte, error) { return premiumAnswer(t), nil },
			signedOut: c.signedOut,
		}
		r := accountResolver(t, y)
		if s, err := r.Resolve(context.Background(), songID); err != nil || !s.AsAccount {
			t.Fatalf("%s, first: %+v, %v", c.name, s, err)
		}
		r.Forget(songID) // as after YouTube refused its URL
		if c.saver {
			r.SetQuality(QualitySaver)
		}
		var signedOut error
		if _, err := c.signedOut(context.Background()); !c.again {
			signedOut = err
		}
		s, err := r.Resolve(context.Background(), songID)
		switch {
		case c.again && (err != nil || !s.AsAccount):
			t.Errorf("%s: not asked again: %+v, %v", c.name, s, err)
		case !c.again && (err == nil || err.Error() != signedOut.Error()):
			t.Errorf("%s: %v, want the signed-out failure %v", c.name, err, signedOut)
		}
		want := map[bool]int{true: 2, false: 1}[c.again]
		if n := asAccountRuns(y); n != want {
			t.Errorf("%s: yt-dlp ran %d times as the account, want %d", c.name, n, want)
		}
	}
}

// TestRefusedAsksAgainAfterThePlayerAPI asks as the account once more when
// YouTube's player API, then yt-dlp's full run, both signed out, met the
// sign-in wall.
func TestRefusedAsksAgainAfterThePlayerAPI(t *testing.T) {
	y := &accountYTDLP{
		asAccount: func(context.Context) ([]byte, error) { return premiumAnswer(t), nil },
		signedOut: func(context.Context) ([]byte, error) { return nil, signInRequired() },
	}
	r := accountResolver(t, y)
	f := newFakeTube(t)
	f.answersWith("-sign-in")
	r.Direct = f.direct()
	if s, err := r.Resolve(context.Background(), songID); err != nil || !s.AsAccount {
		t.Fatalf("first: %+v, %v", s, err)
	}
	r.Forget(songID) // as after YouTube refused its URL
	if s, err := r.Resolve(context.Background(), songID); err != nil || !s.AsAccount {
		t.Errorf("after the refusal: %+v, %v", s, err)
	}
	if len(f.players()) != 1 {
		t.Errorf("the player API was asked %d times, want 1", len(f.players()))
	}
	// The full run alone: the player API told the visionOS client that
	// the video plays to signed-in viewers alone.
	if n, all := asAccountRuns(y), len(y.runs()); n != 2 || all != 3 {
		t.Errorf("yt-dlp ran %d times, %d of them as the account; want 3, 2 as the account", all, n)
	}
}

// TestRefusedAskedAgainFails tells why the signed-out ways found no stream
// when the run as the account once more finds none either, and asks once
// a resolve: the next resolve asks once more again.
func TestRefusedAskedAgainFails(t *testing.T) {
	logs := captureLog(t)
	var asked atomic.Int32
	y := &accountYTDLP{
		asAccount: func(context.Context) ([]byte, error) {
			if asked.Add(1) == 1 {
				return premiumAnswer(t), nil
			}
			return nil, errors.New("yt-dlp: exit status 1")
		},
		signedOut: func(context.Context) ([]byte, error) { return nil, signInRequired() },
	}
	r := accountResolver(t, y)
	if s, err := r.Resolve(context.Background(), testID); err != nil || !s.AsAccount {
		t.Fatalf("first: %+v, %v", s, err)
	}
	r.Forget(testID) // as after YouTube refused its URL
	for i := range 2 {
		_, err := r.Resolve(context.Background(), testID)
		if want := signInRequired(); err == nil || err.Error() != want.Error() {
			t.Errorf("resolve %d after the refusal: %v, want %v", i+1, err, want)
		}
		if n := asAccountRuns(y); n != 2+i {
			t.Errorf("resolve %d after the refusal: yt-dlp ran %d times as the account, want %d", i+1, n, 2+i)
		}
	}
	if !strings.Contains(logs.String(), "yt-dlp found no stream of "+testID+" as your YouTube Music account once more in ") {
		t.Errorf("the log doesn't tell:\n%s", logs.String())
	}
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

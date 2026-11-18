package stream

import (
	"context"
	"errors"
	"iter"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"
)

// signedInOnly is a resolver as the account of testSession, whose yt-dlp
// meets YouTube's "Please sign in" signed out, and as the account answers
// as asAccount does; set changes it before it resolves.
func signedInOnly(t *testing.T, asAccount func(context.Context) ([]byte, error), set func(r *Resolver)) (*Resolver, *accountYTDLP) {
	t.Helper()
	y := &accountYTDLP{
		asAccount: asAccount,
		signedOut: func(context.Context) ([]byte, error) { return nil, signInRequired() },
	}
	r := accountResolver(t, y)
	if set != nil {
		set(r)
	}
	return r, y
}

// stepsOf are the steps err tells, in turn.
func stepsOf(t *testing.T, err error) [][2]string {
	t.Helper()
	var st interface {
		Steps() iter.Seq2[string, string]
	}
	if !errors.As(err, &st) {
		t.Fatalf("%T tells no steps", err)
	}
	var steps [][2]string
	for what, why := range st.Steps() {
		steps = append(steps, [2]string{what, why})
	}
	return steps
}

// TestSignedInOnlyWords tells, of a track YouTube plays only to signed-in
// viewers, what to change for the account to be asked, or that it was
// asked too, and never to turn on Play audio as your account while it's
// on (#577): the switch off, signed in or out; on, but not asked for data
// saver, a missing JavaScript runtime, no session, one YouTube ended, or
// another Google account than the session's first; and on, asked, as
// YouTube refused it the account too, or the run as it failed. Its kind
// tells when the settings change it.
func TestSignedInOnlyWords(t *testing.T) {
	const why = "YouTube plays this track only to signed-in viewers"
	premium := func(context.Context) ([]byte, error) { return premiumAnswer(t), nil }
	// As the app gives no session, and tells why.
	noSession := func(why string) func(r *Resolver) {
		return func(r *Resolver) {
			r.Session = func() []*http.Cookie { return nil }
			r.NoSession = func() string { return why }
		}
	}
	for _, c := range []struct {
		name      string
		asAccount func(context.Context) ([]byte, error)
		set       func(r *Resolver)
		way       string
		words     string
		asked     int
	}{
		{"off, signed in", premium, func(r *Resolver) { r.SetAsAccount(false) }, AccountOff, signedInOff, 0},
		{"off, another Google account", premium, func(r *Resolver) {
			r.SetAsAccount(false)
			noSession(AccountNotFirst)(r)
		}, AccountOff, signedInOff, 0},
		{"off, signed out", premium, func(r *Resolver) {
			r.SetAsAccount(false)
			r.Session = func() []*http.Cookie { return nil }
		}, AccountSignIn, why + "; sign in to YouTube Music in Settings › Accounts, then turn on Play audio as your account in Settings › Playback", 0},
		{"off, signed out, as the app tells", premium, func(r *Resolver) {
			r.SetAsAccount(false)
			noSession(AccountSignedOut)(r)
		}, AccountSignIn, why + "; sign in to YouTube Music in Settings › Accounts, then turn on Play audio as your account in Settings › Playback", 0},
		{"on, signed out", premium, noSession(AccountSignedOut), AccountSignedOut,
			why + "; sign in to YouTube Music in Settings › Accounts for your account to be asked", 0},
		{"on, no Session", premium, func(r *Resolver) { r.Session = nil }, AccountSignedOut,
			why + "; sign in to YouTube Music in Settings › Accounts for your account to be asked", 0},
		{"on, the session ended", premium, noSession(AccountEnded), AccountEnded,
			why + "; sign in to YouTube Music again, in Settings › Accounts, for your account to be asked", 0},
		{"on, no LOGIN_INFO", premium, func(r *Resolver) {
			r.Session = func() []*http.Cookie { return []*http.Cookie{{Name: "SAPISID", Value: "madeup"}} }
		}, AccountEnded, why + "; sign in to YouTube Music again, in Settings › Accounts, for your account to be asked", 0},
		{"on, another Google account", premium, noSession(AccountNotFirst), AccountNotFirst,
			why + "; your account is asked only while MixMusic uses the first Google account of your session, in Settings › Accounts", 0},
		{"on, data saver", premium, func(r *Resolver) { r.SetQuality(QualitySaver) }, AccountSaver,
			why + "; your account is asked only at the best Quality, in Settings › Playback", 0},
		{"on, no runtime", premium, func(r *Resolver) { r.Tools.Configure(yes(t), "none-such") }, AccountNoTools,
			why + "; asking as your account takes yt-dlp and a JavaScript runtime, in Settings › Playback › Tools", 0},
		{"on, refused the account too", func(context.Context) ([]byte, error) { return nil, signInRequired() }, nil,
			AccountRefused, why + "; YouTube didn't give it to your account either", 1},
		{"on, the run as the account failed", func(context.Context) ([]byte, error) { return nil, errors.New("yt-dlp: exit status 1") }, nil,
			AccountRefused, why + "; YouTube didn't give it to your account either", 1},
	} {
		r, y := signedInOnly(t, c.asAccount, c.set)
		_, err := r.Resolve(context.Background(), testID)
		if err == nil {
			t.Fatalf("%s: it played", c.name)
		}
		if err.Error() != c.words || Account(err) != c.way || Reason(err) != ReasonSignedIn {
			t.Errorf("%s: %q, as the account %q, reason %q; want %q, %q", c.name, err, Account(err), Reason(err), c.words, c.way)
		}
		settings := c.way != AccountRefused
		if errors.Is(err, ErrNeedsAccount) != settings || !errors.Is(err, ErrUnavailable) {
			t.Errorf("%s: of the settings %v, want %v; unavailable %v", c.name, errors.Is(err, ErrNeedsAccount), settings, errors.Is(err, ErrUnavailable))
		}
		r.mu.Lock()
		on := r.asAccount
		r.mu.Unlock()
		if on && strings.Contains(err.Error(), "Play audio as your account") {
			t.Errorf("%s: the words name the switch, which is on: %q", c.name, err)
		}
		if n := asAccountRuns(y); n != c.asked {
			t.Errorf("%s: yt-dlp ran %d times as the account, want %d", c.name, n, c.asked)
		}
	}
}

// TestOtherWallsWords tells, of a track YouTube plays only to signed-in
// adults, a channel's members or YouTube Music Premium's, whether the
// account was asked: not while the switch is off, and too when it's on.
func TestOtherWallsWords(t *testing.T) {
	for _, c := range []struct {
		msg, why string
	}{
		{"Sign in to confirm your age. This video may be inappropriate for some users.", "YouTube plays this track only to signed-in adults"},
		{"Join this channel to get access to members-only content like this video, and other exclusive perks.", "only the members of its channel can play this track"},
		{"This video is only available to Music Premium members", "only YouTube Music Premium members can play this track"},
	} {
		wall := func(context.Context) ([]byte, error) {
			return nil, ytdlpFailure("ERROR: [youtube] " + testID + ": " + c.msg + "\n")
		}
		for on, want := range map[bool]string{
			false: c.why + "; your account wasn't asked, as Play audio as your account is off, in Settings › Playback",
			true:  c.why + "; YouTube didn't give it to your account either",
		} {
			y := &accountYTDLP{asAccount: wall, signedOut: wall}
			r := accountResolver(t, y)
			r.SetAsAccount(on)
			_, err := r.Resolve(context.Background(), testID)
			if err == nil || err.Error() != want || errors.Is(err, ErrNeedsAccount) == on {
				t.Errorf("%q, the switch on %v: %v, of the settings %v; want %q", c.msg, on, err, errors.Is(err, ErrNeedsAccount), want)
			}
		}
	}
}

// TestAccountStepsInTheDetails tells, in the details of a track that
// couldn't play, how asking as the account went, first: asked, and why it
// failed, or not asked, and why; then the signed-out ways, then the run
// once more as the account, after a refusal. With the switch off, only a
// sign-in wall's details tell it.
func TestAccountStepsInTheDetails(t *testing.T) {
	const why = "YouTube plays this track only to signed-in viewers"
	crash := func(context.Context) ([]byte, error) { return nil, errors.New("yt-dlp: exit status 1") }
	premium := func(context.Context) ([]byte, error) { return premiumAnswer(t), nil }

	r, _ := signedInOnly(t, premium, func(r *Resolver) { r.SetQuality(QualitySaver) })
	_, err := r.Resolve(context.Background(), testID)
	if got, want := stepsOf(t, err), [][2]string{{"as your account", "not asked: Quality is data saver"}, {"yt-dlp", why}}; !slices.Equal(got, want) {
		t.Errorf("data saver: steps %q, want %q", got, want)
	}

	r, _ = signedInOnly(t, crash, nil)
	f := newFakeTube(t)
	f.answersWith("-sign-in")
	r.Direct = f.direct()
	_, err = r.Resolve(context.Background(), testID)
	steps := stepsOf(t, err)
	if len(steps) != 3 || steps[0] != [2]string{"as your account", "yt-dlp: exit status 1"} ||
		steps[1][0] != "YouTube's player API" || steps[2] != [2]string{"yt-dlp", why} {
		t.Errorf("the run as the account failed: steps %q", steps)
	}

	// After YouTube refused the stream found as the account.
	var asked int
	r, _ = signedInOnly(t, func(ctx context.Context) ([]byte, error) {
		if asked++; asked == 1 {
			return premiumAnswer(t), nil
		}
		return crash(ctx)
	}, nil)
	if _, err := r.Resolve(context.Background(), testID); err != nil {
		t.Fatal(err)
	}
	r.Forget(testID)
	_, err = r.Resolve(context.Background(), testID)
	want := [][2]string{
		{"as your account", "not asked: YouTube refused the audio found as your account less than 10 minutes ago"},
		{"yt-dlp", why},
		{"as your account, tried again", "yt-dlp: exit status 1"},
	}
	if got := stepsOf(t, err); !slices.Equal(got, want) || Account(err) != AccountRefused {
		t.Errorf("after a refusal: steps %q, as the account %q; want %q", got, Account(err), want)
	}

	// Too long a run as the account says so.
	old := accountTimeout
	accountTimeout = 50 * time.Millisecond
	t.Cleanup(func() { accountTimeout = old })
	r, _ = signedInOnly(t, func(ctx context.Context) ([]byte, error) { <-ctx.Done(); return nil, ctx.Err() }, nil)
	_, err = r.Resolve(context.Background(), testID)
	if got := stepsOf(t, err); len(got) == 0 || got[0] != [2]string{"as your account", "yt-dlp gave no answer in 50ms"} {
		t.Errorf("too long: steps %q", got)
	}

	// Another failure keeps its words, and tells the account's step while
	// the switch is on; with it off, neither.
	country := func(context.Context) ([]byte, error) {
		return nil, ytdlpFailure("ERROR: [youtube] " + testID + ": The uploader has not made this video available in your country\n")
	}
	for _, on := range []bool{true, false} {
		y := &accountYTDLP{asAccount: crash, signedOut: country}
		r := accountResolver(t, y)
		r.SetAsAccount(on)
		_, err := r.Resolve(context.Background(), testID)
		if err == nil || err.Error() != "this track is not available in your country" || Account(err) != "" || errors.Is(err, ErrNeedsAccount) {
			t.Errorf("not in the country, the switch on %v: %v", on, err)
			continue
		}
		var st interface {
			Steps() iter.Seq2[string, string]
		}
		if told := errors.As(err, &st); told != on {
			t.Errorf("not in the country, the switch on %v: tells steps %v", on, told)
		} else if on {
			want := [][2]string{{"as your account", "yt-dlp: exit status 1"}, {"yt-dlp", "this track is not available in your country"}}
			if got := stepsOf(t, err); !slices.Equal(got, want) {
				t.Errorf("not in the country: steps %q, want %q", got, want)
			}
		}
	}
}

package stream

import (
	"errors"
	"fmt"
	"iter"
	"strings"
)

// The words of a track YouTube plays only to some signed-in viewers (#577):
// signed-in viewers, signed-in adults, a channel's members or YouTube
// Music Premium's, which the user's account may pass (signInWall). They
// tell why it didn't play, and what to change for the account to be
// asked, or that it was, as asking as the account went, and never name a
// switch that is on.

// The ways asking as the user's account went for such a track, as Account
// names them, for the windows to tell in their language.
const (
	// AccountOff: Play audio as your account is off, while MixMusic is
	// signed in to YouTube Music.
	AccountOff = "off"
	// AccountSignIn: Play audio as your account is off, and MixMusic isn't
	// signed in to YouTube Music.
	AccountSignIn = "signIn"
	// AccountSignedOut: the switch is on, and MixMusic isn't signed in to
	// YouTube Music.
	AccountSignedOut = "signedOut"
	// AccountEnded: the switch is on, and YouTube ended the session, or it
	// lacks the cookies yt-dlp asks as: signing in again helps.
	AccountEnded = "ended"
	// AccountNotFirst: the switch is on, and MixMusic uses another Google
	// account of the session than its first, the one yt-dlp asks as.
	AccountNotFirst = "notFirst"
	// AccountSaver: the switch is on, and Quality is data saver, which
	// never asks as the account.
	AccountSaver = "saver"
	// AccountNoTools: the switch is on, without yt-dlp or a JavaScript
	// runtime, which asking as the account takes.
	AccountNoTools = "noTools"
	// AccountRefused: the account was asked, and YouTube gave it no stream
	// that plays, or refused the one it gave a while ago.
	AccountRefused = "refused"
)

// Why a try as the account didn't ask, besides the Account constants.
const (
	// accountMarked: YouTube refused the stream of the video found as the
	// account less than refusalMark ago.
	accountMarked = "marked"
	// accountOnce: no run once more, after the signed-out ways: the video
	// has no refusal mark, so the account was asked first, or those met
	// no sign-in wall.
	accountOnce = "once"
)

// ErrNeedsAccount is the kind of the failure of a track YouTube plays only
// to some signed-in viewers, whose words tell what to change in the
// settings for the user's account to be asked: signing in, Play audio as
// your account, the best Quality, a JavaScript runtime or the session's
// first Google account. It is an ErrUnavailable too.
var ErrNeedsAccount = fmt.Errorf("stream: YouTube sends this track only to some signed-in viewers: %w", ErrUnavailable)

// accountTry is how a resolve asked for the stream of a video as the
// account: why it didn't (skip), or why the stream found so wasn't taken
// (err).
type accountTry struct {
	// skip is one of the Account constants, accountMarked or accountOnce,
	// or "" when it asked.
	skip string
	err  error
}

// took reports whether the try asked as the account, and took the stream
// it found.
func (t accountTry) took() bool { return t.skip == "" && t.err == nil }

// notAsked are why a try didn't ask as the account, for the details.
var notAsked = map[string]string{
	AccountOff:       "Play audio as your account is off",
	AccountSignedOut: "MixMusic isn't signed in to YouTube Music",
	AccountEnded:     "YouTube ended the session, or it lacks its LOGIN_INFO or SAPISID cookie",
	AccountNotFirst:  "MixMusic uses another Google account of the session than its first, which yt-dlp asks as",
	AccountSaver:     "Quality is data saver",
	AccountNoTools:   "yt-dlp or a JavaScript runtime is missing",
	accountMarked:    fmt.Sprintf("YouTube refused the audio found as your account less than %d minutes ago", int(refusalMark.Minutes())),
}

// said tells how a try that took no stream went, for the details: why the
// stream found as the account wasn't taken, or why it didn't ask.
func (t accountTry) said() string {
	if t.skip == "" {
		return strings.TrimPrefix(t.err.Error(), "stream: ")
	}
	return "not asked: " + notAsked[t.skip]
}

// signedInOff are the words of a track YouTube plays only to signed-in
// viewers while Play audio as your account is off, signed in: it plays
// those.
const signedInOff = "YouTube plays this track only to signed-in viewers; Play audio as your account, in Settings › Playback, can play it"

// accountWords follow why a track YouTube plays only to some signed-in
// viewers didn't play, by the way asking as the account went: what to
// change, or that the account was asked too.
var accountWords = map[string]string{
	AccountOff:       "your account wasn't asked, as Play audio as your account is off, in Settings › Playback",
	AccountSignIn:    "sign in to YouTube Music in Settings › Accounts, then turn on Play audio as your account in Settings › Playback",
	AccountSignedOut: "sign in to YouTube Music in Settings › Accounts for your account to be asked",
	AccountEnded:     "sign in to YouTube Music again, in Settings › Accounts, for your account to be asked",
	AccountNotFirst:  "your account is asked only while MixMusic uses the first Google account of your session, in Settings › Accounts",
	AccountSaver:     "your account is asked only at the best Quality, in Settings › Playback",
	AccountNoTools:   "asking as your account takes yt-dlp and a JavaScript runtime, in Settings › Playback › Tools",
	AccountRefused:   "YouTube didn't give it to your account either",
}

// accountFailure is why a resolve found no stream, err, of the signed-out
// ways, with how it asked as the account: first, and once more after
// those (againAsAccount). Its steps tell both, for the details of the
// track; its words, of a sign-in wall, what to change, or that the
// account was asked too.
type accountFailure struct {
	err          error
	first, again accountTry
	// way is how asking as the account went, one of the Account constants,
	// when err is of a sign-in wall; "" otherwise.
	way string
	// route names the signed-out way whose error err is, when it tells no
	// steps of its own.
	route string
}

func (e *accountFailure) Error() string {
	switch {
	case e.way == "":
		return e.err.Error()
	case e.way == AccountOff && Reason(e.err) == ReasonSignedIn:
		return signedInOff
	}
	return e.err.Error() + "; " + accountWords[e.way]
}

// Unwrap gives err, and ErrNeedsAccount when the words tell what to
// change.
func (e *accountFailure) Unwrap() []error {
	if e.way != "" && e.way != AccountRefused {
		return []error{e.err, ErrNeedsAccount}
	}
	return []error{e.err}
}

// Steps tells how asking as the account went first, then the signed-out
// ways, then the run once more as the account, when there was one.
func (e *accountFailure) Steps() iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		if !yield("as your account", e.first.said()) {
			return
		}
		var st interface {
			Steps() iter.Seq2[string, string]
		}
		if errors.As(e.err, &st) {
			for what, why := range st.Steps() {
				if !yield(what, why) {
					return
				}
			}
		} else if !yield(e.route, strings.TrimPrefix(strings.TrimPrefix(e.err.Error(), "stream: "), "yt-dlp: ")) {
			return
		}
		if e.again.skip == "" {
			yield("as your account, tried again", e.again.said())
		}
	}
}

// Account tells how asking as the user's account went, one of the Account
// constants, when err, of finding a stream, is of a track YouTube plays
// only to some signed-in viewers, whose reason Reason gives: "" for any
// other error.
func Account(err error) string {
	var f *accountFailure
	if errors.As(err, &f) {
		return f.way
	}
	return ""
}

// asAccountFailure is err, why the signed-out ways found no stream of a
// video, told with how asking as the account went: first, and once more
// (again). While the switch is off, only a sign-in wall's is.
func (r *Resolver) asAccountFailure(err error, first, again accountTry) error {
	wall := signInWall(err)
	if first.skip == AccountOff && !wall {
		return err
	}
	f := &accountFailure{err: err, first: first, again: again, route: "yt-dlp"}
	if r.Direct != nil {
		// Without yt-dlp, the player API's plain words.
		f.route = "YouTube's player API"
	}
	if !wall {
		return f
	}
	switch {
	case first.skip == "" || again.skip == "" || first.skip == accountMarked:
		f.way = AccountRefused
	case first.skip == AccountOff && r.signedOut():
		f.way = AccountSignIn
	default:
		f.way = first.skip
	}
	return f
}

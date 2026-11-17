package stream

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

// premiumOnly is what yt-dlp prints, signed out, of a video YouTube plays
// only to YouTube Music Premium members, turned into the error runYTDLP
// returns: plainErrors words it by "premium".
func premiumOnly(videoID string) error {
	return ytdlpFailure("ERROR: [youtube] " + videoID + ": This video is only available to Music Premium members\n")
}

// TestSignInWallOfPremium takes YouTube Music Premium's members only for a
// sign-in wall, beside "Please sign in", the check of age and a channel's
// members only, and never the check of bots.
func TestSignInWallOfPremium(t *testing.T) {
	for _, c := range []struct {
		msg  string
		wall bool
	}{
		{"This video is only available to Music Premium members", true},
		{"Please sign in. This helps protect our community.", true},
		{"Sign in to confirm your age. This video may be inappropriate for some users.", true},
		{"Join this channel to get access to members-only content like this video, and other exclusive perks.", true},
		{"Sign in to confirm you’re not a bot. Use --cookies-from-browser or --cookies for the authentication.", false},
		{"The uploader has not made this video available in your country", false},
		{"Video unavailable", false},
	} {
		err := ytdlpFailure("ERROR: [youtube] " + songID + ": " + c.msg + "\n")
		if got := signInWall(err); got != c.wall {
			t.Errorf("%q (%v): a sign-in wall %v, want %v", c.msg, err, got, c.wall)
		}
	}
	if signInWall(errors.New("yt-dlp: exit status 1")) {
		t.Error("a crash is a sign-in wall")
	}
}

// TestRefusedAsksAgainAtPremium asks as the account once more for a video
// whose stream found so YouTube refused, when the signed-out ways meet
// YouTube Music Premium's members only: an account with Premium plays it,
// and one without fails with the signed-out failure after that one run,
// which the log tells of. A bot check still asks no more.
func TestRefusedAsksAgainAtPremium(t *testing.T) {
	botCheck := func(context.Context) ([]byte, error) {
		return nil, ytdlpFailure("ERROR: [youtube] " + songID + ": Sign in to confirm you’re not a bot. Use --cookies-from-browser or --cookies for the authentication.\n")
	}
	premiumWall := func(context.Context) ([]byte, error) { return nil, premiumOnly(songID) }
	for _, c := range []struct {
		name      string
		signedOut func(context.Context) ([]byte, error)
		premium   bool // the account has Premium when it's asked once more
		plays     bool
		asAccount int
	}{
		{"with Premium", premiumWall, true, true, 2},
		{"without Premium", premiumWall, false, false, 2},
		{"a bot check", botCheck, true, false, 1},
	} {
		logs := captureLog(t)
		var asked atomic.Int32
		y := &accountYTDLP{
			asAccount: func(context.Context) ([]byte, error) {
				// The first run finds the stream; the one once more
				// does as the account has Premium.
				if asked.Add(1) == 1 || c.premium {
					return premiumAnswer(t), nil
				}
				return nil, premiumOnly(songID)
			},
			signedOut: c.signedOut,
		}
		r := accountResolver(t, y)
		if s, err := r.Resolve(context.Background(), songID); err != nil || !s.AsAccount {
			t.Fatalf("%s, first: %+v, %v", c.name, s, err)
		}
		r.Forget(songID) // as after YouTube refused its URL
		_, want := c.signedOut(context.Background())
		s, err := r.Resolve(context.Background(), songID)
		switch {
		case c.plays && (err != nil || !s.AsAccount || s.Route != RouteAccount):
			t.Errorf("%s: not played as the account: %+v, %v", c.name, s, err)
		case !c.plays && (err == nil || err.Error() != want.Error()):
			t.Errorf("%s: %v, want the signed-out failure %v", c.name, err, want)
		}
		if n := asAccountRuns(y); n != c.asAccount {
			t.Errorf("%s: yt-dlp ran %d times as the account, want %d", c.name, n, c.asAccount)
		}
		told := strings.Contains(logs.String(), "yt-dlp found no stream of "+songID+" as your YouTube Music account once more in ") &&
			strings.Contains(logs.String(), "nor is it signed out, where YouTube plays it only to some signed-in viewers")
		if wantTold := c.name == "without Premium"; told != wantTold {
			t.Errorf("%s: the log tells of the run once more that failed: %v, want %v\n%s", c.name, told, wantTold, logs.String())
		}
	}
}

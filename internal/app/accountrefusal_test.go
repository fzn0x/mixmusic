package app

import (
	"context"
	"testing"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// asAccountOn has a ask for each track as the account signed in to
// YouTube Music, with a yt-dlp of the test's, with a runtime, that finds
// every stream either way, and no request leaving the computer.
func asAccountOn(t *testing.T, a *App) {
	t.Helper()
	a.resolver.Direct = nil
	a.resolver.Tools = nil
	a.resolver.Run = (&asAccountYTDLP{}).run
	if _, err := (&Preferences{a}).Update(func(s *settings.Settings) { s.AudioAsAccount = true }); err != nil {
		t.Fatal(err)
	}
}

// refusedAsAccount resolves video, then has YouTube refuse its stream, and
// reports whether the stream was found as the account: one found so marks
// the video, found signed out from then on, while the session is the same.
func refusedAsAccount(t *testing.T, a *App, video string) bool {
	t.Helper()
	s, err := a.resolver.Resolve(context.Background(), video)
	if err != nil {
		t.Fatal(err)
	}
	a.resolver.Forget(video) // as after YouTube refused its URL
	return s.AsAccount
}

// TestYouTubeAccountSwitchForgetsRefusals asks as the account again at once
// for a track whose audio found so YouTube refused, once MixMusic uses
// YouTube Music as another channel, or as another Google account of the
// session and back before the next track, which leave the session the
// resolver is given as it was (#578).
func TestYouTubeAccountSwitchForgetsRefusals(t *testing.T) {
	a, s := channelsApp(t, answeringAs("Ada Records", "@adarecords"))
	// yt-dlp asks as an account only with LOGIN_INFO beside SAPISID.
	if _, err := s.YouTubePasteCookies(context.Background(), "SAPISID=pasted; LOGIN_INFO=pasted"); err != nil {
		t.Fatal(err)
	}
	asAccountOn(t, a)
	const video = "aaaaaaaaaaa"
	if !refusedAsAccount(t, a, video) {
		t.Fatal("first: not as the account")
	}
	if refusedAsAccount(t, a, video) {
		t.Fatal("after the refusal, as the same account: as the account")
	}

	if _, err := chooseRecords(t, s); err != nil {
		t.Fatal(err)
	}
	usesRecords(t, a)
	if !refusedAsAccount(t, a, video) {
		t.Error("as a brand channel: not asked for as the account again")
	}
	if refusedAsAccount(t, a, video) {
		t.Error("refused again, as the same channel: as the account")
	}

	for _, user := range []int{1, 0} {
		if err := a.useYouTubeAccount(user, ""); err != nil {
			t.Fatal(err)
		}
	}
	if !refusedAsAccount(t, a, video) {
		t.Error("as another Google account and back: not asked for as the account again")
	}
}

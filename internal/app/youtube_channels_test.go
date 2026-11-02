package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/youtube"
)

// recordsPage is the page ID of the brand channel of channelsApp's
// account.
const recordsPage = "111111111111111111111"

// channelsApp is an app signed in to YouTube Music with a session of one
// Google account, Ada, which manages two brand channels: Ada Records, and
// Ada Archive, which the list tells no page ID of. Its account's menu
// answers as the brand channel the page ID of a request names, unless
// pageless, as when YouTube takes no page ID.
func channelsApp(t *testing.T, pageless bool) (*App, *Accounts) {
	t.Helper()
	accounts := `{"actions":[{"getMultiPageMenuAction":{"menu":{"multiPageMenuRenderer":{"sections":[
		{"accountSectionListRenderer":{"contents":[{"accountItemSectionRenderer":{"contents":[
			{"accountItem":{"accountName":{"simpleText":"Ada"},"channelHandle":{"simpleText":"@ada"},
				"serviceEndpoint":{"selectActiveIdentityEndpoint":{"supportedTokens":[{"datasyncIdToken":{"datasyncIdToken":"100000000000000000001||"}}]}}}},
			{"accountItem":{"accountName":{"simpleText":"Ada Records"},"channelHandle":{"simpleText":"@adarecords"},
				"serviceEndpoint":{"selectActiveIdentityEndpoint":{"supportedTokens":[{"pageIdToken":{"pageId":"` + recordsPage + `"}}]}}}},
			{"accountItem":{"accountName":{"simpleText":"Ada Archive"},"channelHandle":{"simpleText":"@adaarchive"},
				"serviceEndpoint":{"selectActiveIdentityEndpoint":{"supportedTokens":[{"accountStateToken":{"hasChannel":true}}]}}}}
		]}}]}}
	]}}}}]}`
	a, _ := cookieApp(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/accounts_list") {
			w.Write([]byte(accounts))
			return
		}
		if r.Header.Get("X-Goog-PageId") == recordsPage && !pageless {
			answerMenu(w, "Ada Records", "@adarecords")
			return
		}
		answerMenu(w, "Ada", "@ada")
	})
	s := &Accounts{a}
	if _, err := s.YouTubePasteCookies(context.Background(), "SAPISID=pasted"); err != nil {
		t.Fatal(err)
	}
	return a, s
}

// answerMenu answers a request of the account's menu as YouTube Music does
// for a session it takes, signed in as name, of handle when it isn't "".
func answerMenu(w http.ResponseWriter, name, handle string) {
	header := map[string]any{"accountName": map[string]any{"runs": []any{map[string]any{"text": name}}}}
	if handle != "" {
		header["channelHandle"] = map[string]any{"runs": []any{map[string]any{"text": handle}}}
	}
	json.NewEncoder(w).Encode(map[string]any{
		"responseContext": map[string]any{"serviceTrackingParams": []any{map[string]any{"params": []any{map[string]any{"key": "logged_in", "value": "1"}}}}},
		"actions": []any{map[string]any{"openPopupAction": map[string]any{"popup": map[string]any{"multiPageMenuRenderer": map[string]any{
			"header": map[string]any{"activeAccountHeaderRenderer": header},
		}}}}},
	})
}

// channelNamed returns the channel of list named name.
func channelNamed(t *testing.T, list []youtube.AccountChannel, name string) youtube.AccountChannel {
	t.Helper()
	for _, ch := range list {
		if ch.Name == name {
			return ch
		}
	}
	t.Fatalf("no channel %s in %+v", name, list)
	return youtube.AccountChannel{}
}

// TestYouTubeChannelWithoutPageID lists a brand channel YouTube tells no
// page ID of, which would be the account itself: it is never the channel
// used, and can't be chosen.
func TestYouTubeChannelWithoutPageID(t *testing.T) {
	a, s := channelsApp(t, false)
	ctx := context.Background()
	list, err := s.YouTubeChannels(ctx)
	if err != nil || len(list) != 3 {
		t.Fatalf("channels = %+v, %v", list, err)
	}
	archive := channelNamed(t, list, "Ada Archive")
	if !archive.Unswitchable || archive.Current || !channelNamed(t, list, "Ada").Current {
		t.Errorf("channels = %+v", list)
	}
	if _, err := s.SetYouTubeChannel(ctx, archive); err == nil || err.Error() != "YouTube doesn't let MixMusic switch to this channel" {
		t.Errorf("choosing it: %v", err)
	}
	if a.youtube.Channel() != "" || a.youtube.AuthUser() != 0 || a.settings.Get().YouTubeChannel != "" {
		t.Errorf("choosing it switched to %d, %q", a.youtube.AuthUser(), a.youtube.Channel())
	}
}

// TestYouTubeChannelChecked checks, once switched to a brand channel, that
// YouTube Music answers as it.
func TestYouTubeChannelChecked(t *testing.T) {
	a, s := channelsApp(t, false)
	ctx := context.Background()
	list, _ := s.YouTubeChannels(ctx)
	status, err := s.SetYouTubeChannel(ctx, channelNamed(t, list, "Ada Records"))
	if err != nil {
		t.Fatal(err)
	}
	if me := status.YouTube.Account; me == nil || me.Name != "Ada Records" {
		t.Errorf("signed in as %+v", me)
	}
	if a.youtube.Channel() != recordsPage || a.settings.Get().YouTubeChannel != recordsPage {
		t.Errorf("using %q, keeping %q", a.youtube.Channel(), a.settings.Get().YouTubeChannel)
	}
}

// TestYouTubeChannelAnswersAsAnother goes back to the account and channel
// used before, and says so, when YouTube Music answers as another than the
// channel chosen.
func TestYouTubeChannelAnswersAsAnother(t *testing.T) {
	a, s := channelsApp(t, true)
	ctx := context.Background()
	list, _ := s.YouTubeChannels(ctx)
	_, err := s.SetYouTubeChannel(ctx, channelNamed(t, list, "Ada Records"))
	if err == nil || err.Error() != "YouTube Music answered as Ada, not as Ada Records: MixMusic goes on as before" {
		t.Errorf("choosing a channel YouTube Music doesn't answer as: %v", err)
	}
	if a.youtube.Channel() != "" || a.settings.Get().YouTubeChannel != "" {
		t.Errorf("using %q, keeping %q", a.youtube.Channel(), a.settings.Get().YouTubeChannel)
	}
	a.mu.Lock()
	me := a.youtubeMe
	a.mu.Unlock()
	if me != nil && me.Name != "Ada" {
		t.Errorf("the profile kept is %+v", me)
	}
	if list, _ := s.YouTubeChannels(ctx); !channelNamed(t, list, "Ada").Current {
		t.Errorf("after going back: %+v", list)
	}
}

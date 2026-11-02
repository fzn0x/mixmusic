package app

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/fzn0x/mixmusic/internal/youtube"
)

// recordsPage is the page ID of the brand channel of channelsApp's
// account.
const recordsPage = "111111111111111111111"

// channelsApp is an app signed in to YouTube Music with a session of one
// Google account, Ada, which manages two brand channels: Ada Records, and
// Ada Archive, which the list tells no page ID of. Its account's menu
// answers as Ada, and with records to a request of Ada Records' page ID.
func channelsApp(t *testing.T, records http.HandlerFunc) (*App, *Accounts) {
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
		if r.Header.Get("X-Goog-PageId") == recordsPage {
			records(w, r)
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

// answeringAs answers the account's menu as name, of handle.
func answeringAs(name, handle string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { answerMenu(w, name, handle) }
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
	a, s := channelsApp(t, answeringAs("Ada Records", "@adarecords"))
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

// chooseRecords chooses Ada Records, of the channels s lists.
func chooseRecords(t *testing.T, s *Accounts) (AccountsStatus, error) {
	t.Helper()
	ctx := context.Background()
	list, err := s.YouTubeChannels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return s.SetYouTubeChannel(ctx, channelNamed(t, list, "Ada Records"))
}

// usesRecords fails unless a uses YouTube Music as Ada Records, and keeps
// it in the settings.
func usesRecords(t *testing.T, a *App) {
	t.Helper()
	if a.youtube.Channel() != recordsPage || a.settings.Get().YouTubeChannel != recordsPage {
		t.Errorf("using %q, keeping %q", a.youtube.Channel(), a.settings.Get().YouTubeChannel)
	}
}

// TestYouTubeChannelChosen uses a brand channel YouTube Music answers as.
func TestYouTubeChannelChosen(t *testing.T) {
	a, s := channelsApp(t, answeringAs("Ada Records", "@adarecords"))
	status, err := chooseRecords(t, s)
	if err != nil {
		t.Fatal(err)
	}
	if me := status.YouTube.Account; me == nil || me.Name != "Ada Records" {
		t.Errorf("signed in as %+v", me)
	}
	usesRecords(t, a)
}

// TestYouTubeChannelAnswersAsAccount goes back to the account and channel
// used before, and says so, when YouTube Music answers a brand channel's
// requests as the Google account itself.
func TestYouTubeChannelAnswersAsAccount(t *testing.T) {
	a, s := channelsApp(t, answeringAs("Ada", "@ada"))
	_, err := chooseRecords(t, s)
	if err == nil || err.Error() != "YouTube Music answered as the account Ada, not as its channel Ada Records: MixMusic goes on as before" {
		t.Errorf("choosing a channel YouTube Music answers as the account: %v", err)
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
	if list, _ := s.YouTubeChannels(context.Background()); !channelNamed(t, list, "Ada").Current {
		t.Errorf("after going back: %+v", list)
	}
}

// TestYouTubeChannelAnswersOtherwise keeps a switch YouTube Music answers
// with a name that is neither the channel's nor the account's, as one it
// changed since, and logs that it did, without the names.
func TestYouTubeChannelAnswersOtherwise(t *testing.T) {
	logs := &channelLog{}
	log.SetOutput(logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	a, s := channelsApp(t, answeringAs("Ada Records Official", "@adarecordsofficial"))
	if _, err := chooseRecords(t, s); err != nil {
		t.Fatal(err)
	}
	usesRecords(t, a)
	logs.mu.Lock()
	text := logs.b.String()
	logs.mu.Unlock()
	if !strings.Contains(text, "YouTube Music answers with another name than the account list gives the channel chosen") || strings.Contains(text, "Ada") {
		t.Errorf("logged %q", text)
	}
}

// TestYouTubeChannelProfileUnread keeps a switch when the profile YouTube
// Music answers with can't be read.
func TestYouTubeChannelProfileUnread(t *testing.T) {
	a, s := channelsApp(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	status, err := chooseRecords(t, s)
	if err != nil {
		t.Fatal(err)
	}
	if status.YouTube.Error == "" {
		t.Errorf("the profile read: %+v", status.YouTube)
	}
	usesRecords(t, a)
}

// channelLog keeps what the app logs, as several goroutines log it.
type channelLog struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *channelLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

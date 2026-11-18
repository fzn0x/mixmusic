package app

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// signedOutSession answers as YouTube does for a browser session signed in
// to the Google accounts of index 0 to 4, of which 0 and 3 were signed out
// of since, as from another device: the account's menu of 0 is refused,
// that of 3 answered as to a visitor, and the list still names both.
func signedOutSession(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/accounts_list") {
		var sections []string
		for i := range 5 {
			sections = append(sections, fmt.Sprintf(`{"accountSectionListRenderer":{"header":{"googleAccountHeaderRenderer":{"email":{"simpleText":"user%d@example.com"}}},
				"contents":[{"accountItemSectionRenderer":{"contents":[{"accountItem":{"accountName":{"simpleText":"User %d"},
					"serviceEndpoint":{"selectActiveIdentityEndpoint":{"supportedTokens":[{"accountSigninToken":{"signinUrl":"/signin?authuser=%d"}}]}}}}]}}]}}`, i, i, i))
		}
		w.Write([]byte(`{"actions":[{"getMultiPageMenuAction":{"menu":{"multiPageMenuRenderer":{"sections":[` + strings.Join(sections, ",") + `]}}}}]}`))
		return
	}
	switch user := r.Header.Get("X-Goog-AuthUser"); user {
	case "0":
		w.WriteHeader(http.StatusUnauthorized)
	case "1", "2", "4":
		answerMenu(w, "User "+user, "@user"+user)
	default:
		w.Write([]byte(`{"responseContext":{"serviceTrackingParams":[{"params":[{"key":"logged_in","value":"0"}]}]}}`))
	}
}

// TestSignInFirstAccountSignedOut signs in with a session whose first
// Google account was signed out of, as its first account still signed in,
// and keeps its index; the accounts list leaves out those signed out (#582).
func TestSignInFirstAccountSignedOut(t *testing.T) {
	a, _ := cookieApp(t, signedOutSession)
	s := &Accounts{a}
	ctx := context.Background()
	status, err := s.YouTubePasteCookies(ctx, "SAPISID=pasted")
	if err != nil {
		t.Fatal(err)
	}
	if me := status.YouTube.Account; me == nil || me.Name != "User 1" {
		t.Errorf("signed in as %+v", status.YouTube)
	}
	if a.youtube.AuthUser() != 1 || a.youtube.Rejected() {
		t.Errorf("using account %d, rejected %v", a.youtube.AuthUser(), a.youtube.Rejected())
	}
	kept, err := a.youtubeStore.Load()
	if err != nil || kept.authUser != 1 {
		t.Errorf("kept account %d, %v", kept.authUser, err)
	}
	list, err := s.YouTubeChannels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, ch := range list {
		names = append(names, ch.Name)
	}
	if !slices.Equal(names, []string{"User 1", "User 2", "User 4"}) || !list[0].Current || list[1].Current || list[2].Current {
		t.Errorf("channels = %+v", list)
	}
	// A switch to another account still signed in works as before.
	if _, err := s.SetYouTubeChannel(ctx, list[2]); err != nil {
		t.Fatal(err)
	}
	if kept, _ := a.youtubeStore.Load(); a.youtube.AuthUser() != 4 || kept.authUser != 4 {
		t.Errorf("after switching, using account %d, kept %d", a.youtube.AuthUser(), kept.authUser)
	}
}

package youtube

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestInviteCollaborators turns collaboration on for a playlist of the
// user's, as YouTube Music's website asks, and reads the invite link of
// YouTube's answer: none, or one off YouTube's sites, is an error, as an
// answer that didn't succeed and a 400 are.
func TestInviteCollaborators(t *testing.T) {
	invite := fixture(t, "edit-playlist-invite").v
	answer := func(string) (int, any) { return http.StatusOK, invite }
	c, requests, _ := statusYouTube(t, func(path string) (int, any) { return answer(path) })
	ctx := context.Background()
	if _, err := c.InviteCollaborators(ctx, "PLtestOwnPlaylist"); !errors.Is(err, ErrSignedOut) {
		t.Errorf("signed out: %v", err)
	}
	c.SetCookies(session)
	link, err := c.InviteCollaborators(ctx, "VLPLtestOwnPlaylist")
	if err != nil || link != "https://music.youtube.com/playlist?list=PLtestOwnPlaylist&jct=testJoinToken01" {
		t.Fatalf("InviteCollaborators = %q, %v", link, err)
	}
	r := requests()[0]
	action, _ := r.body["actions"].([]any)[0].(map[string]any)
	if r.path != "/youtubei/v1/browse/edit_playlist" || r.body["playlistId"] != "PLtestOwnPlaylist" || action["action"] != actionInvite || len(action) != 1 {
		t.Errorf("the invite went to %s with %v", r.path, r.body)
	}

	for _, tc := range []struct {
		name   string
		status int
		answer any
		want   error
	}{
		{"no link", http.StatusOK, map[string]any{"status": "STATUS_SUCCEEDED"}, errNoInviteLink},
		{"a link elsewhere", http.StatusOK, map[string]any{"status": "STATUS_SUCCEEDED", "collaborationInviteLink": "https://example.com/?jct=x"}, errNoInviteLink},
		{"not plain https", http.StatusOK, map[string]any{"status": "STATUS_SUCCEEDED", "collaborationInviteLink": "http://music.youtube.com/playlist?list=PLx&jct=x"}, errNoInviteLink},
		{"a failed status", http.StatusOK, map[string]any{"status": "STATUS_FAILED"}, errInviteRefused},
		{"a bad request", http.StatusBadRequest, map[string]any{"error": map[string]any{"code": 400, "message": "Request contains an invalid argument."}}, errInviteDenied},
	} {
		answer = func(string) (int, any) { return tc.status, tc.answer }
		if link, err := c.InviteCollaborators(ctx, "PLtestOwnPlaylist"); link != "" || !errors.Is(err, tc.want) {
			t.Errorf("%s: %q, %v, want %v", tc.name, link, err, tc.want)
		}
	}
}

// TestInviteDenied keeps the session through an invite YouTube refuses
// with 403, as an edit of another account's playlist, and says what may
// be wrong.
func TestInviteDenied(t *testing.T) {
	c, _, _ := statusYouTube(t, func(path string) (int, any) {
		if path == "/youtubei/v1/account/account_menu" {
			return http.StatusOK, accountMenu("1")
		}
		return http.StatusForbidden, denied(http.StatusForbidden)
	})
	c.SetCookies(session)
	if _, err := c.InviteCollaborators(context.Background(), "PLx"); !errors.Is(err, errInviteDenied) || c.Rejected() {
		t.Errorf("InviteCollaborators = %v, rejected %v", err, c.Rejected())
	}
	if err := c.StopCollaboration(context.Background(), "PLx"); !errors.Is(err, errStopDenied) || c.Rejected() {
		t.Errorf("StopCollaboration = %v, rejected %v", err, c.Rejected())
	}
}

// TestStopCollaboration turns collaboration off, closing the playlist to
// contributions, as YouTube Music's website asks.
func TestStopCollaboration(t *testing.T) {
	status := "STATUS_SUCCEEDED"
	c, requests, _ := fakeYouTube(t, func(string) any { return map[string]any{"status": status} })
	c.SetCookies(session)
	ctx := context.Background()
	if err := c.StopCollaboration(ctx, "PLtestOwnPlaylist"); err != nil {
		t.Fatal(err)
	}
	action, _ := requests()[0].body["actions"].([]any)[0].(map[string]any)
	if action["action"] != actionClose || action["closedToContributions"] != true {
		t.Errorf("the stop asked %v", action)
	}
	status = "STATUS_FAILED"
	if err := c.StopCollaboration(ctx, "PLtestOwnPlaylist"); !errors.Is(err, errStopRefused) {
		t.Errorf("a failed status: %v", err)
	}
}

// TestSetPlaylistPrivacy sets who may open a playlist by YouTube's names
// of its privacies, and refuses one it has none for without asking.
func TestSetPlaylistPrivacy(t *testing.T) {
	c, requests, _ := fakeYouTube(t, func(string) any { return map[string]any{"status": "STATUS_SUCCEEDED"} })
	c.SetCookies(session)
	ctx := context.Background()
	for privacy, want := range map[media.Privacy]string{media.Unlisted: "UNLISTED", media.Public: "PUBLIC", media.Private: "PRIVATE"} {
		if err := c.SetPlaylistPrivacy(ctx, "PLx", privacy); err != nil {
			t.Fatal(err)
		}
		got := requests()
		action, _ := got[len(got)-1].body["actions"].([]any)[0].(map[string]any)
		if action["action"] != actionPrivacy || action["playlistPrivacy"] != want {
			t.Errorf("%s asked %v", privacy, action)
		}
	}
	if err := c.SetPlaylistPrivacy(ctx, "PLx", "friends"); err == nil || len(requests()) != 3 {
		t.Errorf("an unknown privacy: %v, %d requests", err, len(requests()))
	}
}

// TestCreateUnlistedPlaylist makes a playlist anyone with its link may
// open, to share it, as CreatePlaylist makes a private one.
func TestCreateUnlistedPlaylist(t *testing.T) {
	c, requests, _ := fakeYouTube(t, func(string) any { return map[string]any{"playlistId": "PLshared"} })
	c.SetCookies(session)
	if id, err := c.CreatePlaylistAs(context.Background(), "Road songs", []string{"v1"}, media.Unlisted); err != nil || id != "PLshared" {
		t.Fatalf("CreatePlaylistAs = %q, %v", id, err)
	}
	if r := requests()[0]; r.body["privacyStatus"] != "UNLISTED" || r.body["title"] != "Road songs" {
		t.Errorf("request = %s %v", r.path, r.body)
	}
	if _, err := c.CreatePlaylistAs(context.Background(), "Road songs", nil, ""); err == nil || len(requests()) != 1 {
		t.Errorf("no privacy: %v, %d requests", err, len(requests()))
	}
}

// collaborationPage is the page of testdata/playlist.json as YouTube Music
// shows it to the playlist's owner, whose tools say privacy, with the
// avatars of the header of testdata/playlist-collaboration-header.json
// named avatars ("collaborators" or "owner"), or none for "".
func collaborationPage(t *testing.T, privacy, avatars string) node {
	t.Helper()
	parts := fixture(t, "playlist-collaboration-header")
	n := fixture(t, "playlist")
	section := n.get("contents", "twoColumnBrowseResultsRenderer", "tabs", 0, "tabRenderer", "content", "sectionListRenderer", "contents").v.([]any)
	header := section[0].(map[string]any)["musicResponsiveHeaderRenderer"].(map[string]any)
	if avatars != "" {
		header["facepile"] = parts.get(avatars).v
	}
	edit := parts.get("editHeader").v.(map[string]any)
	edit["musicPlaylistEditHeaderRenderer"].(map[string]any)["privacy"] = privacy
	section[0] = map[string]any{"musicEditablePlaylistDetailHeaderRenderer": map[string]any{
		"header":     map[string]any{"musicResponsiveHeaderRenderer": header},
		"editHeader": edit,
	}}
	return n
}

// TestParsePlaylistCollaboration reads, from a playlist page's header, who
// may open a playlist of the user's, and whether its avatars are its
// collaborators', whose line names them where the header names no owner.
func TestParsePlaylistCollaboration(t *testing.T) {
	p, _, err := parsePlaylist(fixture(t, "playlist"), "PLx")
	if err != nil || p.Collaborative || p.Privacy != "" || p.Mine || p.Owner != "" {
		t.Fatalf("a playlist of someone else: collaborative %v, privacy %q, mine %v, owner %q, %v", p.Collaborative, p.Privacy, p.Mine, p.Owner, err)
	}

	for _, tc := range []struct {
		privacy, avatars string
		want             media.Privacy
		collaborative    bool
		owner            string
	}{
		{"PRIVATE", "", media.Private, false, ""},
		{"UNLISTED", "owner", media.Unlisted, false, "Ana Testwood"},
		{"PUBLIC", "collaborators", media.Public, true, "by Ana Testwood and 2 others"},
		{"SOMETHING_NEW", "collaborators", "", true, "by Ana Testwood and 2 others"},
	} {
		p, _, err := parsePlaylist(collaborationPage(t, tc.privacy, tc.avatars), "PLx")
		if err != nil || p.Privacy != tc.want || p.Collaborative != tc.collaborative || !p.Editable || p.Owner != tc.owner || p.Mine {
			t.Errorf("%s with avatars of %q: privacy %q, collaborative %v, editable %v, owner %q, mine %v, %v",
				tc.privacy, tc.avatars, p.Privacy, p.Collaborative, p.Editable, p.Owner, p.Mine, err)
		}
	}

	// A header that names its owner keeps that name.
	n := collaborationPage(t, "PUBLIC", "collaborators")
	header := n.get("contents", "twoColumnBrowseResultsRenderer", "tabs", 0, "tabRenderer", "content", "sectionListRenderer", "contents", 0,
		"musicEditablePlaylistDetailHeaderRenderer", "header", "musicResponsiveHeaderRenderer").v.(map[string]any)
	header["straplineTextOne"] = map[string]any{"runs": []any{map[string]any{"text": "Bo Fieldfare",
		"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{"browseId": "UCtestBo"}}}}}
	if p, _, _ := parsePlaylist(n, "PLx"); p.Owner != "Bo Fieldfare" || !p.Collaborative {
		t.Errorf("a header with its owner's line: owner %q, collaborative %v", p.Owner, p.Collaborative)
	}
	// Its owner's, whom YouTube Music offers to delete it, is theirs.
	buttons, _ := header["buttons"].([]any)
	header["buttons"] = append(buttons, map[string]any{"menuRenderer": map[string]any{"items": []any{deleteItem("PLx")}}})
	if p, _, _ := parsePlaylist(n, "PLx"); !p.Mine || !p.Deletable {
		t.Errorf("the owner's page: mine %v, deletable %v", p.Mine, p.Deletable)
	}
}

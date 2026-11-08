package youtube

import (
	"context"
	"errors"
	"testing"
)

// The answer to an add of a video the playlist holds already,
// edit-playlist-duplicate.json, is written by hand from the documented
// shape of YouTube Music's answer, not recorded: a failed status, with a
// toast that offers Add anyway, the same add with the check for a video
// already there skipped.

// editRefused is an answer of a refused edit, with what YouTube says of
// it in a toast, if anything.
func editRefused(words string) map[string]any {
	answer := map[string]any{"status": "STATUS_FAILED"}
	if words != "" {
		answer["actions"] = []any{map[string]any{"addToToastAction": map[string]any{"item": map[string]any{
			"notificationTextRenderer": map[string]any{"responseText": runsOf(words)}}}}}
	}
	return answer
}

func runsOf(text string) map[string]any {
	return map[string]any{"runs": []any{map[string]any{"text": text}}}
}

// TestAddDuplicate tells an add of a video the playlist holds already, by
// the Add anyway YouTube offers, from other refusals, which say what
// YouTube says, or that YouTube didn't add it.
func TestAddDuplicate(t *testing.T) {
	// The duplicate's Add anyway, in a dialog rather than a toast.
	dialog := map[string]any{"status": "STATUS_FAILED", "actions": []any{map[string]any{"openPopupAction": map[string]any{"popup": map[string]any{
		"confirmDialogRenderer": map[string]any{
			"title":          runsOf("Already added"),
			"dialogMessages": []any{runsOf("This song is already in Road trip.")},
			"confirmButton": map[string]any{"buttonRenderer": map[string]any{"text": runsOf("Add anyway"), "serviceEndpoint": map[string]any{
				"playlistEditEndpoint": map[string]any{"playlistId": "PLx", "actions": []any{
					map[string]any{"action": "ACTION_ADD_VIDEO", "addedVideoId": "v1", "dedupeOption": "DEDUPE_OPTION_SKIP"},
				}},
			}}},
		},
	}}}}}
	for _, tc := range []struct {
		name   string
		answer any
		want   string
	}{
		{"the toast's Add anyway", fixture(t, "edit-playlist-duplicate").v, ErrAlreadyInPlaylist.Error()},
		{"the dialog's Add anyway", dialog, ErrAlreadyInPlaylist.Error()},
		{"YouTube's words", editRefused("This playlist is full."), "YouTube Music did not add it: This playlist is full"},
		{"no words", editRefused(""), "YouTube Music did not add it"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _, _ := fakeYouTube(t, func(string) any { return tc.answer })
			c.SetCookies(session)
			err := c.AddToPlaylist(context.Background(), "PLx", "v1")
			if err == nil || err.Error() != tc.want {
				t.Fatalf("AddToPlaylist = %v, want %q", err, tc.want)
			}
			if dup := errors.Is(err, ErrAlreadyInPlaylist); dup != (tc.want == ErrAlreadyInPlaylist.Error()) {
				t.Errorf("a duplicate: %v", dup)
			}
		})
	}
	// A removal YouTube refuses says what YouTube says too.
	c, _, _ := fakeYouTube(t, func(string) any { return editRefused("This video is no longer in the playlist.") })
	c.SetCookies(session)
	if err := c.RemoveFromPlaylist(context.Background(), "PLx", "set1", "v1"); err == nil ||
		err.Error() != "YouTube Music did not take it out of the playlist: This video is no longer in the playlist" {
		t.Errorf("RemoveFromPlaylist = %v", err)
	}
}

// TestAddToPlaylistAnyway adds a video the playlist holds already as Add
// anyway does: the same add, with the check for a video already there
// skipped, which a plain add leaves on.
func TestAddToPlaylistAnyway(t *testing.T) {
	c, requests, _ := fakeYouTube(t, func(string) any { return map[string]any{"status": "STATUS_SUCCEEDED"} })
	ctx := context.Background()
	if err := c.AddToPlaylistAnyway(ctx, "PLx", "v1"); !errors.Is(err, ErrSignedOut) {
		t.Errorf("signed out: %v", err)
	}
	c.SetCookies(session)
	if err := c.AddToPlaylist(ctx, "VLPLx", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := c.AddToPlaylistAnyway(ctx, "VLPLx", "v1"); err != nil {
		t.Fatal(err)
	}
	got := requests()
	if len(got) != 2 {
		t.Fatalf("%d requests", len(got))
	}
	plain := got[0].body["actions"].([]any)[0].(map[string]any)
	if _, ok := plain["dedupeOption"]; ok {
		t.Errorf("a plain add skips the check: %v", plain)
	}
	again := got[1].body["actions"].([]any)[0].(map[string]any)
	if got[1].path != "/youtubei/v1/browse/edit_playlist" || got[1].body["playlistId"] != "PLx" ||
		again["action"] != "ACTION_ADD_VIDEO" || again["addedVideoId"] != "v1" || again["dedupeOption"] != "DEDUPE_OPTION_SKIP" {
		t.Errorf("add anyway sent %s %v", got[1].path, got[1].body)
	}
}

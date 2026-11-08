package youtube

import (
	"context"
	"errors"
	"testing"
)

// TestMoveInPlaylist moves a track of a playlist before another, and to
// the end, as the website asks, by the places of the playlist's entries.
func TestMoveInPlaylist(t *testing.T) {
	status := "STATUS_SUCCEEDED"
	c, requests, _ := fakeYouTube(t, func(string) any { return map[string]any{"status": status} })
	ctx := context.Background()
	if err := c.MoveInPlaylist(ctx, "PLx", "set1", "set2"); !errors.Is(err, ErrSignedOut) {
		t.Errorf("signed out: %v", err)
	}
	c.SetCookies(session)
	if err := c.MoveInPlaylist(ctx, "PLx", "", "set2"); err == nil {
		t.Error("moved a track without its place in the playlist")
	}
	if err := c.MoveInPlaylist(ctx, "VLPLx", "set3", "set1"); err != nil {
		t.Fatal(err)
	}
	if err := c.MoveInPlaylist(ctx, "PLx", "set1", ""); err != nil {
		t.Fatal(err)
	}
	status = "STATUS_FAILED"
	if err := c.MoveInPlaylist(ctx, "PLx", "set1", "set2"); err == nil {
		t.Error("a failed status gave no error")
	}
	got := requests()
	if len(got) != 3 {
		t.Fatalf("%d requests", len(got))
	}
	before, end := got[0].body, got[1].body
	if got[0].path != "/youtubei/v1/browse/edit_playlist" || before["playlistId"] != "PLx" {
		t.Errorf("the move went to %s with %v", got[0].path, before["playlistId"])
	}
	if a := before["actions"].([]any)[0].(map[string]any); a["action"] != "ACTION_MOVE_VIDEO_BEFORE" || a["setVideoId"] != "set3" ||
		a["movedSetVideoIdSuccessor"] != "set1" {
		t.Errorf("the move before another = %v", a)
	}
	if a := end["actions"].([]any)[0].(map[string]any); a["setVideoId"] != "set1" || a["movedSetVideoIdSuccessor"] != nil {
		t.Errorf("the move to the end = %v", a)
	}
}

package youtube

import (
	"context"
	"strings"
	"testing"
)

// TestReportPlayUnplayable tells why YouTube's player gave no report URL,
// as its playabilityStatus says, and reports nothing.
func TestReportPlayUnplayable(t *testing.T) {
	c, requests, _ := fakeYouTube(t, func(path string) any {
		return map[string]any{"playabilityStatus": map[string]any{
			"status": "UNPLAYABLE",
			"reason": "Video unavailable",
			"errorScreen": map[string]any{"playerErrorMessageRenderer": map[string]any{
				"reason":    map[string]any{"runs": []any{map[string]any{"text": "The page needs to be reloaded."}}},
				"subreason": map[string]any{"runs": []any{map[string]any{"text": "The page needs to be reloaded."}}},
			}},
		}}
	})
	c.SetCookies(session)
	err := c.ReportPlay(context.Background(), "v1")
	want := `youtube music: no playback report for this track: YouTube's player answered UNPLAYABLE, "Video unavailable", "The page needs to be reloaded."`
	if err == nil || err.Error() != want {
		t.Errorf("ReportPlay = %v, want %s", err, want)
	}
	if _, err := c.StartWatch(context.Background(), "v1"); err == nil || !strings.Contains(err.Error(), `UNPLAYABLE, "Video unavailable"`) {
		t.Errorf("StartWatch = %v", err)
	}
	if got := requests(); len(got) != 2 {
		t.Errorf("%d requests, want the 2 of the player", len(got))
	}

	// A response that tells no status keeps to what's missing.
	c, _, _ = fakeYouTube(t, func(string) any { return map[string]any{} })
	c.SetCookies(session)
	if err := c.ReportPlay(context.Background(), "v1"); err == nil || err.Error() != "youtube music: no playback report for this track" {
		t.Errorf("without a status: %v", err)
	}
}

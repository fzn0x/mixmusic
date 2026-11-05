package youtube

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestReportPlayTimestamp asks for the player response with the signature
// timestamp of the player's code, as YouTube Music's player does: without
// one, YouTube has every video unavailable.
func TestReportPlayTimestamp(t *testing.T) {
	c, requests := watchYouTube(t, episodeTracking, allowAll)
	c.SetCookies(session)
	if err := c.ReportPlay(context.Background(), "v1"); err != nil {
		t.Fatal(err)
	}
	got := requests()
	if len(got) != 2 || got[0].path != "/youtubei/v1/player" {
		t.Fatalf("requests = %+v", got)
	}
	sts := node{got[0].body}.get("playbackContext", "contentPlaybackContext", "signatureTimestamp").number()
	if want := signatureTimestamp(time.Now()); sts != want {
		t.Errorf("signatureTimestamp = %d, want %d", sts, want)
	}
}

// TestSignatureTimestamp counts the days since 1970 to the day the
// player's code of a few days ago was built.
func TestSignatureTimestamp(t *testing.T) {
	// 2026-11-02 is the 20759th day since 1970; YouTube Music's player of
	// 2026-11-03 tells 20760.
	for _, now := range []time.Time{
		time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 5, 23, 59, 0, 0, time.UTC),
		time.Date(2026, 11, 6, 6, 0, 0, 0, time.FixedZone("WIB", 7*60*60)),
	} {
		if got := signatureTimestamp(now); got != 20759 {
			t.Errorf("signatureTimestamp(%v) = %d, want 20759", now, got)
		}
	}
}

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

// TestLivePlaybackTracking asks YouTube Music's player, signed out, as a
// play is reported, and finds the URLs of the reports in its answer. It
// runs with MIXMUSIC_LIVE=1.
func TestLivePlaybackTracking(t *testing.T) {
	if os.Getenv("MIXMUSIC_LIVE") == "" {
		t.Skip("set MIXMUSIC_LIVE=1 to test against YouTube Music")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := New()
	for _, id := range []string{"dQw4w9WgXcQ", "zjzwQ57PBxc"} {
		n, err := c.player(ctx, id)
		if err != nil {
			t.Fatalf("player of %s: %v", id, err)
		}
		if !n.get("playbackTracking").exists() {
			t.Errorf("%s: %v", id, noReport(n, "playback"))
			continue
		}
		for _, key := range []string{"videostatsPlaybackUrl", "videostatsWatchtimeUrl"} {
			if base := n.get("playbackTracking", key, "baseUrl").str(); base == "" {
				t.Errorf("%s: no %s", id, key)
			} else {
				t.Logf("%s: %s %.40s…", id, key, base)
			}
		}
	}
}

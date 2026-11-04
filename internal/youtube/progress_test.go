package youtube

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// episodeRow is a made-up row of a show's episodes, an hour and 40 minutes
// long, whose musicPlaybackProgressRenderer is progress: progress is
// JSON, or "" for a row without one.
func episodeRow(t *testing.T, progress string) node {
	t.Helper()
	row := `{"title": {"runs": [{"text": "Episode 12: Moss and the long winter"}]},
		"subtitle": {"runs": [{"text": "Oct 26"}]},
		"onTap": {"watchEndpoint": {"videoId": "ep12"}}`
	if progress != "" {
		row += `, "playbackProgress": {"musicPlaybackProgressRenderer": ` + progress + `}`
	}
	var v any
	if err := json.Unmarshal([]byte(row+"}"), &v); err != nil {
		t.Fatal(err)
	}
	return node{v}
}

// TestReadProgress keeps where YouTube Music says the user left an
// episode, from the rows it sends a signed-in session: a share of its
// length, or played once it says so. A row signed out tells nothing.
func TestReadProgress(t *testing.T) {
	at(t)
	length := `"durationText": {"runs": [{"text": " • "}, {"text": "1 hr 40 mins"}]}`
	played := `"playedText": {"runs": [{"text": " • "}, {"text": "Played"}]}`
	for _, c := range []struct {
		name, progress string
		told, played   bool
		resumeMs       int
	}{
		{"signed out", `{"playbackProgressPercentage": 0, ` + length + `}`, false, false, 0},
		{"no renderer", "", false, false, 0},
		{"not started", `{"playbackProgressPercentage": 0, "playbackProgressText": {"runs": [{"text": " • "}, {"text": "1 hr 40 mins"}]}, ` + length + `, ` + played + `}`, true, false, 0},
		{"partway", `{"playbackProgressPercentage": 37, "playbackProgressText": {"runs": [{"text": " • "}, {"text": "63 min left"}]}, ` + length + `, ` + played + `}`, true, false, 2220000},
		{"partway, without the text", `{"playbackProgressPercentage": 50, ` + length + `}`, true, false, 3000000},
		{"played", `{"playbackProgressPercentage": 100, "playbackProgressText": {"runs": [{"text": " • "}, {"text": "Played"}]}, ` + length + `, ` + played + `}`, true, true, 0},
		{"played, at 99%", `{"playbackProgressPercentage": 99, "playbackProgressText": {"runs": [{"text": " • "}, {"text": "Played"}]}, ` + length + `}`, true, true, 0},
		{"partway of a length not known", `{"playbackProgressPercentage": 37}`, false, false, 0},
	} {
		it, ok := parseMultiRowItem(episodeRow(t, c.progress), media.Ref{ID: "yt:MPSPx", Name: "Moss Radio"})
		if !ok || it.Track == nil || it.Track.Episode == nil {
			t.Fatalf("%s: no episode", c.name)
		}
		e := it.Track.Episode
		if told := !e.ToldAt.IsZero(); told != c.told || e.Played != c.played || e.ResumeMs != c.resumeMs {
			t.Errorf("%s: told %v, played %v, at %d ms; want %v, %v, %d", c.name, told, e.Played, e.ResumeMs, c.told, c.played, c.resumeMs)
		}
		if c.told && !e.ToldAt.Equal(time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)) {
			t.Errorf("%s: told at %v", c.name, e.ToldAt)
		}
	}
	// The episodes of a show's page, read signed out, tell no place.
	p, err := parsePodcast(fixture(t, "podcast"), "MPSPPLPNW_gerXa4Pc8S2qoUQc5e8Ir97RLuVW")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range p.Episodes {
		if !e.Episode.ToldAt.IsZero() || e.Episode.ResumeMs != 0 || e.Episode.Played {
			t.Errorf("%s signed out: %+v", e.ID, e.Episode)
		}
	}
}

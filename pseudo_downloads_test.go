package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/downloads"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
)

// TestPseudoLocaleDownloads shows the downloads in en-XA, paused, with a
// track that failed for a reason known and one of the error's own words,
// then a folder that can't be reached: only the error's words are
// English.
func TestPseudoLocaleDownloads(t *testing.T) {
	usePseudoLocale(t)
	dir := filepath.Join(t.TempDir(), "downloads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	kids := testTrack("yt:qqqqqqqqqqq", "Kids", "MGMT", "Oracular Spectacular", media.SourceYouTube)
	dark := testTrack("yt:rrrrrrrrrrr", "Time", "Pink Floyd", "The Dark Side of the Moon", media.SourceYouTube)
	type entry struct {
		Track  media.Track `json:"track"`
		Pins   []string    `json:"pins"`
		Seq    int         `json:"seq"`
		Failed string      `json:"failed"`
		Reason string      `json:"reason,omitempty"`
	}
	data, _ := json.Marshal(map[string]any{"version": 1, "seq": 2, "paused": true, "tracks": map[string]entry{
		kids.ID: {Track: kids, Pins: []string{"track"}, Seq: 1, Failed: "it needs yt-dlp", Reason: downloads.ReasonNeedsYTDLP},
		dark.ID: {Track: dark, Pins: []string{"track"}, Seq: 2, Failed: "YouTube answered HTTP 500 for its audio"},
	}})
	if err := os.WriteFile(filepath.Join(dir, "index.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	s := newTestShellOf(t, func(o *app.Options) { o.DownloadsDir = dir })
	tt := testerOf(t, s, 1280, 1400)
	tt.SetLocale("en-XA")
	show(s, tt, "/library/downloads")
	if !tt.HasText(downloadHold(s.kept.Status())) {
		t.Errorf("the page doesn't say why the downloads wait: %q", tt.Texts())
	}
	if !slices.ContainsFunc(tt.Texts(), func(txt string) bool { return strings.Contains(txt, locales.DownloadsReasonNeedsYTDLP()) }) {
		t.Errorf("the page doesn't say why Kids failed: %q", tt.Texts())
	}
	checkPseudo(t, "The downloads", tt.Texts(), "Kids", "MGMT", "Time", "Pink Floyd", "YouTube answered HTTP 500 for its audio")

	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s = newTestShellOf(t, func(o *app.Options) { o.DownloadsDir = filepath.Join(file, "downloads") })
	tt = tester(t, s)
	tt.SetLocale("en-XA")
	show(s, tt, "/library/downloads")
	checkPseudo(t, "The downloads unreachable", tt.Texts())
	show(s, tt, "/settings/playback")
	checkPseudo(t, "Settings › Playback, of downloads unreachable", tt.Texts())
}

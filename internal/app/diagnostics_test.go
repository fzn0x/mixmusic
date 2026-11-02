package app

import (
	"context"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/tools"
)

// The report of a problem names the version and the setup, in the fields
// of the bug report template, and nothing of the user's accounts.
func TestReportURL(t *testing.T) {
	dir := t.TempDir()
	s := settings.Default()
	s.Theme = "aurora"
	s.EnabledPlugins = []string{"sleep-timer"}
	s.SpotifyClientID = "client-id-of-the-user"
	file, err := store.Open(filepath.Join(dir, "settings.json"), s)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{opts: Options{Version: "9.9.9"}, settings: file, tools: tools.New(filepath.Join(dir, "bin"))}
	t.Setenv("PATH", "")
	raw := (&System{a}).ReportURL(context.Background())
	u, err := url.Parse(raw)
	if err != nil || !strings.HasPrefix(raw, issuesURL+"?") {
		t.Fatalf("ReportURL = %q, %v", raw, err)
	}
	q := u.Query()
	if q.Get("template") != "bug_report.yml" || q.Get("version") == "" {
		t.Errorf("template and version = %q, %q", q.Get("template"), q.Get("version"))
	}
	more := q.Get("more")
	// The first line tells how MixMusic was installed: a Flatpak, a .deb…
	first, _, _ := strings.Cut(more, "\n")
	if want := "MixMusic 9.9.9 on " + runtime.GOOS + " " + runtime.GOARCH + ", installed as " + installKind(); first != want {
		t.Errorf("the report starts %q, want %q", first, want)
	}
	for _, want := range []string{"Theme: aurora", "Plugins: sleep-timer", "yt-dlp: not installed", "mixmusic.log"} {
		if !strings.Contains(more, want) {
			t.Errorf("the report lacks %q:\n%s", want, more)
		}
	}
	if strings.Contains(raw, "client-id-of-the-user") {
		t.Error("the report holds the user's Spotify app")
	}
}

func TestLastCrash(t *testing.T) {
	a := &App{lastCrash: filepath.Join(t.TempDir(), "crash-20261019-210000.log")}
	s := &System{a}
	if got := s.LastCrash(); got != "crash-20261019-210000.log" {
		t.Errorf("LastCrash = %q", got)
	}
	s.DismissCrash()
	if got := s.LastCrash(); got != "" {
		t.Errorf("after DismissCrash, LastCrash = %q", got)
	}
}

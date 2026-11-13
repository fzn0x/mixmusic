package main

import (
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/tools"
)

// Flathub's build updates through the software center: About names it, and
// tells where a new version comes from, without a download.
func TestFlathubUpdates(t *testing.T) {
	if got, _ := buildName("flathub"); got != "Flathub" {
		t.Errorf("About calls Flathub's build %q", got)
	}
	u := app.UpdateStatus{Kind: "flathub", AutomaticChecks: true}
	if got := newVersionDoes(u); !strings.Contains(got, "Flathub") || !strings.Contains(got, "software center") || strings.Contains(got, "download") {
		t.Errorf("the checks say a new version %q", got)
	}
	got := replacing("flathub", "")
	for _, want := range []string{"software center", "Flathub", "flatpak update", "Your library and settings stay."} {
		if !strings.Contains(got, want) {
			t.Errorf("a new version of Flathub's build says %q, without %q", got, want)
		}
	}
	if strings.Contains(got, ".flatpak") || strings.Contains(got, "flatpak install") {
		t.Errorf("Flathub's build is offered a bundle: %q", got)
	}
	// The release's own .flatpak still offers its download.
	if got := newVersionDoes(app.UpdateStatus{Kind: "flatpak"}); got != " A new version offers its download." {
		t.Errorf("the release's .flatpak says a new version %q", got)
	}
}

// The yt-dlp of Flathub's build updates with MixMusic: Settings says so,
// and offers no download of another.
func TestBundledYTDLPRow(t *testing.T) {
	s := newTestShell(t)
	y := tools.Tool{Found: true, Name: "yt-dlp", Path: "/app/bin/yt-dlp", Version: "2026.10.26", Bundled: true}
	s.loads["tools"] = &loaded[tools.Status]{done: true, value: tools.Status{YTDLP: y}}
	tt := testerOf(t, s, 1280, 2400)
	show(s, tt, "/settings/playback")
	if want := "yt-dlp 2026.10.26 at /app/bin/yt-dlp, which comes with MixMusic and updates with it"; !tt.HasText(want) {
		t.Errorf("Settings doesn't say %q: %q", want, tt.Texts())
	}
	for _, label := range []string{"install yt-dlp", "update yt-dlp", "use MixMusic's yt-dlp"} {
		if tt.HasText(label) {
			t.Errorf("Settings offers %q for the bundled yt-dlp", label)
		}
	}
}

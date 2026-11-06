package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// An Arm64 Flatpak is offered the release's Arm64 bundle, and an x64 one
// the x64 bundle, now that each release has both (#294).
func TestLatestReleaseArm64Flatpak(t *testing.T) {
	const body = `{
		"tag_name": "v0.6.0",
		"html_url": "https://github.com/fzn0x/mixmusic/releases/tag/v0.6.0",
		"assets": [
			{"name": "mixmusic-0.6.0-linux-amd64.flatpak", "browser_download_url": "https://example.com/amd64.flatpak"},
			{"name": "mixmusic-0.6.0-linux-arm64.flatpak", "browser_download_url": "https://example.com/arm64.flatpak"},
			{"name": "mixmusic-0.6.0-linux-arm64.AppImage", "browser_download_url": "https://example.com/arm64.AppImage"}
		]
	}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	for arch, want := range map[string]string{
		"arm64": "https://example.com/arm64.flatpak",
		"amd64": "https://example.com/amd64.flatpak",
	} {
		r, err := latestRelease(context.Background(), srv.Client(), srv.URL, "flatpak", arch, "0.5.9")
		if err != nil {
			t.Fatal(err)
		}
		if !r.Newer || r.Download != want || r.File != "mixmusic-0.6.0-linux-"+arch+".flatpak" {
			t.Errorf("latestRelease of an %s Flatpak = %+v, want %s", arch, r, want)
		}
	}
}

// The release notes link each architecture's bundle by the name its
// Flatpak asks for.
func TestReleaseNotesLinkEachFlatpak(t *testing.T) {
	b, err := os.ReadFile("../../.github/release-notes.md")
	if err != nil {
		t.Fatal(err)
	}
	notes := strings.NewReplacer("{repo}", "fzn0x/mixmusic", "{tag}", "v1.2.3", "{version}", "1.2.3").Replace(string(b))
	for _, arch := range []string{"amd64", "arm64"} {
		file := releaseFile("flatpak", "1.2.3", arch)
		if !strings.Contains(notes, "(https://github.com/fzn0x/mixmusic/releases/download/v1.2.3/"+file+")") {
			t.Errorf(".github/release-notes.md links to no %s", file)
		}
	}
}

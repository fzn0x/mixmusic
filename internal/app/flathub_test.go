package app

import (
	"slices"
	"testing"
)

// Flathub's build is a Flatpak that Flathub updates: told apart from the
// release's own .flatpak by its marker, it offers no download.
func TestKindOfFlathub(t *testing.T) {
	exists := func(files ...string) func(string) bool {
		return func(p string) bool { return slices.Contains(files, p) }
	}
	if got := kindOf("linux", "/app/lib/mixmusic", false, exists("/.flatpak-info", flathubMarker)); got != "flathub" {
		t.Errorf("Flathub's build: kindOf = %q, want flathub", got)
	}
	if got := kindOf("linux", "/app/lib/mixmusic", false, exists("/.flatpak-info")); got != "flatpak" {
		t.Errorf("the release's .flatpak: kindOf = %q, want flatpak", got)
	}
	for _, arch := range []string{"amd64", "arm64"} {
		if got := releaseFile("flathub", "0.6.0", arch); got != "" {
			t.Errorf("Flathub's build on %s is offered %q, want no download", arch, got)
		}
	}
}

// Only Flathub's build runs the yt-dlp of its manifest.
func TestBundledYTDLPOutsideFlathub(t *testing.T) {
	if got := bundledYTDLP(); got != "" {
		t.Errorf("a test build runs the bundled yt-dlp %q", got)
	}
}

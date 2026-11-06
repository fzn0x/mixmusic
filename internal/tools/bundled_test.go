package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// bundledManager returns a manager whose yt-dlp comes with MixMusic, as in
// Flathub's build, with a yt-dlp MixMusic installed before beside it, and
// a release server that fails the test when it's asked.
func bundledManager(t *testing.T) (m *Manager, bundled, managed string) {
	t.Helper()
	t.Setenv("PATH", "")
	noToolDirs(t)
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("MixMusic asked %s with a yt-dlp of its package", r.URL)
	}))
	t.Cleanup(srv.Close)
	savedRelease, savedLatest := releaseURL, latestURL
	releaseURL, latestURL = srv.URL+"/download/", srv.URL+"/latest"
	t.Cleanup(func() { releaseURL, latestURL = savedRelease, savedLatest })

	m = New(t.TempDir())
	bundled = filepath.Join(t.TempDir(), exe("yt-dlp"))
	m.Bundled = bundled
	for _, p := range []string{bundled, m.managedPath()} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return m, bundled, m.managedPath()
}

// The yt-dlp of MixMusic's package runs in place of one MixMusic
// installed before, as a copy of the release's Flatpak left it, unless the
// user chose another.
func TestBundledYTDLPRuns(t *testing.T) {
	m, bundled, managed := bundledManager(t)
	if got, err := m.YTDLP(); err != nil || got != bundled {
		t.Errorf("YTDLP() = %q, %v; want the bundled %s", got, err, bundled)
	}
	chosen := filepath.Join(t.TempDir(), "custom-yt-dlp")
	if err := os.WriteFile(chosen, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	m.Configure(chosen, "")
	if got, err := m.YTDLP(); err != nil || got != chosen {
		t.Errorf("YTDLP() = %q, %v; want the chosen path", got, err)
	}
	// Without its file, MixMusic finds yt-dlp as it does elsewhere.
	m.Configure("", "")
	if err := os.Remove(bundled); err != nil {
		t.Fatal(err)
	}
	if got, err := m.YTDLP(); err != nil || got != managed {
		t.Errorf("YTDLP() = %q, %v; want the managed copy without the bundled one", got, err)
	}
}

// MixMusic downloads no yt-dlp beside the one of its package: its package
// updates that one.
func TestBundledYTDLPDownloadsNothing(t *testing.T) {
	m, bundled, _ := bundledManager(t)
	ctx := context.Background()
	if updated, err := m.UpdateYTDLP(ctx); updated || err != nil {
		t.Errorf("UpdateYTDLP() = %v, %v; want nothing done", updated, err)
	}
	if err := m.InstallYTDLP(ctx, nil); err != nil {
		t.Errorf("InstallYTDLP() = %v; want the bundled yt-dlp kept", err)
	}
	got := m.ytdlpTool(ctx, bundled, "2026.10.26")
	if !got.Bundled || got.Managed || got.Latest != "" || got.UpdateCommand != "" {
		t.Errorf("the bundled yt-dlp: %+v", got)
	}
}

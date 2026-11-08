package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/fzn0x/mixmusic/internal/store"
)

func TestNewerVersion(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"0.3.0", "0.2.0", true},
		{"0.2.0", "0.2.0", false},
		{"0.2.0", "0.3.0", false},
		{"0.10.0", "0.9.1", true},
		{"v1.0.0", "0.99.99", true},
		{"0.2.1", "0.2", true},
		{"0.2", "0.2.0", false},
		{"0.3.0", "0.3.0-beta.1", true},
		{"0.3.0-beta.1", "0.3.0", false},
		{"0.3.0-beta.2", "0.3.0-beta.1", true},
		{"0.3.0-beta.1", "0.2.0", true},
	} {
		if got := newerVersion(c.a, c.b); got != c.want {
			t.Errorf("newerVersion(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestKindOf(t *testing.T) {
	const installer = `C:\Users\me\AppData\Local\Programs\MixMusic`
	for _, c := range []struct {
		name     string
		goos     string
		dir      string
		appImage bool
		files    []string
		want     string
	}{
		{"installer", "windows", installer, false, []string{filepath.Join(installer, "Uninstall.exe")}, "installer"},
		{"portable zip", "windows", `D:\Apps\MixMusic`, false, nil, "portable"},
		{"app bundle", "darwin", "/Applications/MixMusic.app/Contents/MacOS", false, nil, "app"},
		{"deb", "linux", "/opt/mixmusic", false, []string{"/var/lib/dpkg/info/mixmusic.list"}, "deb"},
		{"rpm", "linux", "/opt/mixmusic", false, nil, "rpm"},
		{"AUR", "linux", "/opt/mixmusic", false, []string{"/var/lib/pacman/local"}, "pacman"},
		{"AppImage", "linux", "/tmp/.mount_MixMusXYZ/usr/bin", true, nil, "appimage"},
		{"Flatpak", "linux", "/app/lib/mixmusic", false, []string{"/.flatpak-info"}, "flatpak"},
		{"install.sh", "linux", "/home/me/.local/share/mixmusic", false, nil, "archive"},
	} {
		exists := func(p string) bool {
			for _, f := range c.files {
				if f == p {
					return true
				}
			}
			return false
		}
		if got := kindOf(c.goos, c.dir, c.appImage, exists); got != c.want {
			t.Errorf("%s: kindOf = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestReleaseFile(t *testing.T) {
	for kind, want := range map[string]string{
		"installer":   "mixmusic-0.3.0-windows-amd64-setup.exe",
		"portable":    "mixmusic-0.3.0-windows-amd64-portable.zip",
		"app":         "mixmusic-0.3.0-macos-universal.dmg",
		"archive":     "mixmusic-0.3.0-linux-amd64.tar.gz",
		"deb":         "mixmusic-0.3.0-linux-amd64.deb",
		"rpm":         "mixmusic-0.3.0-linux-amd64.rpm",
		"appimage":    "mixmusic-0.3.0-linux-amd64.AppImage",
		"flatpak":     "mixmusic-0.3.0-linux-amd64.flatpak",
		"pacman":      "",
		"development": "",
	} {
		if got := releaseFile(kind, "0.3.0", "amd64"); got != want {
			t.Errorf("releaseFile(%q) = %q, want %q", kind, got, want)
		}
	}
}

func TestLatestRelease(t *testing.T) {
	body := strings.ReplaceAll(`{
		"tag_name": "v0.3.0",
		"html_url": "https://github.com/fzn0x/mixmusic/releases/tag/v0.3.0",
		"published_at": "2026-10-21T10:00:00Z",
		"body": "Playlists of your own, and an [About page](https://example.com) that shows the newest version.\r\n\r\n### Added\r\n\r\n- Make playlists.",
		"assets": [
			{"name": "mixmusic-0.3.0-linux-amd64.deb", "browser_download_url": "https://example.com/amd64.deb"},
			{"name": "mixmusic-0.3.0-linux-arm64.deb", "browser_download_url": "https://example.com/arm64.deb"},
			{"name": "mixmusic-0.3.0-linux-amd64.flatpak", "browser_download_url": "https://example.com/amd64.flatpak"},
			{"name": "update-GOOS-arm64.json", "browser_download_url": "https://example.com/v0.3.0/update-arm64.json"},
			{"name": "update-GOOS-amd64.json", "browser_download_url": "https://example.com/v0.3.0/update-amd64.json"},
			{"name": "SHA256SUMS", "browser_download_url": "https://example.com/SHA256SUMS"}
		]
	}`, "GOOS", runtime.GOOS)
	var agent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent = r.Header.Get("User-Agent")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	r, err := latestRelease(context.Background(), srv.Client(), srv.URL, "deb", "arm64", "0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	want := Release{
		Version:  "0.3.0",
		Newer:    true,
		Date:     "2026-10-21T10:00:00Z",
		Notes:    "Playlists of your own, and an About page that shows the newest version.",
		Page:     "https://github.com/fzn0x/mixmusic/releases/tag/v0.3.0",
		Download: "https://example.com/arm64.deb",
		File:     "mixmusic-0.3.0-linux-arm64.deb",
		manifest: "https://example.com/v0.3.0/update-arm64.json",
	}
	if r != want {
		t.Errorf("latestRelease = %+v, want %+v", r, want)
	}
	if agent != "MixMusic/0.2.0" {
		t.Errorf("User-Agent = %q", agent)
	}

	// The release has no file for this copy: only its page.
	r, err = latestRelease(context.Background(), srv.Client(), srv.URL, "rpm", "amd64", "0.3.0")
	if err != nil {
		t.Fatal(err)
	}
	if r.Newer || r.Download != "" || r.File != "" {
		t.Errorf("latestRelease of an rpm, up to date = %+v", r)
	}

	// A Flatpak is offered the next bundle of its architecture.
	r, err = latestRelease(context.Background(), srv.Client(), srv.URL, "flatpak", "amd64", "0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if r.Download != "https://example.com/amd64.flatpak" || r.File != "mixmusic-0.3.0-linux-amd64.flatpak" {
		t.Errorf("latestRelease of a Flatpak = %+v", r)
	}
	r, err = latestRelease(context.Background(), srv.Client(), srv.URL, "flatpak", "arm64", "0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Newer || r.Download != "" || r.File != "" {
		t.Errorf("latestRelease of an Arm64 Flatpak, which has no bundle = %+v", r)
	}
}

// TestTaggedManifest: when GitHub's latest/download address fails, the
// update is read from the release's own address (#412).
func TestTaggedManifest(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if agent := r.Header.Get("User-Agent"); agent != "mygo-updater" {
			t.Errorf("User-Agent = %q", agent)
		}
		switch r.URL.Path {
		case "/v0.3.0/update.json":
			w.Write([]byte(`{"version":"0.3.0"}`))
		case "/v0.3.0/app.tar.gz":
			w.Write([]byte("archive"))
		default:
			http.Error(w, "", http.StatusGatewayTimeout)
		}
	}))
	defer srv.Close()
	client := updateClient
	updateClient = srv.Client()
	defer func() { updateClient = client }()

	var src mygo.UpdateSource = taggedManifest(srv.URL + "/v0.3.0/update.json")
	read := func(rc io.ReadCloser, err error) string {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		defer rc.Close()
		b, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	ctx := context.Background()
	if got := read(src.Manifest(ctx, false)); got != `{"version":"0.3.0"}` {
		t.Errorf("Manifest = %q", got)
	}
	if got := read(src.Open(ctx, srv.URL+"/v0.3.0/app.tar.gz")); got != "archive" {
		t.Errorf("Open = %q", got)
	}
	if _, err := src.Open(ctx, srv.URL+"/latest/download/app.tar.gz"); err == nil || !strings.Contains(err.Error(), "504 Gateway Timeout") {
		t.Errorf("Open of a file GitHub doesn't send: err = %v, want its status", err)
	}
	if _, err := src.Open(ctx, "http://example.com/app.tar.gz"); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Errorf("Open over HTTP: err = %v, want a refusal", err)
	}
}

func TestLatestReleaseRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"API rate limit exceeded"}`, http.StatusForbidden)
	}))
	defer srv.Close()
	_, err := latestRelease(context.Background(), srv.Client(), srv.URL, "installer", "amd64", "0.2.0")
	if err == nil || !strings.Contains(err.Error(), "try again in an hour") {
		t.Errorf("err = %v, want one that says to try again later", err)
	}
}

func TestSummary(t *testing.T) {
	for _, c := range []struct{ notes, want string }{
		{"MixMusic tells you when a yt-dlp you installed\nis out of date, and offers `its own`.\n\n### Added\n\n- More.", "MixMusic tells you when a yt-dlp you installed is out of date, and offers its own."},
		{"**Discord** shows what plays, on [Windows](https://example.com) too.", "Discord shows what plays, on Windows too."},
		{"### Fixed\n\n- A crash.", ""},
		{"", ""},
	} {
		if got := summary(c.notes); got != c.want {
			t.Errorf("summary(%q) = %q, want %q", c.notes, got, c.want)
		}
	}
}

func TestDecideUpdate(t *testing.T) {
	on := true
	newer := Release{Version: "0.5.0", Newer: true}
	for _, c := range []struct {
		name       string
		r          Release
		p          updatePrefs
		installed  string
		canInstall bool
		want       updateAction
	}{
		{"up to date", Release{Version: "0.4.0"}, updatePrefs{}, "", true, updateNothing},
		{"asks", newer, updatePrefs{AutomaticChecks: &on}, "", true, updateOffer},
		{"installs without asking", newer, updatePrefs{AutomaticDownloads: true}, "", true, updateInstall},
		{"offers a copy that cannot install", newer, updatePrefs{AutomaticDownloads: true}, "", false, updateOffer},
		{"installed already", newer, updatePrefs{}, "0.5.0", true, updateNothing},
		{"newer than the one installed", newer, updatePrefs{}, "0.4.1", true, updateOffer},
		{"skipped", newer, updatePrefs{SkippedVersion: "0.5.0"}, "", true, updateNothing},
		{"a version after the one skipped", newer, updatePrefs{SkippedVersion: "0.4.1"}, "", true, updateOffer},
	} {
		if got := decideUpdate(c.r, c.p, c.installed, c.canInstall); got != c.want {
			t.Errorf("%s: decideUpdate = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestUpdatePrefsOfTheUpdateWindow(t *testing.T) {
	// updater.json as MyGo's update window wrote it.
	path := filepath.Join(t.TempDir(), "updater.json")
	data := `{"automaticChecks":false,"automaticDownloads":true,"skippedVersion":"0.3.1","lastCheck":"2026-10-20T04:04:16.7979378+07:00"}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := store.Open(path, updatePrefs{})
	if err != nil {
		t.Fatal(err)
	}
	p := f.Get()
	if p.checks() || !p.AutomaticDownloads || p.SkippedVersion != "0.3.1" || p.LastCheck.IsZero() {
		t.Errorf("prefs = %+v", p)
	}
	// Until the user chose, MixMusic checks.
	if !(updatePrefs{}).checks() {
		t.Error("new prefs do not check")
	}
}

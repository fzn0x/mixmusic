package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/tools"
)

// flatpakFolder makes the folder where a browser's Flatpak keeps its
// profiles in home, as the browser does, and returns it.
func flatpakFolder(t *testing.T, home, app, dir string) string {
	t.Helper()
	path := filepath.Join(home, ".var", "app", app, filepath.FromSlash(dir))
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestFlatpakBrowsers lists the browsers installed as Flatpaks apart from
// those installed as the system has them, once their folders are there.
func TestFlatpakBrowsers(t *testing.T) {
	home := t.TempDir()
	for _, f := range browserFlatpaks {
		flatpakFolder(t, home, f.app, f.dirs[0])
	}
	var got []string
	for _, b := range youtubeBrowsers("linux", home) {
		got = append(got, b.ID+" "+b.Name)
	}
	want := []string{
		"firefox Firefox", "firefox-flatpak Firefox (Flatpak)",
		"chrome Chrome", "chrome-flatpak Chrome (Flatpak)",
		"chromium Chromium", "chromium-flatpak Chromium (Flatpak)",
		"edge Edge", "edge-flatpak Edge (Flatpak)",
		"brave Brave", "brave-flatpak Brave (Flatpak)",
		"opera Opera", "vivaldi Vivaldi", "helium Helium", "zen Zen",
	}
	if !slices.Equal(got, want) {
		t.Errorf("on Linux, with each browser's Flatpak: %q", got)
	}
	// Only the Flatpaks installed show.
	if err := os.RemoveAll(filepath.Join(home, ".var", "app", "com.google.Chrome")); err != nil {
		t.Fatal(err)
	}
	ids := func(goos string) []string {
		var ids []string
		for _, b := range youtubeBrowsers(goos, home) {
			ids = append(ids, b.ID)
		}
		return ids
	}
	if l := ids("linux"); slices.Contains(l, "chrome-flatpak") || !slices.Contains(l, "chrome") || !slices.Contains(l, "brave-flatpak") {
		t.Errorf("without Chrome's Flatpak: %q", l)
	}
	// Flatpaks are of Linux.
	for _, goos := range []string{"windows", "darwin"} {
		if l := ids(goos); slices.ContainsFunc(l, func(id string) bool { return strings.HasSuffix(id, "-flatpak") }) {
			t.Errorf("on %s: %q", goos, l)
		}
	}
}

// flatpakBrowser returns the browser of id installed as a Flatpak.
func flatpakBrowser(t *testing.T, id string) cookieBrowser {
	t.Helper()
	for _, b := range browsersOn("linux") {
		if b.id == id+"-flatpak" {
			return b
		}
	}
	t.Fatalf("no Flatpak of %s", id)
	return cookieBrowser{}
}

// TestFlatpakBrowserSpec has yt-dlp read each Flatpak's profiles in its
// folder, under a home of the test's.
func TestFlatpakBrowserSpec(t *testing.T) {
	home := t.TempDir()
	getenv := func(string) string { return "" }
	for _, c := range []struct{ browser, app, dir, engine, after string }{
		{"firefox", "org.mozilla.firefox", ".mozilla/firefox", "firefox", "::none"},
		{"chrome", "com.google.Chrome", "config/google-chrome", "chrome", ""},
		{"chromium", "org.chromium.Chromium", "config/chromium", "chromium", ""},
		{"edge", "com.microsoft.Edge", "config/microsoft-edge", "edge", ""},
		{"brave", "com.brave.Browser", "config/BraveSoftware/Brave-Browser", "brave", ""},
	} {
		b := flatpakBrowser(t, c.browser)
		if _, err := b.spec("", "linux", home, getenv); err == nil || !strings.Contains(err.Error(), "profiles of "+b.name) {
			t.Errorf("%s without its folder: %v", b.name, err)
		}
		dir := flatpakFolder(t, home, c.app, c.dir)
		if got, err := b.spec("", "linux", home, getenv); err != nil || got != c.engine+":"+dir+c.after {
			t.Errorf("%s = %q, %v; want its folder %s", b.name, got, err, dir)
		}
		// A profile by its name is one of the folder's.
		if got, err := b.spec(" Profile 1 ", "linux", home, getenv); err != nil || got != c.engine+":"+filepath.Join(dir, "Profile 1")+c.after {
			t.Errorf("%s's Profile 1 = %q, %v", b.name, got, err)
		}
		if got, err := b.spec("/elsewhere/profile", "linux", home, getenv); err != nil || got != c.engine+":/elsewhere/profile"+c.after {
			t.Errorf("%s in a folder given = %q, %v", b.name, got, err)
		}
	}

	// A new Firefox keeps its profiles in its XDG_CONFIG_HOME, unless
	// ~/.mozilla has some.
	other := t.TempDir()
	firefox := flatpakBrowser(t, "firefox")
	xdg := flatpakFolder(t, other, "org.mozilla.firefox", "config/mozilla/firefox")
	if got, err := firefox.spec("", "linux", other, getenv); err != nil || got != "firefox:"+xdg+"::none" {
		t.Errorf("a new Firefox = %q, %v", got, err)
	}
	legacy := flatpakFolder(t, other, "org.mozilla.firefox", ".mozilla/firefox")
	if got, err := firefox.spec("", "linux", other, getenv); err != nil || got != "firefox:"+legacy+"::none" {
		t.Errorf("a Firefox of ~/.mozilla = %q, %v", got, err)
	}

	// Its problems name it.
	if err := flatpakBrowser(t, "chrome").problem(tools.ErrNoBrowserCookies, false); !strings.Contains(err.Error(), "of Chrome (Flatpak) in its folder") {
		t.Errorf("no cookies: %v", err)
	}
	if err := flatpakBrowser(t, "chrome").problem(tools.ErrCookiesSealed, false); !strings.Contains(err.Error(), "paste a cookie header") {
		t.Errorf("the key unread: %v", err)
	}
}

// TestImportFlatpakCookies imports the cookies of a browser installed as a
// Flatpak, from its folder.
func TestImportFlatpakCookies(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Flatpaks are of Linux")
	}
	jar, err := os.ReadFile(filepath.Join("testdata", "cookies.txt"))
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := flatpakFolder(t, home, "com.google.Chrome", "config/google-chrome")
	a, _ := cookieApp(t, signedIn(true))
	browsers := fakeExport(t, string(jar), false, nil)
	listed := (&Accounts{a}).YouTubeBrowsers()
	if !slices.Contains(listed, YouTubeBrowser{"chrome-flatpak", "Chrome (Flatpak)"}) || !slices.Contains(listed, YouTubeBrowser{"chrome", "Chrome"}) {
		t.Errorf("listed %v", listed)
	}
	status, err := (&Accounts{a}).YouTubeImportCookies(context.Background(), "chrome-flatpak", "")
	if err != nil {
		t.Fatal(err)
	}
	if !status.YouTube.SignedIn {
		t.Errorf("status = %+v", status.YouTube)
	}
	if !slices.Equal(*browsers, []string{"chrome:" + dir}) {
		t.Errorf("yt-dlp read %q", *browsers)
	}
}

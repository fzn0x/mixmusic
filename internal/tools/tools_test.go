package tools

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
)

func TestAsset(t *testing.T) {
	for platform, want := range map[[2]string]string{
		{"windows", "amd64"}: "yt-dlp.exe",
		{"linux", "amd64"}:   "yt-dlp_linux",
		{"linux", "arm64"}:   "yt-dlp_linux_aarch64",
	} {
		got, err := asset(platform[0], platform[1])
		if err != nil || got != want {
			t.Errorf("asset(%s) = %q, %v", platform, got, err)
		}
	}
	if _, err := asset("plan9", "amd64"); err == nil {
		t.Error("plan9 has no yt-dlp build")
	}
}

func TestChecksum(t *testing.T) {
	sums := []byte("abc123  yt-dlp\nDEF456  yt-dlp.exe\n789  yt-dlp_linux\n")
	got, err := checksum(sums, "yt-dlp.exe")
	if err != nil || got != "def456" {
		t.Errorf("checksum = %q, %v", got, err)
	}
	if _, err := checksum(sums, "yt-dlp_macos"); err == nil {
		t.Error("a missing file has no checksum")
	}
}

func TestYTDLPPrefersChosenThenManaged(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	t.Setenv("PATH", "")
	if _, err := m.YTDLP(); !errors.Is(err, ErrNoYTDLP) {
		t.Fatalf("YTDLP() error = %v, want ErrNoYTDLP", err)
	}
	managed := filepath.Join(dir, exe("yt-dlp"))
	if err := os.WriteFile(managed, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := m.YTDLP(); err != nil || got != managed {
		t.Errorf("YTDLP() = %q, %v; want the managed copy", got, err)
	}
	chosen := filepath.Join(dir, "custom-yt-dlp")
	if err := os.WriteFile(chosen, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	m.Configure(chosen, "")
	if got, err := m.YTDLP(); err != nil || got != chosen {
		t.Errorf("YTDLP() = %q, %v; want the chosen path", got, err)
	}
	gone := filepath.Join(dir, "gone-yt-dlp")
	m.Configure(gone, "")
	if _, err := m.YTDLP(); !errors.Is(err, ErrNoYTDLP) || err.Error() != "yt-dlp not found at "+gone {
		t.Errorf("YTDLP() error = %v, want ErrNoYTDLP saying where", err)
	}
}

// noToolDirs keeps a test to the PATH and the folders it makes.
func noToolDirs(t *testing.T, dirs ...string) {
	t.Helper()
	saved := toolDirs
	toolDirs = func() []string { return dirs }
	t.Cleanup(func() { toolDirs = saved })
}

func TestJSRuntimeFromDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", "")
	noToolDirs(t)
	m := New(dir)
	if _, _, ok := m.JSRuntime(); ok {
		t.Fatal("found a runtime in an empty PATH")
	}
	name := "deno"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, _, ok := m.JSRuntime(); !ok || got != "deno" {
		t.Errorf("JSRuntime() = %q, %v", got, ok)
	}
	args := m.YTDLPArgs("")
	if args[len(args)-2] != "--js-runtimes" {
		t.Errorf("args = %q", args)
	}
}

// An app opened from the desktop has a short PATH: tools are found in the
// folders installers put them in too.
func TestToolsOutsidePath(t *testing.T) {
	t.Setenv("PATH", "")
	brew := t.TempDir()
	noToolDirs(t, filepath.Join(t.TempDir(), "missing"), brew)
	m := New(t.TempDir())
	if _, err := m.YTDLP(); err == nil {
		t.Fatal("found yt-dlp in empty folders")
	}
	for _, name := range []string{"yt-dlp", "node"} {
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		if err := os.WriteFile(filepath.Join(brew, name), []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := m.YTDLP(); err != nil || filepath.Dir(got) != brew {
		t.Errorf("YTDLP() = %q, %v; want it in %s", got, err, brew)
	}
	if name, got, ok := m.JSRuntime(); !ok || name != "node" || filepath.Dir(got) != brew {
		t.Errorf("JSRuntime() = %q, %q, %v", name, got, ok)
	}
}

func TestLatestYTDLP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("method = %s", r.Method)
		}
		http.Redirect(w, r, "/yt-dlp/yt-dlp/releases/tag/2026.09.01", http.StatusFound)
	}))
	defer srv.Close()
	saved := latestURL
	latestURL = srv.URL + "/yt-dlp/yt-dlp/releases/latest"
	defer func() { latestURL = saved }()
	m := New(t.TempDir())
	if v, err := m.LatestYTDLP(context.Background()); err != nil || v != "2026.09.01" {
		t.Errorf("LatestYTDLP() = %q, %v", v, err)
	}
	// Nothing MixMusic installed: nothing to update.
	if updated, err := m.UpdateYTDLP(context.Background()); updated || err != nil {
		t.Errorf("UpdateYTDLP() = %v, %v without a managed yt-dlp", updated, err)
	}
}

func TestOlder(t *testing.T) {
	for _, c := range []struct {
		v, latest string
		want      bool
	}{
		{"2025.01.15", "2026.09.01", true},
		{"2026.08.19", "2026.09.01", true},
		{"2026.09.01", "2026.09.01", false},
		{"2026.09.01.232811", "2026.09.01", false}, // a nightly after the release
		{"2026.08.31.232811", "2026.09.01", true},
		{"2026.10.02", "2026.09.01", false},
		{"", "2026.09.01", false},
		{"2025.01.15", "", false}, // the latest is not known
		{"yt-dlp 2025.01.15", "2026.09.01", false},
	} {
		if got := older(c.v, c.latest); got != c.want {
			t.Errorf("older(%q, %q) = %v, want %v", c.v, c.latest, got, c.want)
		}
	}
}

func TestUpdateCommand(t *testing.T) {
	for path, want := range map[string]string{
		`C:\Users\a\AppData\Local\Microsoft\WinGet\Links\yt-dlp.exe`:            "winget upgrade yt-dlp",
		`C:\Users\a\scoop\shims\yt-dlp.exe`:                                     "scoop update yt-dlp",
		`C:\ProgramData\chocolatey\bin\yt-dlp.exe`:                              "choco upgrade yt-dlp",
		`C:\Users\a\AppData\Local\Programs\Python\Python312\Scripts\yt-dlp.exe`: "python -m pip install -U yt-dlp",
		"/opt/homebrew/bin/yt-dlp":                                              "brew upgrade yt-dlp",
		"/usr/local/Cellar/yt-dlp/2025.01.15/bin/yt-dlp":                        "brew upgrade yt-dlp",
		"/home/linuxbrew/.linuxbrew/bin/yt-dlp":                                 "brew upgrade yt-dlp",
		"/home/a/.local/share/pipx/venvs/yt-dlp/bin/yt-dlp":                     "pipx upgrade yt-dlp",
		"/snap/bin/yt-dlp":                "sudo snap refresh yt-dlp",
		"/usr/bin/yt-dlp":                 "",
		`C:\Users\a\Downloads\yt-dlp.exe`: "",
	} {
		if got := updateCommand(path); got != want {
			t.Errorf("updateCommand(%s) = %q, want %q", path, got, want)
		}
	}
}

// fakeLatest has GitHub answer that the latest yt-dlp is *latest, or fail
// while it is empty, and counts the requests.
func fakeLatest(t *testing.T, latest *string) *atomic.Int32 {
	t.Helper()
	var asked atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.Add(1)
		if *latest == "" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		http.Redirect(w, r, "/yt-dlp/yt-dlp/releases/tag/"+*latest, http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	saved := latestURL
	latestURL = srv.URL + "/yt-dlp/yt-dlp/releases/latest"
	t.Cleanup(func() { latestURL = saved })
	return &asked
}

func TestYTDLPToolTellsANewerRelease(t *testing.T) {
	latest := "2026.09.01"
	asked := fakeLatest(t, &latest)
	ctx := context.Background()
	m := New(t.TempDir())

	got := m.ytdlpTool(ctx, "/opt/homebrew/bin/yt-dlp", "2025.01.15")
	if got.Latest != "2026.09.01" || got.UpdateCommand != "brew upgrade yt-dlp" || got.Managed {
		t.Errorf("an old yt-dlp of the user's: %+v", got)
	}
	if got := m.ytdlpTool(ctx, "/opt/homebrew/bin/yt-dlp", "2026.09.01"); got.Latest != "" || got.UpdateCommand != "" {
		t.Errorf("a current yt-dlp: %+v", got)
	}
	if got := m.ytdlpTool(ctx, m.managedPath(), "2025.01.15"); got.Latest != "2026.09.01" || got.UpdateCommand != "" || !got.Managed {
		t.Errorf("an old yt-dlp of MixMusic's: %+v", got)
	}
	if n := asked.Load(); n != 1 {
		t.Errorf("GitHub was asked %d times, want once", n)
	}
}

func TestYTDLPToolOffline(t *testing.T) {
	latest := ""
	asked := fakeLatest(t, &latest)
	m := New(t.TempDir())
	if got := m.ytdlpTool(context.Background(), "/usr/bin/yt-dlp", "2025.01.15"); got.Latest != "" {
		t.Errorf("without the latest release: %+v", got)
	}
	latest = "2026.09.01"
	m.ytdlpTool(context.Background(), "/usr/bin/yt-dlp", "2025.01.15")
	if n := asked.Load(); n != 1 {
		t.Errorf("GitHub was asked %d times within %s of a failure, want once", n, retryTTL)
	}
	m.checked = m.checked.Add(-retryTTL)
	if got := m.ytdlpTool(context.Background(), "/usr/bin/yt-dlp", "2025.01.15"); got.Latest != "2026.09.01" {
		t.Errorf("after %s: %+v", retryTTL, got)
	}
}

// TestLiveInstall downloads yt-dlp and runs it. It runs with
// MIXMUSIC_LIVE=1.
func TestLiveInstall(t *testing.T) {
	if os.Getenv("MIXMUSIC_LIVE") == "" {
		t.Skip("set MIXMUSIC_LIVE=1 to download yt-dlp")
	}
	m := New(t.TempDir())
	var last int64
	if err := m.InstallYTDLP(t.Context(), func(done, total int64) { last = done }); err != nil {
		t.Fatal(err)
	}
	if last == 0 {
		t.Error("no progress reported")
	}
	s := m.Status(t.Context())
	if !s.YTDLP.Found || !s.YTDLP.Managed || s.YTDLP.Version == "" {
		t.Errorf("status = %+v", s.YTDLP)
	}
	t.Logf("installed yt-dlp %s; JavaScript runtime: %+v", s.YTDLP.Version, s.JSRuntime)
}

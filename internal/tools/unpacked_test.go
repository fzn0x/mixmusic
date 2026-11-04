package tools

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// onMac has the tools installed for macOS on Apple silicon, from a
// release whose yt-dlp_macos.zip is archive, and records what warm ran.
func onMac(t *testing.T, archive []byte) *[]string {
	t.Helper()
	savedOS, savedArch, savedWarm, savedURL := goos, goarch, warm, releaseURL
	goos, goarch = "darwin", "arm64"
	var warmed []string
	warm = func(_ context.Context, path string) { warmed = append(warmed, path) }
	sum := sha256.Sum256(archive)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/SHA2-256SUMS":
			w.Write([]byte(hex.EncodeToString(sum[:]) + "  yt-dlp_macos.zip\n"))
		case "/yt-dlp_macos.zip":
			w.Write(archive)
		default:
			http.NotFound(w, r)
		}
	}))
	releaseURL = srv.URL + "/"
	t.Cleanup(func() {
		srv.Close()
		goos, goarch, warm, releaseURL = savedOS, savedArch, savedWarm, savedURL
	})
	return &warmed
}

// archiveOf zips files, by name, with their content; a name ending in /
// is a folder, and one starting with * is a program.
func archiveOf(t *testing.T, files ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i := 0; i < len(files); i += 2 {
		name, content := files[i], files[i+1]
		h := &zip.FileHeader{Name: strings.TrimPrefix(name, "*"), Method: zip.Deflate}
		switch {
		case strings.HasSuffix(name, "/"):
			h.SetMode(fs.ModeDir | 0o755)
		case strings.HasPrefix(name, "*"):
			h.SetMode(0o755)
		default:
			h.SetMode(0o644)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(content))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestInstallUnpacked installs yt-dlp for macOS unpacked (#245): in a
// folder of its own, replacing the single file of before, and the folder
// of an earlier install whole.
func TestInstallUnpacked(t *testing.T) {
	warmed := onMac(t, archiveOf(t, "_internal/", "", "_internal/Python", "python", "*yt-dlp_macos", "v2"))
	dir := t.TempDir()
	m := New(dir)
	single := filepath.Join(dir, "yt-dlp")
	if err := os.WriteFile(single, []byte("single"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := m.managedPath(); got != single || !m.singleOnMac() {
		t.Fatalf("before the update, MixMusic's yt-dlp is %q", got)
	}
	// An earlier unpacked install, with a file the new one hasn't.
	if err := os.MkdirAll(filepath.Join(dir, macFolder, "_internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, macFolder, "_internal", "gone.so"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := m.InstallYTDLP(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(dir, macFolder, macProgram)
	if got, err := m.YTDLP(); err != nil || got != program || m.singleOnMac() {
		t.Errorf("YTDLP() = %q, %v; want %q", got, err, program)
	}
	if data, err := os.ReadFile(filepath.Join(dir, macFolder, "_internal", "Python")); err != nil || string(data) != "python" {
		t.Errorf("the unpacked library: %q, %v", data, err)
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(program); err != nil || info.Mode().Perm()&0o111 == 0 {
			t.Errorf("the program doesn't run: %v %v", info.Mode(), err)
		}
	}
	if isFile(single) || isFile(filepath.Join(dir, macFolder, "_internal", "gone.so")) {
		t.Error("the single file, or the earlier install's own file, stayed")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("Dir holds %v, want the folder alone", entries)
	}
	if len(*warmed) != 1 || (*warmed)[0] != program {
		t.Errorf("warmed %q, want the program once", *warmed)
	}
}

// TestUpdateReplacesSingle replaces macOS's single file of before with the
// unpacked build, though its version is the latest.
func TestUpdateReplacesSingle(t *testing.T) {
	onMac(t, archiveOf(t, "*yt-dlp_macos", "v2"))
	latest := "2026.10.26"
	fakeLatest(t, &latest)
	saved := version
	version = func(context.Context, string, string) string { return latest }
	t.Cleanup(func() { version = saved })
	dir := t.TempDir()
	m := New(dir)
	if err := os.WriteFile(filepath.Join(dir, "yt-dlp"), []byte("single"), 0o755); err != nil {
		t.Fatal(err)
	}
	if updated, err := m.UpdateYTDLP(context.Background()); err != nil || !updated {
		t.Fatalf("UpdateYTDLP() = %v, %v", updated, err)
	}
	if got := m.managedPath(); got != filepath.Join(dir, macFolder, macProgram) {
		t.Errorf("after the update, MixMusic's yt-dlp is %q", got)
	}
	if updated, err := m.UpdateYTDLP(context.Background()); err != nil || updated {
		t.Errorf("the unpacked build of the latest release was updated again: %v, %v", updated, err)
	}
}

// TestUnzipRefuses refuses an archive whose files would leave its folder,
// or that holds a link, and installs nothing from one.
func TestUnzipRefuses(t *testing.T) {
	for name, archive := range map[string][]byte{
		"a name with ..":   archiveOf(t, "*yt-dlp_macos", "v2", "../outside", "x"),
		"an absolute name": archiveOf(t, "*yt-dlp_macos", "v2", "/outside", "x"),
		"no program":       archiveOf(t, "_internal/Python", "python"),
	} {
		t.Run(name, func(t *testing.T) {
			onMac(t, archive)
			dir := filepath.Join(t.TempDir(), "bin")
			if err := New(dir).InstallYTDLP(context.Background(), nil); err == nil {
				t.Error("installed")
			}
			if entries, _ := os.ReadDir(dir); len(entries) != 0 {
				t.Errorf("left %v", entries)
			}
			if isFile(filepath.Join(filepath.Dir(dir), "outside")) {
				t.Error("wrote outside the folder")
			}
		})
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	h := &zip.FileHeader{Name: "link"}
	h.SetMode(fs.ModeSymlink | 0o777)
	w, _ := zw.CreateHeader(h)
	w.Write([]byte("/etc/passwd"))
	zw.Close()
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if err := unzip(zr, t.TempDir()); err == nil {
		t.Error("unpacked a link")
	}
}

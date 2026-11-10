package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
)

// github is a fake of GitHub's addresses of yt-dlp's releases, whose
// latest is 2026.09.01, for Linux on x86-64.
type github struct {
	program []byte
	tag     string // what the release page and the API name

	// Which addresses answer 504 Gateway Timeout: latest/download, as on
	// 2026-11-08 (#431), the release page's redirect, the API, and the
	// release's own address.
	filesDown, pageDown, apiDown, taggedDown bool
	// badSum publishes a checksum that isn't the program's.
	badSum bool

	mu    sync.Mutex
	asked []string
}

func newGitHub(t *testing.T, g *github) *github {
	t.Helper()
	if g.program == nil {
		g.program = []byte("#!/bin/sh\necho 2026.09.01\n")
	}
	if g.tag == "" {
		g.tag = "2026.09.01"
	}
	sum := sha256.Sum256(g.program)
	if g.badSum {
		sum = sha256.Sum256([]byte("another program"))
	}
	sums := hex.EncodeToString(sum[:]) + "  yt-dlp_linux\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		g.asked = append(g.asked, r.Method+" "+r.URL.Path)
		g.mu.Unlock()
		p := r.URL.Path
		files, tagged := strings.TrimPrefix(p, "/latest/download/"), strings.TrimPrefix(p, "/download/2026.09.01/")
		switch {
		case p == "/latest" && g.pageDown, p == "/api" && g.apiDown,
			files != p && g.filesDown, tagged != p && g.taggedDown:
			w.WriteHeader(http.StatusGatewayTimeout)
		case p == "/latest":
			http.Redirect(w, r, "/yt-dlp/yt-dlp/releases/tag/"+g.tag, http.StatusFound)
		case p == "/api":
			json.NewEncoder(w).Encode(map[string]string{"tag_name": g.tag, "name": "yt-dlp " + g.tag})
		case files == "SHA2-256SUMS", tagged == "SHA2-256SUMS":
			w.Write([]byte(sums))
		case files == "yt-dlp_linux", tagged == "yt-dlp_linux":
			w.Write(g.program)
		default:
			http.NotFound(w, r)
		}
	}))
	saved := []string{releaseURL, tagURL, latestURL, apiURL}
	savedOS, savedArch := goos, goarch
	releaseURL, tagURL = srv.URL+"/latest/download/", srv.URL+"/download/"
	latestURL, apiURL = srv.URL+"/latest", srv.URL+"/api"
	goos, goarch = "linux", "amd64"
	t.Cleanup(func() {
		srv.Close()
		releaseURL, tagURL, latestURL, apiURL = saved[0], saved[1], saved[2], saved[3]
		goos, goarch = savedOS, savedArch
	})
	return g
}

// askedFor reports whether a request's path began with prefix.
func (g *github) askedFor(prefix string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return slices.ContainsFunc(g.asked, func(a string) bool {
		_, p, _ := strings.Cut(a, " ")
		return strings.HasPrefix(p, prefix)
	})
}

// installed returns what MixMusic installed as its yt-dlp, or "".
func installed(m *Manager) string {
	data, _ := os.ReadFile(m.managedPath())
	return string(data)
}

// TestInstallFromRelease installs yt-dlp from its release's own address
// while latest/download fails (#431), finding the release from its page,
// or from the API when the page fails too.
func TestInstallFromRelease(t *testing.T) {
	for _, c := range []struct {
		name     string
		pageDown bool
	}{
		{"by the release page", false},
		{"by the API", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			g := newGitHub(t, &github{filesDown: true, pageDown: c.pageDown})
			m := New(t.TempDir())
			if err := m.InstallYTDLP(context.Background(), nil); err != nil {
				t.Fatalf("InstallYTDLP: %v; asked %q", err, g.asked)
			}
			if got := installed(m); got != string(g.program) {
				t.Errorf("installed %q, want the release's program", got)
			}
			if !g.askedFor("/download/2026.09.01/yt-dlp_linux") {
				t.Errorf("the release's address wasn't asked: %q", g.asked)
			}
		})
	}
}

// TestInstallFromLatest downloads from latest/download alone while it
// works.
func TestInstallFromLatest(t *testing.T) {
	g := newGitHub(t, &github{})
	m := New(t.TempDir())
	if err := m.InstallYTDLP(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if installed(m) != string(g.program) {
		t.Error("yt-dlp wasn't installed")
	}
	if g.askedFor("/download/") || g.askedFor("/api") || slices.Contains(g.asked, "HEAD /latest") {
		t.Errorf("asked %q, want latest/download alone", g.asked)
	}
}

// TestInstallFailsEverywhere tells the download that failed when the
// release's address can't be found or fails too, and installs nothing.
func TestInstallFailsEverywhere(t *testing.T) {
	for _, c := range []struct {
		name string
		g    *github
	}{
		{"no release found", &github{filesDown: true, pageDown: true, apiDown: true}},
		{"the release's address down", &github{filesDown: true, taggedDown: true}},
	} {
		t.Run(c.name, func(t *testing.T) {
			g := newGitHub(t, c.g)
			m := New(t.TempDir())
			err := m.InstallYTDLP(context.Background(), nil)
			if err == nil || !strings.Contains(err.Error(), "downloading yt-dlp checksums: HTTP 504") {
				t.Errorf("InstallYTDLP = %v, want the checksums' 504; asked %q", err, g.asked)
			}
			if got := installed(m); got != "" {
				t.Errorf("installed %q", got)
			}
		})
	}
}

// TestInstallMismatchStays: a program that doesn't match its checksum
// isn't downloaded again from elsewhere.
func TestInstallMismatchStays(t *testing.T) {
	g := newGitHub(t, &github{badSum: true})
	m := New(t.TempDir())
	err := m.InstallYTDLP(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Errorf("InstallYTDLP = %v, want a checksum that doesn't match", err)
	}
	if g.askedFor("/download/") || g.askedFor("/api") || slices.Contains(g.asked, "HEAD /latest") {
		t.Errorf("asked elsewhere after a checksum that didn't match: %q", g.asked)
	}
	if got := installed(m); got != "" {
		t.Errorf("installed %q", got)
	}
}

// TestLatestYTDLPFromAPI finds the latest release with the API when the
// release page fails, and not otherwise.
func TestLatestYTDLPFromAPI(t *testing.T) {
	ctx := context.Background()
	g := newGitHub(t, &github{pageDown: true})
	if v, err := New(t.TempDir()).LatestYTDLP(ctx); err != nil || v != "2026.09.01" {
		t.Errorf("the page down: LatestYTDLP = %q, %v; want the API's 2026.09.01", v, err)
	}

	g.apiDown = true
	if v, err := New(t.TempDir()).LatestYTDLP(ctx); err == nil || !strings.Contains(err.Error(), "HTTP 504 without a release") {
		t.Errorf("both down: LatestYTDLP = %q, %v; want the page's error", v, err)
	}

	g = newGitHub(t, &github{})
	if _, err := New(t.TempDir()).LatestYTDLP(ctx); err != nil || g.askedFor("/api") {
		t.Errorf("the page up: %v, asked %q; want the page alone", err, g.asked)
	}
}

// TestLatestYTDLPNames refuses a release named as no version is, which
// would lead its address elsewhere.
func TestLatestYTDLPNames(t *testing.T) {
	for _, tag := range []string{"..", "2026.09.01/../../x", "2026.09.01?x", `a\b`, "a b", "a\x08"} {
		newGitHub(t, &github{pageDown: true, tag: tag})
		if v, err := New(t.TempDir()).LatestYTDLP(context.Background()); err == nil {
			t.Errorf("the API's %q: LatestYTDLP = %q", tag, v)
		}
	}
}

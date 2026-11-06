package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// root is the repository's, from this package.
var root = filepath.Join("..", "..")

func read(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

func TestEscape(t *testing.T) {
	for in, want := range map[string]string{
		"github.com/egoist/mygo":     "github.com/egoist/mygo",
		"github.com/BurntSushi/toml": "github.com/!burnt!sushi/toml",
		"v2.1.4+incompatible":        "v2.1.4+incompatible",
	} {
		if got := escape(in); got != want {
			t.Errorf("escape(%q) = %q, want %q", in, got, want)
		}
	}
}

// fakeZip returns a module's zip file, as the proxy serves it.
func fakeZip(t *testing.T, prefix string, files map[string]string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for name, body := range files {
		f, err := w.Create(prefix + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// The sources vendor each module whose packages modules.txt lists, from
// the proxy, as go.sum hashes it, after modules.txt itself.
func TestSourcesOf(t *testing.T) {
	const modulesTxt = "# example.com/Foo/bar v1.2.0\n## explicit; go 1.26\nexample.com/Foo/bar\nexample.com/Foo/bar/baz\n# example.com/unused v0.1.0\n## explicit; go 1.20\n"
	zipFile := fakeZip(t, "example.com/!foo/bar@v1.2.0/", map[string]string{"go.mod": "module example.com/Foo/bar\n", "bar.go": "package bar\n"})
	h1, err := hashZip(zipFile)
	if err != nil {
		t.Fatal(err)
	}
	sums := "example.com/Foo/bar v1.2.0 " + h1 + "\nexample.com/Foo/bar v1.2.0/go.mod h1:x=\n"
	const url = "https://proxy.golang.org/example.com/!foo/bar/@v/v1.2.0.zip"
	fetched := 0
	fetch := func(u string) ([]byte, error) {
		fetched++
		if u != url {
			t.Errorf("downloaded %s", u)
		}
		return zipFile, nil
	}
	data, err := sourcesOf([]byte(modulesTxt), []byte(sums), fetch)
	if err != nil {
		t.Fatal(err)
	}
	var got []source
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(zipFile)
	want := []source{
		{Type: "inline", Dest: "vendor", DestFilename: "modules.txt", Contents: modulesTxt},
		{Type: "archive", URL: url, SHA256: hex.EncodeToString(sum[:]), StripComponents: 3, Dest: "vendor/example.com/Foo/bar"},
	}
	if len(got) != len(want) {
		t.Fatalf("sources = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("source %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if fetched != 1 {
		t.Errorf("downloaded %d files, want the used module's alone", fetched)
	}

	if _, err := sourcesOf([]byte(modulesTxt), []byte("example.com/Foo/bar v1.2.0 h1:other=\n"), fetch); err == nil {
		t.Error("a zip go.sum doesn't hash was taken")
	}
	if _, err := sourcesOf([]byte(modulesTxt), nil, fetch); err == nil {
		t.Error("a module go.sum doesn't name was taken")
	}
	if _, err := sourcesOf([]byte("# a v1 => ./a\n## explicit\na\n"), nil, fetch); err == nil {
		t.Error("a replaced module was taken")
	}
}

// hashZip hashes the zip files of the module cache as go.sum does.
func TestHashZipMatchesGoSum(t *testing.T) {
	out, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		t.Skip("no go command:", err)
	}
	cache := filepath.Join(strings.TrimSpace(string(out)), "cache", "download")
	checked := 0
	for line := range strings.Lines(read(t, "go.sum")) {
		f := strings.Fields(line)
		if len(f) != 3 || strings.HasSuffix(f[1], "/go.mod") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(cache, filepath.FromSlash(escape(f[0])), "@v", escape(f[1])+".zip"))
		if err != nil {
			continue // not downloaded here
		}
		if got, err := hashZip(data); err != nil || got != f[2] {
			t.Errorf("%s %s: hashZip = %s, %v; go.sum has %s", f[0], f[1], got, err, f[2])
		}
		checked++
	}
	if checked == 0 {
		t.Skip("no module of go.sum is in the module cache")
	}
}

// Flathub's copy of the manifest builds a commit of GitHub's repository in
// place of this checkout, and changes nothing else.
func TestWithSource(t *testing.T) {
	text := read(t, filepath.Join(dir, manifest))
	const commit = "0123456789abcdef0123456789abcdef01234567"
	got, err := withSource(text, gitSource(commit, "v0.6.0"))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"      - type: git",
		"        url: https://github.com/fzn0x/mixmusic.git",
		"        tag: v0.6.0",
		"        commit: " + commit,
		"        disable-submodules: true",
		"        x-checker-data:",
		"          type: git",
		`          tag-pattern: ^v([\d.]+)$`,
		"      - go-modules.json",
	}, "\n")
	if !strings.Contains(got, want) {
		t.Errorf("Flathub's copy lacks\n%s\nin\n%s", want, got)
	}
	if strings.Contains(got, "type: dir") || strings.Contains(got, "path: ../..") {
		t.Error("Flathub's copy still builds the checkout")
	}
	before, _, _ := strings.Cut(text, "      - type: dir")
	if !strings.HasPrefix(got, before) {
		t.Error("Flathub's copy changed what comes before the source")
	}
	got, err = withSource(text, gitSource(commit, ""))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "tag: v") || strings.Contains(got, "x-checker-data:\n          type: git") || !strings.Contains(got, "commit: "+commit) {
		t.Errorf("a commit's copy:\n%s", got)
	}
	if _, err := withSource("modules: []\n", gitSource(commit, "")); err == nil {
		t.Error("a manifest without the checkout's source was taken")
	}
}

// build.sh links in what mygo build would: mygo.config.ts's name,
// identifier and URL scheme, its frontendDist embedded, and the executable
// named after its slug, which the release's launcher starts.
func TestBuildAgreesWithConfig(t *testing.T) {
	build := read(t, filepath.Join(dir, "build.sh"))
	config := read(t, "mygo.config.ts")
	field := func(name string) string {
		m := regexp.MustCompile(`(?m)^  ` + name + `: \[?"([^"]+)"`).FindStringSubmatch(config)
		if m == nil {
			t.Fatalf("mygo.config.ts has no %s", name)
		}
		return m[1]
	}
	for _, c := range []struct{ name, flag string }{
		{"name", "packageName"},
		{"identifier", "packageIdentifier"},
		{"urlSchemes", "packageURLSchemes"},
	} {
		if want := "-X $mygo." + c.flag + "=" + field(c.name) + " "; !strings.Contains(build, want) {
			t.Errorf("build.sh doesn't link in %s, %q", c.name, strings.TrimSpace(want))
		}
	}
	if want := "cp -R " + field("frontendDist") + "/. mygo_frontend/"; !strings.Contains(build, want) {
		t.Errorf("build.sh doesn't embed frontendDist: no %q", want)
	}
	slug := field("slug")
	for _, want := range []string{"-o " + slug + " ", "install -Dm755 " + slug + ` "$dest/lib/mixmusic/mixmusic"`, "-tags mygo_noinspector", "-X $mygo.production=1", "-trimpath"} {
		if !strings.Contains(build, want) {
			t.Errorf("build.sh lacks %q", want)
		}
	}
	if !strings.Contains(read(t, "packaging/flatpak/mixmusic.sh"), "exec /app/lib/mixmusic/mixmusic ") {
		t.Error("the launcher doesn't start /app/lib/mixmusic/mixmusic")
	}
}

// The files of Flathub's build agree with each other, with the app, which
// tells Flathub's build by its marker, and with the release's Flatpak.
func TestFlathubFiles(t *testing.T) {
	const id = "io.github.fzn0x.mixmusic"
	m := read(t, filepath.Join(dir, manifest))
	for _, want := range []string{"\nid: " + id + "\n", "\ncommand: mixmusic\n", "\n  - org.freedesktop.Sdk.Extension.golang\n", "\n      - sh packaging/flathub/build.sh\n", "\n      - " + sourcesFile + "\n"} {
		if !strings.Contains(m, want) {
			t.Errorf("the manifest lacks %q", strings.TrimSpace(want))
		}
	}
	// The GNOME runtime and Node.js's extension move together with the
	// release's Flatpak's.
	release := read(t, "packaging/flatpak/"+id+".yml")
	for _, re := range []string{`(?m)^runtime-version: .*$`, `org\.freedesktop\.Sdk\.Extension\.node\d+`} {
		r := regexp.MustCompile(re)
		if a, b := r.FindString(m), r.FindString(release); a == "" || a != b {
			t.Errorf("Flathub's manifest has %q, the release's Flatpak %q", a, b)
		}
	}
	node := regexp.MustCompile(`org\.freedesktop\.Sdk\.Extension\.(node\d+)`).FindStringSubmatch(m)
	if node == nil || !strings.Contains(m, "/usr/lib/sdk/"+node[1]+"/install.sh") {
		t.Errorf("the manifest doesn't install Node.js from its extension")
	}

	build := read(t, filepath.Join(dir, "build.sh"))
	for _, file := range regexp.MustCompile(`install -Dm\d+ ((?:[^ "$]+ )+)`).FindAllStringSubmatch(build, -1) {
		for _, f := range strings.Fields(file[1]) {
			if f == "mixmusic" || strings.HasPrefix(f, "-") {
				continue
			}
			if _, err := os.Stat(filepath.Join(root, f)); err != nil {
				t.Errorf("build.sh installs %s: %v", f, err)
			}
		}
	}
	for _, f := range []string{id + ".desktop", id + ".metainfo.xml"} {
		if !strings.Contains(build, `"packaging/flathub/$id`+strings.TrimPrefix(f, id)+`"`) {
			t.Errorf("build.sh doesn't install %s", f)
		}
	}
	desktop := read(t, filepath.Join(dir, id+".desktop"))
	for _, want := range []string{"\nIcon=" + id + "\n", "\nExec=mixmusic %U\n", "\nMimeType=x-scheme-handler/mixmusic;\n", "\nStartupWMClass=mixmusic\n"} {
		if !strings.Contains(desktop, want) {
			t.Errorf("the desktop entry lacks %q", strings.TrimSpace(want))
		}
	}

	// MixMusic finds the marker and the yt-dlp where the build puts them.
	app := read(t, "internal/app/flathub.go")
	marker := regexp.MustCompile(`flathubMarker = "/app/([^"]+)"`).FindStringSubmatch(app)
	if marker == nil || !strings.Contains(build, `"$dest/`+marker[1]+`"`) {
		t.Errorf("build.sh doesn't leave internal/app's marker %v", marker)
	}
	ytdlp := regexp.MustCompile(`flathubYTDLP = "/app/([^"]+)"`).FindStringSubmatch(app)
	if ytdlp == nil || ytdlp[1] != "bin/yt-dlp" || !strings.Contains(m, "pip3 install --no-index --no-deps --prefix=${FLATPAK_DEST} yt_dlp-") {
		t.Errorf("internal/app runs %v, which the manifest doesn't install", ytdlp)
	}
}

// Command flathub writes what the manifest of Flathub's build of MixMusic
// (packaging/flathub) needs beside it: go-modules.json, the sources of the
// Go modules MixMusic is built with, which flatpak-builder downloads from
// the Go module proxy into vendor/ before the build, which runs without
// the network, with vendor/modules.txt as go mod vendor writes it. It
// downloads each module once and checks it against go.sum.
//
//	go run ./tools/flathub          writes packaging/flathub/go-modules.json
//	go run ./tools/flathub -check   fails when go-modules.json isn't go.sum's
//
// With -o, it writes the folder of a Flathub repository instead: the
// manifest, go-modules.json, and Flathub's shared modules, of the git
// submodule of packaging/flatpak. The manifest's MixMusic is this checkout,
// in that folder too, unless -commit or -tag names a commit of GitHub's
// repository, as Flathub builds; -tag adds the check of Flathub's bot for
// the next release. The flathub workflow builds such a folder.
//
//	go run ./tools/flathub -o build/flathub -commit "$(git rev-parse HEAD)"
package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	// dir is the folder of Flathub's manifest, from the root.
	dir = "packaging/flathub"
	// manifest is its name, the app's ID.
	manifest = "io.github.fzn0x.mixmusic.yml"
	// sourcesFile is the file of the Go modules' sources, beside it.
	sourcesFile = "go-modules.json"
	// sharedModules is the folder of Flathub's shared modules, the git
	// submodule the release's Flatpak takes the tray's library from.
	sharedModules = "packaging/flatpak/shared-modules"
	// repository is MixMusic's, which Flathub's copy builds a commit of.
	repository = "https://github.com/fzn0x/mixmusic.git"
	// proxy serves the modules' zip files, as go.sum hashes them.
	proxy = "https://proxy.golang.org/"
)

func main() {
	root := flag.String("root", ".", "the repository's root")
	check := flag.Bool("check", false, "fail when "+sourcesFile+" is out of date, without writing it")
	out := flag.String("o", "", "write the folder of a Flathub repository here instead")
	commit := flag.String("commit", "", "with -o, build this commit of "+repository)
	tag := flag.String("tag", "", "with -o, build this tag of "+repository+", as vX.Y.Z")
	flag.Parse()
	var err error
	switch {
	case *out != "":
		err = writeRepo(*root, *out, *commit, *tag)
	case *check:
		err = checkSources(*root)
	default:
		err = writeSources(*root)
	}
	if err != nil {
		log.Fatal(err)
	}
}

// writeSources writes go-modules.json of the modules go.sum names.
func writeSources(root string) error {
	data, err := sources(root, download)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, dir, sourcesFile), data, 0o644)
}

// checkSources fails when go-modules.json isn't what writeSources writes.
func checkSources(root string) error {
	want, err := sources(root, download)
	if err != nil {
		return err
	}
	got, err := os.ReadFile(filepath.Join(root, dir, sourcesFile))
	if err != nil {
		return err
	}
	if !bytes.Equal(bytes.ReplaceAll(got, []byte("\r\n"), []byte("\n")), want) {
		return fmt.Errorf("%s is out of date with go.sum: run go run ./tools/flathub", filepath.Join(dir, sourcesFile))
	}
	return nil
}

// sources returns go-modules.json: modules.txt, as go mod vendor writes it,
// then each module vendored, as an archive of the proxy, which fetch
// downloads.
func sources(root string, fetch func(url string) ([]byte, error)) ([]byte, error) {
	modulesTxt, err := vendorModules(root)
	if err != nil {
		return nil, err
	}
	sums, err := os.ReadFile(filepath.Join(root, "go.sum"))
	if err != nil {
		return nil, err
	}
	return sourcesOf(modulesTxt, sums, fetch)
}

// vendorModules returns the modules.txt of go mod vendor, written into a
// folder of its own.
func vendorModules(root string) ([]byte, error) {
	tmp, err := os.MkdirTemp("", "mixmusic-vendor-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	cmd := exec.Command("go", "mod", "vendor", "-o", tmp)
	cmd.Dir = root
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go mod vendor: %w", err)
	}
	b, err := os.ReadFile(filepath.Join(tmp, "modules.txt"))
	return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")), err
}

// source is a source of a flatpak-builder manifest.
type source struct {
	Type            string `json:"type"`
	URL             string `json:"url,omitempty"`
	SHA256          string `json:"sha256,omitempty"`
	StripComponents int    `json:"strip-components,omitempty"`
	Dest            string `json:"dest"`
	DestFilename    string `json:"dest-filename,omitempty"`
	Contents        string `json:"contents,omitempty"`
}

// module is a module whose packages modules.txt lists.
type module struct{ path, version string }

// sourcesOf returns the sources of modulesTxt's modules, each checked
// against its hash in sums, go.sum's lines.
func sourcesOf(modulesTxt, sums []byte, fetch func(url string) ([]byte, error)) ([]byte, error) {
	mods, err := vendored(modulesTxt)
	if err != nil {
		return nil, err
	}
	list := []source{{Type: "inline", Dest: "vendor", DestFilename: "modules.txt", Contents: string(modulesTxt)}}
	for _, m := range mods {
		want := goSum(sums, m)
		if want == "" {
			return nil, fmt.Errorf("go.sum has no hash of %s %s", m.path, m.version)
		}
		url := proxy + escape(m.path) + "/@v/" + escape(m.version) + ".zip"
		data, err := fetch(url)
		if err != nil {
			return nil, fmt.Errorf("downloading %s: %w", url, err)
		}
		got, err := hashZip(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", url, err)
		}
		if got != want {
			return nil, fmt.Errorf("%s is %s, go.sum says %s", url, got, want)
		}
		sum := sha256.Sum256(data)
		list = append(list, source{
			Type:            "archive",
			URL:             url,
			SHA256:          hex.EncodeToString(sum[:]),
			StripComponents: len(strings.Split(m.path, "/")),
			Dest:            "vendor/" + m.path,
		})
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "    ")
	if err := enc.Encode(list); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// vendored returns the modules of modules.txt that have packages vendored,
// in its order.
func vendored(modulesTxt []byte) ([]module, error) {
	var mods []module
	var cur *module
	used := false
	flush := func() {
		if cur != nil && used {
			mods = append(mods, *cur)
		}
	}
	sc := bufio.NewScanner(bytes.NewReader(modulesTxt))
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "## "):
		case strings.HasPrefix(line, "# "):
			flush()
			f := strings.Fields(line[2:])
			if len(f) != 2 {
				return nil, fmt.Errorf("modules.txt: %q: replaced modules aren't supported", line)
			}
			cur, used = &module{f[0], f[1]}, false
		case strings.TrimSpace(line) != "":
			used = true
		}
	}
	flush()
	if len(mods) == 0 {
		return nil, errors.New("modules.txt lists no packages")
	}
	return mods, sc.Err()
}

// goSum returns the hash go.sum has of m's files, or "".
func goSum(sums []byte, m module) string {
	for line := range strings.Lines(string(sums)) {
		if f := strings.Fields(line); len(f) == 3 && f[0] == m.path && f[1] == m.version {
			return f[2]
		}
	}
	return ""
}

// escape escapes a module's path or version as the proxy's URLs do: an
// upper-case letter is "!" and the letter in lower case.
func escape(s string) string {
	var b strings.Builder
	for _, r := range s {
		if 'A' <= r && r <= 'Z' {
			b.WriteByte('!')
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// hashZip returns go.sum's hash of a module's zip file, "h1:" and the
// SHA-256, in base64, of the lines of its files, sorted by name, each the
// SHA-256 of the file, in hex, two spaces and its name.
func hashZip(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	files := slices.Clone(zr.File)
	slices.SortFunc(files, func(a, b *zip.File) int { return strings.Compare(a.Name, b.Name) })
	h := sha256.New()
	for _, f := range files {
		if strings.Contains(f.Name, "\n") {
			return "", fmt.Errorf("a file's name has a new line: %q", f.Name)
		}
		r, err := f.Open()
		if err != nil {
			return "", err
		}
		fh := sha256.New()
		_, err = io.Copy(fh, r)
		r.Close()
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%x  %s\n", fh.Sum(nil), f.Name)
	}
	return "h1:" + base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

// download gets url, trying again when the proxy fails.
func download(url string) ([]byte, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	var err error
	for try := range 3 {
		if try > 0 {
			time.Sleep(time.Duration(try) * 5 * time.Second)
		}
		var res *http.Response
		if res, err = client.Get(url); err != nil {
			continue
		}
		var data []byte
		data, err = io.ReadAll(res.Body)
		res.Body.Close()
		if err == nil && res.StatusCode != http.StatusOK {
			err = fmt.Errorf("HTTP %s", res.Status)
		}
		if err == nil {
			return data, nil
		}
	}
	return nil, err
}

// writeRepo writes into out the folder of a Flathub repository, whose
// manifest builds commit, or tag, of GitHub's repository, else this
// checkout.
func writeRepo(root, out, commit, tag string) error {
	if tag != "" && !regexp.MustCompile(`^v\d+\.\d+\.\d+$`).MatchString(tag) {
		return fmt.Errorf("-tag %s: want a release's tag, as v0.6.0", tag)
	}
	if tag != "" && commit == "" {
		b, err := exec.Command("git", "-C", root, "rev-parse", tag+"^{commit}").Output()
		if err != nil {
			return fmt.Errorf("finding the commit of %s: %w", tag, err)
		}
		commit = strings.TrimSpace(string(b))
	}
	text, err := os.ReadFile(filepath.Join(root, dir, manifest))
	if err != nil {
		return err
	}
	var src []string
	if commit != "" {
		src = gitSource(commit, tag)
	} else {
		abs, err := filepath.Abs(root)
		if err != nil {
			return err
		}
		src = []string{"- type: dir", "  path: " + filepath.ToSlash(abs), "  skip:", "    - .git", "    - .flatpak-builder", "    - build", "    - node_modules"}
	}
	m, err := withSource(string(text), src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, manifest), []byte(m), 0o644); err != nil {
		return err
	}
	sources, err := os.ReadFile(filepath.Join(root, dir, sourcesFile))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, sourcesFile), sources, 0o644); err != nil {
		return err
	}
	shared := filepath.Join(root, sharedModules)
	if _, err := os.Stat(filepath.Join(shared, "libayatana-appindicator")); err != nil {
		return fmt.Errorf("%s is empty: git submodule update --init %s", sharedModules, sharedModules)
	}
	return copyTree(shared, filepath.Join(out, "shared-modules"))
}

// gitSource is the source of MixMusic at commit of GitHub's repository,
// without the git submodule of the release's Flatpak, which the build
// doesn't read. With tag, Flathub's bot proposes the next release's.
func gitSource(commit, tag string) []string {
	s := []string{"- type: git", "  url: " + repository}
	if tag != "" {
		s = append(s, "  tag: "+tag)
	}
	s = append(s, "  commit: "+commit, "  disable-submodules: true")
	if tag != "" {
		s = append(s, "  x-checker-data:", "    type: git", `    tag-pattern: ^v([\d.]+)$`)
	}
	return s
}

// withSource returns the manifest with src, lines of YAML, in place of its
// source of type dir, this checkout, indented as it is.
func withSource(text string, src []string) (string, error) {
	lines := strings.SplitAfter(text, "\n")
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == "- type: dir" {
			if start >= 0 {
				return "", errors.New("the manifest has two sources of type dir")
			}
			start = i
		}
	}
	if start < 0 {
		return "", errors.New("the manifest has no source of type dir, this checkout")
	}
	indent := lines[start][:strings.Index(lines[start], "-")]
	end := start + 1
	for end < len(lines) && strings.HasPrefix(lines[end], indent+"  ") {
		end++
	}
	var b strings.Builder
	for _, l := range lines[:start] {
		b.WriteString(l)
	}
	for _, l := range src {
		b.WriteString(indent + l + "\n")
	}
	for _, l := range lines[end:] {
		b.WriteString(l)
	}
	return b.String(), nil
}

// copyTree copies the folder from into to, without git's files.
func copyTree(from, to string) error {
	return filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == ".git" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
}

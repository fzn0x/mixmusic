// Command cask writes the Homebrew cask of a release, Casks/mixmusic.rb of
// the tap fzn0x/homebrew-tap, from packaging/homebrew/mixmusic.rb.tmpl: with
// the version, the SHA-256 of the release's disk image, and, until the app
// is notarized, caveats that tell how to allow its first launch.
// release.yml runs it once the release is out, and puts the cask in the
// tap.
//
//	go run ./tools/cask -version 0.5.9 -dmg release/mixmusic-0.5.9-macos-universal.dmg -notarized=false -o mixmusic.rb
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

// templateFile is the cask, as a text/template of a release.
const templateFile = "packaging/homebrew/mixmusic.rb.tmpl"

// release is what the cask of a version holds.
type release struct {
	// Version is the version released, as 0.5.9.
	Version string
	// SHA256 is the SHA-256 of its disk image, in hex.
	SHA256 string
	// Notarized leaves out the caveats on the first launch.
	Notarized bool
}

func main() {
	root := flag.String("root", ".", "the repository's root")
	version := flag.String("version", "", "the version released, as 0.5.9")
	dmg := flag.String("dmg", "", "the release's disk image, mixmusic-<version>-macos-universal.dmg")
	notarized := flag.Bool("notarized", false, "the app is notarized: no caveats on its first launch")
	out := flag.String("o", "", "the file to write; standard output when empty")
	flag.Parse()
	tmpl, err := os.ReadFile(filepath.Join(*root, templateFile))
	if err != nil {
		log.Fatal(err)
	}
	r, err := releaseOf(*version, *dmg, *notarized)
	if err != nil {
		log.Fatal(err)
	}
	c, err := cask(string(tmpl), r)
	if err != nil {
		log.Fatal(err)
	}
	if *out == "" {
		fmt.Print(c)
		return
	}
	if err := os.WriteFile(*out, []byte(c), 0o644); err != nil {
		log.Fatal(err)
	}
}

// semver is a version as release tags have it, without the v.
var semver = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$`)

// releaseOf returns the release of version, whose disk image is dmg. The
// cask downloads the image by its name in the release, which dmg must have,
// so that the sum is that of the file the cask names.
func releaseOf(version, dmg string, notarized bool) (release, error) {
	if !semver.MatchString(version) {
		return release{}, fmt.Errorf("-version %q is not a version, as 0.5.9", version)
	}
	if want := "mixmusic-" + version + "-macos-universal.dmg"; filepath.Base(dmg) != want {
		return release{}, fmt.Errorf("-dmg %q is not the release's %s", dmg, want)
	}
	f, err := os.Open(dmg)
	if err != nil {
		return release{}, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return release{}, err
	}
	return release{Version: version, SHA256: hex.EncodeToString(h.Sum(nil)), Notarized: notarized}, nil
}

// cask returns the cask of r, from tmpl.
func cask(tmpl string, r release) (string, error) {
	t, err := template.New(templateFile).Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := t.Execute(&b, r); err != nil {
		return "", err
	}
	return b.String(), nil
}

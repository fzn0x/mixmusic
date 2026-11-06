// Command aur writes the AUR package of a release, mixmusic-bin: its
// PKGBUILD and .SRCINFO, from packaging/aur/PKGBUILD.tmpl and
// packaging/aur/SRCINFO.tmpl, with the version and the SHA-256 of the
// release's two .deb files, which the package installs the files of, and
// of LICENSE-MIT. release.yml runs it once the release is out, and pushes
// the package to the AUR.
//
//	go run ./tools/aur -version 0.5.9 -release release -o aur
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

// templates are the package's files, as text/templates of a release, by
// the names they are written to.
var templates = map[string]string{
	"PKGBUILD": "packaging/aur/PKGBUILD.tmpl",
	".SRCINFO": "packaging/aur/SRCINFO.tmpl",
}

// release is what the package of a version holds.
type release struct {
	// Version is the version released, as 0.5.9, which names its files.
	Version string
	// PkgVer is Version as the PKGBUILD's pkgver: the same, as only
	// releases go to the AUR.
	PkgVer string
	// AMD64SHA256 and ARM64SHA256 are the SHA-256 of the release's .deb
	// files, in hex.
	AMD64SHA256, ARM64SHA256 string
	// LicenseSHA256 is the SHA-256 of LICENSE-MIT.
	LicenseSHA256 string
}

func main() {
	root := flag.String("root", ".", "the repository's root")
	version := flag.String("version", "", "the version released, as 0.5.9")
	dir := flag.String("release", "release", "the folder of the release's files")
	out := flag.String("o", "", "the folder to write PKGBUILD and .SRCINFO to; standard output when empty")
	flag.Parse()
	r, err := releaseOf(*root, *version, *dir)
	if err != nil {
		log.Fatal(err)
	}
	for _, name := range []string{"PKGBUILD", ".SRCINFO"} {
		tmpl, err := os.ReadFile(filepath.Join(*root, templates[name]))
		if err != nil {
			log.Fatal(err)
		}
		f, err := render(name, string(tmpl), r)
		if err != nil {
			log.Fatal(err)
		}
		if *out == "" {
			fmt.Printf("==> %s\n%s", name, f)
			continue
		}
		if err := os.MkdirAll(*out, 0o755); err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(*out, name), []byte(f), 0o644); err != nil {
			log.Fatal(err)
		}
	}
}

// semver is a version as release tags have it, without the v and without
// a pre-release: pacman takes no hyphen in pkgver, and orders a version
// written another way after its release, so that pre-releases stay off
// the AUR.
var semver = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// releaseOf returns the release of version, whose files are in dir.
func releaseOf(root, version, dir string) (release, error) {
	if !semver.MatchString(version) {
		return release{}, fmt.Errorf("-version %q is not a release's version, as 0.5.9", version)
	}
	r := release{Version: version, PkgVer: version}
	for _, f := range []struct {
		path string
		sum  *string
	}{
		{filepath.Join(dir, "mixmusic-"+version+"-linux-amd64.deb"), &r.AMD64SHA256},
		{filepath.Join(dir, "mixmusic-"+version+"-linux-arm64.deb"), &r.ARM64SHA256},
		{filepath.Join(root, "LICENSE-MIT"), &r.LicenseSHA256},
	} {
		sum, err := sha256Of(f.path)
		if err != nil {
			return release{}, err
		}
		*f.sum = sum
	}
	return r, nil
}

func sha256Of(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// render returns the file name of r, from tmpl.
func render(name, tmpl string, r release) (string, error) {
	t, err := template.New(name).Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := t.Execute(&b, r); err != nil {
		return "", err
	}
	return b.String(), nil
}

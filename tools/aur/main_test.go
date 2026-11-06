package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// root is the repository's, from this directory.
const root = "../.."

func renderAll(t *testing.T, r release) map[string]string {
	t.Helper()
	files := map[string]string{}
	for name, path := range templates {
		b, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		f, err := render(name, string(b), r)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(f, "{{") || strings.Contains(f, "}}") {
			t.Errorf("%s: a template action is left in\n%s", name, f)
		}
		files[name] = f
	}
	return files
}

var testRelease = release{
	Version:       "1.2.3",
	PkgVer:        "1.2.3",
	AMD64SHA256:   strings.Repeat("a", 64),
	ARM64SHA256:   strings.Repeat("b", 64),
	LicenseSHA256: strings.Repeat("c", 64),
}

func TestPKGBUILD(t *testing.T) {
	got := renderAll(t, testRelease)["PKGBUILD"]
	for _, want := range []string{
		"\npkgname=mixmusic-bin\n",
		"\n_version=1.2.3\n",
		"\npkgver=1.2.3\n",
		"\nsha256sums_x86_64=('" + testRelease.AMD64SHA256 + "')\n",
		"\nsha256sums_aarch64=('" + testRelease.ARM64SHA256 + "')\n",
		"\nsha256sums=('" + testRelease.LicenseSHA256 + "')\n",
		// The app lands where MixMusic tells a package of pacman's by.
		"ln -s \"/opt/$_pkgname/$_pkgname\" \"$pkgdir/usr/bin/$_pkgname\"",
		"\npackage() {\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in\n%s", want, got)
		}
	}
}

// The .SRCINFO says what the PKGBUILD does: the AUR reads it, not the
// PKGBUILD, and makepkg can't run here to write it.
func TestSRCINFOMatchesPKGBUILD(t *testing.T) {
	files := renderAll(t, testRelease)
	pkgbuild, srcinfo := files["PKGBUILD"], files[".SRCINFO"]
	fields := map[string][]string{}
	for _, l := range strings.Split(srcinfo, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(l), " = ")
		if ok {
			fields[k] = append(fields[k], v)
		}
	}
	// Each array of the PKGBUILD, with its variables put in, has the
	// values the .SRCINFO lists, in order.
	vars := strings.NewReplacer(
		"${_version}", testRelease.Version, "$_version", testRelease.Version,
		"$_pkgname", "mixmusic", "$pkgname", "mixmusic-bin",
	)
	array := regexp.MustCompile(`(?ms)^(\w+)=\((.*?)\)$`)
	item := regexp.MustCompile(`'([^']*)'|"([^"]*)"`)
	checked := 0
	for _, m := range array.FindAllStringSubmatch(pkgbuild, -1) {
		// The .SRCINFO names its fields as the PKGBUILD its arrays.
		name, k := m[1], m[1]
		var values []string
		for _, i := range item.FindAllStringSubmatch(m[2], -1) {
			values = append(values, vars.Replace(i[1]+i[2]))
		}
		got := fields[k]
		if strings.Join(got, "\n") != strings.Join(values, "\n") {
			t.Errorf("%s: the PKGBUILD has\n%q\nthe .SRCINFO's %s\n%q", name, values, k, got)
		}
		checked++
	}
	if checked < 10 {
		t.Errorf("checked %d arrays of the PKGBUILD, want them all", checked)
	}
	for _, scalar := range []string{"pkgver", "pkgrel", "url"} {
		m := regexp.MustCompile(`(?m)^` + scalar + `="?([^"\n]*)"?$`).FindStringSubmatch(pkgbuild)
		if m == nil || len(fields[scalar]) != 1 || fields[scalar][0] != m[1] {
			t.Errorf("%s: the PKGBUILD has %q, the .SRCINFO %q", scalar, m, fields[scalar])
		}
	}
	desc := regexp.MustCompile(`(?m)^pkgdesc="([^"]*)"$`).FindStringSubmatch(pkgbuild)
	if desc == nil || len(fields["pkgdesc"]) != 1 || fields["pkgdesc"][0] != desc[1] {
		t.Errorf("pkgdesc: the PKGBUILD has %q, the .SRCINFO %q", desc, fields["pkgdesc"])
	}
}

// The package downloads the .deb files the release has, by the names the
// release workflow gives them.
func TestSourcesNameTheDownloads(t *testing.T) {
	got := renderAll(t, testRelease)["PKGBUILD"]
	for _, arch := range []string{"amd64", "arm64"} {
		want := "https://github.com/fzn0x/mixmusic/releases/download/v${_version}/mixmusic-${_version}-linux-" + arch + ".deb"
		if !strings.Contains(got, want) {
			t.Errorf("no %q in\n%s", want, got)
		}
	}
}

func TestReleaseOf(t *testing.T) {
	dir := t.TempDir()
	sums := map[string]string{}
	for _, arch := range []string{"amd64", "arm64"} {
		body := []byte("deb of " + arch)
		if err := os.WriteFile(filepath.Join(dir, "mixmusic-1.2.3-linux-"+arch+".deb"), body, 0o644); err != nil {
			t.Fatal(err)
		}
		s := sha256.Sum256(body)
		sums[arch] = hex.EncodeToString(s[:])
	}
	r, err := releaseOf(root, "1.2.3", dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.AMD64SHA256 != sums["amd64"] || r.ARM64SHA256 != sums["arm64"] {
		t.Errorf("sums %s %s, want %s %s", r.AMD64SHA256, r.ARM64SHA256, sums["amd64"], sums["arm64"])
	}
	license, err := os.ReadFile(filepath.Join(root, "LICENSE-MIT"))
	if err != nil {
		t.Fatal(err)
	}
	if s := sha256.Sum256(license); r.LicenseSHA256 != hex.EncodeToString(s[:]) {
		t.Errorf("license sum %s", r.LicenseSHA256)
	}
	for _, v := range []string{"", "v1.2.3", "1.2", "1.2.3-beta.1"} {
		if _, err := releaseOf(root, v, dir); err == nil {
			t.Errorf("version %q: no error", v)
		}
	}
	if _, err := releaseOf(root, "9.9.9", dir); err == nil {
		t.Error("a release without its .deb files: no error")
	}
}

package main

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

const changelog = `# Changelog

## [Unreleased]

### Added

- Something to come.

## [1.2.0] - 2026-11-30

A **new** [About page](https://example.com) for <kbd>Ctrl</kbd> K, and
` + "`flatpak install`" + ` that works: R&D <done>. MixMusic 0.3.0 and later
offer this version in Settings › About.

### Fixed

- A fix.

## [1.1.0] - 2026-10-01

### Added

- No summary above.

[Unreleased]: https://github.com/fzn0x/mixmusic/compare/v1.2.0...HEAD
[1.2.0]: https://github.com/fzn0x/mixmusic/compare/v1.1.0...v1.2.0
`

func TestReleases(t *testing.T) {
	got := releases(changelog)
	want := []release{
		{"1.2.0", "2026-11-30", "A new About page for Ctrl K, and flatpak install that works: R&D ."},
		{"1.1.0", "2026-10-01", ""},
	}
	if len(got) != len(want) {
		t.Fatalf("releases = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("release %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestMetainfo(t *testing.T) {
	tmpl := "<component>\n  <id>x</id>\n  <releases/>\n</component>\n"
	got, err := metainfo(tmpl, changelog)
	if err != nil {
		t.Fatal(err)
	}
	want := `<component>
  <id>x</id>
  <releases>
    <release version="1.2.0" date="2026-11-30">
      <url type="details">https://github.com/fzn0x/mixmusic/releases/tag/v1.2.0</url>
      <description>
        <p>A new About page for Ctrl K, and flatpak install that works: R&amp;D .</p>
      </description>
    </release>
    <release version="1.1.0" date="2026-10-01">
      <url type="details">https://github.com/fzn0x/mixmusic/releases/tag/v1.1.0</url>
    </release>
  </releases>
</component>
`
	if got != want {
		t.Errorf("metainfo =\n%s\nwant\n%s", got, want)
	}
	if _, err := metainfo("<component/>", changelog); err == nil {
		t.Error("a template without <releases/> was filled")
	}
	if _, err := metainfo(tmpl, "## [Unreleased]\n"); err == nil {
		t.Error("a changelog without releases made metadata")
	}
}

// component is what the tests read of AppStream metadata.
type component struct {
	ID         string `xml:"id"`
	License    string `xml:"project_license"`
	Launchable string `xml:"launchable"`
	Releases   []struct {
		Version string `xml:"version,attr"`
		Date    string `xml:"date,attr"`
		URL     string `xml:"url"`
	} `xml:"releases>release"`
}

// The Flatpak's files agree with each other, with package.json, and with
// CHANGELOG.md, whose every release the metadata lists, newest first.
func TestFlatpakFiles(t *testing.T) {
	root := filepath.Join("..", "..")
	read := func(name string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	log := read("CHANGELOG.md")
	meta, err := metainfo(read(template), log)
	if err != nil {
		t.Fatal(err)
	}
	var c component
	if err := xml.Unmarshal([]byte(meta), &c); err != nil {
		t.Fatalf("the metadata isn't XML: %v", err)
	}
	const id = "io.github.fzn0x.mixmusic"
	if c.ID != id || c.Launchable != id+".desktop" {
		t.Errorf("id %q, launchable %q", c.ID, c.Launchable)
	}
	var pkg struct{ License string }
	if err := json.Unmarshal([]byte(read("package.json")), &pkg); err != nil {
		t.Fatal(err)
	}
	if c.License != pkg.License {
		t.Errorf("project_license %q, package.json's license %q", c.License, pkg.License)
	}
	headings := regexp.MustCompile(`(?m)^## \[(\d+\.\d+\.\d+)\] - `).FindAllStringSubmatch(log, -1)
	if len(c.Releases) == 0 || len(c.Releases) != len(headings) {
		t.Fatalf("%d releases in the metadata, %d in CHANGELOG.md", len(c.Releases), len(headings))
	}
	if c.Releases[0].Version != headings[0][1] {
		t.Errorf("the newest release is %s, CHANGELOG.md's %s", c.Releases[0].Version, headings[0][1])
	}
	for _, r := range c.Releases {
		if _, err := time.Parse(time.DateOnly, r.Date); err != nil {
			t.Errorf("%s: %v", r.Version, err)
		}
		if r.URL != releasePage+r.Version {
			t.Errorf("%s links to %s", r.Version, r.URL)
		}
	}

	manifest := read("packaging/flatpak/" + id + ".yml")
	for _, want := range []string{"\nid: " + id + "\n", "\ncommand: mixmusic\n", "/app/share/applications/" + id + ".desktop"} {
		if !strings.Contains(manifest, want) {
			t.Errorf("the manifest lacks %q", strings.TrimSpace(want))
		}
	}
}

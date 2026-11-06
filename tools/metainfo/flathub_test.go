package main

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// flathubTemplate is the metadata of Flathub's build, which its build.sh
// fills with -template.
const flathubTemplate = "packaging/flathub/io.github.fzn0x.mixmusic.metainfo.xml"

// Flathub's metadata fills as the release's Flatpak's does, with what
// Flathub asks for besides: brand colours for light and dark, and
// screenshots at addresses that don't move.
func TestFlathubMetainfo(t *testing.T) {
	root := filepath.Join("..", "..")
	read := func(name string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	meta, err := metainfo(read(flathubTemplate), read("CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	var c struct {
		component
		Developer string `xml:"developer>name"`
		Colors    []struct {
			Scheme string `xml:"scheme_preference,attr"`
			Value  string `xml:",chardata"`
		} `xml:"branding>color"`
		Shots []struct {
			Type  string `xml:"type,attr"`
			Image string `xml:"image"`
		} `xml:"screenshots>screenshot"`
		Rating string `xml:"content_rating>content_attribute"`
	}
	if err := xml.Unmarshal([]byte(meta), &c); err != nil {
		t.Fatalf("the metadata isn't XML: %v", err)
	}
	const id = "io.github.fzn0x.mixmusic"
	if c.ID != id || c.Launchable != id+".desktop" || c.Developer == "" || len(c.Releases) == 0 || c.Rating == "" {
		t.Errorf("id %q, launchable %q, developer %q, %d releases, rating %q", c.ID, c.Launchable, c.Developer, len(c.Releases), c.Rating)
	}
	schemes := map[string]bool{}
	for _, col := range c.Colors {
		if regexp.MustCompile(`^#[0-9a-f]{6}$`).MatchString(col.Value) {
			schemes[col.Scheme] = true
		}
	}
	if !schemes["light"] || !schemes["dark"] {
		t.Errorf("brand colours %+v, want one for light and one for dark", c.Colors)
	}
	// A commit of the repository, not a branch.
	pinned := regexp.MustCompile(`^https://raw\.githubusercontent\.com/fzn0x/mixmusic/[0-9a-f]{40}/docs/shots/[a-z-]+\.(png|webp)$`)
	if len(c.Shots) == 0 || c.Shots[0].Type != "default" {
		t.Errorf("screenshots %+v, want the default first", c.Shots)
	}
	for _, s := range c.Shots {
		if !pinned.MatchString(s.Image) {
			t.Errorf("screenshot %s isn't of a commit's docs/shots", s.Image)
		} else if _, err := os.Stat(filepath.Join(root, "docs", "shots", filepath.Base(s.Image))); err != nil {
			t.Errorf("screenshot %s: %v", s.Image, err)
		}
	}
}

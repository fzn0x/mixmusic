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

func readTemplate(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, templateFile))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCask(t *testing.T) {
	tmpl := readTemplate(t)
	sum := strings.Repeat("0123456789abcdef", 4)
	for _, notarized := range []bool{false, true} {
		got, err := cask(tmpl, release{Version: "1.2.3", SHA256: sum, Notarized: notarized})
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{
			"\ncask \"mixmusic\" do\n",
			"\n  version \"1.2.3\"\n",
			"\n  sha256 \"" + sum + "\"\n",
			"\n  app \"MixMusic.app\"\n",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("notarized %v: no %q in\n%s", notarized, want, got)
			}
		}
		if strings.Contains(got, "{{") || strings.Contains(got, "}}") {
			t.Errorf("notarized %v: a template action is left in\n%s", notarized, got)
		}
		if caveats := strings.Contains(got, "caveats <<~EOS"); caveats == notarized {
			t.Errorf("notarized %v: caveats %v, want %v", notarized, caveats, !notarized)
		}
		// The caveats come last, a stanza of their own.
		end := "  ]\n\n  caveats <<~EOS\n"
		if notarized {
			end = "  ]\nend\n"
		}
		if !strings.Contains(got, end) || !strings.HasSuffix(got, "\nend\n") {
			t.Errorf("notarized %v: no %q, or the cask doesn't end, in\n%s", notarized, end, got)
		}
		// Ruby can't run here: each block that opens closes, and the
		// heredoc ends.
		lines := strings.Split(got, "\n")
		opened, closed := 0, 0
		for _, l := range lines {
			if strings.HasSuffix(l, " do") {
				opened++
			}
			if strings.TrimSpace(l) == "end" {
				closed++
			}
		}
		if opened == 0 || opened != closed {
			t.Errorf("notarized %v: %d blocks open, %d close:\n%s", notarized, opened, closed, got)
		}
		if strings.Count(got, "<<~EOS") != strings.Count(got, "\n  EOS\n") {
			t.Errorf("notarized %v: a heredoc doesn't end:\n%s", notarized, got)
		}
	}
}

// The cask downloads the disk image the release notes link to.
func TestURLNamesTheDownload(t *testing.T) {
	got, err := cask(readTemplate(t), release{Version: "1.2.3", SHA256: strings.Repeat("0", 64)})
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`\n  url "([^"]+)"`).FindStringSubmatch(got)
	if m == nil {
		t.Fatalf("no url in\n%s", got)
	}
	url := strings.ReplaceAll(m[1], "#{version}", "1.2.3")
	notes, err := os.ReadFile(filepath.Join(root, ".github/release-notes.md"))
	if err != nil {
		t.Fatal(err)
	}
	link := regexp.MustCompile(`\(([^)]*-macos-universal\.dmg)\)`).FindStringSubmatch(string(notes))
	if link == nil {
		t.Fatal(".github/release-notes.md links to no -macos-universal.dmg")
	}
	want := strings.NewReplacer("{repo}", "fzn0x/mixmusic", "{tag}", "v1.2.3", "{version}", "1.2.3").Replace(link[1])
	if url != want {
		t.Errorf("the cask downloads\n%s\nthe release notes link to\n%s", url, want)
	}
}

func TestReleaseOf(t *testing.T) {
	dir := t.TempDir()
	dmg := filepath.Join(dir, "mixmusic-1.2.3-macos-universal.dmg")
	if err := os.WriteFile(dmg, []byte("a disk image"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := releaseOf("1.2.3", dmg, true)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("a disk image"))
	if want := (release{"1.2.3", hex.EncodeToString(sum[:]), true}); r != want {
		t.Errorf("releaseOf = %+v, want %+v", r, want)
	}
	if _, err := releaseOf("1.2.4", dmg, false); err == nil {
		t.Error("the disk image of 1.2.3 makes a cask of 1.2.4")
	}
	for _, v := range []string{"", "v1.2.3", "1.2", "1.2.3 beta"} {
		if _, err := releaseOf(v, dmg, false); err == nil {
			t.Errorf("version %q makes a cask", v)
		}
	}
	if _, err := releaseOf("1.2.3", filepath.Join(dir, "missing", filepath.Base(dmg)), false); err == nil {
		t.Error("a missing disk image makes a cask")
	}
}

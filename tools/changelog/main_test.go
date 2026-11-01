package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const changelog = `# Changelog

## [Unreleased]

A summary.

### Added

- A feature.

### Fixed

- A fix
  on two lines.

## [0.1.0] - 2026-01-01

### Fixed

- An old fix.
`

func TestFoldAddsToTheEndOfEachSection(t *testing.T) {
	frags := []fragment{
		{heading: "Added", text: "- Another feature (#7)."},
		{heading: "Removed", text: "- A setting (#8)."},
		{heading: "Fixed", text: "- Another fix (#94)."},
		{heading: "Fixed", text: "- A third fix\n  on two lines (#101)."},
		{heading: "Security", text: "- A hole (#9)."},
	}
	got, notes, err := fold(changelog, frags)
	if err != nil {
		t.Fatal(err)
	}
	want := `# Changelog

## [Unreleased]

A summary.

### Added

- A feature.
- Another feature (#7).

### Removed

- A setting (#8).

### Fixed

- A fix
  on two lines.
- Another fix (#94).
- A third fix
  on two lines (#101).

### Security

- A hole (#9).

## [0.1.0] - 2026-01-01

### Fixed

- An old fix.
`
	if got != want {
		t.Errorf("folded:\n%s\nwant:\n%s", got, want)
	}
	if !strings.HasPrefix(notes, "## [Unreleased]\n\nA summary.") || !strings.HasSuffix(notes, "- A hole (#9).\n\n") {
		t.Errorf("the section printed:\n%s", notes)
	}
}

func TestFoldIntoAnEmptySection(t *testing.T) {
	in := "# Changelog\n\n## [Unreleased]\n\n## [0.1.0] - 2026-01-01\n\n- Old.\n"
	got, _, err := fold(in, []fragment{{heading: "Changed", text: "- A change."}})
	if err != nil {
		t.Fatal(err)
	}
	want := "# Changelog\n\n## [Unreleased]\n\n### Changed\n\n- A change.\n\n## [0.1.0] - 2026-01-01\n\n- Old.\n"
	if got != want {
		t.Errorf("folded:\n%q\nwant:\n%q", got, want)
	}
	if _, _, err := fold("# Changelog\n\n## [0.1.0] - 2026-01-01\n", nil); err == nil {
		t.Error("a changelog without ## [Unreleased] folds")
	}
}

// The repository's own changelog comes out as it is when there is nothing
// to fold.
func TestFoldNothingKeepsTheChangelog(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	in := strings.ReplaceAll(string(data), "\r\n", "\n")
	got, notes, err := fold(in, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != in {
		t.Error("folding nothing changed CHANGELOG.md")
	}
	if !strings.HasPrefix(notes, "## [Unreleased]\n") || strings.Contains(notes, "\n## [") {
		t.Errorf("the section printed runs past ## [Unreleased]:\n%s", notes)
	}
}

func TestFragments(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) {
		file := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("README.md", "# What goes here\n")
	write("fixed/101-later.md", "- Fixed later (#101).\n")
	write("fixed/94-earlier.md", "- Fixed earlier\r\n  on two lines (#94).\r\n")
	write("fixed/no-issue.md", "- Fixed without an issue.\n")
	write("added/7-feature.md", "- A feature (#7).\n\n")
	frags, err := fragments(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range frags {
		got = append(got, f.heading+": "+f.text)
	}
	want := []string{
		"Added: - A feature (#7).",
		"Fixed: - Fixed earlier\n  on two lines (#94).",
		"Fixed: - Fixed later (#101).",
		"Fixed: - Fixed without an issue.",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("fragments %q, want %q", got, want)
	}

	write("misc/1-x.md", "- In no section.\n")
	write("notes.md", "- Beside the sections.\n")
	write("fixed/2-text.txt", "- Not Markdown.\n")
	write("fixed/3-empty.md", "\n")
	write("fixed/4-paragraph.md", "Not a list item.\n")
	write("fixed/5-heading.md", "- An entry.\n### Fixed\n")
	write("fixed/6-spaces.md", "- An entry \n  on two lines.\n")
	_, err = fragments(dir)
	if err == nil {
		t.Fatal("invalid fragments pass")
	}
	for _, name := range []string{"misc", "notes.md", "2-text.txt", "3-empty.md", "4-paragraph.md", "5-heading.md", "6-spaces.md"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("the error doesn't name %s:\n%v", name, err)
		}
	}
	if strings.Contains(err.Error(), "README.md") || strings.Contains(err.Error(), "94-earlier") {
		t.Errorf("the error names a valid file:\n%v", err)
	}
}

func TestNoFragments(t *testing.T) {
	frags, err := fragments(filepath.Join(t.TempDir(), "changelog.d"))
	if err != nil || len(frags) != 0 {
		t.Errorf("a missing changelog.d: %v, %v", frags, err)
	}
}

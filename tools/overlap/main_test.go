package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// repo makes a repository whose main holds lines.txt, ten lines, and
// main.go, and returns its folder.
func repo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "init", "-q", "-b", "main")
	write(t, dir, "lines.txt", 0, "")
	write(t, dir, "main.go", -1, "package main\n")
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-q", "-m", "base")
	return dir
}

// branch makes a branch off main that sets line n of lines.txt, or
// another file when name ends in .go.
func branch(t *testing.T, dir, name string, n int, text string) {
	t.Helper()
	run(t, dir, "checkout", "-q", "-b", name, "main")
	if strings.HasSuffix(name, ".go") {
		write(t, dir, name, -1, text)
	} else {
		write(t, dir, "lines.txt", n, text)
	}
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-q", "-m", name)
	run(t, dir, "checkout", "-q", "main")
}

// write writes the file name: ten numbered lines, with line n set to text,
// or text itself when n is below 0.
func write(t *testing.T, dir, name string, n int, text string) {
	t.Helper()
	content := text
	if n >= 0 {
		var lines []string
		for i := 1; i <= 10; i++ {
			l := strings.Repeat("x", i)
			if i == n {
				l = text
			}
			lines = append(lines, l)
		}
		content = strings.Join(lines, "\n") + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	if _, err := git(dir, args...); err != nil {
		t.Fatal(err)
	}
}

func TestOverlap(t *testing.T) {
	dir := repo(t)
	branch(t, dir, "this", 2, "this pull request's line")
	branch(t, dir, "next", 3, "the line beside it")
	branch(t, dir, "same", 2, "another line in its place")
	branch(t, dir, "far", 9, "a line far from it")
	branch(t, dir, "own.go", 0, "package main\n\nfunc own() {}\n")
	r, err := check(dir, "main", "this", []other{
		{"#1", "next"}, {"#2", "same"}, {"#3", "far"}, {"#4", "own.go"}, {"#5", "this"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.base) > 0 {
		t.Fatalf("it conflicts with the base: %v", r.base)
	}
	var names []string
	for _, c := range r.others {
		names = append(names, c.name)
		if !slices.Equal(c.files, []string{"lines.txt"}) {
			t.Errorf("%s conflicts in %v", c.name, c.files)
		}
	}
	// The line beside it conflicts, as git conflicts on lines next to each
	// other; a line further off, another file and itself don't.
	if !slices.Equal(names, []string{"#1", "#2"}) || r.merged != 3 {
		t.Errorf("conflicts %v, %d merged", names, r.merged)
	}
	if s := r.String(); !strings.Contains(s, "conflicts with #1 in lines.txt") || !strings.Contains(s, "merges with 3 other") {
		t.Errorf("the report:\n%s", s)
	}
	if out, err := git(dir, "worktree", "list"); err != nil || strings.Count(out, "\n") != 0 {
		t.Errorf("worktrees left: %q, %v", out, err)
	}
}

func TestOverlapWithTheBase(t *testing.T) {
	dir := repo(t)
	branch(t, dir, "this", 5, "this pull request's line")
	write(t, dir, "lines.txt", 5, "main's line")
	run(t, dir, "commit", "-q", "-am", "main moved")
	r, err := check(dir, "main", "this", []other{{"#1", "this"}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.base, []string{"lines.txt"}) || len(r.others) > 0 {
		t.Errorf("base %v, others %v", r.base, r.others)
	}
	if !strings.Contains(r.String(), "conflicts with the base in lines.txt") {
		t.Errorf("the report:\n%s", r)
	}
}

// TestOverlapWithoutRerere sees a conflict that rerere, which this
// repository's users turn on, resolved before.
func TestOverlapWithoutRerere(t *testing.T) {
	dir := repo(t)
	branch(t, dir, "this", 4, "this pull request's line")
	branch(t, dir, "other", 4, "the other's line")
	run(t, dir, "config", "rerere.enabled", "true")
	run(t, dir, "config", "rerere.autoUpdate", "true")
	// Record a resolution, as an earlier trial merge would, on a branch
	// of its own.
	cmd := []string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@localhost"}
	gitRaw(t, append(cmd, "checkout", "-q", "-b", "trial", "this")...)
	gitRaw(t, append(cmd, "merge", "-q", "other")...)
	write(t, dir, "lines.txt", 4, "both")
	gitRaw(t, append(cmd, "commit", "-q", "-am", "resolved")...)
	gitRaw(t, append(cmd, "checkout", "-q", "main")...)
	r, err := check(dir, "main", "this", []other{{"#1", "other"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.others) != 1 {
		t.Errorf("rerere hid the conflict: %+v", r)
	}
}

// gitRaw runs git as the user would, with the repository's own settings,
// which may fail.
func gitRaw(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Run()
}

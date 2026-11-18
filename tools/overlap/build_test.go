package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// goRepo makes a repository whose main holds a module with a package,
// lib, whose F returns 1, and a test of it, and returns its folder.
func goRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "init", "-q", "-b", "main")
	files(t, dir, map[string]string{
		"go.mod":          "module example.com/overlap\n\ngo 1.22\n",
		"README.md":       "A module.\n",
		"lib/lib.go":      "package lib\n\n// F is one.\nfunc F() int { return 1 }\n",
		"lib/lib_test.go": "package lib\n\nimport \"testing\"\n\nfunc TestF(t *testing.T) {\n\tif F() != 1 {\n\t\tt.Fatal(F())\n\t}\n}\n",
	})
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-q", "-m", "base")
	return dir
}

// files writes each file, by its path in dir.
func files(t *testing.T, dir string, fs map[string]string) {
	t.Helper()
	for name, text := range fs {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// pull makes a branch off main that writes fs.
func pull(t *testing.T, dir, name string, fs map[string]string) {
	t.Helper()
	run(t, dir, "checkout", "-q", "-b", name, "main")
	files(t, dir, fs)
	run(t, dir, "add", ".")
	run(t, dir, "commit", "-q", "-m", name)
	run(t, dir, "checkout", "-q", "main")
}

// this is a pull request that adds A, which calls F, and a test that A
// returns 1.
var this = map[string]string{
	"lib/a.go":      "package lib\n\n// A is F.\nfunc A() int { return F() }\n",
	"lib/a_test.go": "package lib\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {\n\tif A() != 1 {\n\t\tt.Fatal(A())\n\t}\n}\n",
}

func TestOverlapBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("it runs go vet and go test")
	}
	dir := goRepo(t)
	pull(t, dir, "this", this)
	// F takes an argument, which A doesn't give it.
	pull(t, dir, "signature", map[string]string{
		"lib/lib.go":      "package lib\n\n// F is n.\nfunc F(n int) int { return n }\n",
		"lib/lib_test.go": "package lib\n\nimport \"testing\"\n\nfunc TestF(t *testing.T) {\n\tif F(1) != 1 {\n\t\tt.Fatal(F(1))\n\t}\n}\n",
	})
	// Another A, in a file of its own.
	pull(t, dir, "twice", map[string]string{"lib/b.go": "package lib\n\n// A is two.\nfunc A() int { return 2 }\n"})
	pull(t, dir, "fine", map[string]string{"lib/c.go": "package lib\n\n// C is three.\nfunc C() int { return 3 }\n"})
	pull(t, dir, "docs", map[string]string{"README.md": "A module of one package.\n"})
	// It fails alone, so not with this one.
	pull(t, dir, "broken", map[string]string{"lib/d.go": "package lib\n\n// D isn't a number.\nfunc D() int { return \"d\" }\n"})
	// F returns 2, and its test says so: A's says 1.
	pull(t, dir, "value", map[string]string{
		"lib/lib.go":      "package lib\n\n// F is two.\nfunc F() int { return 2 }\n",
		"lib/lib_test.go": "package lib\n\nimport \"testing\"\n\nfunc TestF(t *testing.T) {\n\tif F() != 2 {\n\t\tt.Fatal(F())\n\t}\n}\n",
	})
	// A package for Windows alone, which go vet for Linux and macOS skips.
	pull(t, dir, "windows", map[string]string{"win/win_windows.go": "package win\n\n// W does nothing.\nfunc W() {}\n"})
	r, err := check(dir, "main", "this", []other{
		{"#1", "signature"}, {"#2", "twice"}, {"#3", "fine"}, {"#4", "docs"},
		{"#5", "broken"}, {"#6", "value"}, {"#7", "windows"},
	}, options{build: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.base) > 0 || len(r.others) > 0 || r.alone != "" {
		t.Fatalf("base %v, conflicts %v, alone %q", r.base, r.others, r.alone)
	}
	var got []string
	for _, c := range r.clashes {
		got = append(got, c.name+": "+c.step)
	}
	want := []string{"#1: go vet for linux", "#2: go vet for linux", "#6: the tests of ./lib"}
	if !slices.Equal(got, want) || r.merged != 7 || r.built != 6 {
		t.Errorf("clashes %q, %d merged, %d built", got, r.merged, r.built)
	}
	if len(r.clashes) == 3 && !strings.Contains(r.clashes[2].out, "TestA") {
		t.Errorf("the tests' failure:\n%s", r.clashes[2].out)
	}
	if s := r.String(); !strings.Contains(s, "merges with #1, but together they fail go vet for linux") ||
		!strings.Contains(s, "with the 6 of them that change Go code") {
		t.Errorf("the report:\n%s", s)
	}
	if out, err := git(dir, "worktree", "list"); err != nil || strings.Count(out, "\n") != 0 {
		t.Errorf("worktrees left: %q, %v", out, err)
	}
}

// TestOverlapBuildAlone builds a pull request that fails go vet alone
// with no other: its own checks tell why.
func TestOverlapBuildAlone(t *testing.T) {
	if testing.Short() {
		t.Skip("it runs go vet")
	}
	dir := goRepo(t)
	pull(t, dir, "this", map[string]string{"lib/a.go": "package lib\n\n// A isn't a number.\nfunc A() int { return \"a\" }\n"})
	pull(t, dir, "fine", map[string]string{"lib/c.go": "package lib\n\n// C is three.\nfunc C() int { return 3 }\n"})
	r, err := check(dir, "main", "this", []other{{"#1", "fine"}}, options{build: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.alone != "go vet for linux" || r.built != 0 || len(r.clashes) > 0 || r.merged != 1 {
		t.Errorf("alone %q, %d built, clashes %v", r.alone, r.built, r.clashes)
	}
}

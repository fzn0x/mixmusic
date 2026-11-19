// Command overlap tells whether a pull request merges with each of the
// other open ones, and with -build, whether they build together. A pull
// request that conflicts with another open one conflicts with main once
// the other merges. Two that pass alone can fail together too: one changes
// a function the other's new code calls, or both add the same name to a
// package. So CI asks it while both are open, and the one pushed last
// makes room. It merges the pull request's head into the base, as merging
// it would, then each other pull request into that, in a worktree of its
// own that it removes after, without the resolutions git rerere recorded,
// and prints the files of each conflict:
//
//	go run ./tools/overlap -base origin/main -head HEAD '#217=refs/overlap/217' '#218=refs/overlap/218'
//
// With -build, where both change Go code, it runs go vet for Linux,
// Windows and macOS on the packages either changes, on every package when
// either changes go.mod or go.sum, then the tests of the packages both
// change, with the command -test gives. A step two fail together counts
// only when each passes it alone. It vets the pull request alone first:
// when that fails, its own checks tell why, and it isn't built with the
// others.
//
// It exits 1 when the pull request conflicts with the base or with
// another, or fails a step with another.
package main

import (
	"errors"
	"flag"
	"fmt"
	"go/build"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// other is another open pull request: its name, as the report gives it,
// and the ref of its head.
type other struct{ name, ref string }

// conflict is a merge that didn't go: whose, and in which files.
type conflict struct {
	name  string
	files []string
}

// clash is a merge that went, but fails a step that each of the two
// passes alone: whose, which step, and the end of what it printed.
type clash struct{ name, step, out string }

// options are what check does beside merging.
type options struct {
	build bool     // vet each merge where both change Go code, and test it
	test  []string // the command that runs tests, as go test does
}

// report is what the merges came to.
type report struct {
	base    []string // the files the pull request conflicts with the base in
	merged  int      // the other pull requests it merges with
	others  []conflict
	alone   string // with -build, the step the pull request fails on its own
	built   int    // the other pull requests it was built with
	clashes []clash
}

func main() {
	base := flag.String("base", "origin/main", "the branch the pull requests merge into")
	head := flag.String("head", "HEAD", "the pull request's head")
	repo := flag.String("repo", ".", "the repository")
	build := flag.Bool("build", false, "vet each merge where both change Go code, and test the packages both change")
	test := flag.String("test", "go test", "the command that runs the tests, with -build")
	flag.Parse()
	var others []other
	for _, a := range flag.Args() {
		name, ref, ok := strings.Cut(a, "=")
		if !ok || name == "" || ref == "" {
			log.Fatalf("%q isn't name=ref", a)
		}
		others = append(others, other{name, ref})
	}
	r, err := check(*repo, *base, *head, others, options{*build, strings.Fields(*test)})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(r)
	if len(r.base) > 0 || len(r.others) > 0 || len(r.clashes) > 0 {
		os.Exit(1)
	}
}

// check merges head into base, then each of others into that.
func check(repo, base, head string, others []other, o options) (report, error) {
	// The worktree has a HEAD of its own, at the base: CI's HEAD, the
	// pull request, is the repository's. Name each commit as the
	// repository does.
	var err error
	if base, err = commit(repo, base); err != nil {
		return report{}, err
	}
	if head, err = commit(repo, head); err != nil {
		return report{}, err
	}
	others = slices.Clone(others)
	for i := range others {
		if others[i].ref, err = commit(repo, others[i].ref); err != nil {
			return report{}, err
		}
	}
	dir, err := os.MkdirTemp("", "overlap")
	if err != nil {
		return report{}, err
	}
	defer os.RemoveAll(dir)
	if _, err := git(repo, "worktree", "add", "-q", "--detach", dir, base); err != nil {
		return report{}, err
	}
	defer git(repo, "worktree", "remove", "--force", dir)
	var r report
	files, err := merge(dir, head, true)
	if err != nil || len(files) > 0 {
		r.base = files
		return r, err
	}
	merged, err := git(dir, "rev-parse", "HEAD")
	if err != nil {
		return r, err
	}
	var b *builder
	if o.build {
		if b, err = newBuilder(dir, base, head, o.test); err != nil {
			return r, err
		}
		if b != nil {
			r.alone = b.alone()
		}
	}
	for _, o := range others {
		files, err := merge(dir, o.ref, false)
		if err != nil {
			return r, err
		}
		if len(files) > 0 {
			r.others = append(r.others, conflict{o.name, files})
		} else {
			r.merged++
			if b != nil && r.alone == "" {
				built, c, err := b.with(o, merged)
				if err != nil {
					return r, err
				}
				if built {
					r.built++
				}
				if c != nil {
					r.clashes = append(r.clashes, *c)
				}
			}
		}
		if err := reset(dir, merged); err != nil {
			return r, err
		}
	}
	return r, nil
}

// commit names the commit ref is, in repo.
func commit(repo, ref string) (string, error) {
	return git(repo, "rev-parse", "--verify", ref+"^{commit}")
}

// reset puts the worktree at dir back at commit, without what a merge or
// a step left.
func reset(dir, commit string) error {
	if _, err := git(dir, "reset", "-q", "--hard", commit); err != nil {
		return err
	}
	_, err := git(dir, "clean", "-q", "-f", "-d")
	return err
}

// merge merges ref into the worktree at dir, committing it when commit is
// set, and returns the files that conflict, after aborting the merge.
func merge(dir, ref string, commit bool) ([]string, error) {
	args := []string{"merge", "-q", "--no-ff", "--no-edit"}
	if !commit {
		args = append(args, "--no-commit")
	}
	if _, err := git(dir, append(args, ref)...); err == nil {
		return nil, nil
	}
	out, err := git(dir, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, fmt.Errorf("merging %s failed, though no file conflicts", ref)
	}
	if _, err := git(dir, "merge", "--abort"); err != nil {
		return nil, err
	}
	return strings.Split(out, "\n"), nil
}

// builder vets and tests the pull request merged with each other one, in
// the worktree at dir.
type builder struct {
	dir, base string
	test      []string
	head      change
}

// change is the Go code a pull request changes.
type change struct {
	packages []string // the folders of the Go files it changes, as go names them
	modules  bool     // it changes go.mod or go.sum
}

func (c change) none() bool { return len(c.packages) == 0 && !c.modules }

// step is what a merge passes or fails: go vet for a system, or the tests
// when goos is empty, of packages.
type step struct {
	goos     string
	packages []string
}

// all is every package of the module, as go names them.
var all = []string{"./..."}

// vetted are the systems go vet checks a merge for, as CI's does.
var vetted = []string{"linux", "windows", "darwin"}

func (s step) String() string {
	if s.goos != "" {
		return "go vet for " + s.goos
	}
	return "the tests of " + strings.Join(s.packages, " ")
}

// newBuilder returns a builder for the pull request at head, or nil when
// it changes no Go code.
func newBuilder(dir, base, head string, test []string) (*builder, error) {
	if len(test) == 0 {
		test = []string{"go", "test"}
	}
	c, err := changed(dir, base, head)
	if err != nil || c.none() {
		return nil, err
	}
	return &builder{dir: dir, base: base, test: test, head: c}, nil
}

// changed tells the Go code that ref changes since it left base.
func changed(dir, base, ref string) (change, error) {
	out, err := git(dir, "diff", "--name-only", "--no-renames", base+"..."+ref)
	if err != nil || out == "" {
		return change{}, err
	}
	var c change
	for _, f := range strings.Split(out, "\n") {
		switch {
		case path.Base(f) == "go.mod" || path.Base(f) == "go.sum":
			c.modules = true
		case strings.HasSuffix(f, ".go"):
			p := "./" + path.Dir(f)
			if p == "./." {
				p = "."
			}
			if !slices.Contains(c.packages, p) {
				c.packages = append(c.packages, p)
			}
		}
	}
	return c, nil
}

// vets returns the steps that vet the packages of c, for each system:
// those of the module when it changes go.mod or go.sum.
func vets(c change) []step {
	pkgs := c.packages
	if c.modules {
		pkgs = all
	}
	var steps []step
	for _, goos := range vetted {
		steps = append(steps, step{goos, pkgs})
	}
	return steps
}

// alone vets the pull request merged into the base, as the worktree holds
// it now, and returns the step it fails, if one.
func (b *builder) alone() string {
	for _, s := range vets(b.head) {
		if _, ok := b.run(s); !ok {
			return s.String()
		}
	}
	return ""
}

// with vets and tests the pull request merged with o, which the worktree
// holds, uncommitted, over merged, the pull request merged into the base.
// It tells whether o changes Go code, so that the two were built, and the
// step they fail together when each passes it alone.
func (b *builder) with(o other, merged string) (bool, *clash, error) {
	c, err := changed(b.dir, b.base, o.ref)
	if err != nil || c.none() {
		return false, nil, err
	}
	either := change{packages: slices.Clone(b.head.packages), modules: b.head.modules || c.modules}
	var both []string
	for _, p := range c.packages {
		if slices.Contains(b.head.packages, p) {
			both = append(both, p)
		} else {
			either.packages = append(either.packages, p)
		}
	}
	steps := vets(either)
	if len(both) > 0 {
		steps = append(steps, step{packages: both})
	}
	for _, s := range steps {
		log.Printf("%s with %s", s, o.name)
		out, ok := b.run(s)
		if ok {
			continue
		}
		// The failure is theirs together only when each passes alone.
		if err := reset(b.dir, merged); err != nil {
			return true, nil, err
		}
		if _, ok := b.run(s); !ok {
			return true, nil, nil
		}
		if err := reset(b.dir, b.base); err != nil {
			return true, nil, err
		}
		if files, err := merge(b.dir, o.ref, false); err != nil || len(files) > 0 {
			return true, nil, err
		}
		if _, ok := b.run(s); !ok {
			return true, nil, nil
		}
		return true, &clash{o.name, s.String(), out}, nil
	}
	return true, nil, nil
}

// run runs s in the worktree, on those of its packages that it holds for
// the system, and returns the end of what it printed and whether it
// passed.
func (b *builder) run(s step) (string, bool) {
	goos := s.goos
	if goos == "" {
		goos = runtime.GOOS
	}
	pkgs := s.packages
	if !slices.Equal(pkgs, all) {
		pkgs = slices.DeleteFunc(slices.Clone(pkgs), func(p string) bool { return !b.holds(p, goos) })
		if len(pkgs) == 0 {
			return "", true
		}
	}
	args := slices.Concat(b.test, pkgs)
	var env []string
	if s.goos != "" {
		args = slices.Concat([]string{"go", "vet"}, pkgs)
		env = []string{"GOOS=" + s.goos}
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = b.dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > 40 {
		lines = lines[len(lines)-40:]
	}
	return strings.Join(lines, "\n"), err == nil
}

// holds tells whether the worktree holds the package p with a Go file for
// goos, as ./... would match it: a change can remove a package, or touch
// one that builds only for another system.
func (b *builder) holds(p, goos string) bool {
	ctx := build.Default
	ctx.GOOS = goos
	ctx.CgoEnabled = ctx.CgoEnabled && goos == runtime.GOOS
	_, err := ctx.ImportDir(filepath.Join(b.dir, filepath.FromSlash(p)), 0)
	var none *build.NoGoError
	if errors.As(err, &none) {
		return false
	}
	if _, err := os.Stat(filepath.Join(b.dir, filepath.FromSlash(p))); err != nil {
		return false
	}
	return true
}

// git runs git in dir without rerere, which would hide a conflict behind a
// resolution recorded before, or the hooks, and returns what it printed,
// trimmed.
func git(dir string, args ...string) (string, error) {
	pre := []string{"-C", dir, "-c", "rerere.enabled=false", "-c", "core.hooksPath=" + os.DevNull,
		"-c", "user.name=overlap", "-c", "user.email=overlap@localhost"}
	cmd := exec.Command("git", append(pre, args...)...)
	var out, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return out.String(), fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
		}
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func (r report) String() string {
	var b strings.Builder
	if len(r.base) > 0 {
		fmt.Fprintf(&b, "This pull request conflicts with the base in %s: merge the base in.\n", strings.Join(r.base, ", "))
		return b.String()
	}
	for _, c := range r.others {
		fmt.Fprintf(&b, "This pull request conflicts with %s in %s.\n", c.name, strings.Join(c.files, ", "))
	}
	if len(r.others) > 0 {
		b.WriteString("Change it so they don't touch the same lines, or wait for the other to merge and merge the base in.\n")
	}
	for _, c := range r.clashes {
		fmt.Fprintf(&b, "This pull request merges with %s, but together they fail %s, which each passes alone:\n\n", c.name, c.step)
		for _, l := range strings.Split(c.out, "\n") {
			fmt.Fprintf(&b, "    %s\n", l)
		}
		b.WriteString("\n")
	}
	if len(r.clashes) > 0 {
		b.WriteString("Change it so they pass together, or wait for the other to merge and merge the base in.\n")
	}
	fmt.Fprintf(&b, "It merges with %d other open pull requests.\n", r.merged)
	if r.alone != "" {
		fmt.Fprintf(&b, "It fails %s on its own, so it wasn't built with them: its own checks tell why.\n", r.alone)
	} else if r.built > 0 {
		fmt.Fprintf(&b, "It was built with the %d of them that change Go code too.\n", r.built)
	}
	return b.String()
}

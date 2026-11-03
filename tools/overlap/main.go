// Command overlap tells whether a pull request merges with each of the
// other open ones. A pull request that conflicts with another open one
// conflicts with main once the other merges, so CI asks it while both are
// open, and the one pushed last makes room. It merges the pull request's
// head into the base, as merging it would, then each other pull request
// into that, in a worktree of its own that it removes after, without the
// resolutions git rerere recorded, and prints the files of each conflict:
//
//	go run ./tools/overlap -base origin/main -head HEAD '#217=refs/overlap/217' '#218=refs/overlap/218'
//
// It exits 1 when the pull request conflicts with the base or with another.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
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

// report is what the merges came to.
type report struct {
	base   []string // the files the pull request conflicts with the base in
	merged int      // the other pull requests it merges with
	others []conflict
}

func main() {
	base := flag.String("base", "origin/main", "the branch the pull requests merge into")
	head := flag.String("head", "HEAD", "the pull request's head")
	repo := flag.String("repo", ".", "the repository")
	flag.Parse()
	var others []other
	for _, a := range flag.Args() {
		name, ref, ok := strings.Cut(a, "=")
		if !ok || name == "" || ref == "" {
			log.Fatalf("%q isn't name=ref", a)
		}
		others = append(others, other{name, ref})
	}
	r, err := check(*repo, *base, *head, others)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(r)
	if len(r.base) > 0 || len(r.others) > 0 {
		os.Exit(1)
	}
}

// check merges head into base, then each of others into that.
func check(repo, base, head string, others []other) (report, error) {
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
	for _, o := range others {
		files, err := merge(dir, o.ref, false)
		if err != nil {
			return r, err
		}
		if len(files) > 0 {
			r.others = append(r.others, conflict{o.name, files})
		} else {
			r.merged++
		}
		if _, err := git(dir, "reset", "-q", "--hard", merged); err != nil {
			return r, err
		}
	}
	return r, nil
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
	fmt.Fprintf(&b, "It merges with %d other open pull requests.\n", r.merged)
	return b.String()
}

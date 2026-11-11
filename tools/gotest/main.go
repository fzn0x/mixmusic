// Command gotest runs go test, then runs again the tests that failed, up
// to 3 more times, so that a test that fails now and then doesn't fail CI,
// while one that fails every time still does. CI runs every Go test with
// it:
//
//	go run ./tools/gotest [-attempts 3] [go test flags] [packages]
//
// It runs go test -json with the arguments given and prints what go test
// would: the output of each test that fails, and with -v, of every test.
// Then it runs again, in their package, the top-level tests that failed,
// with -run '^(TestA|TestB)$' -count=1 and the other flags as given; a
// subtest's failure counts as its test's. A package that failed with no
// test failing, as when it panics outside its tests, or whose test binary
// crashed or timed out, runs again whole. A package that doesn't build, or
// that go vet fails, fails at once.
//
// It exits 0 when every test passed in the end, and reports each that
// passed only when run again as flaky: a warning on GitHub, and a line in
// the job's summary when $GITHUB_STEP_SUMMARY names it. It exits 1 when a
// test failed every time it ran, after printing its last output again.
package main

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// session is a run of gotest: how it runs go test, and where it reports.
type session struct {
	goTest  func(args []string, line func([]byte)) error
	out     io.Writer
	summary string // the file of the job's summary, or ""
}

func main() {
	s := &session{goTest: goTest, out: os.Stdout, summary: os.Getenv("GITHUB_STEP_SUMMARY")}
	os.Exit(s.main(os.Args[1:]))
}

// pkg is a package that failed, through its runs again.
type pkg struct {
	path    string
	runs    int
	build   bool               // it didn't build, or go vet failed
	whole   bool               // its next run runs all its tests
	fails   map[string]int     // the runs each top-level test failed in, "" standing for the package outside its tests
	pending map[string]*result // the tests failing still, with their last run
	flaky   []flake
}

// flake is a test that failed, then passed when run again.
type flake struct {
	test  string
	fails int
}

// update takes in a run of the package.
func (p *pkg) update(r *result, failfast bool) {
	p.runs++
	if r.build {
		p.build = true
		return
	}
	for _, name := range r.passedTests() {
		p.passed(name)
	}
	if !r.failed {
		for _, name := range slices.Sorted(maps.Keys(p.pending)) {
			p.passed(name)
		}
		return
	}
	failed := r.failedTests()
	if len(failed) == 0 {
		failed = []string{""}
	}
	for _, name := range failed {
		p.fails[name]++
		p.pending[name] = r
	}
	// A crash or a timeout may have kept tests from running at all, and
	// -failfast does: those run again too.
	_, outside := p.pending[""]
	p.whole = outside || !r.clean || failfast
}

func (p *pkg) passed(name string) {
	if _, ok := p.pending[name]; ok {
		delete(p.pending, name)
		p.flaky = append(p.flaky, flake{name, p.fails[name]})
	}
}

// main runs go test with args, and the tests that fail again, and returns
// the exit code.
func (s *session) main(args []string) int {
	c, err := parse(args)
	if err != nil {
		fmt.Fprintln(s.out, "gotest:", err)
		return 2
	}
	results, err := s.run(c.verbose, c.all())
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		fmt.Fprintln(s.out, "gotest:", err)
		return 1
	}
	var failed []*pkg
	for _, r := range results {
		if r.failed || !r.ended {
			p := &pkg{path: r.pkg, fails: map[string]int{}, pending: map[string]*result{}}
			p.update(r, c.failfast)
			failed = append(failed, p)
		}
	}
	if err != nil && len(failed) == 0 {
		fmt.Fprintln(s.out, "gotest: go test failed, though no package did: that isn't run again.")
		return 1
	}
	for _, p := range failed {
		for !p.build && len(p.pending) > 0 && p.runs <= c.attempts {
			s.again(c, p)
		}
	}
	return s.report(failed)
}

// again runs the tests of p that failed again, or all of them.
func (s *session) again(c command, p *pkg) {
	try := fmt.Sprintf("(run %d of %d)", p.runs+1, c.attempts+1)
	var args []string
	if p.whole {
		fmt.Fprintf(s.out, "gotest: running %s again, all of it %s\n", p.path, try)
		args = c.whole(p.path)
	} else {
		names := slices.Sorted(maps.Keys(p.pending))
		fmt.Fprintf(s.out, "gotest: running %s of %s again %s\n", strings.Join(names, ", "), p.path, try)
		args = c.tests(p.path, names)
	}
	results, _ := s.run(c.verbose, args)
	for _, r := range results {
		if r.pkg == p.path {
			p.update(r, c.failfast)
			return
		}
	}
	// go test didn't get to the package: that's a failed run too.
	p.runs++
	for name := range p.pending {
		p.fails[name]++
	}
}

// report reports the flaky tests and the ones that failed every time, and
// returns the exit code.
func (s *session) report(pkgs []*pkg) int {
	var flaky []string
	code := 0
	for _, p := range pkgs {
		for _, f := range p.flaky {
			flaky = append(flaky, fmt.Sprintf("%s failed %s, then passed", name(p.path, f.test), times(f.fails)))
		}
	}
	for _, f := range flaky {
		fmt.Fprintf(s.out, "::warning title=Flaky test::%s\n", f)
	}
	if len(flaky) > 0 && s.summary != "" {
		if err := appendFile(s.summary, "### Flaky tests\n\nThese failed, then passed when run again:\n\n- "+strings.Join(flaky, "\n- ")+"\n\n"); err != nil {
			fmt.Fprintln(s.out, "gotest:", err)
		}
	}
	for _, p := range pkgs {
		if p.build {
			fmt.Fprintf(s.out, "gotest: %s doesn't build, or go vet failed: that isn't run again.\n", p.path)
			code = 1
			continue
		}
		for _, test := range slices.Sorted(maps.Keys(p.pending)) {
			fmt.Fprintf(s.out, "gotest: %s failed %s. Its last output:\n%s", name(p.path, test), times(p.fails[test]), p.pending[test].output(test))
			code = 1
		}
	}
	return code
}

// name names a test of a package, or the package outside its tests.
func name(pkg, test string) string {
	if test == "" {
		return pkg + " (outside its tests)"
	}
	return pkg + " " + test
}

func times(n int) string {
	if n == 1 {
		return "1 time"
	}
	return fmt.Sprintf("%d times", n)
}

func appendFile(path, text string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = f.WriteString(text)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

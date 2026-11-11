package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// event is a line of go test -json's output (go doc test2json).
type event struct {
	Action      string
	Package     string
	Test        string
	Output      string
	FailedBuild string // set on a package's fail when it didn't build
}

// result is what a run of go test came to in one package.
type result struct {
	pkg    string
	ended  bool              // the package's own pass, fail or skip came
	failed bool              // that was a fail
	build  bool              // it didn't build, or go vet failed
	clean  bool              // its test binary ended with its own PASS or FAIL, as one does, rather than crash or time out
	tests  map[string]string // how each test ended, pass, fail or skip, or "" while it runs
	paused map[string]bool   // the parallel tests waiting their turn
	fails  map[string]bool   // the top-level tests that failed, a subtest's failure counted as its test's
	lines  []line            // its output, in order
}

// line is a line of a package's output, and the test it came from, ""
// when it came from none.
type line struct{ test, text string }

func newResult(pkg string) *result {
	return &result{pkg: pkg, tests: map[string]string{}, paused: map[string]bool{}, fails: map[string]bool{}}
}

// top is the top-level test of a test or subtest.
func top(test string) string {
	name, _, _ := strings.Cut(test, "/")
	return name
}

func (r *result) add(e event) {
	switch e.Action {
	case "run":
		r.tests[e.Test] = ""
	case "pause":
		r.paused[e.Test] = true
	case "cont":
		delete(r.paused, e.Test)
	case "pass", "fail", "skip":
		if e.Test == "" {
			r.ended = true
			r.failed = e.Action == "fail"
			r.build = e.FailedBuild != ""
			return
		}
		r.tests[e.Test] = e.Action
		delete(r.paused, e.Test)
		if e.Action == "fail" {
			r.fails[top(e.Test)] = true
		}
	case "output":
		r.lines = append(r.lines, line{e.Test, e.Output})
		if e.Test == "" && (e.Output == "PASS\n" || e.Output == "FAIL\n") {
			r.clean = true
		}
	}
}

// running tells whether a test started and hadn't ended when the package
// did, and wasn't waiting its turn: a test the binary crashed or timed out
// in.
func (r *result) running(test string) bool {
	end, ok := r.tests[test]
	return ok && end == "" && !r.paused[test]
}

// failedTests are the top-level tests that failed, or that were running
// when a package that failed ended, sorted.
func (r *result) failedTests() []string {
	var names []string
	for name := range r.fails {
		names = append(names, name)
	}
	if r.failed {
		for test := range r.tests {
			if r.running(test) && !slices.Contains(names, top(test)) {
				names = append(names, top(test))
			}
		}
	}
	slices.Sort(names)
	return names
}

// passedTests are the top-level tests that passed or skipped, and failed
// in no run of -count.
func (r *result) passedTests() []string {
	var names []string
	for test, end := range r.tests {
		if !strings.Contains(test, "/") && (end == "pass" || end == "skip") && !r.fails[test] {
			names = append(names, test)
		}
	}
	slices.Sort(names)
	return names
}

// output is the output of the top-level test named, or of the package
// outside its tests when name is "": the lines of the test and its
// subtests that failed or didn't end, without go test's === lines.
func (r *result) output(name string) string {
	var b strings.Builder
	for _, l := range r.lines {
		if strings.HasPrefix(l.text, "=== ") {
			continue
		}
		mine := l.test == "" || r.running(l.test)
		if name != "" {
			mine = top(l.test) == name && (r.tests[l.test] == "fail" || r.running(l.test))
		}
		if mine {
			b.WriteString(l.text)
		}
	}
	return b.String()
}

// report is what go test prints of a package that ended, without -v: the
// "ok" line of one that passed, and of one that failed, the output of its
// tests that failed and of the package outside them.
func (r *result) report() string {
	var b strings.Builder
	for _, l := range r.lines {
		switch {
		case strings.HasPrefix(l.text, "=== "):
		case !r.failed && r.ended:
			if l.test == "" && (strings.HasPrefix(l.text, "ok ") || strings.HasPrefix(l.text, "?")) {
				b.WriteString(l.text)
			}
		case l.test == "" || r.tests[l.test] == "fail" || r.tests[l.test] == "":
			b.WriteString(l.text)
		}
	}
	return b.String()
}

// printer prints a run's output as it comes. With -v, it prints every
// line of one package at a time, as go test does, the others' waiting in
// turn; without, it prints each package's report as it ends.
type printer struct {
	w       io.Writer
	verbose bool
	queue   []*result       // with -v, the packages started and not yet printed to their end
	printed map[*result]int // with -v, the lines printed of each
}

func (p *printer) event(r *result, e event) {
	if !p.verbose {
		if e.Test == "" && r.ended && (e.Action == "pass" || e.Action == "fail" || e.Action == "skip") {
			fmt.Fprint(p.w, r.report())
		}
		return
	}
	if !slices.Contains(p.queue, r) {
		p.queue = append(p.queue, r)
	}
	for len(p.queue) > 0 {
		front := p.queue[0]
		for _, l := range front.lines[p.printed[front]:] {
			fmt.Fprint(p.w, l.text)
		}
		p.printed[front] = len(front.lines)
		if !front.ended {
			break
		}
		p.queue = p.queue[1:]
	}
}

// end prints what's left when go test ends: the packages that never did.
func (p *printer) end(results []*result) {
	for _, r := range results {
		switch {
		case p.verbose:
			for _, l := range r.lines[p.printed[r]:] {
				fmt.Fprint(p.w, l.text)
			}
		case !r.ended:
			fmt.Fprint(p.w, r.report())
		}
	}
}

// run runs go test with args, prints its output as it goes, and returns
// what it came to in each package, in the order they started, and how go
// test exited.
func (s *session) run(verbose bool, args []string) ([]*result, error) {
	p := &printer{w: s.out, verbose: verbose, printed: map[*result]int{}}
	byPkg := map[string]*result{}
	var results []*result
	err := s.goTest(args, func(b []byte) {
		var e event
		if json.Unmarshal(b, &e) != nil || e.Action == "" {
			fmt.Fprint(s.out, string(b))
			return
		}
		if e.Package == "" {
			// go build's output, of a package that doesn't build.
			fmt.Fprint(s.out, e.Output)
			return
		}
		r := byPkg[e.Package]
		if r == nil {
			r = newResult(e.Package)
			byPkg[e.Package] = r
			results = append(results, r)
		}
		r.add(e)
		p.event(r, e)
	})
	p.end(results)
	return results, err
}

// goTest runs go with args, its errors going to gotest's, and hands each
// line it prints to line.
func goTest(args []string, line func([]byte)) error {
	cmd := exec.Command("go", args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	r := bufio.NewReader(out)
	for {
		b, err := r.ReadBytes('\n')
		if len(b) > 0 {
			line(b)
		}
		if err != nil {
			break
		}
	}
	return cmd.Wait()
}

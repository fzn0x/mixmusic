package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The streams in testdata are go test -json's, recorded with Go 1.27.3 on
// a module of small packages, example.com/demo, whose tests fail when
// REC_FAIL is set: flaky's TestFlaky, subtest's TestTable/b, testpanic's
// TestPanic (a nil map), mainpanic's TestMain (after its tests), and
// timeout's TestSlow (it sleeps past -timeout 2s); build's test doesn't
// compile, and vet's fails go vet. Each was recorded with the command line
// gotest gives go test, and the folder of the module and of Go made
// /home/runner/work/demo and /usr/local/go.

// fake answers each run of go test with the next recording, as go test
// would, and keeps the command lines.
type fake struct {
	t     *testing.T
	runs  []string
	calls [][]string
}

func (f *fake) goTest(args []string, line func([]byte)) error {
	f.calls = append(f.calls, args)
	if len(f.calls) > len(f.runs) {
		f.t.Fatalf("go %s: run %d, of %d recorded", strings.Join(args, " "), len(f.calls), len(f.runs))
	}
	data, err := os.ReadFile(filepath.Join("testdata", f.runs[len(f.calls)-1]+".json"))
	if err != nil {
		f.t.Fatal(err)
	}
	failed := false
	for _, l := range bytes.SplitAfter(data, []byte("\n")) {
		if len(l) > 0 {
			line(l)
			failed = failed || bytes.Contains(l, []byte(`"Action":"fail"`))
		}
	}
	if failed {
		return &exec.ExitError{}
	}
	return nil
}

// gotest runs gotest with args, go test answering with the recordings
// runs, and returns its exit code, its output, the command lines of its
// runs of go test, and what it wrote in the job's summary.
func gotest(t *testing.T, args []string, runs ...string) (int, string, [][]string, string) {
	t.Helper()
	f := &fake{t: t, runs: runs}
	var out strings.Builder
	s := &session{goTest: f.goTest, out: &out, summary: filepath.Join(t.TempDir(), "summary.md")}
	code := s.main(args)
	t.Log(out.String())
	if len(f.calls) != len(runs) {
		t.Errorf("go test ran %d times, want %d: %q", len(f.calls), len(runs), f.calls)
	}
	summary, err := os.ReadFile(s.summary)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return code, out.String(), f.calls, string(summary)
}

func want(t *testing.T, what, got string, in, out []string) {
	t.Helper()
	for _, s := range in {
		if !strings.Contains(got, s) {
			t.Errorf("%s lacks %q", what, s)
		}
	}
	for _, s := range out {
		if strings.Contains(got, s) {
			t.Errorf("%s has %q", what, s)
		}
	}
}

func wantArgs(t *testing.T, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("go %q, want go %q", got, want)
	}
}

func TestPass(t *testing.T) {
	code, out, calls, summary := gotest(t, []string{"-count=1", "./pass/"}, "pass")
	if code != 0 {
		t.Errorf("exit %d, want 0", code)
	}
	wantArgs(t, calls[0], "test", "-count=1", "-json", "./pass/")
	// As go test prints it: a passing package's line alone.
	if out != "ok  \texample.com/demo/pass\t0.010s\n" {
		t.Errorf("output %q, want the ok line alone", out)
	}
	if summary != "" {
		t.Errorf("the summary has %q, want nothing", summary)
	}
}

func TestVerbose(t *testing.T) {
	_, out, calls, _ := gotest(t, []string{"-count=1", "-v", "./pass/"}, "pass")
	wantArgs(t, calls[0], "test", "-count=1", "-v", "-json", "./pass/")
	want(t, "the output", out, []string{"=== RUN   TestOne\n    pass_test.go:6: one is fine\n--- PASS: TestOne", "PASS\nok  \texample.com/demo/pass"}, nil)
}

func TestFailThenPass(t *testing.T) {
	code, out, calls, summary := gotest(t, []string{"-count=1", "./flaky/"}, "fail", "fail-rerun-pass")
	if code != 0 {
		t.Errorf("exit %d, want 0", code)
	}
	wantArgs(t, calls[1], "test", "-json", "-run", "^(TestFlaky)$", "-count=1", "example.com/demo/flaky")
	// The failure in full, then the run again, then the warning.
	want(t, "the output", out, []string{
		"    flaky_test.go:12: waited for the device\n    flaky_test.go:13: the device didn't answer\n--- FAIL: TestFlaky (0.00s)\nFAIL\nFAIL\texample.com/demo/flaky\t0.010s\n",
		"gotest: running TestFlaky of example.com/demo/flaky again (run 2 of 4)\nok  \texample.com/demo/flaky\t0.011s\n",
		"::warning title=Flaky test::example.com/demo/flaky TestFlaky failed 1 time, then passed\n",
	}, []string{"TestBefore", "TestAfter", "Its last output"})
	want(t, "the summary", summary, []string{"### Flaky tests", "- example.com/demo/flaky TestFlaky failed 1 time, then passed\n"}, nil)
}

func TestFailEveryTime(t *testing.T) {
	code, out, calls, summary := gotest(t, []string{"-count=1", "./flaky/"}, "fail", "fail-rerun-fail", "fail-rerun-fail", "fail-rerun-fail")
	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	for _, c := range calls[1:] {
		wantArgs(t, c, "test", "-json", "-run", "^(TestFlaky)$", "-count=1", "example.com/demo/flaky")
	}
	_, last, _ := strings.Cut(out, "(run 4 of 4)")
	want(t, "the output after the last run", last, []string{
		"gotest: example.com/demo/flaky TestFlaky failed 4 times. Its last output:\n    flaky_test.go:12: waited for the device\n    flaky_test.go:13: the device didn't answer\n--- FAIL: TestFlaky (0.00s)\n",
	}, []string{"Flaky test", "=== RUN"})
	if summary != "" {
		t.Errorf("the summary has %q, want nothing", summary)
	}
}

func TestAttempts(t *testing.T) {
	code, _, _, _ := gotest(t, []string{"-attempts", "1", "-count=1", "./flaky/"}, "fail", "fail-rerun-fail")
	if code != 1 {
		t.Errorf("-attempts 1: exit %d, want 1", code)
	}
	code, out, _, _ := gotest(t, []string{"-attempts=0", "-count=1", "./flaky/"}, "fail")
	if code != 1 {
		t.Errorf("-attempts=0: exit %d, want 1", code)
	}
	want(t, "the output", out, []string{"TestFlaky failed 1 time. Its last output:"}, []string{"again"})
}

func TestSubtest(t *testing.T) {
	code, out, calls, _ := gotest(t, []string{"-count=1", "./subtest/"}, "subtest", "subtest-rerun-pass")
	if code != 0 {
		t.Errorf("exit %d, want 0", code)
	}
	wantArgs(t, calls[1], "test", "-json", "-run", "^(TestTable)$", "-count=1", "example.com/demo/subtest")
	// The subtest that failed, not those that passed.
	first, _, _ := strings.Cut(out, "gotest:")
	want(t, "the first run's output", first, []string{"    subtest_test.go:12: b: got 1, want 2\n--- FAIL: TestTable/b (0.00s)\n--- FAIL: TestTable (0.00s)\n"},
		[]string{"TestTable/a", "TestTable/c", "TestOther"})
	want(t, "the output", out, []string{"::warning title=Flaky test::example.com/demo/subtest TestTable failed 1 time, then passed\n"}, nil)
}

func TestPanicOutsideTests(t *testing.T) {
	code, out, calls, _ := gotest(t, []string{"-count=1", "./mainpanic/"}, "panic", "panic-rerun-pass")
	if code != 0 {
		t.Errorf("exit %d, want 0", code)
	}
	wantArgs(t, calls[1], "test", "-count=1", "-json", "example.com/demo/mainpanic")
	want(t, "the output", out, []string{
		"panic: the server didn't stop\n",
		"gotest: running example.com/demo/mainpanic again, all of it (run 2 of 4)\n",
		"::warning title=Flaky test::example.com/demo/mainpanic (outside its tests) failed 1 time, then passed\n",
	}, nil)
}

func TestPanicInTest(t *testing.T) {
	// The panic ended the test binary before TestAfter ran: all of the
	// package runs again.
	code, out, calls, _ := gotest(t, []string{"-count=1", "./testpanic/"}, "testpanic", "testpanic-rerun-pass")
	if code != 0 {
		t.Errorf("exit %d, want 0", code)
	}
	wantArgs(t, calls[1], "test", "-count=1", "-json", "example.com/demo/testpanic")
	want(t, "the output", out, []string{
		"--- FAIL: TestPanic (0.00s)\npanic: assignment to entry in nil map",
		"::warning title=Flaky test::example.com/demo/testpanic TestPanic failed 1 time, then passed\n",
	}, nil)
}

func TestTimeout(t *testing.T) {
	code, out, calls, _ := gotest(t, []string{"-count=1", "-timeout", "2s", "./timeout/"}, "timeout", "timeout-rerun-pass")
	if code != 0 {
		t.Errorf("exit %d, want 0", code)
	}
	wantArgs(t, calls[1], "test", "-count=1", "-timeout", "2s", "-json", "example.com/demo/timeout")
	want(t, "the output", out, []string{
		"panic: test timed out after 2s\n\trunning tests:\n\t\tTestSlow (2s)\n",
		"::warning title=Flaky test::example.com/demo/timeout TestSlow failed 1 time, then passed\n",
	}, nil)
}

func TestTimeoutEveryTime(t *testing.T) {
	code, out, _, _ := gotest(t, []string{"-attempts=1", "-count=1", "-timeout", "2s", "./timeout/"}, "timeout", "timeout")
	if code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
	_, last, _ := strings.Cut(out, "TestSlow failed 2 times. Its last output:\n")
	want(t, "the last output", last, []string{"panic: test timed out after 2s\n", "timeout_test.go:13"}, []string{"TestQuick", "TestAfter"})
}

func TestBuildFailure(t *testing.T) {
	for _, tt := range []struct{ run, says string }{
		{"build", `build\build_test.go:6:14: cannot use "three" (untyped string constant) as int value in variable declaration`},
		{"vet", `vet\vet_test.go:9:14: fmt.Printf format %d has arg "three" of wrong type string`},
	} {
		code, out, _, _ := gotest(t, []string{"-count=1", "./" + tt.run + "/"}, tt.run)
		if code != 1 {
			t.Errorf("%s: exit %d, want 1", tt.run, code)
		}
		want(t, tt.run+"'s output", out, []string{
			tt.says,
			"FAIL\texample.com/demo/" + tt.run + " [build failed]\n",
			"gotest: example.com/demo/" + tt.run + " doesn't build, or go vet failed: that isn't run again.\n",
		}, []string{"again ("})
	}
}

func TestPackages(t *testing.T) {
	// Of two packages, only the one that failed runs again.
	code, out, calls, _ := gotest(t, []string{"-count=1", "./pass/", "./flaky/"}, "mixed", "fail-rerun-pass")
	if code != 0 {
		t.Errorf("exit %d, want 0", code)
	}
	wantArgs(t, calls[1], "test", "-json", "-run", "^(TestFlaky)$", "-count=1", "example.com/demo/flaky")
	want(t, "the output", out, []string{"ok  \texample.com/demo/pass\t0.011s\n", "TestFlaky failed 1 time, then passed"}, []string{"TestOne"})
}

func TestVerbosePackages(t *testing.T) {
	// With -v, one package's lines at a time, though their events mix.
	_, out, _, _ := gotest(t, []string{"-count=1", "-v", "./pass/", "./flaky/"}, "mixed", "fail-rerun-pass")
	pass := strings.Index(out, "=== RUN   TestOne")
	ok := strings.Index(out, "ok  \texample.com/demo/pass")
	flaky := strings.Index(out, "=== RUN   TestBefore")
	if pass < 0 || ok < pass || flaky < ok {
		t.Errorf("pass's lines from %d to %d, then flaky's from %d: want them apart", pass, ok, flaky)
	}
}

func TestGoFails(t *testing.T) {
	// go test failed before any package did, as when it finds none.
	var out strings.Builder
	runs := 0
	s := &session{out: &out, goTest: func(args []string, line func([]byte)) error {
		runs++
		return &exec.ExitError{}
	}}
	if code := s.main([]string{"./nothing/"}); code != 1 || runs != 1 {
		t.Errorf("exit %d after %d runs, want 1 after 1", code, runs)
	}
	want(t, "the output", out.String(), []string{"no package did"}, nil)
}

func TestNoGo(t *testing.T) {
	var out strings.Builder
	s := &session{out: &out, goTest: func(args []string, line func([]byte)) error {
		return errors.New(`exec: "go": executable file not found in %PATH%`)
	}}
	if code := s.main([]string{"./..."}); code != 1 {
		t.Errorf("exit %d, want 1", code)
	}
}

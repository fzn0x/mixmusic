package main

import (
	"slices"
	"testing"
)

func TestParse(t *testing.T) {
	for _, tt := range []struct {
		args              []string
		all, whole, tests []string
		verbose           bool
		attempts          int
	}{
		{
			// CI's key checks: the packages first, the flags after.
			args:    []string{"./internal/keypad/", "-run", "TestWatch|TestPlusPressed", "-count=1", "-v"},
			all:     []string{"test", "-run", "TestWatch|TestPlusPressed", "-count=1", "-v", "-json", "./internal/keypad/"},
			whole:   []string{"test", "-run", "TestWatch|TestPlusPressed", "-count=1", "-v", "-json", "p"},
			tests:   []string{"test", "-v", "-json", "-run", "^(TestA|TestB)$", "-count=1", "p"},
			verbose: true, attempts: 3,
		},
		{
			// -C stays first; -json and -attempts are gotest's; -args
			// stays last.
			args:     []string{"-C", "dir", "-json", "-attempts=1", "-tags", "x y", "-race", "./...", "-args", "-v", "./not-a-package"},
			all:      []string{"test", "-C", "dir", "-tags", "x y", "-race", "-json", "./...", "-args", "-v", "./not-a-package"},
			whole:    []string{"test", "-C", "dir", "-tags", "x y", "-race", "-json", "p", "-args", "-v", "./not-a-package"},
			tests:    []string{"test", "-C", "dir", "-tags", "x y", "-race", "-json", "-run", "^(TestA|TestB)$", "-count=1", "p", "-args", "-v", "./not-a-package"},
			attempts: 1,
		},
		{
			args:     []string{"-v=false", "--count", "2", "-test.run=X", "-attempts", "5"},
			all:      []string{"test", "-v=false", "--count", "2", "-test.run=X", "-json"},
			whole:    []string{"test", "-v=false", "--count", "2", "-test.run=X", "-json", "p"},
			tests:    []string{"test", "-v=false", "-json", "-run", "^(TestA|TestB)$", "-count=1", "p"},
			attempts: 5,
		},
		{
			args:     []string{"-test.v", "-timeout", "10m"},
			all:      []string{"test", "-test.v", "-timeout", "10m", "-json"},
			whole:    []string{"test", "-test.v", "-timeout", "10m", "-json", "p"},
			tests:    []string{"test", "-test.v", "-timeout", "10m", "-json", "-run", "^(TestA|TestB)$", "-count=1", "p"},
			verbose:  true,
			attempts: 3,
		},
	} {
		c, err := parse(tt.args)
		if err != nil {
			t.Errorf("%q: %v", tt.args, err)
			continue
		}
		for _, got := range []struct {
			what      string
			got, want []string
		}{
			{"the first run", c.all(), tt.all},
			{"a run of all of a package", c.whole("p"), tt.whole},
			{"a run of its tests", c.tests("p", []string{"TestA", "TestB"}), tt.tests},
		} {
			if !slices.Equal(got.got, got.want) {
				t.Errorf("%q: %s is go %q, want go %q", tt.args, got.what, got.got, got.want)
			}
		}
		if c.verbose != tt.verbose || c.attempts != tt.attempts {
			t.Errorf("%q: verbose %v, %d attempts; want %v, %d", tt.args, c.verbose, c.attempts, tt.verbose, tt.attempts)
		}
	}
}

func TestParseFailfast(t *testing.T) {
	c, _ := parse([]string{"-failfast", "./..."})
	if !c.failfast {
		t.Error("-failfast isn't seen")
	}
}

func TestParseAttempts(t *testing.T) {
	for _, bad := range [][]string{{"-attempts", "x"}, {"-attempts=-1"}} {
		if _, err := parse(bad); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

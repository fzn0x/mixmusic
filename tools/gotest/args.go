package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// command is a go test command line, taken apart: go test's flags, its
// packages, and what -args passes to the test binaries.
type command struct {
	flags    []flag
	packages []string
	binary   []string // -args and what follows it
	verbose  bool     // -v: every test's output, not only a failed one's
	failfast bool     // -failfast: a failed test stops the others
	attempts int      // the runs again of a failed test
}

// flag is one of go test's flags, with its value when it takes the next
// argument: its name, without dashes or test., and its arguments.
type flag struct {
	name string
	args []string
}

// boolFlags are go test's flags that take no value but with =: any other
// flag without = takes the next argument as its value.
var boolFlags = map[string]bool{
	"a": true, "asan": true, "benchmem": true, "buildvcs": true, "c": true,
	"cover": true, "failfast": true, "fullpath": true, "json": true,
	"linkshared": true, "modcacherw": true, "msan": true, "n": true,
	"paniconexit0": true, "race": true, "short": true, "trimpath": true,
	"v": true, "work": true, "x": true,
}

// parse takes go test's arguments apart, as go test reads them: flags and
// packages in any order, then -args. It takes out -attempts, which is
// gotest's, and -json, which gotest gives every run.
func parse(args []string) (command, error) {
	c := command{attempts: 3}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-args" || a == "--args" {
			c.binary = args[i:]
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			c.packages = append(c.packages, a)
			continue
		}
		name, value, hasValue := strings.Cut(strings.TrimLeft(a, "-"), "=")
		name = strings.TrimPrefix(name, "test.")
		f := flag{name, []string{a}}
		if !hasValue && !boolFlags[name] && i+1 < len(args) {
			i++
			value = args[i]
			f.args = append(f.args, value)
		}
		switch name {
		case "attempts":
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 {
				return c, fmt.Errorf("-attempts %q: want the number of runs again of a failed test, as 3", value)
			}
			c.attempts = n
			continue
		case "json":
			continue
		case "v":
			c.verbose = on(hasValue, value)
		case "failfast":
			c.failfast = on(hasValue, value)
		}
		c.flags = append(c.flags, f)
	}
	return c, nil
}

// on tells whether a boolean flag is set: given alone, or as true.
func on(hasValue bool, value string) bool {
	b, err := strconv.ParseBool(value)
	return !hasValue || err != nil || b
}

// all is the first run: go test as given, with -json.
func (c command) all() []string {
	return c.args(c.flags, nil, c.packages)
}

// whole runs every test of a package again, as the first run did.
func (c command) whole(pkg string) []string {
	return c.args(c.flags, nil, []string{pkg})
}

// tests runs the top-level tests named of a package again, once each.
func (c command) tests(pkg string, names []string) []string {
	var flags []flag
	for _, f := range c.flags {
		if f.name != "run" && f.name != "count" {
			flags = append(flags, f)
		}
	}
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = regexp.QuoteMeta(n)
	}
	run := []string{"-run", "^(" + strings.Join(quoted, "|") + ")$", "-count=1"}
	return c.args(flags, run, []string{pkg})
}

// args is go test's command line: the flags given, then -json, the flags
// of the run, the packages and -args. -C, which must come first, stays
// first.
func (c command) args(flags []flag, run, packages []string) []string {
	args := []string{"test"}
	for _, f := range flags {
		args = append(args, f.args...)
	}
	args = append(args, "-json")
	args = append(args, run...)
	args = append(args, packages...)
	return append(args, c.binary...)
}

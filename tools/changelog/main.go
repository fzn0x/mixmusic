// Command changelog folds the changelog's fragments into CHANGELOG.md. A
// change people will notice adds a file to changelog.d, in the folder of
// its section, rather than lines to CHANGELOG.md, so that pull requests
// open at the same time never change the same lines:
// changelog.d/fixed/101-stream-retry.md holds the entry, one Markdown list
// item or more, written as CHANGELOG.md has them. A release folds them into
// ## [Unreleased], after the entries there, and deletes them.
//
//	go run ./tools/changelog          # fold the fragments into CHANGELOG.md
//	go run ./tools/changelog -print   # print ## [Unreleased] with them folded in
//	go run ./tools/changelog -check   # fail when a fragment is invalid
package main

import (
	"cmp"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// section is a folder of changelog.d and the heading its fragments fold
// under.
type section struct{ folder, heading string }

// sections are those of Keep a Changelog, in its order.
var sections = []section{
	{"added", "Added"},
	{"changed", "Changed"},
	{"deprecated", "Deprecated"},
	{"removed", "Removed"},
	{"fixed", "Fixed"},
	{"security", "Security"},
}

// unreleased heads the section the fragments fold into.
const unreleased = "## [Unreleased]"

// fragment is an entry waiting in changelog.d.
type fragment struct {
	name    string // as messages name it: changelog.d/fixed/101-stream-retry.md
	file    string
	heading string
	text    string // without its trailing newline
}

func main() {
	check := flag.Bool("check", false, "check the fragments instead of folding them")
	show := flag.Bool("print", false, "print ## [Unreleased] with the fragments folded in, changing nothing")
	root := flag.String("root", ".", "the repository's root")
	flag.Parse()
	frags, err := fragments(filepath.Join(*root, "changelog.d"))
	if err != nil {
		log.Fatalf("invalid fragments:\n%v", err)
	}
	file := filepath.Join(*root, "CHANGELOG.md")
	data, err := os.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}
	folded, notes, err := fold(string(data), frags)
	if err != nil {
		log.Fatal(err)
	}
	switch {
	case *check:
		fmt.Printf("ok: %d fragments\n", len(frags))
	case *show:
		fmt.Print(notes)
	case len(frags) == 0:
		fmt.Println("no fragments to fold")
	default:
		if err := os.WriteFile(file, []byte(folded), 0o644); err != nil {
			log.Fatal(err)
		}
		for _, f := range frags {
			if err := os.Remove(f.file); err != nil {
				log.Fatal(err)
			}
		}
		// A section's folder goes with its last fragment; one that keeps
		// other files stays.
		for _, s := range sections {
			os.Remove(filepath.Join(*root, "changelog.d", s.folder))
		}
		fmt.Printf("folded %d fragments into CHANGELOG.md\n", len(frags))
	}
}

// fragments reads and checks the fragments of changelog.d: section by
// section, each in the order of the issue number its name starts with.
// changelog.d may also hold a README.md.
func fragments(dir string) ([]fragment, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	headings := map[string]string{}
	var folders []string
	for _, s := range sections {
		headings[s.folder] = s.heading
		folders = append(folders, s.folder)
	}
	byFolder := map[string][]fragment{}
	var errs []error
	for _, e := range entries {
		heading, ok := headings[e.Name()]
		switch {
		case !e.IsDir() && e.Name() == "README.md":
			continue
		case !e.IsDir() || !ok:
			errs = append(errs, fmt.Errorf("changelog.d/%s: fragments go in the folder of their section: %s", e.Name(), strings.Join(folders, ", ")))
			continue
		}
		files, err := os.ReadDir(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			name := "changelog.d/" + e.Name() + "/" + f.Name()
			if f.IsDir() || filepath.Ext(f.Name()) != ".md" {
				errs = append(errs, fmt.Errorf("%s: a fragment is a .md file", name))
				continue
			}
			file := filepath.Join(dir, e.Name(), f.Name())
			data, err := os.ReadFile(file)
			if err != nil {
				return nil, err
			}
			text, err := entry(string(data))
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", name, err))
				continue
			}
			byFolder[e.Name()] = append(byFolder[e.Name()], fragment{name, file, heading, text})
		}
	}
	var frags []fragment
	for _, s := range sections {
		in := byFolder[s.folder]
		slices.SortFunc(in, func(a, b fragment) int {
			return compareNames(filepath.Base(a.name), filepath.Base(b.name))
		})
		frags = append(frags, in...)
	}
	return frags, errors.Join(errs...)
}

// entry checks a fragment's text and returns it without its trailing
// newlines: list items as CHANGELOG.md has them, under the heading its
// folder names.
func entry(text string) (string, error) {
	text = strings.TrimRight(strings.ReplaceAll(text, "\r\n", "\n"), " \t\n")
	if text == "" {
		return "", errors.New("it is empty")
	}
	if !strings.HasPrefix(text, "- ") {
		return "", errors.New(`an entry starts with "- ", as an item of a list`)
	}
	for i, line := range strings.Split(text, "\n") {
		switch {
		case strings.HasPrefix(line, "#"):
			return "", fmt.Errorf("line %d is a heading: the folder names the section", i+1)
		case strings.TrimRight(line, " \t") != line:
			return "", fmt.Errorf("line %d ends in spaces", i+1)
		}
	}
	return text, nil
}

// compareNames orders fragments by the issue number their name starts
// with, so that 94 comes before 101, then by name. Names without a number
// come last.
func compareNames(a, b string) int {
	na, okA := number(a)
	nb, okB := number(b)
	switch {
	case okA && !okB:
		return -1
	case okB && !okA:
		return 1
	case okA && na != nb:
		return cmp.Compare(na, nb)
	}
	return strings.Compare(a, b)
}

// number returns the number name starts with, if it does.
func number(name string) (int, bool) {
	end := strings.IndexFunc(name, func(r rune) bool { return r < '0' || r > '9' })
	if end <= 0 {
		return 0, false
	}
	n, err := strconv.Atoi(name[:end])
	return n, err == nil
}

// fold returns changelog with the fragments added to the end of their
// sections of ## [Unreleased], and that section as it then reads. A
// section it lacks is added where Keep a Changelog orders it.
func fold(changelog string, frags []fragment) (string, string, error) {
	changelog = strings.ReplaceAll(changelog, "\r\n", "\n")
	start := strings.Index(changelog, "\n"+unreleased+"\n")
	if start < 0 {
		return "", "", fmt.Errorf("CHANGELOG.md has no %q section", unreleased)
	}
	start++
	body := start + len(unreleased) + 1
	end := len(changelog)
	if i := strings.Index(changelog[body:], "\n## ["); i >= 0 {
		end = body + i + 1
	}
	lines := strings.Split(changelog[body:end], "\n")
	for _, s := range sections {
		var add []string
		for _, f := range frags {
			if f.heading == s.heading {
				add = append(add, strings.Split(f.text, "\n")...)
			}
		}
		if len(add) > 0 {
			lines = insert(lines, s.heading, add)
		}
	}
	notes := unreleased + "\n" + strings.Join(lines, "\n")
	return changelog[:start] + notes + changelog[end:], notes, nil
}

// insert adds entries at the end of the ### heading part of lines, those
// of ## [Unreleased] under its heading, and adds that part where Keep a
// Changelog orders it when lines lack it.
func insert(lines []string, heading string, entries []string) []string {
	at := slices.Index(lines, "### "+heading)
	if at >= 0 {
		end := len(lines)
		for i := at + 1; i < len(lines); i++ {
			if strings.HasPrefix(lines[i], "### ") {
				end = i
				break
			}
		}
		end = lastText(lines, at+1, end)
		if end == at+1 {
			entries = append([]string{""}, entries...)
		}
		return slices.Insert(lines, end, entries...)
	}
	order := slices.IndexFunc(sections, func(s section) bool { return s.heading == heading })
	part := append([]string{"### " + heading, ""}, entries...)
	for i, line := range lines {
		later := slices.IndexFunc(sections, func(s section) bool { return line == "### "+s.heading })
		if later > order {
			return slices.Insert(lines, i, append(part, "")...)
		}
	}
	return slices.Insert(lines, lastText(lines, 0, len(lines)), append([]string{""}, part...)...)
}

// lastText returns the index after the last line of lines[from:to] with
// text in it, or from.
func lastText(lines []string, from, to int) int {
	for to > from && strings.TrimSpace(lines[to-1]) == "" {
		to--
	}
	return to
}

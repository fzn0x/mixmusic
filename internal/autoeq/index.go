// Package autoeq lists AutoEq's corrections of headphones, and fetches
// them, as the user opens the list and picks one, from AutoEq's repository
// on GitHub (#616): MixMusic ships none of them. AutoEq, by Jaakko
// Pasanen, is under the MIT license; each profile is made from the
// measurements of the source it names.
package autoeq

import (
	"bufio"
	"bytes"
	"cmp"
	"errors"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

// Entry is a profile of AutoEq's index.
type Entry struct {
	// Name is the model's, as "1MORE Piston", with what sets the profile
	// apart in parentheses, as "(ANC Off)".
	Name string
	// Source measured it, as "crinacle", on Rig, as "711", or on a rig
	// the index doesn't name, "".
	Source, Rig string
	// Form is the headphones', "in-ear", "over-ear" or "earbud", as the
	// profile's folder says, or "".
	Form string
	// Path is the profile's folder in AutoEq's results, as
	// "crinacle/711 in-ear/1MORE Piston", which names it.
	Path string
}

// ErrEmptyIndex is an index that lists no profile.
var ErrEmptyIndex = errors.New("AutoEq's index lists no profiles")

// indexLine is a profile of the index, a link to its folder, by its source
// and on its rig: "- [1MORE Piston](./crinacle/711%20in-ear/1MORE%20Piston)
// by crinacle on 711".
var indexLine = regexp.MustCompile(`^- \[(.+?)\]\(\./(.+)\) by (.+?)(?: on (.+))?$`)

// forms are the forms of headphones the folders of the profiles end with.
var forms = []string{"in-ear", "over-ear", "earbud"}

// ParseIndex reads AutoEq's results/INDEX.md: its profiles, by name,
// ignoring case, those of a name in the index's order. Lines that aren't a
// profile, or name a folder out of the results, are left out.
func ParseIndex(data []byte) ([]Entry, error) {
	var entries []Entry
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 4096), 1<<20)
	for sc.Scan() {
		m := indexLine.FindStringSubmatch(strings.TrimRight(sc.Text(), "\r "))
		if m == nil {
			continue
		}
		path, ok := folder(m[2])
		if !ok {
			continue
		}
		e := Entry{Name: m[1], Source: m[3], Rig: m[4], Path: path}
		parts := strings.Split(path, "/")
		for _, f := range forms {
			if parts[1] == f || strings.HasSuffix(parts[1], " "+f) {
				e.Form = f
			}
		}
		entries = append(entries, e)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, ErrEmptyIndex
	}
	slices.SortStableFunc(entries, func(a, b Entry) int {
		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return entries, nil
}

// folder is the folder of a link of the index, its escapes undone: three
// names or more, none empty, none "." or "..", and none with a backslash.
func folder(link string) (string, bool) {
	parts := strings.Split(link, "/")
	if len(parts) < 3 {
		return "", false
	}
	for i, p := range parts {
		name, err := url.PathUnescape(p)
		if err != nil || name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
			return "", false
		}
		parts[i] = name
	}
	return strings.Join(parts, "/"), true
}

// Model is a model of headphones in the index, with its profiles: one for
// each source, rig and variant that shares its name.
type Model struct {
	Name     string
	Profiles []Entry
}

// Models groups entries, by name as ParseIndex sorts them, by model.
func Models(entries []Entry) []Model {
	var models []Model
	for _, e := range entries {
		if n := len(models); n > 0 && models[n-1].Name == e.Name {
			models[n-1].Profiles = append(models[n-1].Profiles, e)
			continue
		}
		models = append(models, Model{Name: e.Name, Profiles: []Entry{e}})
	}
	return models
}

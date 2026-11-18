package locales

import (
	"embed"
	"encoding/json"
	"io/fs"
	"slices"
	"strings"
	"sync"
)

// more holds the catalogs of more/, which MixMusic adds to Catalog as it
// starts. MyGo's generate, check and build read the JSON files at the
// root of locales/ alone, so they don't see these: every new language
// lands in more/ first, as a file no other pull request touches, and the
// languages MyGo's Windows installer lacks stay there. more/README.md
// tells how one moves to the root.
//
// MyGo's own words come from a catalog's mygo.<key> messages. MyGo's
// packages of its own translations are linked by catalog_gen.go's
// imports, for the languages at the root alone.
//
//go:embed more
var more embed.FS

// The catalogs of more/ join Catalog before anything reads the app's
// languages. This file's init runs after catalog_gen.go's, which makes
// and registers Catalog: Go initializes a package's files in the order of
// their names.
func init() {
	sub, err := fs.Sub(more, "more")
	if err == nil {
		err = AddTranslations(sub)
	}
	if err != nil {
		panic(err)
	}
}

// added holds the files of the languages AddTranslations added, by tag.
var added sync.Map // string -> fs.FS

// AddTranslations adds the catalogs at the root of fsys, "<tag>.json", to
// Catalog, whose languages they become: the Language setting lists them,
// and the computer's languages pick them. It fails on a file not named
// after a language, and on a language Catalog has already.
func AddTranslations(fsys fs.FS) error {
	if err := Catalog.Add(fsys); err != nil {
		return err
	}
	names, err := fs.Glob(fsys, "*.json")
	if err != nil {
		return err
	}
	for _, name := range names {
		added.Store(strings.TrimSuffix(name, ".json"), fsys)
	}
	return nil
}

// fileMarks is what a catalog's file says of its translation, in the
// attributes after "@@locale".
type fileMarks struct {
	Machine     bool     `json:"@@x-machine-translated"`
	Translators []string `json:"@@x-translators"`
}

// marks caches the fileMarks of each language, by tag.
var marks sync.Map // string -> fileMarks

// marksOf reads the fileMarks of the catalog of tag the first time they
// are asked of a language. Each attribute is read alone, so that one of
// the wrong kind loses only itself.
func marksOf(tag string) fileMarks {
	if m, ok := marks.Load(tag); ok {
		return m.(fileMarks)
	}
	var fsys fs.FS = files
	if f, ok := added.Load(tag); ok {
		fsys = f.(fs.FS)
	}
	var m fileMarks
	var attrs map[string]json.RawMessage
	if b, err := fs.ReadFile(fsys, tag+".json"); err == nil && json.Unmarshal(b, &attrs) == nil {
		_ = json.Unmarshal(attrs["@@x-machine-translated"], &m.Machine)
		var names []string
		if json.Unmarshal(attrs["@@x-translators"], &names) == nil {
			for _, n := range names {
				if n = strings.TrimSpace(n); n != "" {
					m.Translators = append(m.Translators, n)
				}
			}
		}
	}
	marks.Store(tag, m)
	return m
}

// TranslatedByMachine reports whether the catalog of tag, one of the
// app's languages as "hi" or "pt-BR", was translated by machine and no
// speaker of it has read it since: its file says
//
//	"@@x-machine-translated": true
func TranslatedByMachine(tag string) bool { return marksOf(tag).Machine }

// Translators returns the people who translated the catalog of tag, or
// fixed its translation, as its file names them, in its order:
//
//	"@@x-translators": ["Ada", "Grace"]
func Translators(tag string) []string { return slices.Clone(marksOf(tag).Translators) }

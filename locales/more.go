package locales

import (
	"embed"
	"encoding/json"
	"io/fs"
	"strings"
	"sync"

	// MyGo's own strings in the languages MyGo has them in, for a catalog
	// of more/ in one of them: catalog_gen.go imports them only for the
	// languages at the root of locales/.
	_ "github.com/egoist/mygo/locales/ja"
	_ "github.com/egoist/mygo/locales/zh-Hans"
	_ "github.com/egoist/mygo/locales/zh-Hant"
)

// more holds the catalogs of more/, which MixMusic adds to Catalog as it
// starts. MyGo's generate, check and build read the JSON files at the
// root of locales/ alone, so they don't see these: every translation
// lands in more/ first, as a file no other pull request touches, and the
// languages MyGo's Windows installer lacks stay there. more/README.md
// tells how one moves to the root.
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

// byMachine caches TranslatedByMachine, by tag.
var byMachine sync.Map // string -> bool

// TranslatedByMachine reports whether the catalog of tag, one of the
// app's languages as "hi" or "pt-BR", was translated by machine and no
// speaker of it has read it since: its file says
//
//	"@@x-machine-translated": true
//
// It reads the file the first time it is asked of a language.
func TranslatedByMachine(tag string) bool {
	if by, ok := byMachine.Load(tag); ok {
		return by.(bool)
	}
	var fsys fs.FS = files
	if f, ok := added.Load(tag); ok {
		fsys = f.(fs.FS)
	}
	var mark struct {
		Machine bool `json:"@@x-machine-translated"`
	}
	by := false
	if b, err := fs.ReadFile(fsys, tag+".json"); err == nil && json.Unmarshal(b, &mark) == nil {
		by = mark.Machine
	}
	byMachine.Store(tag, by)
	return by
}

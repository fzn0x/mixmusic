package locales

import (
	"embed"
	"io/fs"

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

// AddTranslations adds the catalogs at the root of fsys, "<tag>.json", to
// Catalog, whose languages they become: the Language setting lists them,
// and the computer's languages pick them. It fails on a file not named
// after a language, and on a language Catalog has already.
func AddTranslations(fsys fs.FS) error {
	return Catalog.Add(fsys)
}

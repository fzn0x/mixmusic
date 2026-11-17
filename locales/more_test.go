package locales

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/egoist/mygo/i18n"
)

// addFixture adds testdata/qaa.json to Catalog, once: a catalog in a
// language of private use, which no translation will be.
var addFixture = sync.OnceValue(func() error { return AddTranslations(os.DirFS("testdata")) })

// addedCatalogs are the files of the catalogs added as the app starts,
// more/*.json, and the fixture's.
func addedCatalogs(t *testing.T) []string {
	t.Helper()
	var paths []string
	for _, dir := range []string{"more", "testdata"} {
		files, err := filepath.Glob(filepath.Join(dir, "*.json"))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			paths = append(paths, filepath.ToSlash(f))
		}
	}
	return paths
}

// TestAddTranslations adds a catalog to the app's: its language is one of
// the app's, which the computer's languages pick, and says its messages,
// MyGo's own among them, and English's where it lacks one.
func TestAddTranslations(t *testing.T) {
	if err := addFixture(); err != nil {
		t.Fatal(err)
	}
	if langs := i18n.Languages(); !slices.Contains(langs, "qaa") || !slices.Contains(Catalog.Languages(), "qaa") {
		t.Errorf("the app's languages are %v, the catalog's %v", langs, Catalog.Languages())
	}
	if got := i18n.Match([]string{"qaa", "en-US"}, i18n.Languages()); got != "qaa" {
		t.Errorf("a reader of qaa gets %q", got)
	}
	qaa := In("qaa")
	for _, say := range [][2]string{
		{qaa.SettingsTabAbout(), "qaa: about"},
		{qaa.AlreadyInTracks(1), "qaa: 1 tracks"},
		{qaa.AboutVersion(), In("en").AboutVersion()},
		{Catalog.Format("qaa", "mygo.copy", nil), "qaa: Copy"},
	} {
		if say[0] != say[1] {
			t.Errorf("qaa says %q, want %q", say[0], say[1])
		}
	}
	// A language the catalog has already, and a file not named after one,
	// are refused.
	for _, name := range []string{"qaa.json", "en.json", "pt_BR.json"} {
		if err := AddTranslations(fstest.MapFS{name: {Data: []byte(`{}`)}}); err == nil {
			t.Errorf("added %s", name)
		}
	}
}

// TestAddedCatalogsLoad finds each language of more/ among the app's, and
// each message of its catalog said in it: one that doesn't parse would say
// English's.
func TestAddedCatalogsLoad(t *testing.T) {
	if err := addFixture(); err != nil {
		t.Fatal(err)
	}
	for _, path := range addedCatalogs(t) {
		tag := strings.TrimSuffix(filepath.Base(path), ".json")
		if !slices.Contains(i18n.Languages(), tag) {
			t.Errorf("%s: the app's languages %v lack %s", path, i18n.Languages(), tag)
			continue
		}
		for key := range readCatalog(t, strings.TrimSuffix(path, ".json")).messages {
			m, lang, ok := Catalog.Message(tag, key)
			if !ok || lang != tag {
				t.Errorf("%s: %s says %s's message, not its own", path, key, lang)
				continue
			}
			if m.Format(tag, sampleArgs(m)) == "" && m.String() != "" {
				t.Errorf("%s: %s says nothing", path, key)
			}
		}
	}
}

// sampleArgs are arguments for each of m's, of their kinds.
func sampleArgs(m *i18n.Message) map[string]any {
	args := map[string]any{}
	for _, a := range m.Args() {
		switch a.Kind {
		case "plural", "selectordinal", "number":
			args[a.Name] = 2
		case "date", "time":
			args[a.Name] = time.Date(2026, 1, 5, 13, 7, 0, 0, time.UTC)
		case "select":
			args[a.Name] = m.Cases(a.Name)[0]
		default:
			args[a.Name] = "x"
		}
	}
	return args
}

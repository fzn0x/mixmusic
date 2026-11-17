package locales

import (
	"bytes"
	"encoding/json"
	"fmt"
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

// TestTranslatedByMachine reads the mark of a catalog a machine
// translated, which a speaker's review removes.
func TestTranslatedByMachine(t *testing.T) {
	if err := addFixture(); err != nil {
		t.Fatal(err)
	}
	read := fstest.MapFS{"qac.json": {Data: []byte(`{"@@locale": "qac", "aboutContribute": "qac: contribute"}`)}}
	if err := AddTranslations(read); err != nil {
		t.Fatal(err)
	}
	// qtz, a private-use tag, stands for a language without a catalog: a
	// real one, as de, gets a machine's file in more/ sooner or later.
	for tag, want := range map[string]bool{"qaa": true, "qac": false, "en": false, "en-XA": false, "qtz": false} {
		if got := TranslatedByMachine(tag); got != want {
			t.Errorf("%s translated by machine: %v", tag, got)
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

// TestAddedCatalogsAreWellFormed holds each catalog of more/, and the
// fixture, to what MyGo's check and the tests of locales/ ask of the
// catalogs there: a file named after its language's tag in its canonical
// form, which locales/ has no catalog of; "@@locale" first, saying the
// tag; "@@x-machine-translated" true, or not there; keys of en.json or of
// MyGo's own strings (mygo.<key>), each once, in order; and messages that
// parse, with en.json's arguments of the same kinds, its select cases,
// and the plural cases of their language with en.json's exact ones.
func TestAddedCatalogsAreWellFormed(t *testing.T) {
	source := map[string]*i18n.Message{}
	for k, v := range readCatalog(t, "en").messages {
		if m, err := i18n.ParseMessage(v); err == nil {
			source[k] = m
		}
	}
	for _, path := range addedCatalogs(t) {
		tag := strings.TrimSuffix(filepath.Base(path), ".json")
		if i18n.Normalize(tag) != tag || i18n.IsPseudo(tag) {
			t.Errorf("%s: name it after its language's BCP 47 tag in its canonical form, as hi.json or pt-BR.json", path)
			continue
		}
		if _, err := os.Stat(tag + ".json"); err == nil {
			t.Errorf("%s: locales/%s.json is the catalog of %s already", path, tag, tag)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		dec := json.NewDecoder(bytes.NewReader(b))
		if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
			t.Errorf("%s: not a JSON object of messages", path)
			continue
		}
		var keys []string
		seen := map[string]bool{}
		for dec.More() {
			tok, err := dec.Token()
			if err != nil {
				t.Errorf("%s: %v", path, err)
				break
			}
			key := tok.(string)
			var raw json.RawMessage
			if err := dec.Decode(&raw); err != nil {
				t.Errorf("%s: %s: %v", path, key, err)
				break
			}
			first := len(seen) == 0
			if seen[key] {
				t.Errorf("%s: %s comes twice", path, key)
			}
			seen[key] = true
			switch {
			case key == "@@locale":
				var locale string
				if !first || json.Unmarshal(raw, &locale) != nil || locale != tag {
					t.Errorf(`%s: "@@locale" comes first, and says %q`, path, tag)
				}
				continue
			case key == "@@x-machine-translated":
				if string(raw) != "true" {
					t.Errorf(`%s: "@@x-machine-translated" is true, or isn't there`, path)
				}
				continue
			case strings.HasPrefix(key, "@"):
				continue // ARB's other attributes and descriptions, which MyGo reads past
			}
			var msg string
			if json.Unmarshal(raw, &msg) != nil {
				t.Errorf("%s: %s is not a string", path, key)
				continue
			}
			keys = append(keys, key)
			got, err := i18n.ParseMessage(msg)
			if err != nil {
				t.Errorf("%s: %s: %v", path, key, err)
				continue
			}
			if ns, name, ok := strings.Cut(key, "."); ok {
				// MyGo's check, which catalogs.sh runs, knows MyGo's keys.
				if ns != "mygo" || name == "" {
					t.Errorf("%s: %s is a key of neither en.json nor MyGo (mygo.<key>)", path, key)
				}
				continue
			}
			want, ok := source[key]
			if !ok {
				t.Errorf("%s: %s is not a key of en.json", path, key)
				continue
			}
			if problem := unlikeEnglish(tag, want, got); problem != "" {
				t.Errorf("%s: %s %s", path, key, problem)
			}
		}
		if !seen["@@locale"] {
			t.Errorf(`%s: "@@locale": %q comes first`, path, tag)
		}
		for i := 1; i < len(keys); i++ {
			if keys[i] < keys[i-1] {
				t.Errorf("%s: %s comes after %s", path, keys[i], keys[i-1])
			}
		}
	}
}

// unlikeEnglish says how got, a message of the language tag, takes other
// arguments or cases than want, en.json's, as TestTranslationsMatchEnglish
// checks them, or "" when it doesn't.
func unlikeEnglish(tag string, want, got *i18n.Message) string {
	if a, b := argList(want), argList(got); !slices.Equal(a, b) {
		return fmt.Sprintf("has the arguments %v, en.json's %v", b, a)
	}
	var categories []string
	for _, p := range i18n.PluralCategories(tag) {
		categories = append(categories, p.String())
	}
	for _, a := range want.Args() {
		switch a.Kind {
		case "select":
			if !sameSet(want.Cases(a.Name), got.Cases(a.Name)) {
				return fmt.Sprintf("selects %s by %v, en.json's by %v", a.Name, got.Cases(a.Name), want.Cases(a.Name))
			}
		case "plural":
			exactW, _ := splitCases(want.Cases(a.Name))
			exactG, cats := splitCases(got.Cases(a.Name))
			if !sameSet(exactW, exactG) || !sameSet(cats, categories) {
				return fmt.Sprintf("counts %s by %v; %s has the cases %v, and en.json the exact ones %v", a.Name, got.Cases(a.Name), tag, categories, exactW)
			}
		}
	}
	return ""
}

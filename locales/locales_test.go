package locales

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/i18n"
)

// catalog is a catalog's messages by key, and its keys in the file's order.
type catalog struct {
	messages map[string]string
	keys     []string
}

// readCatalog reads the catalog of tag, its messages without the
// descriptions under "@" keys.
func readCatalog(t *testing.T, tag string) catalog {
	t.Helper()
	b, err := os.ReadFile(tag + ".json")
	if err != nil {
		t.Fatal(err)
	}
	// Read token by token, for the order of the keys, which a map forgets.
	c := catalog{messages: map[string]string{}}
	dec := json.NewDecoder(bytes.NewReader(b))
	if _, err := dec.Token(); err != nil {
		t.Fatal(tag, err)
	}
	for dec.More() {
		k, err := dec.Token()
		if err != nil {
			t.Fatal(tag, err)
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			t.Fatal(tag, err)
		}
		key := k.(string)
		if strings.HasPrefix(key, "@") {
			continue
		}
		var msg string
		if err := json.Unmarshal(v, &msg); err != nil {
			t.Fatalf("%s.json's %s: %v", tag, key, err)
		}
		c.messages[key] = msg
		c.keys = append(c.keys, key)
	}
	return c
}

// translations are the catalogs other than the source's, English.
func translations(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("*.json")
	if err != nil {
		t.Fatal(err)
	}
	var tags []string
	for _, f := range files {
		if tag := strings.TrimSuffix(f, ".json"); tag != "en" {
			tags = append(tags, tag)
		}
	}
	return tags
}

// TestEveryLanguageTranslatesEveryMessage keeps each translation in step
// with en.json: a message added in English alone would show in English.
// A pull request may add English alone, so it runs only as the release
// workflow checks the version, which sets MIXMUSIC_RELEASE.
func TestEveryLanguageTranslatesEveryMessage(t *testing.T) {
	if os.Getenv("MIXMUSIC_RELEASE") == "" {
		t.Skip("a release checks it: set MIXMUSIC_RELEASE")
	}
	en := readCatalog(t, "en")
	for _, tag := range translations(t) {
		tr := readCatalog(t, tag)
		for k, v := range en.messages {
			if _, ok := tr.messages[k]; !ok {
				t.Errorf("%s.json lacks %s: %q", tag, k, v)
			}
		}
		for k := range tr.messages {
			if _, ok := en.messages[k]; !ok && !strings.HasPrefix(k, "mygo.") {
				t.Errorf("%s.json has %s, which en.json lacks", tag, k)
			}
		}
	}
}

// TestTranslationsMatchEnglish checks that each translated message parses
// and has the English one's arguments, each of the same kind; the plural
// cases of its own language, and the English one's exact ones ("=0"); and
// the English one's select cases.
func TestTranslationsMatchEnglish(t *testing.T) {
	en := readCatalog(t, "en")
	source := map[string]*i18n.Message{}
	for k, v := range en.messages {
		m, err := i18n.ParseMessage(v)
		if err != nil {
			t.Errorf("en.json's %s: %v", k, err)
			continue
		}
		source[k] = m
	}
	for _, tag := range translations(t) {
		var categories []string
		for _, p := range i18n.PluralCategories(tag) {
			categories = append(categories, p.String())
		}
		for k, v := range readCatalog(t, tag).messages {
			want, ok := source[k]
			if !ok {
				continue
			}
			got, err := i18n.ParseMessage(v)
			if err != nil {
				t.Errorf("%s.json's %s: %v", tag, k, err)
				continue
			}
			if a, b := argList(want), argList(got); !slices.Equal(a, b) {
				t.Errorf("%s.json's %s has the arguments %v, en.json's %v", tag, k, b, a)
				continue
			}
			for _, a := range want.Args() {
				switch a.Kind {
				case "select":
					if !sameSet(want.Cases(a.Name), got.Cases(a.Name)) {
						t.Errorf("%s.json's %s selects %s by %v, en.json's by %v", tag, k, a.Name, got.Cases(a.Name), want.Cases(a.Name))
					}
				case "plural":
					exactW, _ := splitCases(want.Cases(a.Name))
					exactG, cats := splitCases(got.Cases(a.Name))
					if !sameSet(exactW, exactG) || !sameSet(cats, categories) {
						t.Errorf("%s.json's %s counts %s by %v; %s has the cases %v, and en.json the exact ones %v", tag, k, a.Name, got.Cases(a.Name), tag, categories, exactW)
					}
				}
			}
		}
	}
}

// argList is the arguments of m, as name:kind, sorted.
func argList(m *i18n.Message) []string {
	var out []string
	for _, a := range m.Args() {
		out = append(out, a.Name+":"+a.Kind)
	}
	slices.Sort(out)
	return out
}

// splitCases parts the cases of a plural into the exact ones, as "=0", and
// the categories, as "one".
func splitCases(cases []string) (exact, categories []string) {
	for _, c := range cases {
		if strings.HasPrefix(c, "=") {
			exact = append(exact, c)
		} else {
			categories = append(categories, c)
		}
	}
	return exact, categories
}

func sameSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

// called matches a message said in the app's Go code: locales.Title() or
// locales.In(tag).Title().
var called = regexp.MustCompile(`\blocales\.(?:In\([^()]*\)\.)?([A-Z][A-Za-z0-9_]*)\b`)

// TestEveryMessageIsSaid finds, for each message of en.json, the app's Go
// code that says it: a message nothing says is left for translators for
// nothing.
func TestEveryMessageIsSaid(t *testing.T) {
	said := map[string]bool{}
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch name := d.Name(); {
			case path == "..":
			case strings.HasPrefix(name, ".") || name == "node_modules" || name == "testdata" || name == "build" || name == "locales":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range called.FindAllSubmatch(b, -1) {
			said[string(m[1])] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range readCatalog(t, "en").keys {
		if !said[strings.ToUpper(k[:1])+k[1:]] {
			t.Errorf("nothing says en.json's %s", k)
		}
	}
}

// TestEveryLanguageGivesThePermissionReasons finds each reason of
// mygo.config.ts's infoPlist, which macOS shows as it asks for a
// permission, in the InfoPlist.strings of each translation, among the
// resources of macOS: without it, macOS asks in English.
func TestEveryLanguageGivesThePermissionReasons(t *testing.T) {
	config, err := os.ReadFile("../mygo.config.ts")
	if err != nil {
		t.Fatal(err)
	}
	reasons := regexp.MustCompile(`\bNS\w+UsageDescription\b`).FindAllString(string(config), -1)
	if len(reasons) == 0 {
		t.Fatal("mygo.config.ts has no reason")
	}
	for _, tag := range translations(t) {
		// The folder is named as Xcode names it, which is the tag for each
		// of the catalogs so far (zh-Hans).
		b, err := os.ReadFile(filepath.Join("..", "resources", "darwin", tag+".lproj", "InfoPlist.strings"))
		if err != nil {
			t.Error(err)
			continue
		}
		for _, r := range reasons {
			if !regexp.MustCompile(`(?m)^\s*"` + r + `"\s*=\s*"[^"]+"\s*;`).Match(b) {
				t.Errorf("%s.lproj/InfoPlist.strings lacks %s", tag, r)
			}
		}
		if _, ok := readCatalog(t, tag).messages["mygo.appName"]; !ok {
			t.Errorf("%s.json lacks mygo.appName, the app's name in the language", tag)
		}
	}
}

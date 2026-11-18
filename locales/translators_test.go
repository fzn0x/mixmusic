package locales

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// TestTranslators reads the names a catalog's file gives its translators,
// in their order, without blank ones; none where it gives none, or gives
// them as something other than a list, which loses its machine mark no
// more than its names.
func TestTranslators(t *testing.T) {
	read := fstest.MapFS{
		"qad.json": {Data: []byte(`{"@@locale": "qad", "@@x-translators": ["Ada", " ", "Grace"], "aboutContribute": "qad: contribute"}`)},
		"qae.json": {Data: []byte(`{"@@locale": "qae", "@@x-machine-translated": true, "@@x-translators": "Ada", "aboutContribute": "qae: contribute"}`)},
	}
	if err := AddTranslations(read); err != nil {
		t.Fatal(err)
	}
	for tag, want := range map[string][]string{"qad": {"Ada", "Grace"}, "qae": nil, "en": nil, "qtz": nil} {
		if got := Translators(tag); !slices.Equal(got, want) {
			t.Errorf("%s's translators: %q, want %q", tag, got, want)
		}
	}
	if !TranslatedByMachine("qae") {
		t.Error("qae, whose translators aren't a list, lost its machine mark")
	}
}

// TestTranslatorsAreNamed finds, in each catalog that names its
// translators, a list of names, none blank.
func TestTranslatorsAreNamed(t *testing.T) {
	root, err := filepath.Glob("*.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range append(root, addedCatalogs(t)...) {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var attrs map[string]json.RawMessage
		if err := json.Unmarshal(b, &attrs); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		raw, ok := attrs["@@x-translators"]
		if !ok {
			continue
		}
		var names []string
		if json.Unmarshal(raw, &names) != nil || len(names) == 0 || slices.ContainsFunc(names, func(n string) bool { return strings.TrimSpace(n) == "" }) {
			t.Errorf(`%s: "@@x-translators" is a list of names, as ["Ada", "Grace"]: %s`, path, raw)
		}
	}
}

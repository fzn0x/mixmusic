package settings

import (
	"bytes"
	"encoding"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/default.json from Default")

// stored is v as the store writes settings.json.
func stored(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(data, '\n')
}

// TestDefaultFile writes the settings of a new install as testdata's
// default.json holds them: the same keys, in the same order, with the
// same values. A new setting adds its key there, with -update.
func TestDefaultFile(t *testing.T) {
	d := Default()
	// The file is Windows'; TestDefaultSpotifyPlayback tells the others.
	d.SpotifyPlayback = defaultSpotifyPlayback("windows")
	got := stored(t, d)
	path := filepath.Join("testdata", "default.json")
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if want := readTestdata(t, "default.json"); !bytes.Equal(got, want) {
		t.Errorf("the defaults write as\n%s\nnot as %s holds them:\n%s", got, path, want)
	}
}

// TestFullFile reads testdata's full.json, which sets every setting to a
// value other than its default, and writes it back as it was: no key is
// lost, renamed or moved, and no value changes. A new setting adds its key
// there, with a value other than its default.
func TestFullFile(t *testing.T) {
	data := readTestdata(t, "full.json")
	s := Default()
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	if got := stored(t, s); !bytes.Equal(got, data) {
		t.Errorf("full.json writes back as\n%s", got)
	}
	var full, def map[string]json.RawMessage
	if err := json.Unmarshal(data, &full); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(stored(t, Default()), &def); err != nil {
		t.Fatal(err)
	}
	for key, v := range def {
		if bytes.Equal(full[key], v) {
			t.Errorf("full.json sets %s to its default, %s", key, v)
		}
	}
	// Every value of it is one Clean keeps.
	s.DownloadsFolder = t.TempDir() // the file's is absolute on Unix only
	if c := s.Clean(); !bytes.Equal(stored(t, c), stored(t, s)) {
		t.Errorf("Clean changed full.json's settings:\n%s", stored(t, c))
	}
}

// TestSectionsFlatten keeps each section of Settings a set of its fields
// in settings.json: a section that marshaled or unmarshaled itself would
// lend its method to Settings, which would then write or read only it.
func TestSectionsFlatten(t *testing.T) {
	for _, f := range reflect.VisibleFields(reflect.TypeFor[Settings]()) {
		if !f.Anonymous {
			continue
		}
		for _, i := range []reflect.Type{
			reflect.TypeFor[json.Marshaler](), reflect.TypeFor[json.Unmarshaler](),
			reflect.TypeFor[encoding.TextMarshaler](), reflect.TypeFor[encoding.TextUnmarshaler](),
		} {
			if f.Type.Implements(i) || reflect.PointerTo(f.Type).Implements(i) {
				t.Errorf("the section %s is a %s", f.Name, i)
			}
		}
	}
}

package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

// cfg is a value whose file is edited by hand.
type cfg struct {
	Theme  string         `json:"theme"`
	Volume float64        `json:"volume"`
	Keys   map[string]int `json:"keys"`
}

var defCfg = cfg{Theme: "mix", Volume: 0.8, Keys: map[string]int{}}

// decodeCfg reads a cfg as settings.Decode reads settings: from the
// defaults, and whole or not at all.
func decodeCfg(data []byte) (cfg, error) {
	c := cfg{Theme: "mix", Volume: 0.8, Keys: map[string]int{}}
	if err := json.Unmarshal(data, &c); err != nil {
		return cfg{}, err
	}
	return c, nil
}

func openCfg(t *testing.T, path string) *File[cfg] {
	t.Helper()
	f, err := OpenWith(path, defCfg, decodeCfg)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func writeText(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func onDisk(t *testing.T, path string) cfg {
	t.Helper()
	c, err := decodeCfg([]byte(readText(t, path)))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func setTheme(theme string) func(*cfg) error {
	return func(c *cfg) error { c.Theme = theme; return nil }
}

// TestVersionCountsEdits counts an edit of the file, read in, as a change
// of the value, and a reload that finds nothing new as none: what readers
// made of the value follows an edit by hand too.
func TestVersionCountsEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg.json")
	f := openCfg(t, path)
	if err := f.Update(setTheme("aurora")); err != nil {
		t.Fatal(err)
	}
	v := f.Version()
	if c, err := f.Reload(); err != nil || c.Changed {
		t.Fatalf("a reload of MixMusic's own save: changed %v, %v", c.Changed, err)
	}
	if got := f.Version(); got != v {
		t.Errorf("a reload that found nothing new moved the version from %d to %d", v, got)
	}
	writeText(t, path, `{"theme": "sakura", "volume": 0.8}`)
	if _, err := f.Reload(); err != nil {
		t.Fatal(err)
	}
	if got := f.Version(); got <= v {
		t.Errorf("an edit read in left the version at %d", got)
	}
}

func setVolume(v float64) func(*cfg) error {
	return func(c *cfg) error { c.Volume = v; return nil }
}

func noBroken(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path + ".broken"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s.broken: %v, want none", filepath.Base(path), err)
	}
}

func TestReloadIgnoresOwnWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	f := openCfg(t, path)
	if err := f.Update(setTheme("aurora")); err != nil {
		t.Fatal(err)
	}
	c, err := f.Reload()
	if err != nil || c.Changed || c.Data != nil {
		t.Errorf("Reload after its own write = %+v, %v; want no change", c, err)
	}
}

func TestReloadTakesAnEdit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	writeText(t, path, `{"theme": "aurora"}`)
	f := openCfg(t, path)
	writeText(t, path, `{"theme": "daylight", "volume": 0.5}`)
	c, err := f.Reload()
	if err != nil {
		t.Fatal(err)
	}
	if !c.Changed || c.Prev.Theme != "aurora" || c.Value.Theme != "daylight" || c.Value.Volume != 0.5 || string(c.Data) != `{"theme": "daylight", "volume": 0.5}` {
		t.Errorf("Reload = %+v", c)
	}
	if got := f.Get(); got.Theme != "daylight" {
		t.Errorf("Get = %+v", got)
	}
	// The edit stays as it was saved: MixMusic doesn't write it again.
	if got := readText(t, path); got != `{"theme": "daylight", "volume": 0.5}` {
		t.Errorf("the file was written: %q", got)
	}
	// The same bytes again, as a save that changes nothing, are no edit.
	writeText(t, path, `{"theme": "daylight", "volume": 0.5}`)
	if c, err := f.Reload(); err != nil || c.Changed || c.Data != nil {
		t.Errorf("Reload of the same text = %+v, %v", c, err)
	}
	// Text that changes nothing is read, but changes nothing.
	writeText(t, path, `{"theme":"daylight","volume":0.5}`)
	if c, err := f.Reload(); err != nil || c.Changed || c.Data == nil {
		t.Errorf("Reload of new text of the same value = %+v, %v", c, err)
	}
}

func TestReloadBrokenLeavesTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	f := openCfg(t, path)
	if err := f.Update(setTheme("aurora")); err != nil {
		t.Fatal(err)
	}
	const broken = "{\n  \"theme\": \"daylight\",\n}"
	writeText(t, path, broken)
	c, err := f.Reload()
	if err == nil || c.Changed || f.Held() == nil || !errors.Is(f.Held(), err) {
		t.Fatalf("Reload of a broken file = %+v, %v; held %v", c, err, f.Held())
	}
	if f.Get().Theme != "aurora" {
		t.Errorf("the value changed: %+v", f.Get())
	}
	if got := readText(t, path); got != broken {
		t.Errorf("the broken file was changed: %q", got)
	}
	noBroken(t, path)
	// Read again, it is still broken.
	if _, err := f.Reload(); err == nil {
		t.Error("the broken file read the second time")
	}
}

func TestReloadMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	f := openCfg(t, path)
	if err := f.Update(setTheme("aurora")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	c, err := f.Reload()
	if err != nil || c.Changed || f.Get().Theme != "aurora" {
		t.Errorf("Reload of a file gone = %+v, %v; value %+v", c, err, f.Get())
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Reload wrote the file again: %v", err)
	}
	// The next write saves it again.
	if err := f.Update(setVolume(0.3)); err != nil {
		t.Fatal(err)
	}
	if got := onDisk(t, path); got.Theme != "aurora" || got.Volume != 0.3 {
		t.Errorf("the file holds %+v", got)
	}
}

// TestReloadEmptyFile reads an empty file where there was none, as an
// editor leaves one for a moment as it saves: it doesn't decode.
func TestReloadEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	f := openCfg(t, path)
	writeText(t, path, "")
	if _, err := f.Reload(); err == nil || f.Held() == nil {
		t.Errorf("an empty file read: held %v", f.Held())
	}
	if err := f.Update(setTheme("aurora")); err != nil || readText(t, path) != "" {
		t.Errorf("Update over an empty file: %v, the file %q", err, readText(t, path))
	}
}

func TestUpdateMergesAnUnreadEdit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	f := openCfg(t, path)
	if err := f.Update(setVolume(0.5)); err != nil {
		t.Fatal(err)
	}
	// Saved by hand just before MixMusic saves a change of its own, as
	// the volume, 600 ms after it moved.
	writeText(t, path, `{"theme": "aurora", "volume": 0.5, "keys": {"a": 1}}`)
	if err := f.Update(setVolume(0.3)); err != nil {
		t.Fatal(err)
	}
	want := cfg{Theme: "aurora", Volume: 0.3, Keys: map[string]int{"a": 1}}
	if got := f.Get(); !reflect.DeepEqual(got, want) {
		t.Errorf("Get = %+v, want %+v", got, want)
	}
	if got := onDisk(t, path); !reflect.DeepEqual(got, want) {
		t.Errorf("the file holds %+v, want %+v", got, want)
	}
	// Where both changed a key, the newer change wins: MixMusic's own.
	writeText(t, path, `{"theme": "daylight", "volume": 0.9}`)
	if err := f.Update(setVolume(0.1)); err != nil {
		t.Fatal(err)
	}
	if got := onDisk(t, path); got.Theme != "daylight" || got.Volume != 0.1 {
		t.Errorf("the file holds %+v", got)
	}
}

func TestUpdateHeldWhileBroken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	f := openCfg(t, path)
	if err := f.Update(setTheme("aurora")); err != nil {
		t.Fatal(err)
	}
	const broken = `{"theme": "daylight",}`
	writeText(t, path, broken)
	// MixMusic hasn't read it yet: the write finds it broken.
	if err := f.Update(setVolume(0.3)); err != nil {
		t.Fatal(err)
	}
	if f.Held() == nil {
		t.Error("the write isn't held")
	}
	if got := f.Get(); got.Theme != "aurora" || got.Volume != 0.3 {
		t.Errorf("Get = %+v; the change applies at once", got)
	}
	if got := readText(t, path); got != broken {
		t.Errorf("the broken file was written: %q", got)
	}
	// And again, now that it read it.
	if err := f.Update(setVolume(0.2)); err != nil || readText(t, path) != broken {
		t.Errorf("the second write: %v, the file %q", err, readText(t, path))
	}
	noBroken(t, path)
}

func TestReloadAfterFixKeepsHeldChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	f := openCfg(t, path)
	if err := f.Update(setTheme("aurora")); err != nil {
		t.Fatal(err)
	}
	writeText(t, path, `{"theme": "daylight", "volume": 0.9,}`)
	if _, err := f.Reload(); err == nil {
		t.Fatal("the broken file read")
	}
	// Changed while the file doesn't read: the volume, and the theme.
	if err := f.Update(func(c *cfg) error { c.Volume, c.Theme = 0.3, "mochi"; return nil }); err != nil {
		t.Fatal(err)
	}
	// Fixed: the file changed the theme too, which it wins, being newer.
	writeText(t, path, `{"theme": "daylight", "keys": {"b": 2}}`)
	c, err := f.Reload()
	if err != nil || f.Held() != nil {
		t.Fatalf("Reload of the fixed file: %v, held %v", err, f.Held())
	}
	want := cfg{Theme: "daylight", Volume: 0.3, Keys: map[string]int{"b": 2}}
	if !c.Changed || !reflect.DeepEqual(c.Value, want) || !reflect.DeepEqual(f.Get(), want) {
		t.Errorf("Reload = %+v, want the value %+v", c, want)
	}
	// What changed meanwhile is saved with the fix.
	if got := onDisk(t, path); !reflect.DeepEqual(got, want) {
		t.Errorf("the file holds %+v, want %+v", got, want)
	}
	if c, err := f.Reload(); err != nil || c.Changed {
		t.Errorf("Reload after saving the merge = %+v, %v", c, err)
	}
}

func TestFlush(t *testing.T) {
	t.Run("broken", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.json")
		f := openCfg(t, path)
		if err := f.Update(setTheme("aurora")); err != nil {
			t.Fatal(err)
		}
		const broken = `{"theme": "daylight",}`
		writeText(t, path, broken)
		if err := f.Update(setVolume(0.3)); err != nil {
			t.Fatal(err)
		}
		if err := f.Flush(); err != nil {
			t.Fatal(err)
		}
		if got := readText(t, path+".broken"); got != broken {
			t.Errorf("settings.json.broken holds %q, want the text as saved", got)
		}
		if got := onDisk(t, path); got.Theme != "aurora" || got.Volume != 0.3 {
			t.Errorf("the file holds %+v", got)
		}
		if f.Held() != nil {
			t.Errorf("still held: %v", f.Held())
		}
	})
	t.Run("missing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.json")
		f := openCfg(t, path)
		if err := f.Update(setTheme("aurora")); err != nil {
			t.Fatal(err)
		}
		os.Remove(path)
		if err := f.Flush(); err != nil {
			t.Fatal(err)
		}
		if got := onDisk(t, path); got.Theme != "aurora" {
			t.Errorf("the file holds %+v", got)
		}
	})
	t.Run("an edit not read yet", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.json")
		f := openCfg(t, path)
		if err := f.Update(setTheme("aurora")); err != nil {
			t.Fatal(err)
		}
		writeText(t, path, `{"theme": "daylight"}`)
		if err := f.Flush(); err != nil {
			t.Fatal(err)
		}
		if got := readText(t, path); got != `{"theme": "daylight"}` {
			t.Errorf("Flush wrote over the edit: %q", got)
		}
		noBroken(t, path)
	})
	t.Run("fine", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "settings.json")
		writeText(t, path, `{"theme": "daylight"}`)
		f := openCfg(t, path)
		if err := f.Flush(); err != nil {
			t.Fatal(err)
		}
		if got := readText(t, path); got != `{"theme": "daylight"}` {
			t.Errorf("Flush wrote the file: %q", got)
		}
	})
}

func TestReplaceMovesBrokenAside(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	f := openCfg(t, path)
	const broken = `{"theme": `
	writeText(t, path, broken)
	if _, err := f.Reload(); err == nil {
		t.Fatal("the broken file read")
	}
	if err := f.Replace(defCfg); err != nil {
		t.Fatal(err)
	}
	if got := readText(t, path+".broken"); got != broken {
		t.Errorf("settings.json.broken holds %q", got)
	}
	if got := onDisk(t, path); !reflect.DeepEqual(got, defCfg) || f.Held() != nil {
		t.Errorf("the file holds %+v, held %v", got, f.Held())
	}
}

func TestOpenWithAside(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	const broken = `{"theme": "aurora",}`
	writeText(t, path, broken)
	f := openCfg(t, path)
	if f.Aside() == nil || !reflect.DeepEqual(f.Get(), defCfg) {
		t.Errorf("Aside = %v, Get = %+v", f.Aside(), f.Get())
	}
	if got := readText(t, path+".broken"); got != broken {
		t.Errorf("settings.json.broken holds %q", got)
	}
	// The defaults take its place, which read at the next start.
	if got := onDisk(t, path); !reflect.DeepEqual(got, defCfg) {
		t.Errorf("the file holds %+v", got)
	}
	if again := openCfg(t, path); again.Aside() != nil {
		t.Errorf("the next start moved it aside again: %v", again.Aside())
	}
	if c, err := f.Reload(); err != nil || c.Changed {
		t.Errorf("Reload = %+v, %v", c, err)
	}
}

// TestSymlinkStaysALink saves through a settings.json that links to a
// file of the user's dotfiles.
func TestSymlinkStaysALink(t *testing.T) {
	dir := t.TempDir()
	dotfiles := filepath.Join(dir, "dotfiles", "settings.json")
	if err := os.MkdirAll(filepath.Dir(dotfiles), 0o755); err != nil {
		t.Fatal(err)
	}
	writeText(t, dotfiles, `{"theme": "aurora"}`)
	path := filepath.Join(dir, "data", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(dotfiles, path); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	f := openCfg(t, path)
	if f.Get().Theme != "aurora" {
		t.Fatalf("Get = %+v", f.Get())
	}
	if err := f.Update(setVolume(0.3)); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(path); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("settings.json isn't a link anymore: %v, %v", fi.Mode(), err)
	}
	if got := onDisk(t, dotfiles); got.Theme != "aurora" || got.Volume != 0.3 {
		t.Errorf("the dotfiles' file holds %+v", got)
	}
	// An edit of the file it links to is read through it.
	writeText(t, dotfiles, `{"theme": "daylight", "volume": 0.3}`)
	if c, err := f.Reload(); err != nil || !c.Changed || c.Value.Theme != "daylight" {
		t.Errorf("Reload = %+v, %v", c, err)
	}
}

func TestMerge(t *testing.T) {
	base := cfg{Theme: "mix", Volume: 0.8, Keys: map[string]int{"a": 1}}
	for _, c := range []struct {
		name string
		a, b cfg
		want cfg
	}{
		{"nothing changed", base, base, base},
		{"a changed", cfg{Theme: "aurora", Volume: 0.8, Keys: map[string]int{"a": 1}}, base,
			cfg{Theme: "aurora", Volume: 0.8, Keys: map[string]int{"a": 1}}},
		{"b changed", base, cfg{Theme: "mix", Volume: 0.3, Keys: map[string]int{"a": 1}},
			cfg{Theme: "mix", Volume: 0.3, Keys: map[string]int{"a": 1}}},
		{"each changed another", cfg{Theme: "aurora", Volume: 0.8, Keys: map[string]int{"a": 1}}, cfg{Theme: "mix", Volume: 0.3, Keys: map[string]int{"a": 1}},
			cfg{Theme: "aurora", Volume: 0.3, Keys: map[string]int{"a": 1}}},
		{"both changed one", cfg{Theme: "aurora", Volume: 0.8, Keys: map[string]int{"a": 1}}, cfg{Theme: "daylight", Volume: 0.8, Keys: map[string]int{"a": 1}},
			cfg{Theme: "daylight", Volume: 0.8, Keys: map[string]int{"a": 1}}},
		// A map is one member: b's whole map wins.
		{"both changed the map", cfg{Theme: "mix", Volume: 0.8, Keys: map[string]int{"a": 1, "x": 1}}, cfg{Theme: "mix", Volume: 0.8, Keys: map[string]int{"a": 2}},
			cfg{Theme: "mix", Volume: 0.8, Keys: map[string]int{"a": 2}}},
	} {
		got, err := Merge(base, c.a, c.b)
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: Merge = %+v, %v; want %+v", c.name, got, err, c.want)
		}
	}
	if _, err := Merge([]int{1}, []int{2}, []int{3}); err == nil {
		t.Error("Merge of lists didn't fail")
	}
}

// TestOpenIgnoresEdits keeps the files Open opened as they were: only
// OpenWith's read the file again.
func TestOpenIgnoresEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	f, err := Open(path, defCfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Update(setTheme("aurora")); err != nil {
		t.Fatal(err)
	}
	writeText(t, path, `{"theme": "daylight",}`)
	if c, err := f.Reload(); err != nil || c.Changed || f.Held() != nil || f.Aside() != nil || f.Flush() != nil {
		t.Errorf("Reload = %+v, %v", c, err)
	}
	if err := f.Update(setVolume(0.1)); err != nil {
		t.Fatal(err)
	}
	if got := onDisk(t, path); got.Theme != "aurora" || got.Volume != 0.1 {
		t.Errorf("the file holds %+v", got)
	}
}

// TestConcurrentReloadUpdate edits the file while the app writes it and
// reads it again, for the race detector.
func TestConcurrentReloadUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	f := openCfg(t, path)
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Go(func() {
			for j := range 25 {
				switch i {
				case 0:
					f.Update(setVolume(float64(j) / 100))
				case 1:
					f.Reload()
				case 2:
					// Atomic, as editors save.
					WriteFile(path, fmt.Appendf(nil, `{"theme": "t%d"}`, j))
				default:
					_ = f.Get()
					_ = f.Held()
				}
			}
		})
	}
	wg.Wait()
	if _, err := f.Reload(); err != nil {
		t.Fatal(err)
	}
	if got, want := onDisk(t, path), f.Get(); got.Theme != want.Theme {
		t.Errorf("the file holds %+v, the value %+v", got, want)
	}
}

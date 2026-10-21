package addons

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func themeJSON(id string) string { return strings.Replace(validTheme, `"test-theme"`, `"`+id+`"`, 1) }

const pluginJSON = `{"id": "hello", "name": "Hello", "version": "1.0.0", "author": "A", "main": "index.js"}`

func testManager(t *testing.T) *Manager {
	t.Helper()
	builtin := fstest.MapFS{
		"themes/mix/theme.json":  {Data: []byte(themeJSON("mix"))},
		"themes/mix/preview.png": {Data: []byte("mix")},
	}
	dir := t.TempDir()
	write := func(name, data string) {
		p := filepath.Join(dir, filepath.FromSlash(name))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("themes/ocean/theme.json", themeJSON("ocean"))
	write("themes/ocean/preview.png", "ocean")
	write("themes/mix/theme.json", themeJSON("mix"))             // takes a built-in id
	write("themes/misnamed/theme.json", themeJSON("other-name")) // folder and id differ
	write("themes/broken/theme.json", "{")
	write("plugins/hello/plugin.json", pluginJSON)
	write("plugins/hello/index.js", "export default () => {}")
	return NewManager(dir, builtin)
}

func TestList(t *testing.T) {
	c := testManager(t).List()
	if len(c.Themes) != 2 || c.Themes[0].ID != "mix" || !c.Themes[0].Builtin || c.Themes[1].ID != "ocean" || c.Themes[1].Builtin {
		t.Errorf("themes = %+v", c.Themes)
	}
	if len(c.Plugins) != 1 || c.Plugins[0].ID != "hello" {
		t.Errorf("plugins = %+v", c.Plugins)
	}
	if len(c.Problems) != 3 {
		t.Errorf("problems = %+v", c.Problems)
	}
}

func TestFiles(t *testing.T) {
	m := testManager(t)
	for _, f := range []struct {
		kind     Kind
		id, name string
		want     string
	}{
		{KindTheme, "ocean", "preview.png", "ocean"},
		{KindTheme, "mix", "preview.png", "mix"}, // the built-in theme, not the user's
		{KindPlugin, "hello", "index.js", "export default () => {}"},
	} {
		fsys, err := m.Files(f.kind, f.id)
		if err != nil {
			t.Fatalf("%s %s: %v", f.kind, f.id, err)
		}
		data, err := fs.ReadFile(fsys, f.name)
		if err != nil || string(data) != f.want {
			t.Errorf("%s %s/%s = %q, %v", f.kind, f.id, f.name, data, err)
		}
	}
	for _, id := range []string{"../themes", "Ocean", ""} {
		if _, err := m.Files(KindTheme, id); err == nil {
			t.Errorf("files of %q", id)
		}
	}
	if _, err := m.Files("other", "hello"); err == nil {
		t.Error("files of an unknown kind")
	}
}

func TestUninstall(t *testing.T) {
	m := testManager(t)
	if err := m.Uninstall(KindTheme, "mix"); !errors.Is(err, ErrBuiltin) {
		t.Errorf("uninstalling a built-in theme: %v", err)
	}
	if err := m.Uninstall(KindTheme, "ocean"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(m.Dir, "themes", "ocean")); !os.IsNotExist(err) {
		t.Error("the theme is still there")
	}
	if err := m.Uninstall(KindPlugin, "../themes"); err == nil {
		t.Error("uninstalled outside the add-ons folder")
	}
}

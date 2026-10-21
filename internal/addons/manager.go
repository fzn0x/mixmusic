package addons

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// Problem is an add-on that could not be loaded, and why.
type Problem struct {
	Kind   Kind   `json:"kind"`
	Folder string `json:"folder"`
	Error  string `json:"error"`
}

// Inventory lists the add-ons available.
type Inventory struct {
	Themes   []Theme   `json:"themes"`
	Plugins  []Plugin  `json:"plugins"`
	Problems []Problem `json:"problems"`
}

// Manager finds add-ons and installs them.
type Manager struct {
	// Dir holds the add-ons the user installed: themes/<id> and
	// plugins/<id>.
	Dir string
	// Builtin holds the add-ons MixMusic ships with, laid out as Dir.
	Builtin fs.FS
	// HTTP fetches the community registry and the add-ons it lists.
	HTTP *http.Client
}

// NewManager returns a manager of the add-ons in dir and builtin.
func NewManager(dir string, builtin fs.FS) *Manager {
	return &Manager{Dir: dir, Builtin: builtin, HTTP: &http.Client{Timeout: time.Minute}}
}

// folder returns the folder of a kind of add-on: "themes" or "plugins".
func folder(kind Kind) string { return string(kind) + "s" }

// manifestName returns the manifest file of a kind of add-on.
func manifestName(kind Kind) string { return string(kind) + ".json" }

type source struct {
	fsys    fs.FS
	builtin bool
}

// sources returns where add-ons are found, the built-in ones first.
func (m *Manager) sources() []source {
	var out []source
	if m.Builtin != nil {
		out = append(out, source{m.Builtin, true})
	}
	if m.Dir != "" {
		out = append(out, source{os.DirFS(m.Dir), false})
	}
	return out
}

// List finds every add-on. Add-ons the user installed cannot replace those
// MixMusic ships with: they are listed as problems.
func (m *Manager) List() Inventory {
	c := Inventory{Themes: []Theme{}, Plugins: []Plugin{}, Problems: []Problem{}}
	for _, kind := range []Kind{KindTheme, KindPlugin} {
		seen := map[string]bool{}
		for _, src := range m.sources() {
			entries, err := fs.ReadDir(src.fsys, folder(kind))
			if err != nil {
				continue
			}
			for _, e := range entries {
				if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
					continue
				}
				problem := func(msg string) { c.Problems = append(c.Problems, Problem{kind, e.Name(), msg}) }
				data, err := fs.ReadFile(src.fsys, path.Join(folder(kind), e.Name(), manifestName(kind)))
				if err != nil {
					problem("no " + manifestName(kind))
					continue
				}
				switch kind {
				case KindTheme:
					t, err := ParseTheme(data)
					if msg := check(err, t.ID, e.Name(), seen); msg != "" {
						problem(msg)
						continue
					}
					t.Builtin = src.builtin
					if t.Options == nil {
						t.Options = []Option{}
					}
					c.Themes = append(c.Themes, t)
				case KindPlugin:
					p, err := ParsePlugin(data)
					if msg := check(err, p.ID, e.Name(), seen); msg != "" {
						problem(msg)
						continue
					}
					p.Builtin = src.builtin
					c.Plugins = append(c.Plugins, p)
				}
			}
		}
	}
	return c
}

// check returns what keeps an add-on, whose manifest parsed with err, from
// loading, or "" when nothing does, and marks its id seen.
func check(err error, id, dir string, seen map[string]bool) string {
	switch {
	case err != nil:
		return err.Error()
	case id != dir:
		return "the folder must be named after the id, " + id
	case seen[id]:
		return "an add-on MixMusic ships with has this id"
	}
	seen[id] = true
	return ""
}

func (m *Manager) isBuiltin(kindFolder, id string) bool {
	if m.Builtin == nil {
		return false
	}
	_, err := fs.Stat(m.Builtin, path.Join(kindFolder, id))
	return err == nil
}

// ErrBuiltin is returned when removing an add-on MixMusic ships with.
var ErrBuiltin = errors.New("addons: add-ons MixMusic ships with cannot be removed")

// Uninstall removes an add-on the user installed.
func (m *Manager) Uninstall(kind Kind, id string) error {
	if !settings.ValidID(id) || (kind != KindTheme && kind != KindPlugin) {
		return errors.New("addons: bad add-on")
	}
	if m.isBuiltin(folder(kind), id) {
		return ErrBuiltin
	}
	return os.RemoveAll(filepath.Join(m.Dir, folder(kind), id))
}

// Files returns the files of an add-on, in its folder: that of the add-on
// MixMusic ships with, when there is one of the id, as List chooses, or
// the user's.
func (m *Manager) Files(kind Kind, id string) (fs.FS, error) {
	if !settings.ValidID(id) || (kind != KindTheme && kind != KindPlugin) {
		return nil, errors.New("addons: bad add-on")
	}
	if m.isBuiltin(folder(kind), id) {
		return fs.Sub(m.Builtin, path.Join(folder(kind), id))
	}
	if m.Dir == "" {
		return nil, errors.New("addons: no folder of add-ons")
	}
	return os.DirFS(filepath.Join(m.Dir, folder(kind), id)), nil
}

// FolderOf returns the folder the user's add-ons of a kind are in,
// creating it.
func (m *Manager) FolderOf(kind Kind) (string, error) {
	dir := filepath.Join(m.Dir, folder(kind))
	return dir, os.MkdirAll(dir, 0o755)
}

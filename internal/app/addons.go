package app

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/addons"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// registryTTL is how long a fetched registry is reused.
const registryTTL = 10 * time.Minute

var registryCache struct {
	sync.Mutex
	url     string
	at      time.Time
	content addons.Registry
}

// Addons lists, installs and removes themes and plugins, and keeps what
// plugins store.
type Addons struct{ a *App }

// List returns the themes and plugins available, and those that could not
// load.
func (s *Addons) List() addons.Inventory { return s.a.addons.List() }

// Reload finds the add-ons again, after the user changed the add-ons
// folder, and tells every window.
func (s *Addons) Reload() addons.Inventory {
	inv := s.a.addons.List()
	notify()
	return inv
}

// Registry returns the community registry of themes and plugins.
func (s *Addons) Registry(ctx context.Context, refresh bool) (addons.Registry, error) {
	url := s.a.settings.Get().Clean().RegistryURL
	registryCache.Lock()
	defer registryCache.Unlock()
	if !refresh && registryCache.url == url && time.Since(registryCache.at) < registryTTL {
		return registryCache.content, nil
	}
	r, err := s.a.addons.FetchRegistry(ctx, url)
	if err != nil {
		return addons.Registry{}, err
	}
	registryCache.url, registryCache.at, registryCache.content = url, time.Now(), r
	return r, nil
}

// Install installs, or updates, an add-on of the community registry.
func (s *Addons) Install(ctx context.Context, kind addons.Kind, id string) (addons.Inventory, error) {
	r, err := s.Registry(ctx, false)
	if err != nil {
		return addons.Inventory{}, err
	}
	if err := s.a.addons.Install(ctx, r, kind, id); err != nil {
		return addons.Inventory{}, err
	}
	return s.Reload(), nil
}

// Uninstall removes an add-on the user installed.
func (s *Addons) Uninstall(kind addons.Kind, id string) (addons.Inventory, error) {
	if err := s.a.addons.Uninstall(kind, id); err != nil {
		return addons.Inventory{}, err
	}
	return s.Reload(), nil
}

// OpenFolder opens the folder of the user's add-ons of a kind, where they
// can put add-ons they make.
func (s *Addons) OpenFolder(kind addons.Kind) error {
	dir, err := s.a.addons.FolderOf(kind)
	if err != nil {
		return err
	}
	return mygo.Shell.OpenPath(dir)
}

// AddonFiles returns the files of an add-on, for the native interface's
// plugins.
func (a *App) AddonFiles(kind addons.Kind, id string) (fs.FS, error) { return a.addons.Files(kind, id) }

// ReadCommunityFrom has the marketplace read the community registry, and
// the files of the add-ons it lists, from dir, a copy of the repository's
// community folder, instead of from GitHub: for screenshots of add-ons not
// published yet. Call it before the marketplace opens.
func (a *App) ReadCommunityFrom(dir string) {
	a.addons.HTTP.Transport = communityDir(dir)
}

// communityDir answers the requests for the community folder on GitHub
// from a folder on disk, and passes the others on.
type communityDir string

func (d communityDir) RoundTrip(r *http.Request) (*http.Response, error) {
	base := settings.DefaultRegistryURL[:strings.LastIndex(settings.DefaultRegistryURL, "/")+1]
	name, ok := strings.CutPrefix(r.URL.String(), base)
	if !ok {
		return http.DefaultTransport.RoundTrip(r)
	}
	if !filepath.IsLocal(filepath.FromSlash(name)) {
		return nil, fmt.Errorf("not a file of the community folder: %s", name)
	}
	data, err := os.ReadFile(filepath.Join(string(d), filepath.FromSlash(name)))
	status := http.StatusOK
	if err != nil {
		status, data = http.StatusNotFound, nil
	}
	return &http.Response{
		Status:        http.StatusText(status),
		StatusCode:    status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{},
		Body:          io.NopCloser(bytes.NewReader(data)),
		ContentLength: int64(len(data)),
		Request:       r,
	}, nil
}

// Storage returns every value a plugin keeps.
func (s *Addons) Storage(plugin string) (map[string]string, error) { return s.a.storage.All(plugin) }

// StorageSet keeps a value of a plugin.
func (s *Addons) StorageSet(plugin, key, value string) error {
	return s.a.storage.Set(plugin, key, value)
}

// StorageDelete removes a value of a plugin.
func (s *Addons) StorageDelete(plugin, key string) error { return s.a.storage.Delete(plugin, key) }

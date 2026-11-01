package addons

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/fzn0x/mixmusic/internal/settings"
)

const (
	// maxFileSize bounds each file of an add-on downloaded.
	maxFileSize = 8 << 20
	// maxFiles bounds how many files an add-on downloaded has.
	maxFiles = 64
)

// Entry is an add-on of the community registry.
type Entry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	Description string `json:"description,omitempty"`
	// Path is the add-on's folder, relative to the registry's base URL.
	Path  string   `json:"path"`
	Files []string `json:"files"`
	// Preview is an image of the add-on, relative to its folder.
	Preview string `json:"preview,omitempty"`
	// Colors are a theme's colors, which the app shows as swatches.
	Colors map[string]string `json:"colors,omitempty"`
	// Permissions are those a plugin declares, and API the version of the
	// plugin API it was written for.
	Permissions []string `json:"permissions,omitempty"`
	API         int      `json:"api,omitempty"`
}

// CommunityFilesURL is where the community's add-ons are downloaded from,
// the baseUrl of its registries: the registry branch, where the Registry
// workflow publishes them, with the plugins compiled from their TypeScript.
const CommunityFilesURL = "https://raw.githubusercontent.com/fzn0x/mixmusic/registry/"

// Registry is the community registry of themes and plugins.
type Registry struct {
	Version int `json:"version"`
	// BaseURL is the address add-on paths are relative to.
	BaseURL string  `json:"baseUrl"`
	Themes  []Entry `json:"themes"`
	Plugins []Entry `json:"plugins"`
}

// Validate checks the entries of a registry.
func (r Registry) Validate() error {
	var errs []error
	for kind, entries := range map[Kind][]Entry{KindTheme: r.Themes, KindPlugin: r.Plugins} {
		for _, e := range entries {
			if err := e.validate(kind); err != nil {
				errs = append(errs, fmt.Errorf("%s %q: %w", kind, e.ID, err))
			}
		}
	}
	return errors.Join(errs...)
}

func (e Entry) validate(kind Kind) error {
	var errs []error
	errs = append(errs, validateCommon(e.ID, e.Name, e.Version, e.Author))
	if e.Path != folder(kind)+"/"+e.ID {
		errs = append(errs, fmt.Errorf("path must be %s/%s", folder(kind), e.ID))
	}
	if len(e.Files) == 0 || len(e.Files) > maxFiles {
		errs = append(errs, fmt.Errorf("an add-on has 1 to %d files", maxFiles))
	}
	hasManifest := false
	for _, f := range e.Files {
		if !ValidFile(f) {
			errs = append(errs, fmt.Errorf("%q is not a file of the add-on", f))
		}
		hasManifest = hasManifest || f == manifestName(kind)
	}
	if !hasManifest {
		errs = append(errs, fmt.Errorf("files must list %s", manifestName(kind)))
	}
	return errors.Join(errs...)
}

// Find returns the entry of an add-on.
func (r Registry) Find(kind Kind, id string) (Entry, bool) {
	entries := r.Themes
	if kind == KindPlugin {
		entries = r.Plugins
	}
	for _, e := range entries {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

// FetchRegistry downloads the registry at url. Its add-ons are relative to
// its base URL, by default the registry's folder.
func (m *Manager) FetchRegistry(ctx context.Context, url string) (Registry, error) {
	if !strings.HasPrefix(url, "https://") {
		return Registry{}, errors.New("addons: the registry must be served over HTTPS")
	}
	data, err := m.download(ctx, url)
	if err != nil {
		return Registry{}, fmt.Errorf("addons: fetching the registry: %w", err)
	}
	var r Registry
	if err := json.Unmarshal(data, &r); err != nil {
		return Registry{}, fmt.Errorf("addons: the registry is not valid JSON: %w", err)
	}
	if r.BaseURL == "" {
		r.BaseURL = url[:strings.LastIndex(url, "/")+1]
	}
	if !strings.HasPrefix(r.BaseURL, "https://") {
		return Registry{}, errors.New("addons: the registry's add-ons must be served over HTTPS")
	}
	if !strings.HasSuffix(r.BaseURL, "/") {
		r.BaseURL += "/"
	}
	// Leave out invalid entries rather than the whole registry.
	keep := func(kind Kind, in []Entry) []Entry {
		out := []Entry{}
		for _, e := range in {
			if e.validate(kind) == nil {
				out = append(out, e)
			}
		}
		return out
	}
	r.Themes, r.Plugins = keep(KindTheme, r.Themes), keep(KindPlugin, r.Plugins)
	return r, nil
}

// Install downloads an add-on of the registry into the user's add-ons,
// replacing the version installed. The add-on's manifest must be valid and
// of the id the registry lists, or nothing changes.
func (m *Manager) Install(ctx context.Context, r Registry, kind Kind, id string) error {
	if !settings.ValidID(id) {
		return errors.New("addons: bad add-on id")
	}
	e, ok := r.Find(kind, id)
	if !ok {
		return fmt.Errorf("addons: the registry has no %s %q", kind, id)
	}
	if err := e.validate(kind); err != nil {
		return err
	}
	if m.isBuiltin(folder(kind), id) {
		return ErrBuiltin
	}
	parent, err := m.FolderOf(kind)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(parent, "."+id+"-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	for _, f := range e.Files {
		data, err := m.download(ctx, r.BaseURL+e.Path+"/"+f)
		if err != nil {
			return fmt.Errorf("addons: downloading %s: %w", f, err)
		}
		if f == manifestName(kind) {
			if err := checkManifest(kind, id, data); err != nil {
				return err
			}
		}
		dst := filepath.Join(tmp, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return err
		}
	}
	final := filepath.Join(parent, id)
	old := final + ".old"
	os.RemoveAll(old)
	if err := os.Rename(final, old); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(tmp, final); err != nil {
		os.Rename(old, final) // put the installed version back
		return err
	}
	return os.RemoveAll(old)
}

// checkManifest checks a downloaded manifest is valid and of id.
func checkManifest(kind Kind, id string, data []byte) error {
	var gotID string
	var err error
	if kind == KindTheme {
		var t Theme
		t, err = ParseTheme(data)
		gotID = t.ID
	} else {
		var p Plugin
		p, err = ParsePlugin(data)
		gotID = p.ID
	}
	switch {
	case err != nil:
		return fmt.Errorf("addons: the %s's manifest is invalid: %w", kind, err)
	case gotID != id:
		return fmt.Errorf("addons: the manifest is of %q, not %q", gotID, id)
	}
	return nil
}

func (m *Manager) download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := m.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	var buf bytes.Buffer
	n, err := io.Copy(&buf, io.LimitReader(res.Body, maxFileSize+1))
	if err != nil {
		return nil, err
	}
	if n > maxFileSize {
		return nil, fmt.Errorf("larger than %d MB", maxFileSize>>20)
	}
	return buf.Bytes(), nil
}

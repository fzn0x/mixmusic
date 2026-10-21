package addons

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// registryServer serves a registry listing the theme "ocean" and the
// plugin "hello", whose files are those of files.
func registryServer(t *testing.T, files map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/community/registry.json" {
			w.Write([]byte(`{"version": 1,
				"themes": [
					{"id": "ocean", "name": "Ocean", "version": "1.0.0", "author": "A", "path": "themes/ocean", "files": ["theme.json", "theme.css"]},
					{"id": "bad", "name": "Bad", "version": "1.0.0", "author": "A", "path": "../../etc", "files": ["theme.json"]}
				],
				"plugins": [{"id": "hello", "name": "Hello", "version": "1.0.0", "author": "A", "path": "plugins/hello", "files": ["plugin.json", "index.js"]}]}`))
			return
		}
		data, ok := files[strings.TrimPrefix(r.URL.Path, "/community/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(data))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestInstallFromRegistry(t *testing.T) {
	srv := registryServer(t, map[string]string{
		"themes/ocean/theme.json":   themeJSON("ocean"),
		"themes/ocean/theme.css":    "body{}",
		"plugins/hello/plugin.json": strings.Replace(pluginJSON, `"main"`, `"files": ["plugin.json", "index.js"], "main"`, 1),
		"plugins/hello/index.js":    "export default () => {}",
	})
	m := NewManager(t.TempDir(), nil)
	m.HTTP = srv.Client()
	r, err := m.FetchRegistry(context.Background(), srv.URL+"/community/registry.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Themes) != 1 || r.BaseURL != srv.URL+"/community/" {
		t.Fatalf("registry = %+v", r)
	}
	if err := m.Install(context.Background(), r, KindTheme, "ocean"); err != nil {
		t.Fatal(err)
	}
	if err := m.Install(context.Background(), r, KindPlugin, "hello"); err != nil {
		t.Fatal(err)
	}
	c := m.List()
	if len(c.Themes) != 1 || len(c.Plugins) != 1 || len(c.Problems) != 0 {
		t.Errorf("installed = %+v", c)
	}
	// Installing again replaces the installed version.
	if err := m.Install(context.Background(), r, KindTheme, "ocean"); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(m.Dir, "themes"))
	if len(entries) != 1 {
		t.Errorf("leftovers: %v", entries)
	}
}

func TestInstallRejectsMismatchedManifest(t *testing.T) {
	srv := registryServer(t, map[string]string{
		"themes/ocean/theme.json": themeJSON("someone-else"),
		"themes/ocean/theme.css":  "body{}",
	})
	m := NewManager(t.TempDir(), nil)
	m.HTTP = srv.Client()
	r, err := m.FetchRegistry(context.Background(), srv.URL+"/community/registry.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Install(context.Background(), r, KindTheme, "ocean"); err == nil {
		t.Fatal("installed a theme whose manifest names another")
	}
	if entries, _ := os.ReadDir(filepath.Join(m.Dir, "themes")); len(entries) != 0 {
		t.Errorf("a failed install left %v", entries)
	}
}

func TestFetchRegistryNeedsHTTPS(t *testing.T) {
	m := NewManager(t.TempDir(), nil)
	if _, err := m.FetchRegistry(context.Background(), "http://example.com/registry.json"); err == nil {
		t.Error("fetched a registry over plain HTTP")
	}
}

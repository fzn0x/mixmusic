package app

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/fzn0x/mixmusic/internal/addons"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestCommunityDir answers from a copy of the community folder both the
// registry, which main keeps, and the add-ons' files, which the registry
// branch publishes.
func TestCommunityDir(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("registry-2.json", "{}")
	write("plugins/hello/index.js", "export default () => {}")
	get := func(url string) (int, string) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		res, err := communityDir(dir).RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(body)
	}
	if code, body := get(settings.DefaultRegistryURL); code != http.StatusOK || body != "{}" {
		t.Errorf("the registry: %d %q", code, body)
	}
	if code, body := get(addons.CommunityFilesURL + "plugins/hello/index.js"); code != http.StatusOK || body != "export default () => {}" {
		t.Errorf("a plugin's file: %d %q", code, body)
	}
	if code, _ := get(addons.CommunityFilesURL + "plugins/hello/missing.js"); code != http.StatusNotFound {
		t.Errorf("a missing file: %d", code)
	}
}

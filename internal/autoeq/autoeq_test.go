package autoeq

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// AutoEq's index and profiles, as a server of made-up ones gives them: the
// tests never reach GitHub.

// fakeIndex is an index in AutoEq's form, of made-up models.
const fakeIndex = `# Index
This is a list of all equalization profiles. Target is in parentheses if there are results with multiple targets
from the same source.

- [Acme One](./Lab%20A/711%20in-ear/Acme%20One) by Lab A on 711
- [Acme One](./Lab%20B/in-ear/Acme%20One) by Lab B
- [acme Buds (ANC Off)](./Lab%20B/earbud/acme%20Buds%20(ANC%20Off)) by Lab B
- [Zed & Co Max+](./Ted's%20Lab/EARS%20+%20711%20over-ear/Zed%20&%20Co%20Max+) by Ted's Lab on EARS + 711
- [AüR Duo](./Lab%20A/over-ear/A%C3%BCR%20Duo) by Lab A
- [Escape](./../../etc/passwd) by Nobody
- [Short](./Lab%20A/Short) by Lab A
- not a profile
`

// fakeProfile is a profile in AutoEq's form, of made-up values.
const fakeProfile = "Preamp: -4.2 dB\nFilter 1: ON LSC Fc 105 Hz Gain 3.5 dB Q 0.70\nFilter 2: ON PK Fc 2400 Hz Gain -2.0 dB Q 1.50\n"

// TestParseIndex reads the profiles of the index, by name, ignoring case:
// their sources, rigs and forms, and their folders, their escapes undone.
// A line that isn't a profile, or whose folder leaves the results or is
// too short, is left out.
func TestParseIndex(t *testing.T) {
	entries, err := ParseIndex([]byte(fakeIndex))
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{"acme Buds (ANC Off)", "Lab B", "", "earbud", "Lab B/earbud/acme Buds (ANC Off)"},
		{"Acme One", "Lab A", "711", "in-ear", "Lab A/711 in-ear/Acme One"},
		{"Acme One", "Lab B", "", "in-ear", "Lab B/in-ear/Acme One"},
		{"AüR Duo", "Lab A", "", "over-ear", "Lab A/over-ear/AüR Duo"},
		{"Zed & Co Max+", "Ted's Lab", "EARS + 711", "over-ear", "Ted's Lab/EARS + 711 over-ear/Zed & Co Max+"},
	}
	if !slices.Equal(entries, want) {
		t.Errorf("read\n%+v\nwant\n%+v", entries, want)
	}
	models := Models(entries)
	if len(models) != 4 || models[0].Name != "acme Buds (ANC Off)" || models[1].Name != "Acme One" || len(models[1].Profiles) != 2 {
		t.Errorf("grouped as %+v", models)
	}
	for _, index := range []string{"", "# Index\n- not a profile\n", "- [Escape](./a/../b) by x"} {
		if _, err := ParseIndex([]byte(index)); !errors.Is(err, ErrEmptyIndex) {
			t.Errorf("%q: %v", index, err)
		}
	}
}

// fakeAutoEq is a server of AutoEq's results: its index, whose ETag is
// etag, and profiles by their folders. It counts the requests, and keeps
// what they asked for.
type fakeAutoEq struct {
	*httptest.Server
	mu       sync.Mutex
	index    string
	etag     string
	profiles map[string]string // by the path asked for
	asked    []*http.Request
}

func newFakeAutoEq(t *testing.T) *fakeAutoEq {
	f := &fakeAutoEq{index: fakeIndex, etag: `"one"`, profiles: map[string]string{
		"/results/Lab A/711 in-ear/Acme One/Acme One ParametricEQ.txt":                        fakeProfile,
		"/results/Ted's Lab/EARS + 711 over-ear/Zed & Co Max+/Zed & Co Max+ ParametricEQ.txt": fakeProfile,
		"/results/Lab B/earbud/acme Buds (ANC Off)/acme Buds (ANC Off) ParametricEQ.txt":      "Filter 1: ON BP Fc 100 Hz Q 1\n",
		"/results/Lab A/over-ear/AüR Duo/AüR Duo ParametricEQ.txt":                            fakeProfile,
		"/results/Lab B/in-ear/Acme One/Acme One ParametricEQ.txt":                            strings.Repeat("#", maxProfile+1),
	}}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.asked = append(f.asked, r)
		if r.URL.Path == "/results/INDEX.md" {
			w.Header().Set("ETag", f.etag)
			if r.Header.Get("If-None-Match") == f.etag {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Write([]byte(f.index))
			return
		}
		p, ok := f.profiles[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(p))
	}))
	t.Cleanup(f.Close)
	return f
}

// requests returns how many requests the server had.
func (f *fakeAutoEq) requests() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.asked)
}

// last returns the last request the server had.
func (f *fakeAutoEq) last() *http.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.asked[len(f.asked)-1]
}

// newTestClient is a client of the server f, keeping what it fetches in
// dir, at the time *now.
func newTestClient(f *fakeAutoEq, dir string, now *time.Time) *Client {
	c := New(dir, "MixMusic/test")
	c.Base = f.URL + "/results/"
	c.now = func() time.Time { return *now }
	return c
}

// TestIndexFetchedAndKept fetches the index the first time, keeps it, and
// checks it again a week later, with its ETag: unchanged, it isn't fetched
// again; changed, it is.
func TestIndexFetchedAndKept(t *testing.T) {
	f := newFakeAutoEq(t)
	dir := t.TempDir()
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	c := newTestClient(f, dir, &now)
	list, err := c.Index(context.Background())
	if err != nil || len(list.Entries) != 5 || list.Offline != nil {
		t.Fatalf("the index: %d entries, %v, %v", len(list.Entries), list.Offline, err)
	}
	if r := f.last(); r.Header.Get("User-Agent") != "MixMusic/test" || r.Header.Get("If-None-Match") != "" || r.Header.Get("Cookie") != "" {
		t.Errorf("asked with %v", r.Header)
	}

	// Within the week, neither this client nor another of the folder asks.
	now = now.Add(6 * 24 * time.Hour)
	for _, c := range []*Client{c, newTestClient(f, dir, &now)} {
		if list, err := c.Index(context.Background()); err != nil || len(list.Entries) != 5 {
			t.Fatalf("within the week: %d entries, %v", len(list.Entries), err)
		}
	}
	if n := f.requests(); n != 1 {
		t.Errorf("%d requests within the week", n)
	}

	// After it, the ETag asks whether it changed: it didn't.
	now = now.Add(2 * 24 * time.Hour)
	c = newTestClient(f, dir, &now)
	if list, err := c.Index(context.Background()); err != nil || len(list.Entries) != 5 {
		t.Fatalf("a week on: %d entries, %v", len(list.Entries), err)
	}
	if r := f.last(); f.requests() != 2 || r.Header.Get("If-None-Match") != `"one"` {
		t.Errorf("%d requests, the last with %v", f.requests(), r.Header)
	}
	c.Index(context.Background())
	if n := f.requests(); n != 2 {
		t.Errorf("checked again at once: %d requests", n)
	}

	// Another week on, it changed.
	f.mu.Lock()
	f.index, f.etag = "- [New One](./Lab%20C/in-ear/New%20One) by Lab C\n", `"two"`
	f.mu.Unlock()
	now = now.Add(8 * 24 * time.Hour)
	if list, err := c.Index(context.Background()); err != nil || len(list.Entries) != 1 || list.Entries[0].Name != "New One" {
		t.Fatalf("changed: %+v, %v", list, err)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "INDEX.md")); err != nil || !strings.Contains(string(data), "New One") {
		t.Errorf("the index kept: %q, %v", data, err)
	}
}

// TestIndexOffline returns the index kept, and why it couldn't be
// checked, when the server can't be reached; with none kept, the error.
// The next call asks again.
func TestIndexOffline(t *testing.T) {
	f := newFakeAutoEq(t)
	dir := t.TempDir()
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	if _, err := newTestClient(f, dir, &now).Index(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.Close()
	now = now.Add(30 * 24 * time.Hour)
	c := newTestClient(f, dir, &now)
	for range 2 {
		list, err := c.Index(context.Background())
		if err != nil || len(list.Entries) != 5 || list.Offline == nil {
			t.Fatalf("offline: %d entries, %v, %v", len(list.Entries), list.Offline, err)
		}
	}
	if _, err := newTestClient(f, t.TempDir(), &now).Index(context.Background()); err == nil {
		t.Error("offline with no index kept: no error")
	}
}

// TestIndexAnswers turns down an answer other than the index, or an index
// of no profiles, keeping none.
func TestIndexAnswers(t *testing.T) {
	f := newFakeAutoEq(t)
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	f.index = "<html>rate limited</html>"
	dir := t.TempDir()
	if _, err := newTestClient(f, dir, &now).Index(context.Background()); !errors.Is(err, ErrEmptyIndex) {
		t.Errorf("a page of no profiles: %v", err)
	}
	c := newTestClient(f, dir, &now)
	c.Base = f.URL + "/gone/"
	var status *StatusError
	if _, err := c.Index(context.Background()); !errors.As(err, &status) || status.Code != http.StatusNotFound {
		t.Errorf("a 404: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "INDEX.md")); err == nil {
		t.Error("kept what isn't an index")
	}
}

// TestProfile fetches a profile from its folder, its name and path
// escaped, reads it, and keeps it: the next time, it plays without the
// network. One that doesn't read, is too big, or isn't there says why, and
// isn't kept.
func TestProfile(t *testing.T) {
	f := newFakeAutoEq(t)
	dir := t.TempDir()
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	c := newTestClient(f, dir, &now)
	list, err := c.Index(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byPath := func(path string) Entry {
		i := slices.IndexFunc(list.Entries, func(e Entry) bool { return e.Path == path })
		if i < 0 {
			t.Fatalf("no %s", path)
		}
		return list.Entries[i]
	}
	want := []settings.HeadphoneFilter{{Type: settings.FilterLowShelf, Fc: 105, Gain: 3.5, Q: 0.7}, {Type: settings.FilterPeak, Fc: 2400, Gain: -2, Q: 1.5}}
	for _, path := range []string{"Lab A/711 in-ear/Acme One", "Ted's Lab/EARS + 711 over-ear/Zed & Co Max+", "Lab A/over-ear/AüR Duo"} {
		e := byPath(path)
		if c.Kept(e) {
			t.Errorf("%s: kept before it was picked", path)
		}
		h, err := c.Profile(context.Background(), e)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if h.ID != e.Path || h.Name != e.Name || h.Source != e.Source || h.Rig != e.Rig || h.Preamp != -4.2 || !slices.Equal(h.Filters, want) {
			t.Errorf("%s: %+v", path, h)
		}
		if !c.Kept(e) {
			t.Errorf("%s: not kept", path)
		}
		if r := f.last(); r.Header.Get("User-Agent") != "MixMusic/test" {
			t.Errorf("%s: asked with %v", path, r.Header)
		}
	}
	if got := c.ProfileURL(byPath("Ted's Lab/EARS + 711 over-ear/Zed & Co Max+")); got != f.URL+"/results/Ted%27s%20Lab/EARS%20+%20711%20over-ear/Zed%20&%20Co%20Max+/Zed%20&%20Co%20Max+%20ParametricEQ.txt" {
		t.Errorf("the address: %s", got)
	}

	// Kept, it plays without the network.
	f.Close()
	if h, err := c.Profile(context.Background(), byPath("Lab A/711 in-ear/Acme One")); err != nil || !slices.Equal(h.Filters, want) {
		t.Errorf("offline: %+v, %v", h, err)
	}
	if _, err := c.Profile(context.Background(), Entry{Name: "Gone", Path: "Lab A/in-ear/Gone"}); err == nil {
		t.Error("offline, a profile not kept: no error")
	}
}

// TestProfileFails says why a profile can't be had: one that doesn't read,
// one too big, one that isn't there; none of them is kept. One kept that
// doesn't read any more is fetched again.
func TestProfileFails(t *testing.T) {
	f := newFakeAutoEq(t)
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	c := newTestClient(f, t.TempDir(), &now)
	var pe *settings.ParametricEQError
	bad := Entry{Name: "acme Buds (ANC Off)", Source: "Lab B", Path: "Lab B/earbud/acme Buds (ANC Off)"}
	if _, err := c.Profile(context.Background(), bad); !errors.As(err, &pe) || pe.Why != settings.UnknownFilter {
		t.Errorf("a profile of a band-pass: %v", err)
	}
	big := Entry{Name: "Acme One", Source: "Lab B", Path: "Lab B/in-ear/Acme One"}
	if _, err := c.Profile(context.Background(), big); !errors.Is(err, ErrTooBig) {
		t.Errorf("a profile too big: %v", err)
	}
	var status *StatusError
	if _, err := c.Profile(context.Background(), Entry{Name: "Gone", Path: "Lab A/in-ear/Gone"}); !errors.As(err, &status) || status.Code != 404 {
		t.Errorf("no such profile: %v", err)
	}
	for _, e := range []Entry{bad, big} {
		if c.Kept(e) {
			t.Errorf("%s kept", e.Path)
		}
	}

	good := Entry{Name: "Acme One", Source: "Lab A", Rig: "711", Path: "Lab A/711 in-ear/Acme One"}
	if err := os.MkdirAll(filepath.Dir(c.profileFile(good.Path)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(c.profileFile(good.Path), []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	if h, err := c.Profile(context.Background(), good); err != nil || h.Preamp != -4.2 {
		t.Errorf("a profile kept broken: %+v, %v", h, err)
	}
	if data, _ := os.ReadFile(c.profileFile(good.Path)); string(data) != fakeProfile {
		t.Errorf("kept %q", data)
	}
}

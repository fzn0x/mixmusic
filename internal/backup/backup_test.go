package backup

import (
	"archive/zip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// writeFiles writes files, by their paths with forward slashes, under dir.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func marshal(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func track(id, title string) media.Track {
	return media.Track{ID: id, Source: media.SourceYouTube, Title: title, Artists: []media.Ref{{Name: "Artist"}}}
}

// secrets are files of the data folder no backup holds.
var secrets = map[string]string{
	"spotify-token.json":      `{"access":"secret"}`,
	"spotify-token.dpapi":     "secret",
	"lastfm.json":             `{"key":"secret"}`,
	"listenbrainz.json":       `{"token":"secret"}`,
	"youtube-session.bin":     "secret",
	"youtube-session.json":    `{"cookies":"secret"}`,
	"remote.json":             `{"paired":[]}`,
	"mcp.json":                `{"key":"secret"}`,
	"lastfm-queue.json":       `[]`,
	"listenbrainz-queue.json": `[]`,
	"local-files.json":        `{}`,
	"updater.json":            `{}`,
	"bin/yt-dlp":              "binary",
	"logs/mixmusic.log":       "log",
}

// sourceData makes a data folder with a file of each part, and secrets.
func sourceData(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	lib := library.Data{
		Liked:     []media.Track{track("yt:a", "A"), track("yt:b", "B")},
		Disliked:  []media.Track{track("yt:c", "C")},
		Playlists: []library.Playlist{{ID: "m1", Title: "Mix", Cover: "m1-00.jpg", Tracks: []media.Track{track("yt:a", "A")}}},
		History:   []library.Play{{Track: track("yt:a", "A"), PlayedAt: time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)}},
		Artists:   []library.FollowedArtist{{ID: "ar1", Name: "Someone"}},
		Podcasts:  []media.Podcast{{ID: "pod1", Title: "Show"}},
	}
	s := settings.Default()
	s.Theme = "midnight"
	s.DownloadsFolder = filepath.Join(dir, "there-downloads")
	s.EnabledPlugins = []string{"spectrum"}
	writeFiles(t, dir, map[string]string{
		"library.json":                        marshal(t, lib),
		"plays.jsonl":                         `{"at":"2026-11-01T10:00:00Z","id":"yt:a","title":"A"}` + "\n" + `{"at":"2026-11-02T10:00:00Z","id":"yt:b","title":"B"}` + "\n",
		"mix-covers/m1-00.jpg":                "jpeg",
		"settings.json":                       marshal(t, s),
		"addons/themes/midnight/theme.json":   `{"id":"midnight"}`,
		"addons/plugins/spectrum/plugin.json": `{"id":"spectrum"}`,
		"addons/.storage/spectrum.json":       `{"k":"v"}`,
		"queue.json":                          `{"tracks":[` + marshal(t, track("yt:q", "Q")) + `],"index":0}`,
		"positions.json":                      `{}`,
	})
	writeFiles(t, dir, secrets)
	return dir
}

func source(dir string) Source {
	return Source{DataDir: dir, App: "0.5.12", MusicFolders: []string{`C:\Users\me\Music`},
		Downloads: DownloadList{Tracks: []media.Track{track("yt:d", "D")}},
		Now:       time.Date(2026, 11, 18, 14, 5, 9, 0, time.UTC)}
}

// TestWriteOpen writes a backup of every part and opens it: the manifest
// tells where it came from and lists each file with its size and hash,
// the preview counts what it holds, and no sign-in, pairing, scrobble
// queue, index, yt-dlp or log is in it.
func TestWriteOpen(t *testing.T) {
	dir := sourceData(t)
	path := filepath.Join(t.TempDir(), "b.zip")
	m, err := Write(path, source(dir), Parts)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.Parts, Parts) || m.Format != Format || m.App != "0.5.12" || m.System == "" {
		t.Errorf("manifest: %+v", m)
	}
	b, err := Open(path, filepath.Join(t.TempDir(), "work"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	var names []string
	for _, e := range b.Manifest.Entries {
		names = append(names, e.Path)
		if e.Size <= 0 || len(e.SHA256) != 64 {
			t.Errorf("entry %+v", e)
		}
	}
	want := []string{"addons/.storage/spectrum.json", "addons/plugins/spectrum/plugin.json", "addons/themes/midnight/theme.json",
		"downloads.json", "library.json", "mix-covers/m1-00.jpg", "plays.jsonl", "positions.json", "queue.json", "settings.json"}
	if !slices.Equal(names, want) {
		t.Errorf("entries %q, want %q", names, want)
	}
	for name := range secrets {
		if slices.Contains(names, name) {
			t.Errorf("%s is in the backup", name)
		}
	}
	if got := b.Manifest.MusicFolders; !slices.Equal(got, []string{`C:\Users\me\Music`}) {
		t.Errorf("music folders %q", got)
	}
	c := b.Counts()
	want2 := Counts{Liked: 2, Disliked: 1, Mixes: 1, Plays: 2, Artists: 1, Podcasts: 1, Addons: 2, Queue: 1, Downloads: 1}
	if c != want2 {
		t.Errorf("counts %+v, want %+v", c, want2)
	}
	sum, err := Sum(source(dir), Parts)
	if err != nil || sum != b.Manifest.Sum() {
		t.Errorf("sum %s (%v), manifest's %s", sum, err, b.Manifest.Sum())
	}
	writeFiles(t, dir, map[string]string{"positions.json": `{"x":1}`})
	if again, _ := Sum(source(dir), Parts); again == sum {
		t.Error("the sum didn't change with a file")
	}
}

// zipOf writes a zip of entries, with the manifest m unless it's nil.
func zipOf(t *testing.T, m *Manifest, entries map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "b.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	names := slices.Sorted(func(yield func(string) bool) {
		for k := range entries {
			if !yield(k) {
				return
			}
		}
	})
	for _, name := range names {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(entries[name]))
	}
	if m != nil {
		w, _ := z.Create(ManifestName)
		w.Write([]byte(marshal(t, m)))
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return path
}

// TestOpenRefuses refuses what isn't a backup, a damaged one, one from a
// newer MixMusic, one too large packed or unpacked, and one with an entry
// that would land outside the data folder, or that no part holds, telling
// which.
func TestOpenRefuses(t *testing.T) {
	dir := sourceData(t)
	good := filepath.Join(t.TempDir(), "b.zip")
	m, err := Write(good, source(dir), []Part{Settings})
	if err != nil {
		t.Fatal(err)
	}
	settingsText, _ := os.ReadFile(filepath.Join(dir, "settings.json"))
	entry := func(path string, part Part, text string) Entry {
		return Entry{Path: path, Part: part, Size: int64(len(text)), SHA256: m.Entries[0].SHA256}
	}
	newer := m
	newer.Format = Format + 1
	newer.App = "9.0.0"
	wrongHash := m
	wrongHash.Entries = []Entry{entry("settings.json", Settings, "{}")}
	outside := m
	outside.Entries = []Entry{entry("../settings.json", Settings, "{}")}
	foreign := m
	foreign.Parts = []Part{Settings, Library}
	foreign.Entries = []Entry{entry("bin/yt-dlp", Library, "{}")}
	device := m
	device.Parts = []Part{Addons}
	device.Entries = []Entry{entry("addons/themes/x/CON", Addons, "{}")}
	plain := filepath.Join(t.TempDir(), "notes.zip")
	os.WriteFile(plain, []byte("hello"), 0o644)
	truncated := filepath.Join(t.TempDir(), "cut.zip")
	data, _ := os.ReadFile(good)
	os.WriteFile(truncated, data[:len(data)-30], 0o644)

	for _, c := range []struct {
		name    string
		path    string
		problem Problem
		entry   string
	}{
		{"plain", plain, NotBackup, ""},
		{"truncated", truncated, Damaged, ""},
		{"no manifest", zipOf(t, nil, map[string]string{"settings.json": "{}"}), NotBackup, ""},
		{"newer", zipOf(t, &newer, map[string]string{"settings.json": string(settingsText)}), Newer, ""},
		{"hash", zipOf(t, &wrongHash, map[string]string{"settings.json": "{}"}), Damaged, "settings.json"},
		{"outside", zipOf(t, &outside, map[string]string{"../settings.json": "{}"}), Outside, "../settings.json"},
		{"unlisted", zipOf(t, &m, map[string]string{"settings.json": string(settingsText), "bin/yt-dlp": "x"}), Damaged, "bin/yt-dlp"},
		{"foreign", zipOf(t, &foreign, map[string]string{"bin/yt-dlp": "{}"}), Damaged, "bin/yt-dlp"},
		{"device", zipOf(t, &device, map[string]string{"addons/themes/x/CON": "{}"}), Outside, "addons/themes/x/CON"},
	} {
		t.Run(c.name, func(t *testing.T) {
			b, err := Open(c.path, t.TempDir())
			var be *Error
			if !errors.As(err, &be) || be.Problem != c.problem || be.Entry != c.entry {
				b.Close()
				t.Fatalf("Open: %v, want %s of %q", err, c.problem, c.entry)
			}
			if c.problem == Newer && be.App != "9.0.0" {
				t.Errorf("newer of %q", be.App)
			}
		})
	}

	maxPacked = 10
	t.Cleanup(func() { maxPacked = MaxPacked })
	if _, err := Open(good, t.TempDir()); !isProblem(err, TooLarge) {
		t.Errorf("a file past the bound: %v", err)
	}
	maxPacked, maxUnpacked = MaxPacked, 10
	t.Cleanup(func() { maxUnpacked = MaxUnpacked })
	if _, err := Open(good, t.TempDir()); !isProblem(err, TooLargeUnpacked) {
		t.Errorf("a backup unpacking past the bound: %v", err)
	}
}

func isProblem(err error, p Problem) bool {
	var be *Error
	return errors.As(err, &be) && be.Problem == p
}

// TestOpenUpgrades brings a backup of an older format up to date as it
// opens.
func TestOpenUpgrades(t *testing.T) {
	dir := sourceData(t)
	path := filepath.Join(t.TempDir(), "b.zip")
	m, err := Write(path, source(dir), []Part{Settings})
	if err != nil {
		t.Fatal(err)
	}
	old := m
	old.Format = 0
	text, _ := os.ReadFile(filepath.Join(dir, "settings.json"))
	path = zipOf(t, &old, map[string]string{"settings.json": string(text)})
	if _, err := Open(path, t.TempDir()); !isProblem(err, NotBackup) {
		t.Fatalf("format 0 opened: %v", err)
	}
	// A format before this one, which an upgrade brings up.
	upgraded := false
	saved := upgrades
	upgrades = map[int]func(*Manifest, string) error{1: func(m *Manifest, dir string) error {
		upgraded = true
		return os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{}"), 0o644)
	}}
	t.Cleanup(func() { upgrades = saved })
	format := Format
	old.Format = format
	path = zipOf(t, &old, map[string]string{"settings.json": string(text)})
	b, err := openAs(path, t.TempDir(), format+1)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if !upgraded || b.Manifest.Format != format+1 {
		t.Errorf("upgraded %v, format %d", upgraded, b.Manifest.Format)
	}
}

// TestStageApplyUndo restores the library and the settings of a backup
// made elsewhere: the next start swaps them in, the settings keep this
// computer's folders and devices, and plugins come back switched off;
// the queue, not chosen, stays. Undo puts back what it replaced.
func TestStageApplyUndo(t *testing.T) {
	there := sourceData(t)
	path := filepath.Join(t.TempDir(), "b.zip")
	if _, err := Write(path, source(there), Parts); err != nil {
		t.Fatal(err)
	}
	here := t.TempDir()
	mine := settings.Default()
	mine.DownloadsFolder = filepath.Join(here, "my-downloads")
	mine.MusicFolders = []string{filepath.Join(here, "Music")}
	mine.SpotifyDevice = "laptop"
	mine.EnabledPlugins = []string{"mine"}
	writeFiles(t, here, map[string]string{
		"library.json":       marshal(t, library.Data{Liked: []media.Track{track("yt:z", "Z")}}),
		"settings.json":      marshal(t, mine),
		"queue.json":         `{"tracks":[],"index":-1}`,
		"local-copies.json":  `{"version":1,"songs":[]}`,
		"spotify-token.json": "mine",
	})
	b, err := Open(path, Work(here))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Stage(here, Choice{Parts: []Part{Library, Settings, Addons, Downloads}, Download: true}); err != nil {
		t.Fatal(err)
	}
	b.Close()
	if !Staged(here) {
		t.Fatal("nothing staged")
	}
	// Nothing changes until the next start.
	if got, _ := os.ReadFile(filepath.Join(here, "queue.json")); string(got) != `{"tracks":[],"index":-1}` {
		t.Fatalf("the queue changed before the start: %s", got)
	}
	a := Apply(here)
	if a.Err != nil || !a.Restored || a.Downloads == nil || a.Downloads.Songs() != 1 {
		t.Fatalf("Apply: %+v", a)
	}
	var lib library.Data
	readJSON(filepath.Join(here, "library.json"), &lib)
	if len(lib.Liked) != 2 || lib.Liked[0].ID != "yt:a" {
		t.Errorf("library after: %+v", lib.Liked)
	}
	if _, err := os.Stat(filepath.Join(here, "local-copies.json")); err == nil {
		t.Error("the backup had no linked copies, which replaced those here: the file stayed")
	}
	data, _ := os.ReadFile(filepath.Join(here, "settings.json"))
	s, err := settings.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if s.Theme != "midnight" || s.DownloadsFolder != mine.DownloadsFolder || !slices.Equal(s.MusicFolders, mine.MusicFolders) ||
		s.SpotifyDevice != "laptop" || len(s.EnabledPlugins) != 0 {
		t.Errorf("settings after: theme %q, downloads %q, music %q, device %q, plugins %q", s.Theme, s.DownloadsFolder, s.MusicFolders, s.SpotifyDevice, s.EnabledPlugins)
	}
	if _, err := os.Stat(filepath.Join(here, "addons", "plugins", "spectrum", "plugin.json")); err != nil {
		t.Error("the add-ons didn't come")
	}
	if got, _ := os.ReadFile(filepath.Join(here, "queue.json")); string(got) != `{"tracks":[],"index":-1}` {
		t.Errorf("the queue, not chosen, changed: %s", got)
	}
	if got, _ := os.ReadFile(filepath.Join(here, "spotify-token.json")); string(got) != "mine" {
		t.Errorf("the sign-in changed: %s", got)
	}
	if Staged(here) {
		t.Error("the restore stays staged")
	}
	if again := Apply(here); again.Restored || again.Err != nil {
		t.Errorf("a second start applied again: %+v", again)
	}

	if _, ok := LastRestore(here); !ok {
		t.Fatal("no restore to undo")
	}
	if err := StageUndo(here); err != nil {
		t.Fatal(err)
	}
	if a := Apply(here); a.Err != nil || !a.Undone {
		t.Fatalf("undo: %+v", a)
	}
	readJSON(filepath.Join(here, "library.json"), &lib)
	if len(lib.Liked) != 1 || lib.Liked[0].ID != "yt:z" {
		t.Errorf("library after the undo: %+v", lib.Liked)
	}
	if got, _ := os.ReadFile(filepath.Join(here, "local-copies.json")); string(got) != `{"version":1,"songs":[]}` {
		t.Errorf("linked copies after the undo: %s", got)
	}
	if _, err := os.Stat(filepath.Join(here, "addons")); err == nil {
		t.Error("the add-ons the restore brought stayed")
	}
	data, _ = os.ReadFile(filepath.Join(here, "settings.json"))
	if s, _ := settings.Decode(data); !slices.Equal(s.EnabledPlugins, []string{"mine"}) {
		t.Errorf("plugins after the undo: %q", s.EnabledPlugins)
	}
	if _, ok := LastRestore(here); ok {
		t.Error("the undo can be undone")
	}
}

// TestApplyFails puts every file back when a swap fails partway, and
// tells why.
func TestApplyFails(t *testing.T) {
	there := sourceData(t)
	path := filepath.Join(t.TempDir(), "b.zip")
	if _, err := Write(path, source(there), Parts); err != nil {
		t.Fatal(err)
	}
	here := t.TempDir()
	writeFiles(t, here, map[string]string{"library.json": `{"liked":[]}`, "queue.json": `{"tracks":[]}`, "positions.json": `{"p":1}`})
	b, err := Open(path, Work(here))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Stage(here, Choice{Parts: []Part{Library, Queue}}); err != nil {
		t.Fatal(err)
	}
	b.Close()
	calls := 0
	swapRename = func(from, to string) error {
		calls++
		if calls == 4 {
			return errors.New("busy")
		}
		return os.Rename(from, to)
	}
	t.Cleanup(func() { swapRename = renameRetry })
	a := Apply(here)
	if a.Err == nil || a.Restored {
		t.Fatalf("Apply: %+v", a)
	}
	for name, want := range map[string]string{"library.json": `{"liked":[]}`, "queue.json": `{"tracks":[]}`, "positions.json": `{"p":1}`} {
		if got, err := os.ReadFile(filepath.Join(here, name)); err != nil || string(got) != want {
			t.Errorf("%s after a failed swap: %s (%v)", name, got, err)
		}
	}
	if _, ok := LastRestore(here); ok || Staged(here) {
		t.Error("a failed restore can be undone, or stays staged")
	}
}

// TestStageMerge adds the backup's library to the one here: likes, mixes
// and follows joined, a like here over a dislike in the file, the mix
// changed last kept, every play of both logs, the covers of both.
func TestStageMerge(t *testing.T) {
	there := sourceData(t)
	path := filepath.Join(t.TempDir(), "b.zip")
	if _, err := Write(path, source(there), Parts); err != nil {
		t.Fatal(err)
	}
	here := t.TempDir()
	newer := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	writeFiles(t, here, map[string]string{
		"library.json": marshal(t, library.Data{
			Liked:     []media.Track{track("yt:c", "C"), track("yt:z", "Z")},
			Playlists: []library.Playlist{{ID: "m1", Title: "Mine, newer", Updated: newer}},
		}),
		"plays.jsonl":          `{"at":"2026-11-02T10:00:00Z","id":"yt:b","title":"B"}` + "\n" + `{"at":"2026-11-05T10:00:00Z","id":"yt:z","title":"Z"}` + "\n",
		"mix-covers/m2-00.jpg": "mine",
	})
	b, err := Open(path, Work(here))
	if err != nil {
		t.Fatal(err)
	}
	added, err := b.Added(here, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Counts{Liked: 2, Disliked: 0, Mixes: 0, Plays: 1, Artists: 1, Podcasts: 1}); added != want {
		t.Errorf("added %+v, want %+v", added, want)
	}
	if err := b.Stage(here, Choice{Parts: []Part{Library}, Merge: true}); err != nil {
		t.Fatal(err)
	}
	b.Close()
	if a := Apply(here); a.Err != nil {
		t.Fatal(a.Err)
	}
	var lib library.Data
	readJSON(filepath.Join(here, "library.json"), &lib)
	var liked []string
	for _, x := range lib.Liked {
		liked = append(liked, x.ID)
	}
	if !slices.Equal(liked, []string{"yt:c", "yt:z", "yt:a", "yt:b"}) || len(lib.Disliked) != 0 {
		t.Errorf("liked %q, disliked %+v", liked, lib.Disliked)
	}
	if len(lib.Playlists) != 1 || lib.Playlists[0].Title != "Mine, newer" {
		t.Errorf("mixes %+v", lib.Playlists)
	}
	if len(lib.Artists) != 1 || len(lib.Podcasts) != 1 || len(lib.History) != 1 {
		t.Errorf("follows %d %d, history %d", len(lib.Artists), len(lib.Podcasts), len(lib.History))
	}
	if n, _ := library.CountPlays(filepath.Join(here, "plays.jsonl")); n != 3 {
		t.Errorf("%d plays, want 3", n)
	}
	for _, name := range []string{"m1-00.jpg", "m2-00.jpg"} {
		if _, err := os.Stat(filepath.Join(here, "mix-covers", name)); err != nil {
			t.Errorf("cover %s: %v", name, err)
		}
	}
}

// TestStageMovesSongs gives the songs of a backup made on Windows the
// IDs of the folder here that matches theirs, in the library, the play log
// and the queue, whatever the case of the folder's name.
func TestStageMovesSongs(t *testing.T) {
	there := t.TempDir()
	winID := func(p string) string { return "file:" + b64(p) }
	song := media.Track{ID: winID(`C:\Users\me\MUSIC\Album\one.flac`), Source: media.SourceFile, Title: "One"}
	other := media.Track{ID: winID(`D:\Elsewhere\two.flac`), Source: media.SourceFile, Title: "Two"}
	writeFiles(t, there, map[string]string{
		"library.json": marshal(t, library.Data{Liked: []media.Track{song, other}}),
		"plays.jsonl":  `{"at":"2026-11-01T10:00:00Z","id":"` + song.ID + `","source":"file","title":"One"}` + "\n",
		"queue.json":   `{"tracks":[` + marshal(t, song) + `],"index":0}`,
	})
	src := source(there)
	path := filepath.Join(t.TempDir(), "b.zip")
	m, err := Write(path, src, []Part{Library, Queue})
	if err != nil {
		t.Fatal(err)
	}
	// As made on Windows.
	m.System = "windows"
	entries := map[string]string{}
	for _, e := range m.Entries {
		data, _ := os.ReadFile(filepath.Join(there, filepath.FromSlash(e.Path)))
		entries[e.Path] = string(data)
	}
	path = zipOf(t, &m, entries)

	here := t.TempDir()
	music := filepath.Join(here, "Music")
	runtimeGOOS = "linux"
	t.Cleanup(func() { runtimeGOOS = goos() })
	folders := Suggest("windows", m.MusicFolders, []string{music})
	if folders[`C:\Users\me\Music`] != music {
		t.Fatalf("suggested %q", folders)
	}
	if got := Moved("windows", m.MusicFolders, []string{music}); len(got) != 1 {
		t.Errorf("moved %q", got)
	}
	runtimeGOOS = goos()
	b, err := Open(path, Work(here))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Stage(here, Choice{Parts: []Part{Library, Queue}, Folders: folders}); err != nil {
		t.Fatal(err)
	}
	b.Close()
	if a := Apply(here); a.Err != nil {
		t.Fatal(a.Err)
	}
	want := localfiles.TrackID(filepath.Join(music, "Album", "one.flac"))
	var lib library.Data
	readJSON(filepath.Join(here, "library.json"), &lib)
	if lib.Liked[0].ID != want || lib.Liked[1].ID != other.ID {
		t.Errorf("liked %q %q, want %q and the other as it was", lib.Liked[0].ID, lib.Liked[1].ID, want)
	}
	plays, _ := os.ReadFile(filepath.Join(here, "plays.jsonl"))
	if !strings.Contains(string(plays), want) {
		t.Errorf("plays %s", plays)
	}
	var q struct{ Tracks []media.Track }
	readJSON(filepath.Join(here, "queue.json"), &q)
	if len(q.Tracks) != 1 || q.Tracks[0].ID != want {
		t.Errorf("queue %+v", q.Tracks)
	}
}

// TestSchedule tells when an automatic backup is due, and keeps the last
// of them, deleting only its own.
func TestSchedule(t *testing.T) {
	last := time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		every string
		now   time.Time
		due   bool
	}{
		{settings.BackupOff, last.AddDate(1, 0, 0), false},
		{settings.BackupDaily, last.Add(23 * time.Hour), false},
		{settings.BackupDaily, last.Add(24 * time.Hour), true},
		{settings.BackupWeekly, last.AddDate(0, 0, 6), false},
		{settings.BackupWeekly, last.AddDate(0, 0, 7), true},
		{settings.BackupMonthly, last.AddDate(0, 0, 27), false},
		{settings.BackupMonthly, last.AddDate(0, 1, 0), true},
		{settings.BackupAtQuit, last, true},
	} {
		if got := Due(c.every, last, c.now); got != c.due {
			t.Errorf("Due(%q, %v) = %v", c.every, c.now.Sub(last), got)
		}
	}
	if !Due(settings.BackupWeekly, time.Time{}, last) {
		t.Error("no backup yet isn't due")
	}

	data := sourceData(t)
	folder := t.TempDir()
	writeFiles(t, folder, map[string]string{"MixMusic backup 2026-11-18.zip": "made by hand", "notes.txt": "mine"})
	src := source(data)
	for i := range 4 {
		src.Now = time.Date(2026, 11, 18, 10, i, 0, 0, time.UTC)
		if _, _, err := Auto(folder, 3, src); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(folder)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	want := []string{"MixMusic backup 2026-11-18 100100.zip", "MixMusic backup 2026-11-18 100200.zip", "MixMusic backup 2026-11-18 100300.zip",
		"MixMusic backup 2026-11-18.zip", "notes.txt"}
	if !slices.Equal(names, want) {
		t.Errorf("folder holds %q, want %q", names, want)
	}
	if _, path, err := Auto(folder, 1, src); err != nil || filepath.Base(path) != SingleName {
		t.Fatalf("one file: %s, %v", path, err)
	}
	entries, _ = os.ReadDir(folder)
	if len(entries) != 3 {
		t.Errorf("one file written over left %d files", len(entries))
	}
	_, _, err := Auto(filepath.Join(folder, "gone"), 5, src)
	var fe *FolderError
	if !errors.As(err, &fe) || !errors.Is(err, ErrFolderMissing) {
		t.Errorf("a missing folder: %v", err)
	}
}

func b64(p string) string { return base64.RawURLEncoding.EncodeToString([]byte(p)) }

func goos() string { return runtime.GOOS }

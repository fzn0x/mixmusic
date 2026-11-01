package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/volume"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// fileApp is an app of a folder of its own, whose settings.json holds
// text when it isn't empty. Its Spotify token is kept in a fake credential
// store, and a mistake in the file is told of at once.
func fileApp(t *testing.T, text string) *App {
	t.Helper()
	dir := t.TempDir()
	data := filepath.Join(dir, "data")
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}
	if text != "" {
		writeSettings(t, filepath.Join(data, "settings.json"), text)
	}
	a, err := New(Options{DataDir: data, CacheDir: filepath.Join(dir, "cache"), Builtin: builtin.FS, Version: "test", ProblemWait: -1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.artStore.Close)
	a.tokens = tokenStore{&secretStore{key: tokenSecret, keep: newKeeper()}}
	return a
}

// writeSettings saves text as an editor does: to a temporary file, renamed
// over the file.
func writeSettings(t *testing.T, path, text string) {
	t.Helper()
	if err := store.WriteFile(path, []byte(text)); err != nil {
		t.Fatal(err)
	}
}

func readSettings(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// told counts the times the windows are told the app changed.
func told(t *testing.T, a *App) *atomic.Int32 {
	var n atomic.Int32
	t.Cleanup(a.Watch(func() { n.Add(1) }))
	return &n
}

// until waits for ok, which an answer of a goroutine or a timer makes so.
func until(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); !ok(); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("%s didn't happen", what)
		}
	}
}

func TestReloadSettingsApplies(t *testing.T) {
	a := fileApp(t, `{"theme": "mix"}`)
	n := told(t, a)
	prefs := &Preferences{a}
	writeSettings(t, a.settings.Path(), `{"theme": "aurora", "spotifyClientId": "abc", "crosfade": 6, "fontScale": 2}`)
	a.ReloadSettings()
	if got := prefs.Get(); got.Theme != "aurora" || got.SpotifyClientID != "abc" {
		t.Errorf("Get = %q, %q", got.Theme, got.SpotifyClientID)
	}
	// As Settings applies a change.
	if got := a.auth.ClientID(); got != "abc" {
		t.Errorf("the Spotify client ID is %q", got)
	}
	if n.Load() != 1 {
		t.Errorf("the windows were told %d times, want once", n.Load())
	}
	f := prefs.File()
	if f.Seq != 1 || f.Notice.Kind != NoticeApplied || !slices.Equal(f.Notice.Unknown, []string{"crosfade"}) ||
		!slices.Equal(f.Notice.Adjusted, []string{"fontScale"}) || f.Problem != "" {
		t.Errorf("File = %+v", f)
	}
	// The edit stays as it was saved.
	if got := readSettings(t, a.settings.Path()); !strings.Contains(got, "crosfade") {
		t.Errorf("the file was written: %s", got)
	}
}

func TestReloadSettingsIgnoresOwnSave(t *testing.T) {
	a := fileApp(t, "")
	prefs := &Preferences{a}
	if _, err := prefs.Update(func(s *settings.Settings) { s.Crossfade = 4 }); err != nil {
		t.Fatal(err)
	}
	n := told(t, a)
	a.ReloadSettings()
	if n.Load() != 0 || prefs.File().Seq != 0 {
		t.Errorf("MixMusic's own save told the windows %d times, File %+v", n.Load(), prefs.File())
	}
}

func TestReloadSettingsBroken(t *testing.T) {
	a := fileApp(t, `{"theme": "aurora"}`)
	a.opts.ProblemWait = 50 * time.Millisecond
	prefs := &Preferences{a}
	path := a.settings.Path()
	const broken = "{\n  \"theme\": \"daylight\",\n}"
	writeSettings(t, path, broken)
	a.ReloadSettings()
	// A file caught in the middle of a save isn't told of at once.
	if f := prefs.File(); f.Problem != "" {
		t.Errorf("told at once: %+v", f)
	}
	until(t, "the mistake told", func() bool { return prefs.File().Problem != "" })
	f := prefs.File()
	if f.Problem != "line 3, column 1: a comma before } has nothing after it" || f.Notice.Kind != NoticeProblem || f.Notice.Problem != f.Problem {
		t.Errorf("File = %+v", f)
	}
	if got := prefs.Get().Theme; got != "aurora" {
		t.Errorf("the theme is %q, want the one it had", got)
	}
	// MixMusic's own changes apply, and wait to be saved.
	if _, err := prefs.Update(func(s *settings.Settings) { s.Crossfade = 5 }); err != nil {
		t.Fatal(err)
	}
	if got := prefs.Get().Crossfade; got != 5 {
		t.Errorf("crossfade %d, want 5", got)
	}
	if got := readSettings(t, path); got != broken {
		t.Errorf("the broken file was written: %q", got)
	}
	if _, err := os.Stat(path + ".broken"); !os.IsNotExist(err) {
		t.Errorf("settings.json.broken: %v", err)
	}
	// Fixed, it applies, with what changed meanwhile.
	seq := prefs.File().Seq
	writeSettings(t, path, `{"theme": "daylight"}`)
	a.ReloadSettings()
	f = prefs.File()
	if f.Problem != "" || f.Seq != seq+1 || f.Notice.Kind != NoticeApplied {
		t.Errorf("File after the fix = %+v", f)
	}
	if got := prefs.Get(); got.Theme != "daylight" || got.Crossfade != 5 {
		t.Errorf("Get = %q, %d", got.Theme, got.Crossfade)
	}
	saved, err := settings.Decode([]byte(readSettings(t, path)))
	if err != nil || saved.Theme != "daylight" || saved.Crossfade != 5 {
		t.Errorf("the file holds %q, %d: %v", saved.Theme, saved.Crossfade, err)
	}
}

// TestBrokenAtQuit quits while settings.json has a mistake: it is kept as
// settings.json.broken, and the settings in its place.
func TestBrokenAtQuit(t *testing.T) {
	a := fileApp(t, `{"theme": "aurora"}`)
	path := a.settings.Path()
	writeSettings(t, path, `{"theme": `)
	a.ReloadSettings()
	if _, err := (&Preferences{a}).Update(func(s *settings.Settings) { s.Crossfade = 3 }); err != nil {
		t.Fatal(err)
	}
	a.flush()
	if got := readSettings(t, path+".broken"); got != `{"theme": ` {
		t.Errorf("settings.json.broken holds %q", got)
	}
	s, err := settings.Decode([]byte(readSettings(t, path)))
	if err != nil || s.Theme != "aurora" || s.Crossfade != 3 {
		t.Errorf("settings.json holds %q, %d: %v", s.Theme, s.Crossfade, err)
	}
	// Gone, it is written again.
	os.Remove(path)
	a.flush()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("settings.json wasn't written again: %v", err)
	}
}

func TestResetWhileBroken(t *testing.T) {
	a := fileApp(t, `{"theme": "aurora", "ytdlpPath": "/opt/yt-dlp"}`)
	prefs := &Preferences{a}
	path := a.settings.Path()
	writeSettings(t, path, `{"theme": "daylight",}`)
	a.ReloadSettings()
	until(t, "the mistake told", func() bool { return prefs.File().Problem != "" })
	s, err := prefs.Reset()
	if err != nil {
		t.Fatal(err)
	}
	if s.Theme != "mix" || s.YTDLPPath != "/opt/yt-dlp" || prefs.File().Problem != "" {
		t.Errorf("Reset = %q, %q; File %+v", s.Theme, s.YTDLPPath, prefs.File())
	}
	if got := readSettings(t, path+".broken"); got != `{"theme": "daylight",}` {
		t.Errorf("settings.json.broken holds %q", got)
	}
	if saved, err := settings.Decode([]byte(readSettings(t, path))); err != nil || saved.Theme != "mix" {
		t.Errorf("the file holds %q: %v", saved.Theme, err)
	}
}

// TestSaveVolumeKeepsAnUnreadEdit saves the volume, as the player does
// 600 ms after it stops moving, just after the file was edited.
func TestSaveVolumeKeepsAnUnreadEdit(t *testing.T) {
	a := fileApp(t, `{"theme": "mix"}`)
	writeSettings(t, a.settings.Path(), `{"theme": "aurora"}`)
	playerServices{a}.SaveVolume(0.3)
	if got := a.settings.Get(); got.Theme != "aurora" || got.Volume != 0.3 {
		t.Errorf("Get = %q, %v", got.Theme, got.Volume)
	}
	saved, err := settings.Decode([]byte(readSettings(t, a.settings.Path())))
	if err != nil || saved.Theme != "aurora" || saved.Volume != 0.3 {
		t.Errorf("the file holds %q, %v: %v", saved.Theme, saved.Volume, err)
	}
}

// TestChangeKeepsAReload saves a window's change from the settings it read
// before an edit applied: the edit stays.
func TestChangeKeepsAReload(t *testing.T) {
	a := fileApp(t, `{"theme": "mix"}`)
	prefs := &Preferences{a}
	base := prefs.Get()
	writeSettings(t, a.settings.Path(), `{"theme": "aurora", "crossfade": 6}`)
	a.ReloadSettings()
	next := base
	next.Density = settings.DensityCompact
	s, err := prefs.Change(base, next)
	if err != nil {
		t.Fatal(err)
	}
	if s.Theme != "aurora" || s.Crossfade != 6 || s.Density != settings.DensityCompact {
		t.Errorf("Change = %q, %d, %q", s.Theme, s.Crossfade, s.Density)
	}
}

func TestReloadSettingsSignsOutOnNewClientID(t *testing.T) {
	a := fileApp(t, `{"spotifyClientId": "former"}`)
	a.spotify.SetToken(spotify.Token{AccessToken: "ACCESS", Expiry: time.Now().Add(time.Hour)})
	if !a.spotify.SignedIn() {
		t.Fatal("not signed in")
	}
	writeSettings(t, a.settings.Path(), `{"spotifyClientId": "another"}`)
	a.ReloadSettings()
	if a.spotify.SignedIn() || a.auth.ClientID() != "another" {
		t.Errorf("signed in %v, client ID %q", a.spotify.SignedIn(), a.auth.ClientID())
	}
}

func TestReloadSettingsForgetsTheChannelsProfile(t *testing.T) {
	a := fileApp(t, `{"youtubeChannel": "111"}`)
	a.youtubeMe = &youtube.Account{Name: "The former channel"}
	writeSettings(t, a.settings.Path(), `{"youtubeChannel": "222"}`)
	a.ReloadSettings()
	a.mu.Lock()
	me := a.youtubeMe
	a.mu.Unlock()
	if me != nil {
		t.Errorf("the profile of the former channel stays: %+v", me)
	}
}

func TestReloadSettingsFollowsVolume(t *testing.T) {
	a := fileApp(t, `{"volume": 0.8}`)
	var err error
	if a.queue, err = store.Open(filepath.Join(a.opts.DataDir, "queue.json"), player.Saved{}); err != nil {
		t.Fatal(err)
	}
	stream := &quietStream{}
	p := player.New(offlineServices{playerServices{a}}, stream, connectDevice{a}, player.Saved{})
	stream.p.Store(p)
	a.goPlayer.Store(p)
	t.Cleanup(p.Close)
	writeSettings(t, a.settings.Path(), `{"volume": 0.37}`)
	a.ReloadSettings()
	until(t, "the player at 0.37", func() bool { return p.State().Volume == volume.Of(0.37) })
	// Nothing saves over the edit: the player saves no volume it got from
	// the settings.
	p.Flush()
	if got := readSettings(t, a.settings.Path()); got != `{"volume": 0.37}` {
		t.Errorf("the file was written: %s", got)
	}
}

func TestTellStart(t *testing.T) {
	t.Run("aside", func(t *testing.T) {
		a := fileApp(t, `{"theme": "aurora",}`)
		a.tellStart()
		f := (&Preferences{a}).File()
		if f.Seq != 1 || f.Notice.Kind != NoticeAside || f.Notice.Problem != "line 1, column 20: a comma before } has nothing after it" {
			t.Errorf("File = %+v", f)
		}
		if got := readSettings(t, a.settings.Path()+".broken"); got != `{"theme": "aurora",}` {
			t.Errorf("settings.json.broken holds %q", got)
		}
	})
	t.Run("not UTF-8", func(t *testing.T) {
		a := fileApp(t, "{\"ytdlpPath\": \"C:\\\\\x91\xBE\"}")
		a.tellStart()
		if f := (&Preferences{a}).File(); f.Seq != 1 || f.Notice.Kind != NoticeStart || !f.Notice.NotUTF8 {
			t.Errorf("File = %+v", f)
		}
	})
	t.Run("fine", func(t *testing.T) {
		a := fileApp(t, `{"theme": "aurora"}`)
		a.tellStart()
		if f := (&Preferences{a}).File(); f.Seq != 0 {
			t.Errorf("File = %+v", f)
		}
	})
	// A byte order mark is fine now: the file isn't moved aside.
	t.Run("byte order mark", func(t *testing.T) {
		a := fileApp(t, "\xEF\xBB\xBF{\"theme\": \"aurora\"}")
		if a.settings.Aside() != nil || a.settings.Get().Theme != "aurora" {
			t.Errorf("Aside %v, theme %q", a.settings.Aside(), a.settings.Get().Theme)
		}
	})
}

// TestWatchSettingsFile edits settings.json while MixMusic watches it, as
// it does once it started.
func TestWatchSettingsFile(t *testing.T) {
	if testing.Short() {
		t.Skip("watches files")
	}
	a := fileApp(t, `{"theme": "mix"}`)
	changed := make(chan struct{}, 16)
	t.Cleanup(a.Watch(func() {
		select {
		case changed <- struct{}{}:
		default:
		}
	}))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); a.watchSettings(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
	until(t, "the watch", func() bool { return a.watcher.Load() != nil })

	wait := func(what string, ok func(s settings.Settings) bool) {
		t.Helper()
		deadline := time.After(10 * time.Second)
		for !ok(a.settings.Get()) {
			select {
			case <-changed:
			case <-deadline:
				t.Fatalf("%s didn't apply: %+v", what, a.settings.Get())
			}
		}
	}
	writeSettings(t, a.settings.Path(), `{"theme": "aurora"}`)
	wait("the theme", func(s settings.Settings) bool { return s.Theme == "aurora" })
	// Written in place, as some editors save.
	if err := os.WriteFile(a.settings.Path(), []byte(`{"theme": "aurora", "crossfade": 7}`), 0o644); err != nil {
		t.Fatal(err)
	}
	wait("the crossfade", func(s settings.Settings) bool { return s.Crossfade == 7 })
}

// TestWatchFollowsALink watches the file a settings.json that is a link
// links to, and the one it links to next.
func TestWatchFollowsALink(t *testing.T) {
	if testing.Short() {
		t.Skip("watches files")
	}
	a := fileApp(t, "")
	dir := t.TempDir()
	first, second := filepath.Join(dir, "first.json"), filepath.Join(dir, "second.json")
	writeSettings(t, first, `{"theme": "aurora"}`)
	writeSettings(t, second, `{"theme": "daylight"}`)
	if err := os.Symlink(first, a.settings.Path()); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); a.watchSettings(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
	until(t, "the watch", func() bool { return a.watcher.Load() != nil })
	until(t, "the first file read", func() bool { return a.settings.Get().Theme == "aurora" })

	writeSettings(t, first, `{"theme": "mochi"}`)
	until(t, "an edit of the first file", func() bool { return a.settings.Get().Theme == "mochi" })
	// The link goes to the second file now; MixMusic saves through it.
	w := a.watcher.Load()
	if err := os.Remove(a.settings.Path()); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(second, a.settings.Path()); err != nil {
		t.Fatal(err)
	}
	if _, err := (&Preferences{a}).Update(func(s *settings.Settings) { s.Crossfade = 2 }); err != nil {
		t.Fatal(err)
	}
	until(t, "the watch of the second file", func() bool { w2 := a.watcher.Load(); return w2 != nil && w2 != w })
	writeSettings(t, second, `{"theme": "glasshouse", "crossfade": 2}`)
	until(t, "an edit of the second file", func() bool { return a.settings.Get().Theme == "glasshouse" })
	var kept settings.Settings
	if err := json.Unmarshal([]byte(readSettings(t, first)), &kept); err != nil || kept.Theme != "mochi" {
		t.Errorf("the first file holds %q: %v", kept.Theme, err)
	}
}

func TestStatChanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	writeSettings(t, path, "{}")
	a, _ := os.Stat(path)
	if statChanged(a, a) || !statChanged(nil, a) || !statChanged(a, nil) || statChanged(nil, nil) {
		t.Error("statChanged is wrong about the same file, or a missing one")
	}
	writeSettings(t, path, `{"theme": "aurora"}`)
	b, _ := os.Stat(path)
	if !statChanged(a, b) {
		t.Error("a file saved again didn't change")
	}
}

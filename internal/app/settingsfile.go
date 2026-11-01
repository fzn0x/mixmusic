package app

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/volume"
)

// The kinds of FileNotice.
const (
	// NoticeAside tells that settings.json didn't read as MixMusic started,
	// which kept it as settings.json.broken, and started with the
	// settings of a new install.
	NoticeAside = "aside"
	// NoticeStart tells that settings.json read as MixMusic started, but
	// wasn't UTF-8.
	NoticeStart = "start"
	// NoticeProblem tells that settings.json has a mistake.
	NoticeProblem = "problem"
	// NoticeApplied tells that an edit of settings.json applied.
	NoticeApplied = "applied"
)

// SettingsFile is settings.json, as the windows tell of it.
type SettingsFile struct {
	// Path is where it is.
	Path string
	// Problem is the mistake it has, once it has had it a moment, as
	// "line 4, column 1: a comma before } has nothing after it"; "" while
	// it reads. MixMusic doesn't save to it meanwhile.
	Problem string
	// Seq counts the notices, the last of which is Notice: a window tells
	// of it once Seq grows.
	Seq    int
	Notice FileNotice
}

// FileNotice is what became of settings.json: Kind says what.
type FileNotice struct {
	Kind string
	// Problem is why the file doesn't read, of NoticeAside and
	// NoticeProblem.
	Problem string
	// Of NoticeApplied and NoticeStart: what of the file MixMusic didn't
	// take as it was.
	settings.Report
}

// problemWait is how long a settings.json that doesn't read waits to be
// told of: an editor that writes a file in place leaves it empty, or half
// written, for a moment.
const problemWait = time.Second

// fileState is what the windows are told of settings.json, which
// App.fileMu guards.
type fileState struct {
	// broken is the mistake the file has, told or not yet; problem the one
	// told, "" while none; wait waits to tell of broken.
	broken  error
	problem string
	wait    *time.Timer
	seq     int
	notice  FileNotice
}

// Watching settings.json.
var (
	// watchOptions waits for an editor to stop saving, a quarter of a
	// second, before the file is read again.
	watchOptions = mygo.FileWatchOptions{Debounce: 250 * time.Millisecond, MaxDelay: 2 * time.Second}
	// pollEvery is how often the file is looked at where it can't be
	// watched, as when Linux has no inotify watches left; for pollFor,
	// before it is watched again.
	pollEvery = 2 * time.Second
	pollFor   = time.Minute
	// watchAgain is how long a watch waits to start again once it ended,
	// as when the data folder went away.
	watchAgain = 5 * time.Second
)

// File tells of settings.json.
func (p *Preferences) File() SettingsFile {
	a := p.a
	a.fileMu.Lock()
	defer a.fileMu.Unlock()
	return SettingsFile{Path: a.settings.Path(), Problem: a.file.problem, Seq: a.file.seq, Notice: a.file.notice}
}

// watchSettings reads settings.json again whenever it changes, until ctx
// is done. Where it can't watch it, it looks at it every few seconds.
func (a *App) watchSettings(ctx context.Context) {
	path := a.settings.Path()
	told := false
	for ctx.Err() == nil {
		// A missing file can't be watched; nor opened, in the settings.
		if err := a.settings.Create(); err != nil {
			log.Printf("mixmusic: saving settings.json: %v", err)
		}
		w, err := mygo.WatchFiles(ctx, path, watchOptions)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if !told {
				log.Printf("mixmusic: watching settings.json: %v; looking at it every %v instead", err, pollEvery)
				told = true
			}
			a.pollSettings(ctx, pollFor)
			continue
		}
		a.watcher.Store(w)
		// What changed since MixMusic read it, or since the watch before.
		a.ReloadSettings()
		for {
			if _, err := w.Next(ctx); err != nil {
				break
			}
			a.ReloadSettings()
			if moved(path, w.Root()) {
				break
			}
		}
		a.watcher.CompareAndSwap(w, nil)
		w.Close()
		if !moved(path, w.Root()) && !sleep(ctx, watchAgain) {
			return
		}
	}
}

// moved tells whether path, a link, links to a file other than root now:
// a watch follows links only as it starts.
func moved(path, root string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	now, err := filepath.EvalSymlinks(abs)
	return err == nil && now != root
}

// rewatch starts the watch of settings.json again when it links to
// another file now, which MixMusic just saved to.
func (a *App) rewatch() {
	if w := a.watcher.Load(); w != nil && moved(a.settings.Path(), w.Root()) {
		w.Close()
	}
}

// sleep waits d, or until ctx is done, which it reports as false.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// pollSettings looks at settings.json every pollEvery, for d or until ctx
// is done, and reads it again when it changed.
func (a *App) pollSettings(ctx context.Context, d time.Duration) {
	path := a.settings.Path()
	a.ReloadSettings()
	last, _ := os.Stat(path)
	tick := time.NewTicker(pollEvery)
	defer tick.Stop()
	end := time.NewTimer(d)
	defer end.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-end.C:
			return
		case <-tick.C:
		}
		fi, _ := os.Stat(path)
		if statChanged(last, fi) {
			a.ReloadSettings()
		}
		last = fi
	}
}

// statChanged tells whether a file changed from what was, as os.Stat
// tells of it: nil when it is missing.
func statChanged(was, now os.FileInfo) bool {
	if was == nil || now == nil {
		return (was == nil) != (now == nil)
	}
	return was.Size() != now.Size() || !was.ModTime().Equal(now.ModTime()) || !os.SameFile(was, now)
}

// ReloadSettings reads settings.json again, and applies what changed, as
// Settings applies a change: MixMusic's own saves change nothing. A file
// that doesn't read is told of once it has had a mistake for problemWait;
// the settings stay as they were meanwhile. The watch of the file calls it
// as the file changes.
func (a *App) ReloadSettings() {
	c, err := a.settings.Reload()
	if held := a.settings.Held(); held != nil {
		a.settingsBroken(held)
		return
	}
	if err != nil {
		log.Printf("mixmusic: reading settings.json again: %v", err)
	}
	told := a.settingsRead()
	var report settings.Report
	if c.Data != nil {
		report = settings.Inspect(c.Data, c.Value)
	}
	if c.Data != nil && (c.Changed || told || !report.Empty()) {
		a.fileMu.Lock()
		a.file.seq++
		a.file.notice = FileNotice{Kind: NoticeApplied, Report: report}
		a.fileMu.Unlock()
	}
	switch {
	case c.Changed:
		a.settingsChanged(c.Prev.Clean(), c.Value.Clean(), true)
	case told || c.Data != nil && !report.Empty():
		notify()
	}
}

// settingsBroken has the windows told, once it stays a moment, that
// settings.json has the mistake err.
func (a *App) settingsBroken(err error) {
	a.fileMu.Lock()
	defer a.fileMu.Unlock()
	a.file.broken = err
	if a.file.problem == err.Error() {
		return
	}
	if a.file.wait != nil {
		a.file.wait.Stop()
	}
	wait := problemWait
	if a.opts.ProblemWait != 0 {
		wait = max(a.opts.ProblemWait, 0)
	}
	a.file.wait = time.AfterFunc(wait, a.tellProblem)
}

// tellProblem tells the windows of the mistake settings.json has, and the
// log of the first of a run of them.
func (a *App) tellProblem() {
	a.fileMu.Lock()
	if a.file.broken == nil || a.file.problem == a.file.broken.Error() {
		a.fileMu.Unlock()
		return
	}
	first := a.file.problem == ""
	a.file.problem = a.file.broken.Error()
	a.file.seq++
	a.file.notice = FileNotice{Kind: NoticeProblem, Problem: a.file.problem}
	problem := a.file.problem
	a.fileMu.Unlock()
	if first {
		log.Printf("mixmusic: settings.json has a mistake (%s): MixMusic goes on with the settings it had, and saves to it once it reads again", problem)
	}
	notify()
}

// settingsRead forgets the mistake settings.json had, now that it reads
// again, and tells whether the windows were told of it.
func (a *App) settingsRead() (told bool) {
	a.fileMu.Lock()
	defer a.fileMu.Unlock()
	if a.file.wait != nil {
		a.file.wait.Stop()
		a.file.wait = nil
	}
	told = a.file.problem != ""
	a.file.broken, a.file.problem = nil, ""
	return told
}

// tellStart tells the windows how settings.json read as MixMusic started:
// that it didn't, and is settings.json.broken now, or that it wasn't
// UTF-8. It runs once the main window opened.
func (a *App) tellStart() {
	n := FileNotice{}
	if aside := a.settings.Aside(); aside != nil {
		n = FileNotice{Kind: NoticeAside, Problem: aside.Error()}
	} else if data, err := os.ReadFile(a.settings.Path()); err == nil {
		if _, ok := settings.Text(data); !ok {
			n = FileNotice{Kind: NoticeStart, Report: settings.Report{NotUTF8: true}}
		}
	}
	if n.Kind == "" {
		return
	}
	a.fileMu.Lock()
	a.file.seq++
	a.file.notice = n
	a.fileMu.Unlock()
	notify()
}

// settingsChanged makes the settings s take effect, which were prev: as
// Settings applies a change, and the same way whether it came from a
// window or, outside, from an edit of settings.json. Then it tells the
// windows.
func (a *App) settingsChanged(prev, s settings.Settings, outside bool) {
	if prev.SpotifyClientID != s.SpotifyClientID && a.spotify.SignedIn() {
		// The token belongs to the former app.
		a.signOutSpotify()
	}
	if prev.YouTubeChannel != s.YouTubeChannel {
		// The profile is the former channel's.
		a.mu.Lock()
		a.youtubeMe, a.profileErrs.youtube = nil, ""
		a.mu.Unlock()
	}
	a.apply(s)
	if p := a.goPlayer.Load(); outside && p != nil && prev.Volume != s.Volume {
		// The player keeps the volume it plays at, which it saves.
		p.SetVolume(volume.Of(s.Volume))
	}
	notify()
}

// ErrNoJSONApp is what OpenSettingsFile returns where no app opens JSON
// files, once it showed settings.json in its folder instead.
var ErrNoJSONApp = errors.New("no app on this computer opens .json files, so MixMusic showed settings.json in its folder")

// OpenSettingsFile opens settings.json in the app the computer opens JSON
// files with; where none does, it shows the file in its folder, and
// returns ErrNoJSONApp.
func (s *System) OpenSettingsFile() error {
	path := s.a.settings.Path()
	if err := s.a.settings.Create(); err != nil {
		return err
	}
	if err := mygo.Shell.OpenPath(path); err != nil {
		log.Printf("mixmusic: opening settings.json: %v", err)
		mygo.Shell.ShowItemInFolder(path)
		return ErrNoJSONApp
	}
	return nil
}

// ShowSettingsFile shows settings.json in its folder.
func (s *System) ShowSettingsFile() error {
	if err := s.a.settings.Create(); err != nil {
		return err
	}
	mygo.Shell.ShowItemInFolder(s.a.settings.Path())
	return nil
}

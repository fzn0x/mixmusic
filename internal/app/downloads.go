package app

// Downloads: the tracks the user chose to play without the network.

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/downloads"
	"github.com/fzn0x/mixmusic/internal/media"
)

// Downloads keeps the tracks the user chose to play without the network,
// apart from the offline cache, for the windows.
type Downloads struct{ a *App }

// openDownloads opens the downloads in the folder of the settings, else
// opts.DownloadsDir, else the data folder's.
func (a *App) openDownloads(folder string) {
	if folder == "" {
		folder = a.defaultDownloads()
	}
	o := downloads.Options{Fetcher: a.proxy, Matcher: a.matcher, Cache: a.cache, Network: a.online, Art: a.artStore.Bytes,
		Avoid: []string{filepath.Join(a.opts.CacheDir, "audio"), filepath.Join(a.opts.CacheDir, "artwork")}}
	a.downloads = downloads.Open(folder, o)
	a.proxy.Kept = a.downloads
}

// defaultDownloads is the folder the downloads are in unless the user
// chose another.
func (a *App) defaultDownloads() string {
	if a.opts.DownloadsDir != "" {
		return a.opts.DownloadsDir
	}
	return filepath.Join(a.opts.DataDir, "downloads")
}

// quality is the quality downloads take: the settings'.
func (a *App) quality() string { return string(a.settings.Get().Clean().Quality) }

// Snapshot returns what the downloads hold and do now: the windows read it
// in each frame.
func (d *Downloads) Snapshot() *downloads.Snapshot { return d.a.downloads.Snapshot() }

// Watch has fn told, from any goroutine, when what the downloads hold or do
// changes, until cancel.
func (d *Downloads) Watch(fn func()) (cancel func()) { return d.a.downloads.Watch(fn) }

// OnNetwork has fn told, from any goroutine, when the network goes, or
// comes back, until cancel.
func (d *Downloads) OnNetwork(fn func(online bool)) (cancel func()) {
	if d.a.online == nil {
		return func() {}
	}
	return d.a.online.OnChange(fn)
}

// Add downloads tracks, in the quality of the settings, and returns how
// many it added.
func (d *Downloads) Add(tracks []media.Track) int { return d.a.downloads.Add(tracks, d.a.quality()) }

// AddCollection downloads the tracks of an album, a playlist, a mix or the
// liked songs, and returns how many it added.
func (d *Downloads) AddCollection(c downloads.Collection, tracks []media.Track) int {
	return d.a.downloads.AddCollection(c, tracks, d.a.quality())
}

// Remove removes the downloads of tracks.
func (d *Downloads) Remove(ids ...string) { d.a.downloads.Remove(ids...) }

// RemoveCollection removes the download of a collection, and its tracks
// that no other download keeps.
func (d *Downloads) RemoveCollection(id string) { d.a.downloads.RemoveCollection(id) }

// RemoveAll removes every download.
func (d *Downloads) RemoveAll() { d.a.downloads.RemoveAll() }

// CancelAll stops the downloads under way and those that wait.
func (d *Downloads) CancelAll() { d.a.downloads.CancelAll() }

// Retry tries a download that failed again.
func (d *Downloads) Retry(id string) { d.a.downloads.Retry(id) }

// RetryFailed tries the downloads that failed again, and those that wait
// for YouTube.
func (d *Downloads) RetryFailed() { d.a.downloads.RetryFailed() }

// SetPaused pauses or resumes the downloads.
func (d *Downloads) SetPaused(paused bool) { d.a.downloads.SetPaused(paused) }

// Estimate is about how many bytes tracks take, downloaded in the quality
// of the settings.
func (d *Downloads) Estimate(tracks []media.Track) int64 {
	return downloads.Estimate(tracks, d.a.quality())
}

// Folder returns the folder the downloads are in, and Default the one they
// are in unless the user chose another.
func (d *Downloads) Folder() string  { return d.a.downloads.Folder() }
func (d *Downloads) Default() string { return d.a.defaultDownloads() }

// OpenFolder opens the folder of the downloads in the file manager.
func (d *Downloads) OpenFolder() error {
	dir := d.a.downloads.Folder()
	if dir == "" {
		return errors.New("MixMusic keeps no downloads here")
	}
	return mygo.Shell.OpenPath(dir)
}

// ChooseFolder asks the user for a folder to move the downloads to, over
// parent: "" when they chose none. It waits for their answer.
func (d *Downloads) ChooseFolder(parent *mygo.Window) (string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: parent, Title: "Choose where MixMusic keeps downloads",
		Directory: true, CreateDirectories: true, DefaultPath: d.Folder()})
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

// Move moves the downloads to dir, telling progress how many of their bytes
// moved, and returns the folder they are in then, which the settings keep
// as DownloadsFolder: "" for the default one, where they move whole.
func (d *Downloads) Move(ctx context.Context, dir string, progress func(done, total int64)) (string, error) {
	def := d.Default()
	exact := filepath.Clean(dir) == filepath.Clean(def)
	got, err := d.a.downloads.Move(ctx, dir, exact, progress)
	if err != nil {
		return "", err
	}
	if filepath.Clean(got) == filepath.Clean(def) {
		return "", nil
	}
	return got, nil
}

// ArtworkFile returns the file that keeps the cover at url, if a download
// keeps it.
func (d *Downloads) ArtworkFile(url string) (string, bool) { return d.a.downloads.ArtworkFile(url) }

// Online reports whether the network answers, as last checked.
func (d *Downloads) Online() bool { return d.a.online == nil || !d.a.online.Offline() }

// offline reports, without checking, whether the network was found gone.
func (a *App) offline() bool { return a.online != nil && a.online.Offline() }

// playsOffline reports whether a track plays without the network: it is
// downloaded, or its recording's audio is, or in the offline cache.
func (a *App) playsOffline(t media.Track) bool {
	if _, ok := a.downloads.Lookup(t.ID); ok {
		return true
	}
	if a.matcher == nil {
		return false
	}
	yt, ok := a.matcher.KnownMatch(t)
	if !ok {
		return false
	}
	_, video, err := media.ParseID(yt.ID)
	if err != nil {
		return false
	}
	return a.downloads.Snapshot().HasVideo(video) || a.cache != nil && a.cache.Has(video)
}

package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/store"
)

// LocalFiles is the music on the user's computer, as the windows show it:
// the folders of the settings, their index and what it holds, and files
// opened without a folder. Nothing of it goes to a service: its tracks
// play from their files, match no recording on YouTube Music, and don't
// play on cast devices, which can't reach them.
type LocalFiles struct{ a *App }

// errFileMissing is why a local track whose file is gone doesn't play.
var errFileMissing = errors.New("File not found")

// localScans keeps the scans of the index: one at a time, the folders it
// scanned last, and its end, which Close waits for; and the watch of the
// folders (localwatch.go).
type localScans struct {
	mu      sync.Mutex
	started bool
	folders []string
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	watch   *localfiles.Watcher
}

// openLocalFiles opens the index of the music on the computer, in the
// data folder, its covers in the cache.
func (a *App) openLocalFiles() error {
	x, err := localfiles.Open(filepath.Join(a.opts.DataDir, "local-files.json"), filepath.Join(a.opts.CacheDir, "local-art"), audio.Probe)
	if err != nil {
		return err
	}
	a.localFiles = x
	a.localScans.ctx, a.localScans.cancel = context.WithCancel(context.Background())
	return nil
}

// startLocalFiles scans the folders of the settings, quietly, as MixMusic
// starts, and from then on as the settings change them.
func (a *App) startLocalFiles() {
	a.localScans.mu.Lock()
	a.localScans.started = true
	a.localScans.mu.Unlock()
	a.scanLocalFiles(a.settings.Get().Clean().MusicFolders, false)
}

// followLocalFiles scans the folders the settings s name, once they differ
// from those scanned last.
func (a *App) followLocalFiles(s settings.Settings) {
	if a.localFiles == nil {
		return
	}
	a.localScans.mu.Lock()
	started, same := a.localScans.started, slices.Equal(a.localScans.folders, s.MusicFolders)
	a.localScans.mu.Unlock()
	if started && !same {
		a.scanLocalFiles(s.MusicFolders, false)
	}
}

// scanLocalFiles scans folders beside the caller; again when force is set,
// though they are those scanned last.
func (a *App) scanLocalFiles(folders []string, force bool) {
	sc := &a.localScans
	sc.mu.Lock()
	defer sc.mu.Unlock()
	a.watchLocalFiles(folders)
	if sc.ctx.Err() != nil || !force && len(folders) == 0 && len(sc.folders) == 0 && len(a.localFiles.Tracks()) == 0 {
		sc.folders = slices.Clone(folders)
		return
	}
	sc.folders = slices.Clone(folders)
	sc.wg.Add(1)
	go func() {
		defer sc.wg.Done()
		if err := a.localFiles.Scan(sc.ctx, folders); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("mixmusic: scanning the music on this computer: %v", err)
		}
	}()
}

// closeLocalFiles stops a scan and the watch of the folders, and waits for
// them, as MixMusic quits.
func (a *App) closeLocalFiles() {
	if a.localFiles == nil {
		return
	}
	a.localScans.cancel()
	a.closeLocalWatch()
	a.localScans.wg.Wait()
}

// Status returns what the index holds and does: its folders, how many
// tracks each holds and how many files it skipped, and the scan under way.
func (l *LocalFiles) Status() localfiles.Status { return l.a.localFiles.Status() }

// Version counts the changes to what the index holds, which the windows'
// views are of.
func (l *LocalFiles) Version() uint64 { return l.a.localFiles.Version() }

// Watch has fn told, from any goroutine, when what the index holds or does
// changes, until cancel.
func (l *LocalFiles) Watch(fn func()) (cancel func()) { return l.a.localFiles.Watch(fn) }

// Rescan scans the folders of the settings again, now.
func (l *LocalFiles) Rescan() { l.a.scanLocalFiles(l.a.settings.Get().Clean().MusicFolders, true) }

// Tracks, Albums and Artists return what the index holds.
func (l *LocalFiles) Tracks() []media.Track   { return l.a.localFiles.Tracks() }
func (l *LocalFiles) Albums() []media.Album   { return l.a.localFiles.Albums() }
func (l *LocalFiles) Artists() []media.Artist { return l.a.localFiles.Artists() }

// Search returns the tracks, albums and artists of the index that hold the
// words of q, up to limit of each.
func (l *LocalFiles) Search(q string, limit int) media.SearchResults {
	return l.a.localFiles.Search(q, limit)
}

// Missing reports whether the file of a local track isn't there, as when
// it moved, or its drive is unplugged: the windows show it greyed.
func (l *LocalFiles) Missing(id string) bool { return l.a.localFiles.Missing(id) }

// ArtworkFile returns the file of a cover the index keeps, by its address.
func (l *LocalFiles) ArtworkFile(url string) (string, bool) { return l.a.localFiles.ArtFile(url) }

// controlsArtwork is the cover the system's media controls show of a
// track: its largest https image, else the cover the index keeps of a
// track on this computer.
func controlsArtwork(images []media.Image) string {
	if u := largeArtwork(images); u != "" {
		return u
	}
	for _, img := range images {
		if localfiles.IsArt(img.URL) {
			return img.URL
		}
	}
	return ""
}

// coverBytes returns the cover at an address, for the system's media
// controls: one the index keeps from its file, else the artwork store's,
// which downloads it unless it has it.
func (a *App) coverBytes(ctx context.Context, rawURL string) ([]byte, error) {
	if path, ok := a.localFiles.ArtFile(rawURL); ok {
		return os.ReadFile(path)
	}
	return a.artStore.Bytes(ctx, rawURL)
}

// ChooseFolder asks for a folder of music to add to the library.
func (l *LocalFiles) ChooseFolder(parent *mygo.Window) (string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: parent, Title: "Add a folder of music",
		Directory: true, ButtonLabel: "Add"})
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

// ChooseFiles asks for music files to play.
func (l *LocalFiles) ChooseFiles(parent *mygo.Window) ([]string, error) {
	exts := []string{"mp3", "flac", "ogg", "oga", "opus", "m4a", "mp4", "wav", "wave"}
	return mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: parent, Title: "Open music files", Multiple: true,
		Filters: []mygo.FileFilter{{Name: "Music", Extensions: exts}}})
}

// maxOpened bounds the files opened at once, of folders dropped whole.
const maxOpened = 2000

// Opened is what opening files found: the tracks that play, and how many
// files didn't.
type Opened struct {
	Tracks  []media.Track
	Skipped int
}

// Open reads files the user opened or dropped, and the music files of the
// folders among them, in the order of their names: those that play, as
// tracks, and how many don't. The index keeps none of them; their tracks
// play as long as their files are there.
func (l *LocalFiles) Open(paths []string) Opened {
	var files []string
	for _, p := range paths {
		info, err := os.Stat(p)
		switch {
		case err != nil:
			continue
		case info.IsDir():
			var in []string
			_ = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
				if err != nil || len(files)+len(in) >= maxOpened {
					return nil
				}
				if d.IsDir() && path != p && strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				if !d.IsDir() && localfiles.Plays(path) {
					in = append(in, path)
				}
				return nil
			})
			slices.Sort(in)
			files = append(files, in...)
		default:
			files = append(files, p)
		}
	}
	var out Opened
	for _, f := range files[:min(len(files), maxOpened)] {
		e, err := l.a.localFiles.ReadFile(f)
		if err != nil {
			out.Skipped++
			continue
		}
		out.Tracks = append(out.Tracks, localfiles.TrackOf(e))
	}
	return out
}

// prepareFile is how a local track plays: from its file, in the engine,
// without the network. One whose file is gone doesn't, and shows as
// missing from then on.
func (a *App) prepareFile(track media.Track) (PlaybackSource, error) {
	path, ok := localfiles.PathOf(track.ID)
	if !ok {
		return PlaybackSource{}, fmt.Errorf("not a file of this computer: %s", track.ID)
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		a.localFiles.Gone(track.ID)
		return PlaybackSource{}, errFileMissing
	}
	a.localFiles.Found(track.ID)
	_, native, _ := media.ParseID(track.ID)
	return PlaybackSource{Mode: PlaybackStream, URL: schemeURL(streamScheme) + "file/" + native, Track: track, Cached: true}, nil
}

// openFile opens, for the engine, the file of a local track, by the native
// part of its ID that the address of prepareFile ends with. Its tags can
// change meanwhile, the new file taking its place, as the engine goes on
// playing it (store.OpenShared).
func (a *App) openFile(native string) (audio.Source, error) {
	path, ok := localfiles.PathOf(media.ID(media.SourceFile, native))
	if !ok {
		return nil, fmt.Errorf("not a file of this computer: %s", native)
	}
	f, err := store.OpenShared(path)
	if err != nil {
		a.localFiles.Gone(media.ID(media.SourceFile, native))
		return nil, errFileMissing
	}
	return fileSource{f}, nil
}

// fileSource is a file as the engine reads it: all of it is there, so a
// read never waits, and one that fails is the file's failure, not the
// network's.
type fileSource struct{ *os.File }

func (fileSource) Wake() {}

func (fileSource) Local() bool { return true }

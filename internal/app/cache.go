package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/downloads"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/stream"
	"github.com/fzn0x/mixmusic/locales"
)

// cacheLimit converts a CacheSizeMB setting to bytes, keeping negative values
// as stream.Unlimited.
func cacheLimit(mb int) int64 {
	if mb < 0 {
		return stream.Unlimited
	}
	return int64(mb) << 20
}

func (a *App) defaultCacheDir() string {
	return filepath.Join(a.opts.CacheDir, "audio")
}

func (a *App) resolveCacheDir(s settings.Settings) (dir, root string, mustExist bool) {
	if s.CacheWithDownloads && a.downloads != nil {
		dl := a.downloads.Folder()
		return filepath.Join(dl, "audio"), dl, s.DownloadsFolder != ""
	}
	if s.CacheFolder != "" {
		return s.CacheFolder, s.CacheFolder, true
	}
	def := a.defaultCacheDir()
	return def, def, false
}

func (a *App) applyCache(s settings.Settings) {
	dir, root, mustExist := a.resolveCacheDir(s)
	a.cache.SetDir(dir, root, mustExist)
	a.cache.SetLimit(cacheLimit(s.CacheSizeMB))
}

func (a *App) cacheProtected(dir, name string) bool {
	return a.downloads != nil &&
		sameFolder(dir, filepath.Join(a.downloads.Folder(), "audio")) &&
		a.downloads.OwnsAudioFile(name)
}

func (a *App) downloadsAvoid() []string {
	avoid := []string{
		filepath.Join(a.opts.CacheDir, "artwork"),
		filepath.Join(a.opts.CacheDir, "moving-covers"),
		a.defaultCacheDir(),
	}
	if !a.settings.Get().Clean().CacheWithDownloads {
		avoid = append(avoid, a.cache.Folder())
	}
	return avoid
}

func (a *App) cacheAvoid() []string {
	avoid := []string{
		filepath.Join(a.opts.CacheDir, "artwork"),
		filepath.Join(a.opts.CacheDir, "moving-covers"),
	}
	if a.downloads != nil {
		avoid = append(avoid, filepath.Join(a.downloads.Folder(), "art"))
	}
	return avoid
}

func sameFolder(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func insideFolder(a, b string) bool {
	rel, err := filepath.Rel(b, a)
	return err == nil && rel != "." && rel != ".." &&
		!strings.HasPrefix(rel, ".."+string(filepath.Separator)) &&
		!filepath.IsAbs(rel)
}

// MoveCacheResult describes the resulting cache location after moving or
// toggling downloads merge.
type MoveCacheResult struct {
	Folder string
	Merged bool
	Active string
}

// CacheFolder returns the active directory storing cached audio.
func (s *System) CacheFolder() string { return s.a.cache.Folder() }

// DefaultCacheFolder returns the system default directory for cached audio.
func (s *System) DefaultCacheFolder() string { return s.a.defaultCacheDir() }

// UnmergeCacheTarget returns the folder cached tracks move to when un-merging
// from Downloads.
func (s *System) UnmergeCacheTarget() string {
	target, _ := s.unmergeTarget(s.a.settings.Get().Clean(), s.a.downloads.Folder())
	return target
}

func (s *System) unmergeTarget(cur settings.Settings, dl string) (target string, mustExist bool) {
	def := s.DefaultCacheFolder()
	if cur.CacheFolder != "" &&
		!sameFolder(cur.CacheFolder, dl) &&
		!insideFolder(cur.CacheFolder, dl) &&
		!insideFolder(dl, cur.CacheFolder) {
		return cur.CacheFolder, true
	}
	return def, false
}

// CacheProblem returns a localized description if the configured cache
// folder cannot be reached, or "" otherwise.
func (s *System) CacheProblem() string {
	if s.a.settings.Get().Clean().CacheWithDownloads && s.a.downloads != nil {
		st := s.a.downloads.Snapshot().Status()
		if st.Problem != "" {
			if st.HoldKind == downloads.HoldFolder && len(st.HoldArgs) > 1 {
				return locales.DownloadsHoldUnreadable(st.HoldArgs[0], st.HoldArgs[1])
			}
			return locales.DownloadsHoldUnreachable(st.Folder)
		}
	}
	if u := s.a.cache.Unreachable(); u != "" {
		return locales.CacheHoldUnreachable(u)
	}
	return ""
}

// OpenCacheFolder opens the active audio cache directory in the file manager.
func (s *System) OpenCacheFolder() error {
	dir := s.a.cache.Folder()
	if dir == "" {
		return wordedLater("MixMusic keeps no offline cache here", locales.CacheNoFolder, nil)
	}
	cur := s.a.settings.Get().Clean()
	if (!cur.CacheWithDownloads && sameFolder(dir, s.a.defaultCacheDir())) ||
		(cur.CacheWithDownloads && cur.DownloadsFolder == "") {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return mygo.Shell.OpenPath(dir)
}

// ChooseCacheFolder opens a directory picker for the audio cache folder.
func (s *System) ChooseCacheFolder(parent *mygo.Window) (string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:            parent,
		Title:             locales.CacheFolderDialog(),
		Directory:         true,
		CreateDirectories: true,
		DefaultPath:       s.CacheFolder(),
	})
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

// MoveCache moves the audio cache to dir (or merges with downloads if dir
// names the downloads folder or its audio folder).
func (s *System) MoveCache(ctx context.Context, dir string, progress func(done, total int64)) (MoveCacheResult, error) {
	dl := s.a.downloads.Folder()
	dlAudio := filepath.Join(dl, "audio")
	if sameFolder(dir, dl) || sameFolder(dir, dlAudio) {
		return s.SetCacheMerged(ctx, true, progress)
	}
	if insideFolder(dir, dl) {
		return MoveCacheResult{}, stream.ErrCacheConflict
	}
	def := s.DefaultCacheFolder()
	exact := sameFolder(dir, def)
	avoid := append(s.a.cacheAvoid(), dl)
	got, err := s.a.cache.Move(ctx, dir, stream.MoveOptions{
		Exact:     exact,
		MustExist: !exact,
		Avoid:     avoid,
		Progress:  progress,
	})
	if err != nil {
		return MoveCacheResult{}, err
	}
	folder := got
	if sameFolder(got, def) {
		folder = ""
	}
	return MoveCacheResult{Folder: folder, Merged: false, Active: got}, nil
}

// SetCacheMerged moves the audio cache into or out of the downloads audio
// folder.
func (s *System) SetCacheMerged(ctx context.Context, merged bool, progress func(done, total int64)) (MoveCacheResult, error) {
	cur := s.a.settings.Get().Clean()
	dl := s.a.downloads.Folder()
	if merged {
		if st := s.a.downloads.Snapshot().Status(); st.Problem != "" {
			return MoveCacheResult{}, wordedLater(st.Problem, func() string {
				if st.HoldKind == downloads.HoldFolder && len(st.HoldArgs) > 1 {
					return locales.DownloadsHoldUnreadable(st.HoldArgs[0], st.HoldArgs[1])
				}
				return locales.DownloadsHoldUnreachable(st.Folder)
			}, errors.New(st.Problem))
		}
		target := filepath.Join(dl, "audio")
		mustExist := cur.DownloadsFolder != ""
		if !sameFolder(s.a.cache.Folder(), target) {
			if _, err := s.a.cache.Move(ctx, target, stream.MoveOptions{
				Exact:     true,
				RootDir:   dl,
				MustExist: mustExist,
				Avoid:     s.a.cacheAvoid(),
				Progress:  progress,
			}); err != nil {
				return MoveCacheResult{}, err
			}
		} else {
			s.a.cache.SetDir(target, dl, mustExist)
		}
		return MoveCacheResult{Folder: cur.CacheFolder, Merged: true, Active: target}, nil
	}

	target, mustExist := s.unmergeTarget(cur, dl)
	if !sameFolder(s.a.cache.Folder(), target) {
		got, err := s.a.cache.Move(ctx, target, stream.MoveOptions{
			Exact:     true,
			RootDir:   target,
			MustExist: mustExist,
			Avoid:     append(s.a.cacheAvoid(), dl),
			Progress:  progress,
		})
		if err != nil {
			return MoveCacheResult{}, err
		}
		target = got
	} else {
		s.a.cache.SetDir(target, target, mustExist)
	}
	folder := target
	if sameFolder(target, s.DefaultCacheFolder()) {
		folder = ""
	}
	return MoveCacheResult{Folder: folder, Merged: false, Active: target}, nil
}

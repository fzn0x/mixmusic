package tools

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/fzn0x/mixmusic/internal/proc"
)

// goos and goarch are the platform yt-dlp is installed for. They are
// variables for tests.
var goos, goarch = runtime.GOOS, runtime.GOARCH

// On macOS, MixMusic installs yt-dlp unpacked, from yt-dlp_macos.zip: the
// single file yt-dlp_macos unpacks Python into a new folder at each start,
// and starts in 8 s, where the unpacked build starts in 0.2 s (#245).
// macFolder is the folder it goes in, under Dir, and macProgram the
// program in it.
const (
	macFolder  = "yt-dlp_macos"
	macProgram = "yt-dlp_macos"
)

// maxUnpacked bounds the size of the files an archive unpacks to: the
// build of 2026.10 is 123 MB.
const maxUnpacked = 1 << 30

// unpackedPath is the yt-dlp MixMusic installed on macOS: the unpacked
// build's program, or the single file installed before, until an update
// replaces it.
func (m *Manager) unpackedPath() string {
	p := filepath.Join(m.Dir, macFolder, macProgram)
	if single := m.singlePath(); !isFile(p) && isFile(single) {
		return single
	}
	return p
}

// singlePath is where MixMusic put the single file of yt-dlp for macOS.
func (m *Manager) singlePath() string { return filepath.Join(m.Dir, "yt-dlp") }

// singleOnMac tells whether the yt-dlp MixMusic installed is macOS's
// single file of before.
func (m *Manager) singleOnMac() bool {
	return goos == "darwin" && m.managedPath() == m.singlePath()
}

// installUnpacked unpacks the zip archive data into Dir's macFolder, in
// place of the one there, and removes the single file of before. It then
// runs the program once: macOS checks a program's files the first time it
// runs, for 20 s, which a track would otherwise wait for.
func (m *Manager) installUnpacked(ctx context.Context, data []byte) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("reading yt-dlp's archive: %w", err)
	}
	tmp, err := os.MkdirTemp(m.Dir, ".yt-dlp_macos-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	// MkdirTemp makes it for its owner alone, as no other folder of Dir is.
	if err := os.Chmod(tmp, 0o755); err != nil {
		return err
	}
	if err := unzip(zr, tmp); err != nil {
		return fmt.Errorf("unpacking yt-dlp: %w", err)
	}
	if !isFile(filepath.Join(tmp, macProgram)) {
		return errors.New("yt-dlp's archive has no " + macProgram)
	}
	dst := filepath.Join(m.Dir, macFolder)
	old := dst + ".old"
	if err := os.RemoveAll(old); err != nil {
		return err
	}
	if err := os.Rename(dst, old); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Rename(old, dst)
		return err
	}
	_ = os.RemoveAll(old)
	if err := os.Remove(m.singlePath()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	warm(ctx, filepath.Join(dst, macProgram))
	return nil
}

// warm runs yt-dlp once, for macOS to check its files. It is a variable for
// tests.
var warm = func(ctx context.Context, path string) {
	if runtime.GOOS != "darwin" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	_ = proc.Command(ctx, path, "--version").Run()
}

// unzip writes the folders and files of zr under dir, keeping whether each
// file runs. It refuses names that would leave dir, links, and more than
// maxUnpacked bytes.
func unzip(zr *zip.Reader, dir string) error {
	var total int64
	for _, f := range zr.File {
		name := filepath.FromSlash(f.Name)
		if !filepath.IsLocal(name) {
			return fmt.Errorf("%q is outside the archive's folder", f.Name)
		}
		path := filepath.Join(dir, name)
		mode := f.Mode()
		switch {
		case mode.IsDir():
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
			continue
		case !mode.IsRegular():
			return fmt.Errorf("%q isn't a file or a folder", f.Name)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		perm := fs.FileMode(0o644)
		if mode&0o111 != 0 {
			perm = 0o755
		}
		n, err := writeFile(f, path, perm, maxUnpacked-total)
		total += n
		if err != nil {
			return err
		}
	}
	return nil
}

// writeFile writes f's content to path, at most limit bytes, and returns
// how many it wrote.
func writeFile(f *zip.File, path string, perm fs.FileMode, limit int64) (int64, error) {
	r, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer r.Close()
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(out, io.LimitReader(r, limit+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > limit {
		err = fmt.Errorf("the archive unpacks to more than %d MB", maxUnpacked>>20)
	}
	return n, err
}

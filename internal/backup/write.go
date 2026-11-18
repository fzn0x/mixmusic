package backup

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/store"
)

// Source is what a backup is made of: the data folder, and what the
// manifest tells of where it comes from.
type Source struct {
	DataDir string
	// App is the version of MixMusic that runs.
	App string
	// MusicFolders are the folders of music, and DownloadsFolder the
	// downloads', as the settings have them.
	MusicFolders    []string
	DownloadsFolder string
	// Downloads lists the downloads, for the Downloads part.
	Downloads DownloadList
	// Now is when the backup is made: time.Now when zero.
	Now time.Time
}

// file is a file a backup holds: its entry's path and part, and where it
// is, or, for the Downloads part, its bytes.
type file struct {
	name string
	part Part
	abs  string
	data []byte
	size int64
}

// collect lists the files of parts in src, sorted by their entries' paths,
// or tells why it can't. A part whose files aren't there holds none of
// them. Only regular files go: no link, nor the temporary files of a save
// under way, nor a file kept aside as broken.
func collect(src Source, parts []Part) ([]file, error) {
	var out []file
	for _, p := range Parts {
		if !slices.Contains(parts, p) {
			continue
		}
		if p == Downloads {
			data, err := json.MarshalIndent(src.Downloads, "", "  ")
			if err != nil {
				return nil, err
			}
			out = append(out, file{name: DownloadsName, part: p, data: data, size: int64(len(data))})
			continue
		}
		for _, q := range p.paths() {
			root := filepath.Join(src.DataDir, filepath.FromSlash(q))
			err := filepath.WalkDir(root, func(abs string, d fs.DirEntry, err error) error {
				switch {
				case errors.Is(err, fs.ErrNotExist) && abs == root:
					return nil
				case err != nil:
					return err
				case d.IsDir() || !d.Type().IsRegular() || skipped(d.Name()):
					return nil
				}
				rel, err := filepath.Rel(src.DataDir, abs)
				if err != nil {
					return err
				}
				info, err := d.Info()
				if err != nil {
					return err
				}
				name := filepath.ToSlash(rel)
				if !p.holds(name) {
					return nil
				}
				out = append(out, file{name: name, part: p, abs: abs, size: info.Size()})
				return nil
			})
			if err != nil {
				return nil, err
			}
		}
	}
	slices.SortFunc(out, func(a, b file) int { return strings.Compare(a.name, b.name) })
	var total int64
	for _, f := range out {
		total += f.size
	}
	if total > maxUnpacked {
		return nil, &Error{Problem: TooLargeUnpacked}
	}
	return out, nil
}

// skipped reports whether a file of name stays out of a backup: a
// temporary file of a save under way, as store writes them, or one kept
// aside as broken.
func skipped(name string) bool {
	return strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".tmp") || strings.HasSuffix(name, ".broken")
}

// open opens a file to read, sharing it with a save meanwhile.
func (f file) open() (io.ReadCloser, error) {
	if f.data != nil {
		return io.NopCloser(bytes.NewReader(f.data)), nil
	}
	return store.OpenShared(f.abs)
}

// Sum is a fingerprint of what a backup of parts in src would hold, which
// changes as soon as any of it changes: a backup made while it is the
// same as the last one's adds nothing.
func Sum(src Source, parts []Part) (string, error) {
	files, err := collect(src, parts)
	if err != nil {
		return "", err
	}
	entries := make([]Entry, 0, len(files))
	for _, f := range files {
		r, err := f.open()
		if err != nil {
			return "", err
		}
		h := sha256.New()
		n, err := io.Copy(h, r)
		r.Close()
		if err != nil {
			return "", err
		}
		entries = append(entries, Entry{Path: f.name, Part: f.part, Size: n, SHA256: hex.EncodeToString(h.Sum(nil))})
	}
	return sumOf(parts, entries), nil
}

// sumOf is the fingerprint of a backup of parts holding entries.
func sumOf(parts []Part, entries []Entry) string {
	h := sha256.New()
	for _, p := range Parts {
		if slices.Contains(parts, p) {
			fmt.Fprintf(h, "part %s\n", p)
		}
	}
	for _, e := range entries {
		fmt.Fprintf(h, "%s\x00%s\n", e.Path, e.SHA256)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Sum is the fingerprint of the backup m tells of, as Sum makes it.
func (m Manifest) Sum() string { return sumOf(m.Parts, m.Entries) }

// Write writes a backup of parts in src to the file at dst, which takes
// the place of what it held only once whole. It returns the backup's
// manifest.
func Write(dst string, src Source, parts []Part) (Manifest, error) {
	m := Manifest{Format: Format, App: src.App, Created: src.Now, System: runtime.GOOS, Arch: runtime.GOARCH,
		MusicFolders: src.MusicFolders, DownloadsFolder: src.DownloadsFolder, Parts: []Part{}, Entries: []Entry{}}
	if m.Created.IsZero() {
		m.Created = time.Now()
	}
	m.Created = m.Created.Round(time.Second)
	for _, p := range Parts {
		if slices.Contains(parts, p) {
			m.Parts = append(m.Parts, p)
		}
	}
	files, err := collect(src, parts)
	if err != nil {
		return m, err
	}
	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(dst)+".*.tmp")
	if err != nil {
		return m, err
	}
	defer os.Remove(tmp.Name()) // a no-op once renamed
	if err := writeZip(tmp, &m, files); err != nil {
		tmp.Close()
		return m, err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return m, err
	}
	info, err := tmp.Stat()
	if err != nil {
		tmp.Close()
		return m, err
	}
	if err := tmp.Close(); err != nil {
		return m, err
	}
	if info.Size() > maxPacked {
		return m, &Error{Problem: TooLarge}
	}
	return m, renameRetry(tmp.Name(), dst)
}

// writeZip writes the zip of files, and of the manifest m, which takes
// their entries, to w.
func writeZip(w io.Writer, m *Manifest, files []file) error {
	z := zip.NewWriter(w)
	for _, f := range files {
		r, err := f.open()
		if err != nil {
			return err
		}
		e, err := addEntry(z, f, r, m.Created)
		r.Close()
		if err != nil {
			return err
		}
		m.Entries = append(m.Entries, e)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	out, err := z.CreateHeader(&zip.FileHeader{Name: ManifestName, Method: zip.Deflate, Modified: m.Created})
	if err != nil {
		return err
	}
	if _, err := out.Write(append(data, '\n')); err != nil {
		return err
	}
	return z.Close()
}

// addEntry adds f, read from r, to z.
func addEntry(z *zip.Writer, f file, r io.Reader, at time.Time) (Entry, error) {
	if f.abs != "" {
		if info, err := os.Stat(f.abs); err == nil {
			at = info.ModTime()
		}
	}
	out, err := z.CreateHeader(&zip.FileHeader{Name: f.name, Method: zip.Deflate, Modified: at})
	if err != nil {
		return Entry{}, err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(out, h), r)
	if err != nil {
		return Entry{}, err
	}
	return Entry{Path: f.name, Part: f.part, Size: n, SHA256: hex.EncodeToString(h.Sum(nil))}, nil
}

// renameRetry renames from to to, trying again for a moment while another
// program, as a search indexer or an antivirus, has one of them open.
func renameRetry(from, to string) error {
	var err error
	for i := range 10 {
		if err = os.Rename(from, to); err == nil || errors.Is(err, fs.ErrNotExist) {
			return err
		}
		time.Sleep(time.Duration(i+1) * 20 * time.Millisecond)
	}
	return err
}

// FileName is the name a backup made at t takes at first, as the user
// saves it: "MixMusic backup 2026-11-18.zip".
func FileName(t time.Time) string { return "MixMusic backup " + t.Format("2006-01-02") + ".zip" }

package backup

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/fzn0x/mixmusic/internal/library"
)

// Backup is a backup opened: its manifest, brought up to date, and its
// entries, checked and unpacked in a folder of their own until Close.
type Backup struct {
	// Path is the backup's file.
	Path     string
	Manifest Manifest
	dir      string
}

// upgrades bring a backup of an older format up to date: upgrades[n]
// makes one of format n of format n+1, its manifest and the files dir
// holds. Format 1 is the first, which none brings up yet.
var upgrades = map[int]func(m *Manifest, dir string) error{}

// Open opens the backup at path, checks it, and unpacks it in a folder of
// its own in work, which Close removes. It refuses, with an *Error, a
// file that isn't a backup, or is damaged, a backup of a newer format,
// one larger than MaxPacked or that unpacks to more than MaxUnpacked, and
// one whose entry would land outside the data folder.
func Open(path, work string) (*Backup, error) { return openAs(path, work, Format) }

// openAs is Open, as a MixMusic of the format current would open it.
func openAs(path, work string, current int) (*Backup, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxPacked {
		return nil, &Error{Problem: TooLarge}
	}
	z, err := zip.OpenReader(path)
	if err != nil {
		if errors.Is(err, zip.ErrFormat) && !startsAsZip(path) {
			return nil, &Error{Problem: NotBackup}
		}
		return nil, &Error{Problem: Damaged, Err: err}
	}
	defer z.Close()
	m, err := readManifest(&z.Reader, current)
	if err != nil {
		return nil, err
	}
	byName, err := check(&z.Reader, m)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(work, "open-")
	if err != nil {
		return nil, err
	}
	b := &Backup{Path: path, Manifest: m, dir: dir}
	for _, e := range m.Entries {
		if err := unpack(byName[e.Path], e, dir); err != nil {
			b.Close()
			return nil, err
		}
	}
	for n := m.Format; n < current; n++ {
		up := upgrades[n]
		if up == nil {
			b.Close()
			return nil, &Error{Problem: Damaged}
		}
		if err := up(&b.Manifest, dir); err != nil {
			b.Close()
			return nil, &Error{Problem: Damaged, Err: err}
		}
		b.Manifest.Format = n + 1
	}
	return b, nil
}

// startsAsZip reports whether the file at path starts as a zip does.
func startsAsZip(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, 4)
	_, err = io.ReadFull(f, head)
	return err == nil && bytes.Equal(head, []byte("PK\x03\x04"))
}

// readManifest reads the manifest of z, and checks that MixMusic reads its
// format.
func readManifest(z *zip.Reader, current int) (Manifest, error) {
	var m Manifest
	var mf *zip.File
	for _, f := range z.File {
		if f.Name == ManifestName {
			mf = f
		}
	}
	if mf == nil {
		return m, &Error{Problem: NotBackup}
	}
	r, err := mf.Open()
	if err != nil {
		return m, &Error{Problem: Damaged, Entry: ManifestName, Err: err}
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(r, 16<<20))
	if err != nil {
		return m, &Error{Problem: Damaged, Entry: ManifestName, Err: err}
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, &Error{Problem: Damaged, Entry: ManifestName, Err: err}
	}
	switch {
	case m.Format > current:
		return m, &Error{Problem: Newer, App: m.App}
	case m.Format < 1:
		return m, &Error{Problem: NotBackup}
	}
	return m, nil
}

// check checks the entries of z against the manifest m: each lands inside
// the data folder, in a part that holds it, once, at the size m tells,
// and they unpack to no more than MaxUnpacked. It returns the files of z
// by their entries' paths.
func check(z *zip.Reader, m Manifest) (map[string]*zip.File, error) {
	listed := map[string]Entry{}
	for _, e := range m.Entries {
		if !local(e.Path) {
			return nil, &Error{Problem: Outside, Entry: e.Path}
		}
		if _, twice := listed[e.Path]; twice || !e.Part.known() || !e.Part.holds(e.Path) || !m.Has(e.Part) || e.Size < 0 {
			return nil, &Error{Problem: Damaged, Entry: e.Path}
		}
		listed[e.Path] = e
	}
	byName := map[string]*zip.File{}
	var total int64
	for _, f := range z.File {
		if f.Name == ManifestName || strings.HasSuffix(f.Name, "/") && f.UncompressedSize64 == 0 {
			continue
		}
		if !local(f.Name) {
			return nil, &Error{Problem: Outside, Entry: f.Name}
		}
		e, ok := listed[f.Name]
		if _, twice := byName[f.Name]; twice || !ok || f.UncompressedSize64 != uint64(e.Size) {
			return nil, &Error{Problem: Damaged, Entry: f.Name}
		}
		byName[f.Name] = f
		total += e.Size
		if total > maxUnpacked {
			return nil, &Error{Problem: TooLargeUnpacked}
		}
	}
	for _, e := range m.Entries {
		if byName[e.Path] == nil {
			return nil, &Error{Problem: Damaged, Entry: e.Path}
		}
	}
	return byName, nil
}

// local reports whether name, an entry's path, lands inside the folder it
// unpacks to, as a plain file: relative, with forward slashes, without
// "..", and no name Windows keeps for a device.
func local(name string) bool {
	return name != "" && !strings.ContainsAny(name, "\\:\x00") && filepath.IsLocal(filepath.FromSlash(name)) &&
		filepath.ToSlash(filepath.Clean(filepath.FromSlash(name))) == name && !reserved(name)
}

// reserved reports whether a part of name is one Windows keeps for a
// device, as CON or NUL, whatever system unpacks it: a backup made on
// Linux may be restored on Windows.
func reserved(name string) bool {
	for _, part := range strings.Split(name, "/") {
		base := strings.ToUpper(strings.TrimRight(part, ". "))
		base, _, _ = strings.Cut(base, ".")
		switch base {
		case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
			return true
		}
		if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9' {
			return true
		}
	}
	return false
}

// unpack writes the entry e, of the file f, under dir, checking its size
// and its SHA-256 as it reads it.
func unpack(f *zip.File, e Entry, dir string) error {
	r, err := f.Open()
	if err != nil {
		return &Error{Problem: Damaged, Entry: e.Path, Err: err}
	}
	defer r.Close()
	dst := filepath.Join(dir, filepath.FromSlash(e.Path))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(out, h), io.LimitReader(r, e.Size+1))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	switch {
	case errors.Is(err, zip.ErrChecksum) || errors.Is(err, zip.ErrFormat) || errors.Is(err, io.ErrUnexpectedEOF):
		return &Error{Problem: Damaged, Entry: e.Path, Err: err}
	case err != nil:
		return err
	case n != e.Size || !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), e.SHA256):
		return &Error{Problem: Damaged, Entry: e.Path}
	}
	return nil
}

// Close removes what the backup unpacked.
func (b *Backup) Close() error {
	if b == nil || b.dir == "" {
		return nil
	}
	err := os.RemoveAll(b.dir)
	b.dir = ""
	return err
}

// file returns the path of what the backup unpacked of an entry's path.
func (b *Backup) file(name string) string { return filepath.Join(b.dir, filepath.FromSlash(name)) }

// Counts are what a backup holds, as its preview tells it, or what a
// merge adds of it to the library here.
type Counts struct {
	Liked, Disliked, Mixes, Plays, Artists, Podcasts int
	Addons, Queue, Downloads                         int
}

// Counts counts what the backup holds.
func (b *Backup) Counts() Counts {
	var c Counts
	if d, ok := b.library(); ok {
		c.Liked, c.Disliked, c.Mixes = len(d.Liked), len(d.Disliked), len(d.Playlists)
		c.Artists, c.Podcasts = len(d.Artists), len(d.Podcasts)
		c.Plays = len(d.History)
		if b.has("plays.jsonl") {
			if n, err := library.CountPlays(b.file("plays.jsonl")); err == nil {
				c.Plays = n
			}
		}
	}
	addons := map[string]bool{}
	for _, e := range b.Manifest.Entries {
		parts := strings.Split(e.Path, "/")
		if e.Part == Addons && len(parts) >= 4 && (parts[1] == "themes" || parts[1] == "plugins") {
			addons[parts[1]+"/"+parts[2]] = true
		}
	}
	c.Addons = len(addons)
	if b.has("queue.json") {
		var q struct {
			Tracks []json.RawMessage `json:"tracks"`
		}
		if readJSON(b.file("queue.json"), &q) == nil {
			c.Queue = len(q.Tracks)
		}
	}
	if l, ok := b.Downloads(); ok {
		c.Downloads = l.Songs()
	}
	return c
}

// has reports whether the backup holds the entry of name.
func (b *Backup) has(name string) bool {
	return slices.ContainsFunc(b.Manifest.Entries, func(e Entry) bool { return e.Path == name })
}

// library reads the library the backup holds.
func (b *Backup) library() (library.Data, bool) {
	var d library.Data
	if !b.has("library.json") {
		return d, false
	}
	return d, readJSON(b.file("library.json"), &d) == nil
}

// Downloads returns the list of the downloads the backup holds.
func (b *Backup) Downloads() (DownloadList, bool) {
	var l DownloadList
	if !b.has(DownloadsName) {
		return l, false
	}
	return l, readJSON(b.file(DownloadsName), &l) == nil
}

// readJSON decodes the JSON file at path into v.
func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

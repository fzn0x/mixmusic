package downloads

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// rename renames a file; tests have it fail, as across drives.
var rename = os.Rename

// moveFolder is the folder Move makes for the downloads in a folder that
// holds other files.
const moveFolder = "MixMusic downloads"

// Move moves the downloads to the folder dir, telling progress how many of
// their bytes moved, and returns the folder they are in then: dir itself
// when it's empty, or holds MixMusic's downloads, as on a drive used
// before, whose downloads the moved ones join; else a folder in it, so as
// to keep apart from its files, unless exact. The downloads wait
// meanwhile, and go on from their parts there. A move that fails leaves
// them where they were.
func (m *Manager) Move(ctx context.Context, dir string, exact bool, progress func(done, total int64)) (string, error) {
	if m == nil {
		return "", errors.New("MixMusic keeps no downloads here")
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	target := dir
	if !exact && !emptyOrOurs(dir) {
		target = filepath.Join(dir, moveFolder)
	}
	m.mu.Lock()
	cur := m.dir
	switch {
	case m.holds[holdMoving].why != "":
		m.mu.Unlock()
		return "", errors.New("the downloads are moving already")
	case same(target, cur):
		m.mu.Unlock()
		return "", errors.New("the downloads are in that folder already")
	case inside(target, cur):
		m.mu.Unlock()
		return "", errors.New("the downloads can't move into a folder of their own")
	}
	for _, avoid := range m.opts.Avoid {
		if same(target, avoid) || inside(target, avoid) || inside(avoid, target) {
			m.mu.Unlock()
			return "", errors.New("the downloads can't share a folder with MixMusic's cache")
		}
	}
	m.setHold(holdMoving, true, "Moving to "+target, time.Time{})
	m.publish()
	m.mu.Unlock()
	release := func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.setHold(holdMoving, false, "", time.Time{})
		m.publish()
	}
	if err := m.idle(ctx); err != nil {
		release()
		return "", err
	}
	if err := writable(target); err != nil {
		release()
		return "", fmt.Errorf("MixMusic can't write to %s: %w", target, err)
	}
	theirs, _, err := readIndex(target)
	if err != nil {
		release()
		return "", err
	}
	mv, err := moveFiles(ctx, cur, target, progress)
	if err != nil {
		mv.undo()
		release()
		return "", err
	}
	m.mu.Lock()
	merged := merge(theirs, m.idx)
	if err := merged.write(target); err != nil {
		m.mu.Unlock()
		mv.undo()
		release()
		return "", err
	}
	m.dir, m.idx, m.err = target, merged, nil
	delete(m.holds, holdFolder)
	m.dirty = false
	m.setHold(holdMoving, false, "", time.Time{})
	m.publish()
	m.mu.Unlock()
	// What stays of the old folder goes: the files copied rather than
	// moved, those the new folder held already, and its index.
	for _, p := range mv.leave {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			log.Printf("mixmusic: removing a download moved: %v", err)
		}
	}
	os.Remove(filepath.Join(cur, "index.json"))
	for _, sub := range []string{"audio", "art", ""} {
		os.Remove(filepath.Join(cur, sub)) // only once empty
	}
	return target, nil
}

// idle waits until no download is under way: the moving hold stops them.
func (m *Manager) idle(ctx context.Context) error {
	for {
		m.mu.Lock()
		n := len(m.jobs)
		m.mu.Unlock()
		if n == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// emptyOrOurs reports whether dir is empty, doesn't exist yet, or holds
// MixMusic's downloads.
func emptyOrOurs(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return errors.Is(err, fs.ErrNotExist)
	}
	if len(entries) == 0 {
		return true
	}
	_, err = os.Stat(filepath.Join(dir, "index.json"))
	return err == nil
}

// writable makes dir, and reports whether a file can be written in it.
func writable(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".mixmusic-*.tmp")
	if err != nil {
		return err
	}
	f.Close()
	return os.Remove(f.Name())
}

// same reports whether a and b are the same folder.
func same(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if strings.EqualFold(a, b) && (os.PathSeparator == '\\' || a == b) {
		return true
	}
	ia, err1 := os.Stat(a)
	ib, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(ia, ib)
}

// inside reports whether a is a folder inside b.
func inside(a, b string) bool {
	rel, err := filepath.Rel(b, a)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// moved is what moveFiles did: the files it renamed, from and to, and
// those it copied, to, to take back; and the files to delete once the
// move is done.
type moved struct {
	renamed [][2]string
	copied  []string
	leave   []string
}

// undo takes back a move that failed.
func (mv moved) undo() {
	for _, r := range slices.Backward(mv.renamed) {
		if err := os.Rename(r[1], r[0]); err != nil {
			log.Printf("mixmusic: putting a download back: %v", err)
		}
	}
	for _, p := range mv.copied {
		os.Remove(p)
	}
}

// moveFiles moves the audio and the covers of the folder from to the
// folder to: it renames each where it can, and copies it where it can't,
// as to another drive. Files to holds already stay, the moved ones' copies
// left to delete.
func moveFiles(ctx context.Context, from, to string, progress func(done, total int64)) (moved, error) {
	var mv moved
	type item struct {
		src, dst string
		size     int64
	}
	var items []item
	var total int64
	for _, sub := range []string{"audio", "art"} {
		entries, err := os.ReadDir(filepath.Join(from, sub))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return mv, err
		}
		for _, e := range entries {
			info, err := e.Info()
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			items = append(items, item{filepath.Join(from, sub, e.Name()), filepath.Join(to, sub, e.Name()), info.Size()})
			total += info.Size()
		}
	}
	var done int64
	tell := func() {
		if progress != nil {
			progress(done, total)
		}
	}
	tell()
	for _, it := range items {
		if err := ctx.Err(); err != nil {
			return mv, err
		}
		if err := os.MkdirAll(filepath.Dir(it.dst), 0o755); err != nil {
			return mv, err
		}
		if _, err := os.Stat(it.dst); err == nil {
			mv.leave = append(mv.leave, it.src) // the folder has it already
		} else if err := rename(it.src, it.dst); err == nil {
			mv.renamed = append(mv.renamed, [2]string{it.src, it.dst})
		} else if err := copyTo(ctx, it.src, it.dst, func(n int64) { done += n; tell() }); err != nil {
			return mv, err
		} else {
			mv.copied = append(mv.copied, it.dst)
			mv.leave = append(mv.leave, it.src)
			continue
		}
		done += it.size
		tell()
	}
	return mv, nil
}

// copyTo copies src to dst, through a file beside dst that takes its name
// once whole, telling wrote of the bytes as they go.
func copyTo(ctx context.Context, src, dst string, wrote func(n int64)) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(out.Name()) // a no-op once renamed
	buf := make([]byte, 1<<20)
	for {
		if err := ctx.Err(); err != nil {
			out.Close()
			return err
		}
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				out.Close()
				return err
			}
			wrote(int64(n))
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			out.Close()
			return rerr
		}
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(out.Name(), dst)
}

// merge returns the index of the downloads of ours joining theirs, those a
// folder held already: theirs keep their places, and ours come after.
func merge(theirs, ours *index) *index {
	x := theirs
	if x == nil {
		x = newIndex()
	}
	x.Paused = ours.Paused
	order := make([]string, 0, len(ours.Tracks))
	for id := range ours.Tracks {
		order = append(order, id)
	}
	slices.SortFunc(order, func(a, b string) int { return cmp.Compare(ours.Tracks[a].Seq, ours.Tracks[b].Seq) })
	for _, id := range order {
		e := ours.Tracks[id]
		if have, ok := x.Tracks[id]; ok {
			for _, p := range e.Pins {
				if !have.pinned(p) {
					have.Pins = append(have.Pins, p)
				}
			}
			if have.Art == "" {
				have.Art = e.Art
			}
			continue
		}
		x.Seq++
		c := *e
		c.Seq = x.Seq
		x.Tracks[id] = &c
	}
	for video, f := range ours.Files {
		if _, ok := x.Files[video]; !ok {
			x.Files[video] = f
		}
	}
	for video, v := range ours.Parts {
		if _, ok := x.Parts[video]; !ok && x.Files[video] == nil {
			x.Parts[video] = v
		}
	}
	for id, c := range ours.Collections {
		have, ok := x.Collections[id]
		if !ok {
			x.Collections[id] = c
			continue
		}
		for _, t := range c.Tracks {
			if !slices.Contains(have.Tracks, t) {
				have.Tracks = append(have.Tracks, t)
			}
		}
	}
	for u, p := range ours.Art {
		if _, ok := x.Art[u]; !ok {
			x.Art[u] = p
		}
	}
	// Ours's pending deletes are of the old folder's files, which the move
	// takes care of.
	return x
}

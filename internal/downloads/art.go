package downloads

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"path/filepath"
	"slices"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/store"
)

const (
	// artSize is the width of the cover a download keeps, at least: the
	// largest the window shows, as the player's sleeve, takes 768 pixels.
	artSize = 768
	// artLimit bounds a cover's bytes, and artTimeout its download.
	artLimit   = 8 << 20
	artTimeout = 20 * time.Second
)

// keepArtSoon keeps the cover of the track id, and of the collections that
// hold it, beside the worker. Its caller holds mu.
func (m *Manager) keepArtSoon(id string) {
	if m.opts.Art == nil {
		return
	}
	e := m.idx.Tracks[id]
	if e == nil {
		return
	}
	type want struct {
		images []media.Image
		set    func(p string)
	}
	var wants []want
	if e.Art == "" && len(e.Track.Artwork) > 0 {
		wants = append(wants, want{e.Track.Artwork, func(p string) {
			if e := m.idx.Tracks[id]; e != nil {
				e.Art = p
			}
		}})
	}
	for _, pin := range e.Pins {
		if c := m.idx.Collections[pin]; c != nil && c.Art == "" && len(c.Artwork) > 0 {
			cid := c.ID
			wants = append(wants, want{c.Artwork, func(p string) {
				if c := m.idx.Collections[cid]; c != nil {
					c.Art = p
				}
			}})
		}
	}
	if len(wants) == 0 {
		return
	}
	dir := m.dir
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for _, w := range wants {
			p, err := m.fetchArt(dir, w.images)
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					log.Printf("mixmusic: keeping a cover of the downloads: %v", err)
				}
				continue
			}
			m.mu.Lock()
			if m.dir == dir {
				for _, img := range w.images {
					m.idx.Art[img.URL] = p
				}
				w.set(p)
				m.changed()
			}
			m.mu.Unlock()
		}
	}()
}

// fetchArt keeps the cover of images, of artSize or more where it has one,
// in the folder dir, as it came, and returns its path in the folder.
func (m *Manager) fetchArt(dir string, images []media.Image) (string, error) {
	url := pickArt(images)
	if url == "" {
		return "", errors.New("no cover")
	}
	m.mu.Lock()
	p, ok := m.idx.Art[url]
	m.mu.Unlock()
	if ok {
		return p, nil
	}
	ctx, cancel := context.WithTimeout(m.ctx, artTimeout)
	defer cancel()
	data, err := m.opts.Art(ctx, url)
	if err != nil {
		return "", err
	}
	if len(data) > artLimit {
		return "", errors.New("a cover too large to keep")
	}
	ext := ""
	switch http.DetectContentType(data) {
	case "image/jpeg":
		ext = ".jpg"
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	case "image/gif":
		ext = ".gif"
	default:
		return "", errors.New("a cover that isn't an image")
	}
	sum := sha256.Sum256([]byte(url))
	p = "art/" + hex.EncodeToString(sum[:8]) + ext
	if err := store.WriteFile(filepath.Join(dir, filepath.FromSlash(p)), data); err != nil {
		return "", err
	}
	return p, nil
}

// pickArt picks, of images, the smallest of artSize or more across, else
// the largest.
func pickArt(images []media.Image) string {
	if len(images) == 0 {
		return ""
	}
	sorted := slices.Clone(images)
	slices.SortStableFunc(sorted, func(a, b media.Image) int { return a.Width - b.Width })
	for _, img := range sorted {
		if img.Width >= artSize && img.URL != "" {
			return img.URL
		}
	}
	return sorted[len(sorted)-1].URL
}

// ArtworkFile returns the file that keeps the cover at url, if a download
// keeps it.
func (m *Manager) ArtworkFile(url string) (string, bool) { return m.Snapshot().ArtworkFile(url) }

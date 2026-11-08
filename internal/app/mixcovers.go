package app

// The covers the user chooses for mixes: an image file of theirs, or the
// cover of one of the mix's tracks, kept in the data folder's mix-covers,
// its centre square at most mixCoverSize pixels across, as a JPEG, or a
// PNG where it isn't opaque.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"

	"github.com/fzn0x/mixmusic/internal/artwork"
	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
)

const (
	// mixCoverSize is how many pixels across a mix's cover is kept at most.
	mixCoverSize = 600
	// maxCoverBytes and maxCoverPixels bound the images a cover is made
	// of: one larger isn't read.
	maxCoverBytes  = 40 << 20
	maxCoverPixels = 80_000_000
)

// errNotCover is an image file MixMusic doesn't make covers of.
var errNotCover = errors.New("a cover is a PNG, JPEG or WebP image")

// mixCovers is the folder of the covers chosen for mixes.
func (a *App) mixCovers() string { return filepath.Join(a.opts.DataDir, "mix-covers") }

// SetMixCover gives a mix the picture of an image file, a PNG, a JPEG or
// a WebP, in place of its tracks' covers.
func (c *Collection) SetMixCover(id, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("couldn't read the image: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxCoverBytes+1))
	switch {
	case err != nil:
		return fmt.Errorf("couldn't read the image: %w", err)
	case len(data) > maxCoverBytes:
		return errors.New("the image is too large for a cover: choose one under 40 MB")
	}
	return c.a.keepMixCover(id, data)
}

// SetMixCoverOf gives a mix the cover of one of its tracks.
func (c *Collection) SetMixCoverOf(ctx context.Context, id string, track media.Track) error {
	url := trackCover(track.Artwork)
	if url == "" {
		return errors.New(track.Title + " has no cover")
	}
	data, err := c.a.imageBytes(ctx, url)
	if err != nil {
		return fmt.Errorf("couldn't get the cover of %s: %w", track.Title, err)
	}
	return c.a.keepMixCover(id, data)
}

// UseSongsCovers gives a mix its tracks' covers again, and removes the
// file of the cover it had.
func (c *Collection) UseSongsCovers(id string) error {
	native, err := localID(id)
	if err != nil {
		return err
	}
	old, err := c.a.library.SetPlaylistCover(native, "")
	if err != nil {
		return err
	}
	c.a.removeMixCover(old)
	return nil
}

// MixCoverFile returns the file of the cover chosen for a mix, by its
// address in the mix's artwork.
func (c *Collection) MixCoverFile(url string) (string, bool) {
	name, ok := library.CoverName(url)
	if !ok {
		return "", false
	}
	return filepath.Join(c.a.mixCovers(), name), true
}

// keepMixCover makes a mix's cover of an encoded image: it keeps its
// centre square in a file of a name of its own, which the windows show at
// once rather than an older image of the same address, and removes the
// file of the cover the mix had.
func (a *App) keepMixCover(id string, data []byte) error {
	native, err := localID(id)
	if err != nil {
		return err
	}
	if _, err := a.library.PlaylistCover(native); err != nil {
		return err
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	switch {
	case err != nil || format != "png" && format != "jpeg" && format != "webp":
		return errNotCover
	case cfg.Width*cfg.Height > maxCoverPixels:
		return errors.New("the image is too large for a cover")
	}
	square, err := artwork.Square(data, mixCoverSize)
	if err != nil {
		return errNotCover
	}
	ext := ".jpg"
	if _, format, _ := image.DecodeConfig(bytes.NewReader(square)); format == "png" {
		ext = ".png"
	}
	b := make([]byte, 6)
	rand.Read(b)
	name := native + "-" + hex.EncodeToString(b) + ext
	if err := os.MkdirAll(a.mixCovers(), 0o755); err != nil {
		return err
	}
	path := filepath.Join(a.mixCovers(), name)
	if err := os.WriteFile(path, square, 0o644); err != nil {
		return err
	}
	old, err := a.library.SetPlaylistCover(native, name)
	if err != nil {
		os.Remove(path)
		return err
	}
	a.removeMixCover(old)
	return nil
}

// removeMixCover removes the file of a cover no mix shows any more.
func (a *App) removeMixCover(name string) {
	if name == "" {
		return
	}
	if path, ok := (&Collection{a}).MixCoverFile(library.CoverImage(name).URL); ok {
		os.Remove(path)
	}
}

// trackCover is the address of a track's cover to make a mix's of: its
// largest of the web, else the first it has, as a cover of a file on the
// computer.
func trackCover(images []media.Image) string {
	if u := largeArtwork(images); u != "" {
		return u
	}
	for _, img := range images {
		if img.URL != "" {
			return img.URL
		}
	}
	return ""
}

// imageBytes returns the image at an address, as the windows show it: a
// cover the index of the music on the computer or the downloads keep, one
// chosen for a mix, or the artwork store's, which downloads it unless it
// has it.
func (a *App) imageBytes(ctx context.Context, url string) ([]byte, error) {
	if path, ok := a.localFiles.ArtFile(url); ok {
		return os.ReadFile(path)
	}
	if path, ok := a.downloads.ArtworkFile(url); ok {
		return os.ReadFile(path)
	}
	if path, ok := (&Collection{a}).MixCoverFile(url); ok {
		return os.ReadFile(path)
	}
	return a.artStore.Bytes(ctx, url)
}

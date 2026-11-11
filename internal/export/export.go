// Package export writes copies of tracks to a folder the user chose, to
// take them to a phone or another player: named "Artist - Title", with
// their tags and their cover in each file. The files they're copied from
// stay as they are.
//
// A download's audio keeps its format: an M4A file gets its tags as
// internal/tagwrite writes them, and Opus in WebM, which carries no tags
// other players read, becomes Ogg Opus, packet for packet
// (tagwrite.RemuxWebM). A file on this computer goes as it is, with its
// own tags and cover, as internal/tagwrite writes them when the user
// edits them.
package export

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // the covers' kinds, which turn to JPEG
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/webp"

	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/tagwrite"
)

// Track is a track to copy: its file, and what the copy is to tell of
// itself.
type Track struct {
	// Path is the file the copy is made of.
	Path string
	// Artists and Title name the copy.
	Artists []string
	Title   string
	// Tags are the copy's tags, Tags.Cover its cover, which Cover makes of
	// an image of any kind. They're written whole: an empty field leaves
	// the copy without it.
	Tags tagwrite.Tags
	// AsIs copies the file as it is, its tags too: a file on this
	// computer, whose tags are the user's.
	AsIs bool
}

// Write writes a copy of t to dir, named as Name names it, and numbered
// when a file of that name is there, and returns its path. It writes the
// copy beside, under another name, and gives it its own once it's whole:
// a failure leaves nothing behind.
func Write(dir string, t Track) (string, error) {
	src, err := os.Open(t.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", errors.New("its file isn't there any more")
		}
		return "", err
	}
	defer src.Close()
	ext, write, err := how(src, t)
	if err != nil {
		return "", err
	}
	path, err := claim(dir, Name(t.Artists, t.Title), ext)
	if err != nil {
		return "", err
	}
	err = store.Replace(path, 0o644, func(out *os.File) error {
		if _, err := src.Seek(0, io.SeekStart); err != nil {
			return err
		}
		return write(out, src)
	})
	if err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

// how tells how t's file src is copied: the extension of the copy, and
// what writes it.
func how(src io.ReadSeeker, t Track) (string, func(w io.Writer, src io.ReadSeeker) error, error) {
	asIs := func(w io.Writer, src io.ReadSeeker) error {
		_, err := io.Copy(w, src)
		return err
	}
	own := strings.ToLower(filepath.Ext(t.Path))
	if t.AsIs {
		return own, asIs, nil
	}
	if tagwrite.IsWebM(src) {
		return ".opus", func(w io.Writer, src io.ReadSeeker) error {
			return tagwrite.RemuxWebM(w, src, t.Tags, tagwrite.All)
		}, nil
	}
	kind, err := tagwrite.KindOf(src)
	switch {
	case errors.Is(err, tagwrite.ErrUnsupported):
		return own, asIs, nil
	case err != nil:
		return "", nil, err
	}
	ext := map[tagwrite.Kind]string{tagwrite.MP3: ".mp3", tagwrite.FLAC: ".flac", tagwrite.Opus: ".opus",
		tagwrite.Vorbis: ".ogg", tagwrite.MP4: ".m4a"}[kind]
	if kind == tagwrite.MP4 && (own == ".mp4" || own == ".m4b") {
		ext = own
	}
	return ext, func(w io.Writer, src io.ReadSeeker) error {
		return tagwrite.Rewrite(w, src, t.Tags, tagwrite.All)
	}, nil
}

// Cover makes the cover of a copy of an image: as it is when it's a JPEG
// or PNG image that tagwrite takes, else, as the downloads keep WebP
// covers, as a JPEG image of it. An image it can't read is an error.
func Cover(data []byte) (*tagwrite.Picture, error) {
	if mime, err := tagwrite.CoverType(data); err == nil {
		return &tagwrite.Picture{MIME: mime, Data: data}, nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("a cover MixMusic can't read: %w", err)
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}
	if b.Len() > tagwrite.MaxCover {
		return nil, tagwrite.ErrCover
	}
	return &tagwrite.Picture{MIME: "image/jpeg", Data: b.Bytes()}, nil
}

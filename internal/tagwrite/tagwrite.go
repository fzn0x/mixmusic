// Package tagwrite writes the tags of music files kept on a computer, as
// internal/tags reads them: the title, artists, album, album artists,
// track and disc numbers, year, genre, ISRC and cover. It writes ID3v2 in
// MP3 files, Vorbis comments and PICTURE blocks in FLAC files, the
// comment header of Ogg Opus and Ogg Vorbis files, and the items of an
// MP4 file's ilst, as M4A files hold them. WAV files it leaves as they
// are. RemuxWebM writes the Opus of a WebM file, whose tags other players
// don't read, as an Ogg Opus file with tags, for an export.
//
// Only the tags change. The audio is copied byte for byte, as are the
// tags a file holds that the write doesn't name, and the new tags come
// first, so that a reader taking the first of each finds them. Rewrite
// writes the file anew to a writer, as an export would; Write changes a
// file on the disk safely: it writes the new file beside the old one,
// checks it, puts it on the disk, and only then renames it over the old,
// so that a failure leaves the file as it was.
package tagwrite

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/tags"
)

// Tags are what a file is to tell of itself. An empty field, or 0,
// written takes the tag away.
type Tags struct {
	Title   string
	Artists []string
	Album   string
	// AlbumArtists are the artists of the album, as a compilation's
	// differ from its tracks'.
	AlbumArtists []string
	// Track and Disc are the track's numbers on its album, from 1. Of a
	// file that tells how many there are, as "3/12", the count stays.
	Track, Disc int
	Year        int
	Genre       string
	// ISRC is the recording's International Standard Recording Code.
	ISRC string
	// Cover is the front cover: written, it takes the place of every
	// picture the file holds.
	Cover *Picture
}

// Picture is an image a file holds.
type Picture struct {
	// MIME is its type: image/jpeg or image/png.
	MIME string
	Data []byte
}

// Fields name the fields of Tags a write changes: the others stay as the
// file holds them.
type Fields uint16

const (
	Title Fields = 1 << iota
	Artists
	Album
	AlbumArtists
	Track
	Disc
	Year
	Genre
	ISRC
	Cover
)

// All are every field.
const All = Title | Artists | Album | AlbumArtists | Track | Disc | Year | Genre | ISRC | Cover

// Why a file's tags can't be written, in the words the windows tell them
// in, after "Couldn't save the tags of <file>: ".
var (
	// ErrUnsupported is returned for a file of a kind whose tags MixMusic
	// doesn't write, as WAV.
	ErrUnsupported = errors.New("MixMusic doesn't change the tags of this kind of file")
	// ErrFormat is wrapped by the errors of a file whose layout MixMusic
	// can't change safely, as one that is damaged.
	ErrFormat = errors.New("its layout isn't one MixMusic can change safely")
	// ErrReadOnly is returned for a file that is read-only, and
	// ErrFolderReadOnly for one whose folder MixMusic can't write in,
	// which a safe write writes the new file in.
	ErrReadOnly       = errors.New("the file is read-only")
	ErrFolderReadOnly = errors.New("MixMusic can't write in its folder")
	// ErrCover is returned for a cover that isn't a JPEG or PNG image, or
	// is larger than MaxCover.
	ErrCover = errors.New("the cover must be a JPEG or PNG image of at most 10 MB")
)

// MaxCover bounds the bytes of a cover written.
const MaxCover = 10 << 20

// maxRead bounds the bytes of a file's tags read whole: an ID3v2 tag,
// FLAC's metadata, Ogg's header packets, an MP4 file's moov. Covers of 16
// MB, as internal/tags reads, fit, as do the sample tables of hours of
// audio.
const maxRead = 64 << 20

// Kind is a kind of file whose tags are written.
type Kind string

const (
	MP3    Kind = "MP3"
	FLAC   Kind = "FLAC"
	Opus   Kind = "Ogg Opus"
	Vorbis Kind = "Ogg Vorbis"
	MP4    Kind = "MP4"
)

// KindOf tells the kind of file r holds, by its first bytes: ErrUnsupported
// for one whose tags aren't written, as WAV.
func KindOf(r io.ReadSeeker) (Kind, error) {
	f, err := newFile(r)
	if err != nil {
		return "", err
	}
	return f.kind()
}

// Rewrite writes to w the file src holds, with the fields of t that fields
// names as its tags. What else src holds is copied as it is: its audio,
// and its other tags. A cover must be a JPEG or PNG image of at most
// MaxCover bytes.
func Rewrite(w io.Writer, src io.ReadSeeker, t Tags, fields Fields) error {
	if fields&Cover != 0 && t.Cover != nil {
		mime, err := checkCover(t.Cover)
		if err != nil {
			return err
		}
		t.Cover = &Picture{MIME: mime, Data: t.Cover.Data}
	}
	t = t.clean()
	if err := t.check(); err != nil {
		return err
	}
	f, err := newFile(src)
	if err != nil {
		return err
	}
	kind, err := f.kind()
	if err != nil {
		return err
	}
	bw := bufio.NewWriterSize(w, 256<<10)
	switch kind {
	case MP3:
		err = f.writeMP3(bw, t, fields)
	case FLAC:
		err = f.writeFLAC(bw, t, fields)
	case Opus, Vorbis:
		err = f.writeOgg(bw, t, fields)
	case MP4:
		err = f.writeMP4(bw, t, fields)
	}
	if err != nil {
		return err
	}
	return bw.Flush()
}

// Options tell how Write writes.
type Options struct {
	// Check reads the new file before it takes the old one's place, as
	// the audio engine would play it: an error keeps the old file.
	Check func(r io.ReadSeeker) error
}

// Write changes the tags of the file at path: the fields of t that fields
// names. It writes the new file beside the old one, reads its tags back,
// has o.Check read it, puts it on the disk and renames it over the old,
// so that a failure, as of a full disk, leaves the file as it was. The
// new file keeps the old one's permissions.
func Write(path string, t Tags, fields Fields, o Options) error {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real // a link stays a link to the file changed
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if err := Writable(path); err != nil {
		return err
	}
	err = store.Replace(path, info.Mode().Perm(), func(out *os.File) error {
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		if err := Rewrite(out, src, t, fields); err != nil {
			return err
		}
		return check(out, t, fields, o)
	})
	return describe(err)
}

// check reads the tags of the file written back, and has o.Check read it.
func check(out *os.File, t Tags, fields Fields, o Options) error {
	if _, err := out.Seek(0, io.SeekStart); err != nil {
		return err
	}
	got, err := tags.Read(out)
	if err != nil {
		return fmt.Errorf("%w: its tags didn't read back: %v", ErrFormat, err)
	}
	if !wrote(got, t.clean(), fields) {
		return fmt.Errorf("%w: its tags didn't read back as written", ErrFormat)
	}
	if o.Check != nil {
		if _, err := out.Seek(0, io.SeekStart); err != nil {
			return err
		}
		if err := o.Check(out); err != nil {
			return fmt.Errorf("%w: it didn't play once written: %v", ErrFormat, err)
		}
	}
	return nil
}

// wrote reports whether got, read back, holds what was written of t.
func wrote(got tags.Tags, t Tags, fields Fields) bool {
	same := func(f Fields, ok bool) bool { return fields&f == 0 || ok }
	return same(Title, got.Title == t.Title) && same(Artists, slices.Equal(got.Artists, t.Artists)) &&
		same(Album, got.Album == t.Album) && same(AlbumArtists, slices.Equal(got.AlbumArtists, t.AlbumArtists)) &&
		same(Track, got.Track == t.Track) && same(Disc, got.Disc == t.Disc) && same(Year, got.Year == t.Year) &&
		same(Genre, got.Genre == t.Genre || numeric(t.Genre)) && same(ISRC, got.ISRC == t.ISRC) &&
		same(Cover, (got.Picture == nil) == (t.Cover == nil) && (t.Cover == nil || string(got.Picture.Data) == string(t.Cover.Data)))
}

// numeric reports whether a genre is all digits, which ID3 reads as the
// number of a genre of ID3v1.
func numeric(genre string) bool {
	return genre != "" && strings.Trim(genre, "0123456789") == ""
}

// Writable tells why the tags of the file at path can't be written, nil
// when they can: a file read-only, a folder MixMusic can't write the new
// file in, or a file of a kind whose tags aren't written.
func Writable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return describe(err)
	}
	if info.IsDir() {
		return errors.New("it's a folder")
	}
	if err := writableFile(path, info); err != nil {
		return err
	}
	if err := writableDir(filepath.Dir(path)); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return describe(err)
	}
	defer f.Close()
	_, err = KindOf(f)
	return err
}

// describe words the errors of the system as the windows tell them.
func describe(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, fs.ErrNotExist):
		return errors.New("the file isn't there any more")
	case inUse(err):
		return errors.New("another program has it open: close it there, and try again")
	case errors.Is(err, fs.ErrPermission):
		return errors.New("MixMusic isn't allowed to change it, or another program has it open")
	case diskFull(err):
		return errors.New("the disk is full")
	}
	return err
}

// clean is t as it's written: text as tags.Read reads it, without
// control characters, trimmed, and without empty values.
func (t Tags) clean() Tags {
	t.Title, t.Album, t.Genre = text(t.Title), text(t.Album), text(t.Genre)
	t.ISRC = strings.ToUpper(strings.ReplaceAll(text(t.ISRC), "-", ""))
	t.Artists, t.AlbumArtists = names(t.Artists), names(t.AlbumArtists)
	t.Track, t.Disc, t.Year = max(t.Track, 0), max(t.Disc, 0), max(t.Year, 0)
	return t
}

// check tells why t can't be written as tags.Read reads it back.
func (t Tags) check() error {
	switch {
	case t.Track > maxNumber || t.Disc > maxNumber:
		return fmt.Errorf("a track's or a disc's number must be from 1 to %d", maxNumber)
	case t.Year != 0 && (t.Year < 1000 || t.Year > 9999):
		return errors.New("the year must be one of four digits")
	}
	return nil
}

// maxNumber bounds a track's and a disc's numbers, as tags.Read reads
// them.
const maxNumber = 9999

// text is s without control characters but tabs, trimmed.
func text(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\t' {
			return -1
		}
		return r
	}, strings.ToValidUTF8(s, ""))
	return strings.TrimSpace(s)
}

// names are the names of vs, as text, without empty ones.
func names(vs []string) []string {
	var out []string
	for _, v := range vs {
		if v = text(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// file reads the file whose tags are written.
type file struct {
	r    io.ReadSeeker
	size int64
}

func newFile(r io.ReadSeeker) (*file, error) {
	size, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	return &file{r: r, size: size}, nil
}

// kind tells the kind of the file by its first bytes, as tags.Read does.
func (f *file) kind() (Kind, error) {
	var head [12]byte
	n, _ := f.readAt(0, head[:])
	at := int64(0)
	if n >= 10 && string(head[:3]) == "ID3" {
		end, ok := f.id3End()
		if !ok {
			return "", fmt.Errorf("%w: its ID3 tag is broken", ErrFormat)
		}
		at = end
		n, _ = f.readAt(at, head[:])
		if n >= 4 && string(head[:4]) == "fLaC" {
			return FLAC, nil
		}
		return MP3, nil
	}
	switch {
	case n >= 4 && string(head[:4]) == "fLaC":
		return FLAC, nil
	case n >= 4 && string(head[:4]) == "OggS":
		return f.oggKind()
	case n >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WAVE":
		return "", ErrUnsupported
	case n >= 8 && mp4Start(string(head[4:8])):
		return MP4, nil
	case n >= 2 && head[0] == 0xFF && head[1]&0xE0 == 0xE0:
		return MP3, nil
	}
	return "", ErrUnsupported
}

// readAt reads len(b) bytes at offset off, or as many as there are.
func (f *file) readAt(off int64, b []byte) (int, error) {
	if _, err := f.r.Seek(off, io.SeekStart); err != nil {
		return 0, err
	}
	return io.ReadFull(f.r, b)
}

// bytesAt reads n bytes at offset off, which must all be there, and be
// no more than maxRead.
func (f *file) bytesAt(off, n int64) ([]byte, error) {
	if n < 0 || off < 0 || off+n > f.size {
		return nil, fmt.Errorf("%w: it ends before what its tags say", ErrFormat)
	}
	if n > maxRead {
		return nil, fmt.Errorf("%w: its tags are larger than %d MB", ErrFormat, maxRead>>20)
	}
	b := make([]byte, n)
	if _, err := f.readAt(off, b); err != nil {
		return nil, err
	}
	return b, nil
}

// copyTo copies the bytes from offset from to offset to to w, as they
// are.
func (f *file) copyTo(w io.Writer, from, to int64) error {
	if to <= from {
		return nil
	}
	if _, err := f.r.Seek(from, io.SeekStart); err != nil {
		return err
	}
	n, err := io.CopyN(w, f.r, to-from)
	if err == io.EOF && n < to-from {
		return fmt.Errorf("%w: it ended while it was copied", ErrFormat)
	}
	return err
}

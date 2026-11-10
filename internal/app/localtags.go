package app

// Editing the tags of the music on this computer (#339): Edit info… of a
// song, Edit album… of an album and Move to album… of a song, which write
// the files' tags through internal/tagwrite, each file safely, and have the
// library read them again at once, and the queue show them.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/tags"
	"github.com/fzn0x/mixmusic/internal/tagwrite"
)

// FileTags are the tags of a file on this computer, as the dialog that
// edits them starts from: Cover is the cover the file holds, which
// CoverURL shows, "" without one.
type FileTags struct {
	Tags     tagwrite.Tags
	CoverURL string
}

// pathOf is the path of the file of a local track, by its ID.
func pathOf(id string) (string, error) {
	path, ok := localfiles.PathOf(id)
	if !ok {
		return "", errors.New("it isn't a song on this computer")
	}
	return path, nil
}

// cantEdit words why the tags of the file at path can't be changed.
func cantEdit(path string, err error) error {
	return fmt.Errorf("MixMusic can't change %s: %w", filepath.Base(path), err)
}

// EditableTags reads the tags of the file of a local track, for the dialog
// that edits them, once it tells why they can't be written: a file
// read-only, a folder MixMusic can't write in, or a kind of file whose
// tags it doesn't write, as WAV.
func (l *LocalFiles) EditableTags(id string) (FileTags, error) {
	path, err := pathOf(l.a.fileID(id))
	if err != nil {
		return FileTags{}, err
	}
	if err := tagwrite.Writable(path); err != nil {
		return FileTags{}, cantEdit(path, err)
	}
	f, err := os.Open(path)
	if err != nil {
		return FileTags{}, cantEdit(path, err)
	}
	defer f.Close()
	t, err := tags.Read(f)
	if err != nil {
		return FileTags{}, cantEdit(path, err)
	}
	ft := FileTags{Tags: tagwrite.Tags{Title: t.Title, Artists: t.Artists, Album: t.Album, AlbumArtists: t.AlbumArtists,
		Track: t.Track, Disc: t.Disc, Year: t.Year, Genre: t.Genre, ISRC: t.ISRC}}
	if p := t.Picture; p != nil {
		ft.Tags.Cover = &tagwrite.Picture{MIME: p.MIME, Data: p.Data}
		ft.CoverURL, _ = l.a.localFiles.KeepCover(p.Data, p.MIME)
	}
	return ft, nil
}

// Editable tells why the tags of the files of tracks can't be written, as
// EditableTags does: of the first that can't, by its name.
func (l *LocalFiles) Editable(ids []string) error {
	for _, id := range l.a.copyIDs(ids) {
		path, err := pathOf(id)
		if err != nil {
			return err
		}
		if err := tagwrite.Writable(path); err != nil {
			return cantEdit(path, err)
		}
	}
	return nil
}

// AlbumTags are the tags an album's songs share, as the dialog that edits
// them starts from: its title, album artists, year, and the cover its
// first song holds, which CoverURL shows.
func (l *LocalFiles) AlbumTags(a media.Album) (FileTags, error) {
	var ft FileTags
	for i, t := range a.Tracks {
		path, err := pathOf(t.ID)
		if err != nil {
			return FileTags{}, err
		}
		e, ok := l.a.localFiles.Entry(path)
		if !ok {
			continue
		}
		if i == 0 {
			first, err := l.EditableTags(t.ID)
			if err != nil {
				return FileTags{}, err
			}
			ft.Tags.Cover, ft.CoverURL = first.Tags.Cover, first.CoverURL
			ft.Tags.Album = e.Album
		}
		if len(ft.Tags.AlbumArtists) == 0 {
			ft.Tags.AlbumArtists = e.AlbumArtists
		}
	}
	ft.Tags.Year, _ = strconv.Atoi(a.Year)
	return ft, nil
}

// CoverOf reads an image file the user chose as a cover: a JPEG or PNG
// image of at most 10 MB, as tags hold them. Its address shows it in the
// dialog until it's written.
func (l *LocalFiles) CoverOf(path string) (tagwrite.Picture, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return tagwrite.Picture{}, "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, tagwrite.MaxCover+1))
	if err != nil {
		return tagwrite.Picture{}, "", err
	}
	mime, err := tagwrite.CoverType(data)
	if err != nil {
		return tagwrite.Picture{}, "", err
	}
	url, _ := l.a.localFiles.KeepCover(data, mime)
	return tagwrite.Picture{MIME: mime, Data: data}, url, nil
}

// writeOptions are how MixMusic writes tags: the new file must play as
// the engine reads files before it takes the old one's place.
var writeOptions = tagwrite.Options{Check: func(r io.ReadSeeker) error {
	_, err := audio.Probe(r)
	return err
}}

// EditTags writes the fields of t that fields names to the files of the
// tracks of ids, each safely, so that one that fails stays as it was,
// telling progress how many it wrote so far. The library reads them again
// at once, and the queue shows them. It returns how many files it changed,
// and why the first that failed did, by its name; the others are written
// all the same.
func (l *LocalFiles) EditTags(ids []string, t tagwrite.Tags, fields tagwrite.Fields, progress func(done int)) (int, error) {
	var paths []string
	for _, id := range l.a.copyIDs(ids) {
		path, err := pathOf(id)
		if err != nil {
			return 0, err
		}
		paths = append(paths, path)
	}
	return l.write(paths, func(string) (tagwrite.Tags, tagwrite.Fields) { return t, fields }, progress)
}

// MoveToAlbum files the song of id under the album albumID of the
// library, by writing its album's tags: the album's title and album
// artists. A song from another folder joining an album its folder groups,
// whose songs have no album artists, gives them all the album artists the
// album shows, so that they stay together.
func (l *LocalFiles) MoveToAlbum(id, albumID string) error {
	id = l.a.fileID(id)
	path, err := pathOf(id)
	if err != nil {
		return err
	}
	m, err := l.a.localFiles.MoveTo(path, albumID)
	if err != nil {
		return err
	}
	if err := l.Editable(append([]string{id}, idsOf(m.Others)...)); err != nil {
		return err
	}
	paths := append([]string{path}, m.Others...)
	_, err = l.write(paths, func(p string) (tagwrite.Tags, tagwrite.Fields) {
		t := tagwrite.Tags{Album: m.Album, AlbumArtists: m.AlbumArtists}
		if p == path {
			return t, tagwrite.Album | tagwrite.AlbumArtists
		}
		return t, tagwrite.AlbumArtists
	}, nil)
	return err
}

// MovePlan tells what MoveToAlbum writes: the album's title and album
// artists, and the album's songs that take its album artists too.
func (l *LocalFiles) MovePlan(id, albumID string) (localfiles.Move, error) {
	path, err := pathOf(l.a.fileID(id))
	if err != nil {
		return localfiles.Move{}, err
	}
	return l.a.localFiles.MoveTo(path, albumID)
}

// MoveToNewAlbum files the song of id under a new album of a title, by
// writing its album's tag; its album artists stay.
func (l *LocalFiles) MoveToNewAlbum(id, title string) error {
	_, err := l.EditTags([]string{id}, tagwrite.Tags{Album: title}, tagwrite.Album, nil)
	return err
}

// idsOf are the IDs of the tracks of the files at paths.
func idsOf(paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = localfiles.TrackID(p)
	}
	return out
}

// write writes the tags edit gives each file of paths, then has the
// library read those it changed again, and the queue show them.
func (l *LocalFiles) write(paths []string, edit func(path string) (tagwrite.Tags, tagwrite.Fields), progress func(int)) (int, error) {
	var (
		done  []string
		first error
	)
	for _, path := range paths {
		t, fields := edit(path)
		if err := tagwrite.Write(path, t, fields, writeOptions); err != nil {
			if first == nil {
				first = fmt.Errorf("couldn't save %s: %w", filepath.Base(path), err)
			}
			continue
		}
		done = append(done, path)
		if progress != nil {
			progress(len(done))
		}
	}
	if len(done) > 0 {
		l.a.localFiles.Reread(done)
		var tracks []media.Track
		for _, p := range done {
			if t, ok := l.a.localFiles.Track(p); ok {
				tracks = append(tracks, t)
			} else if e, err := l.a.localFiles.ReadFile(p); err == nil {
				tracks = append(tracks, localfiles.TrackOf(e)) // a file opened
			}
		}
		if p := l.a.goPlayer.Load(); p != nil {
			p.Retagged(l.a.withSongs(tracks))
		}
	}
	return len(done), first
}

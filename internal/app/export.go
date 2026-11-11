package app

// Exporting tracks (#341): copies of the tracks downloaded, and of the
// songs on this computer, in a folder the user chose, named "Artist -
// Title", with their tags and cover in each file, through
// internal/export. The downloads and the files stay as they are.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/export"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/tagwrite"
)

// albumWait bounds how long an export waits for the page of an album, for
// its year and its tracks' numbers.
const albumWait = 10 * time.Second

// ExportOptions tell how Export exports.
type ExportOptions struct {
	// Album returns the page of an album, by its ID, for its year, its
	// artists and the numbers of its tracks: nil, or an error, leaves
	// them out of the copies.
	Album func(ctx context.Context, id string) (media.Album, error)
	// Progress is told, from Export's goroutine, how many tracks of how
	// many were exported, or failed to be.
	Progress func(done, total int)
}

// ExportResult is what an export came to: the files it wrote, and the
// tracks that couldn't be, with why.
type ExportResult struct {
	Files  []string
	Failed []ExportFailure
}

// ExportFailure is a track that couldn't be exported, and why.
type ExportFailure struct {
	Track media.Track
	Err   error
}

// Exportable reports whether a track exports: a download that plays
// without the network, or a song on this computer whose file is there.
func (d *Downloads) Exportable(t media.Track) bool {
	path, _ := d.a.exportFile(t)
	return path != ""
}

// exportFile returns the file a track exports from, "" for none, and
// whether it's the user's own, which goes as it is: of a song with
// copies, its best copy.
func (a *App) exportFile(t media.Track) (path string, own bool) {
	if t.Source == media.SourceFile {
		if a.localFiles == nil {
			return "", false
		}
		p, ok := a.localFiles.FileOf(t.ID)
		if !ok || a.localFiles.Missing(t.ID) {
			return "", false
		}
		return p, true
	}
	if k, ok := a.downloads.Lookup(t.ID); ok && k.Path != "" {
		return k.Path, false
	}
	// A recording on YouTube Music the downloads hold for a Spotify track.
	if source, video, err := media.ParseID(t.ID); err == nil && source == media.SourceYouTube {
		if p, ok := a.downloads.Snapshot().VideoFile(video); ok {
			return p, false
		}
	}
	return "", false
}

// ChooseExportFolder asks the user for the folder to export tracks to,
// over parent: "" when they chose none. It waits for their answer.
func (d *Downloads) ChooseExportFolder(parent *mygo.Window) (string, error) {
	start := ""
	if home, err := os.UserHomeDir(); err == nil {
		if music := filepath.Join(home, "Music"); dirExists(music) {
			start = music
		}
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: parent, Title: "Choose where to export the tracks",
		Directory: true, CreateDirectories: true, DefaultPath: start})
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// Export writes copies of tracks to the folder dir, each named "Artist -
// Title", numbered when its name is taken. A download gets the title,
// artists and album of its track, the cover the downloads keep, and, of
// the page of its album, which o.Album gives, the year, the album's
// artists and its number there, and its ISRC where its service gave one.
// A song on this computer goes as it is, its best copy of a song with
// copies. The tracks are exported in turn, until ctx ends.
func (d *Downloads) Export(ctx context.Context, tracks []media.Track, dir string, o ExportOptions) ExportResult {
	a := d.a
	var res ExportResult
	albums := albumPages{get: o.Album}
	for i, t := range tracks {
		if ctx.Err() != nil {
			res.Failed = append(res.Failed, ExportFailure{t, ctx.Err()})
			continue
		}
		path, err := a.exportOne(ctx, t, dir, &albums)
		if err != nil {
			res.Failed = append(res.Failed, ExportFailure{t, err})
		} else {
			res.Files = append(res.Files, path)
		}
		if o.Progress != nil {
			o.Progress(i+1, len(tracks))
		}
	}
	return res
}

// exportOne writes the copy of t to dir.
func (a *App) exportOne(ctx context.Context, t media.Track, dir string, albums *albumPages) (string, error) {
	path, own := a.exportFile(t)
	if path == "" {
		return "", errors.New("it isn't downloaded, nor on this computer")
	}
	names := make([]string, 0, len(t.Artists))
	for _, ar := range t.Artists {
		names = append(names, ar.Name)
	}
	tr := export.Track{Path: path, Artists: names, Title: t.Title, AsIs: own}
	if !own {
		tr.Tags = tagwrite.Tags{Title: t.Title, Artists: names, ISRC: t.ISRC}
		if t.Album != nil {
			tr.Tags.Album = t.Album.Name
			albums.fill(ctx, t, &tr.Tags)
		}
		tr.Tags.Cover = a.exportCover(t)
	}
	return export.Write(dir, tr)
}

// exportCover is the cover the downloads keep of t, nil for none.
func (a *App) exportCover(t media.Track) *tagwrite.Picture {
	for _, img := range t.Artwork {
		p, ok := a.downloads.ArtworkFile(img.URL)
		if !ok {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if c, err := export.Cover(data); err == nil {
			return c
		}
	}
	return nil
}

// albumPages are the pages of the albums of an export, each asked for
// once. Once two in a row failed, as they do without the network, the
// others aren't asked for.
type albumPages struct {
	get    func(ctx context.Context, id string) (media.Album, error)
	pages  map[string]*media.Album
	failed int
}

// fill adds to tg, of t, what the page of its album tells: the year, the
// album's artists and t's number on it.
func (p *albumPages) fill(ctx context.Context, t media.Track, tg *tagwrite.Tags) {
	id := t.Album.ID
	if p.get == nil || id == "" {
		return
	}
	if p.pages == nil {
		p.pages = map[string]*media.Album{}
	}
	al, asked := p.pages[id]
	if !asked && p.failed < 2 {
		actx, cancel := context.WithTimeout(ctx, albumWait)
		got, err := p.get(actx, id)
		cancel()
		if err == nil {
			al, p.failed = &got, 0
		} else {
			p.failed++
		}
		p.pages[id] = al
	}
	if al == nil {
		return
	}
	year := strings.TrimSpace(al.Year)
	if y, err := strconv.Atoi(year[:min(4, len(year))]); err == nil && y >= 1000 {
		tg.Year = y
	}
	for _, ar := range al.Artists {
		tg.AlbumArtists = append(tg.AlbumArtists, ar.Name)
	}
	for i, x := range al.Tracks {
		if x.ID == t.ID {
			tg.Track = i + 1
			return
		}
	}
	for i, x := range al.Tracks {
		if strings.EqualFold(x.Title, t.Title) {
			tg.Track = i + 1
			return
		}
	}
}

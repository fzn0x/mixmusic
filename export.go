package main

// Exporting tracks (#341): Export… in the menu of a track downloaded or on
// this computer, and in the bar of the tracks selected, asks for a folder
// and writes copies of them there, named "Artist - Title", with their
// tags and cover, beside the main thread. internal/app's Downloads.Export
// makes the copies, and a toast tells how it went, with a way to the
// files.

import (
	"context"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/locales"
)

// chooseExportFolder asks the user for the folder to export to, "" when
// they chose none; tests replace it.
var chooseExportFolder = func(s *shell) (string, error) { return s.svc.Downloads.ChooseExportFolder(s.win) }

// exportAlbum reads the page of an album, for the year and the track
// numbers the copies tell; tests replace it.
var exportAlbum = func(s *shell, ctx context.Context, id string) (media.Album, error) {
	return s.svc.Catalog.Album(ctx, id)
}

// exportMemo is whether the tracks selected had any to export, as found
// for the selection of seq, the downloads of kept and the music on this
// computer of local: once for each change, not in each frame.
type exportMemo struct {
	seq, kept, local uint64
	known, any       bool
}

// exportItem is Export… in the menu of a track downloaded or on this
// computer.
func (s *shell) exportItem(m *ui.Menu, t media.Track) {
	if s.svc.Downloads.Exportable(t) && m.Item(locales.ExportMenu()).Chosen() {
		s.exportTracks([]media.Track{t})
	}
}

func init() { addPickAction(40, (*shell).exportPicked) }

// exportPicked is the bar's Export, while some of the tracks selected are
// downloaded or on this computer.
func (s *shell) exportPicked(c *ui.Context, tracks []media.Track) {
	e := &s.exports
	if kept, local := s.kept.Version(), s.svc.LocalFiles.Version(); !e.known || e.seq != s.picks.seq || e.kept != kept || e.local != local {
		e.any = false
		for _, t := range tracks {
			if s.svc.Downloads.Exportable(t) {
				e.any = true
				break
			}
		}
		e.seq, e.kept, e.local, e.known = s.picks.seq, kept, local, true
	}
	if e.any && s.button(c, "picks-export", locales.ExportButton(), false).Label(locales.ExportSelected()).Clicked() {
		s.exportTracks(tracks)
	}
}

// exportTracks asks for a folder, and exports there those of tracks that
// are downloaded or on this computer.
func (s *shell) exportTracks(tracks []media.Track) {
	var out []media.Track
	for _, t := range tracks {
		if s.svc.Downloads.Exportable(t) {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		s.notes = append(s.notes, notice(locales.ExportOnlyDownloaded()))
		return
	}
	go func() {
		dir, err := chooseExportFolder(s)
		s.update(func() {
			switch {
			case err != nil:
				s.notes = append(s.notes, noticeOf(err))
			case dir != "":
				s.exportTo(out, dir)
			}
		})
	}()
}

// exportNoun names what an export writes: a track by its title, or how
// many.
func exportNoun(tracks []media.Track) string {
	if len(tracks) == 1 {
		return tracks[0].Title
	}
	return locales.ExportTracks(len(tracks))
}

// exportTo writes copies of tracks to dir, beside the main thread, and
// tells how it went.
func (s *shell) exportTo(tracks []media.Track, dir string) {
	s.notes = append(s.notes, notice(locales.ExportingTo(exportNoun(tracks), dir)))
	go func() {
		res := s.svc.Downloads.Export(context.Background(), tracks, dir, app.ExportOptions{
			Album: func(ctx context.Context, id string) (media.Album, error) { return exportAlbum(s, ctx, id) },
		})
		s.update(func() { s.exported(tracks, dir, res) })
	}()
}

// exported tells how an export of tracks to dir went: a toast with a way
// to the files, and why those that failed did.
func (s *shell) exported(tracks []media.Track, dir string, res app.ExportResult) {
	if len(res.Files) > 0 {
		first := res.Files[0]
		msg := locales.ExportedTo(exportNoun(tracks), dir)
		if len(res.Failed) > 0 {
			msg = locales.ExportedPartial(len(res.Files), len(tracks), dir)
		}
		s.actionToasts = append(s.actionToasts, actionToast{msg, locales.ExportShow(), func() { showInFolder(first) }})
	}
	switch n := len(res.Failed); {
	case n == 1:
		f := res.Failed[0]
		s.notes = append(s.notes, player.Notice{Message: locales.ExportFailed(f.Track.Title, f.Err.Error()), Error: true})
	case n > 1:
		f := res.Failed[0]
		s.notes = append(s.notes, player.Notice{Message: locales.ExportFailedSeveral(n, f.Track.Title, f.Err.Error()), Error: true})
	}
}

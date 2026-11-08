package main

import (
	"context"
	"slices"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// A mix's cover is its tracks' covers, until the user chooses one of their
// own: Change cover…, in the menu of the mix's page and of its tile, asks
// for an image file or one of its tracks' covers, which the app keeps in
// the data folder (internal/app/mixcovers.go). Use the songs' covers goes
// back. The cover shows wherever the mix does, as its artwork.

// coverChoices is how many of a mix's tracks' covers the dialog offers.
const coverChoices = 12

// mixCovering is the dialog that changes the cover of a mix: whether it's
// open, the mix's ID and title, and whether the mix shows a cover the
// user chose.
type mixCovering struct {
	open      bool
	id, title string
	chosen    bool
}

// chosenCover reports whether art is a cover the user chose for a mix.
func chosenCover(art []media.Image) bool {
	if len(art) != 1 {
		return false
	}
	_, ok := library.CoverName(art[0].URL)
	return ok
}

// changeCoverItem is Change cover… in the menu of a mix, p, which opens
// the dialog that changes its cover.
func (s *shell) changeCoverItem(m *ui.Menu, p media.Playlist) {
	if p.Source != media.SourceLocal {
		return
	}
	if m.Item("Change cover…").Chosen() {
		s.covering = mixCovering{open: true, id: p.ID, title: p.Title, chosen: chosenCover(p.Artwork)}
	}
}

// coverDialog changes the cover of the mix s.covering names: to an image
// file the system's dialog opens, to the cover of one of its tracks, or
// back to its tracks' covers.
func (s *shell) coverDialog(c *ui.Context) {
	k := &s.lk
	cv := s.covering
	key := "playlist:" + cv.id
	s.dialog(c, &s.covering.open, dialogOptions{key: "mix-cover", title: "change cover", width: 560}, func() {
		ui.Text(c, "Choose an image, a PNG, JPEG or WebP, or the cover of one of the songs of "+cv.title+
			". It shows cut to a square.").FontSize(k.bodySize()).TextColor(k.muted)
		res := fetch(s, key, func(ctx context.Context) (media.Playlist, error) { return s.svc.Catalog.PlaylistStart(ctx, cv.id) })
		covers := songCovers(res.value.Tracks)
		if len(covers) == 0 {
			return
		}
		ui.Row(c).Wrap().Gap(8).Margin(16, 0, 0).Children(func() {
			for i, t := range covers {
				b := s.buttonBase(c.Key("mix-cover-" + count(i))).Radius(k.radiusSm).Label("Use the cover of " + t.Title).
					Cursor(ui.CursorPointer)
				if b.Clicked() {
					s.covering.open = false
					s.coverOfTrack(cv, t)
				}
				b.Children(func() { s.cover(c, "mix-cover-art-"+count(i), t.Artwork, 72, false) })
			}
		})
	}, func() {
		if cv.chosen && s.dialogButton(c, "mix-cover-songs", "use the songs' covers", false).Clicked() {
			s.covering.open = false
			s.useSongsCovers(cv)
		}
		if s.dialogButton(c, "mix-cover-cancel", "cancel", false).Clicked() {
			s.covering.open = false
		}
		if s.dialogButton(c, "mix-cover-file", "choose an image…", true).Clicked() {
			s.covering.open = false
			s.coverOfFile(cv)
		}
	})
}

// songCovers are the first of tracks with a cover each, one a cover.
func songCovers(tracks []media.Track) []media.Track {
	var out []media.Track
	var seen []string
	for _, t := range tracks {
		u := artworkURL(t.Artwork, 72)
		if u == "" || slices.Contains(seen, u) {
			continue
		}
		seen = append(seen, u)
		out = append(out, t)
		if len(out) == coverChoices {
			break
		}
	}
	return out
}

// coverOfFile asks for an image file, beside the main thread, and makes
// the cover of the mix of cv of the one the user chooses.
func (s *shell) coverOfFile(cv mixCovering) {
	win := s.win
	go func() {
		paths, err := openFileDialog(mygo.OpenDialogOptions{Parent: win, Title: "Choose a cover for " + cv.title,
			Filters: []mygo.FileFilter{{Name: "Images", Extensions: []string{"png", "jpg", "jpeg", "webp"}}}})
		if err == nil && len(paths) == 0 {
			return
		}
		if err == nil {
			err = s.svc.Collection.SetMixCover(cv.id, paths[0])
		}
		s.coverChanged(cv, err)
	}()
}

// coverOfTrack makes the cover of the mix of cv of the cover of t, one of
// its tracks, beside the main thread: it may download.
func (s *shell) coverOfTrack(cv mixCovering, t media.Track) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.coverChanged(cv, s.svc.Collection.SetMixCoverOf(ctx, cv.id, t))
	}()
}

// useSongsCovers gives the mix of cv its tracks' covers again.
func (s *shell) useSongsCovers(cv mixCovering) {
	if err := s.svc.Collection.UseSongsCovers(cv.id); err != nil {
		s.notes = append(s.notes, noticeOf(err))
		return
	}
	s.refetch("playlist:" + cv.id)
	s.notes = append(s.notes, notice(cv.title+" shows its songs' covers again"))
}

// coverChanged tells how changing the cover of the mix of cv went, from
// any goroutine, and has its page show the new one.
func (s *shell) coverChanged(cv mixCovering, err error) {
	s.update(func() {
		if err != nil {
			s.notes = append(s.notes, player.Notice{Message: "Couldn't change the cover of " + cv.title + ": " + err.Error(), Error: true})
			return
		}
		s.refetch("playlist:" + cv.id)
		s.notes = append(s.notes, notice("Changed the cover of "+cv.title))
	})
}

// mixCoverFile returns the file of the cover chosen for a mix, by its
// address, for the windows to show.
func (s *shell) mixCoverFile(url string) (string, bool) { return s.svc.Collection.MixCoverFile(url) }

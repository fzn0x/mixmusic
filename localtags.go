package main

// Editing the tags of the music on this computer (#339): Edit info… and
// Move to album… in the menu of a song on this computer, and Edit album…
// in the menu of an album of it. Each opens its dialog once MixMusic knows
// it can change the files: a file that is read-only, in a folder it can't
// write in, or of a kind whose tags it doesn't write, as WAV, says so
// instead. internal/app's LocalFiles writes each file safely, has the
// library read it again at once, and the queue show it.

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/tagwrite"
)

// tagDialogs are the dialogs that change tags, and seq counts their
// openings, so that what a write tells once it's done goes to the dialog
// that asked for it, if it's still open.
type tagDialogs struct {
	edit tagEditor
	move albumMover
	seq  int
}

// tagForm is the text of the fields of Edit info… and Edit album….
type tagForm struct {
	title, artists, album, albumArtists, track, disc, year, genre string
}

// tagEditor is the dialog of Edit info… of a song, or of Edit album…: the
// songs it changes, by their IDs, and the name it gives them; the album
// they were on, which its page may show; the form, and as it opened; the
// cover the files are to hold, which coverURL shows, nil for none, and
// coverSet once the user chose another or took it away; and while it
// writes, how many songs it wrote, and why the last save failed.
type tagEditor struct {
	open     bool
	album    bool
	ids      []string
	name     string
	from     string
	form     tagForm
	was      tagForm
	cover    *tagwrite.Picture
	coverURL string
	coverSet bool
	saving   bool
	done     int
	problem  string
	seq      int
}

// albumMover is the dialog of Move to album…: the song it moves, the
// search of the albums, the albums it found of the index's version, the
// one chosen, "" while the search names a new one, what moving there
// writes besides, and while it writes, why the last move failed.
type albumMover struct {
	open    bool
	track   media.Track
	query   string
	shownQ  string
	version uint64
	found   []media.Album
	chosen  string
	note    string
	saving  bool
	problem string
	seq     int
}

// moveChoices is how many albums Move to album… shows at once.
const moveChoices = 8

// tagItems are the items of the menu of a song on this computer that
// change its tags. The mini player has none: its window is too small for
// their dialogs.
func (s *shell) tagItems(m *ui.Menu, t media.Track) {
	if s.mini.win != nil || t.Source != media.SourceFile {
		return
	}
	if m.Item("Edit info…").Chosen() {
		s.editInfo(t)
	}
	if m.Item("Move to album…").Chosen() {
		s.moveToAlbum(t)
	}
}

// editAlbumOf is Edit album… in the menu of the page of an album on this
// computer, of its tracks.
func (s *shell) editAlbumOf(m *ui.Menu, source media.Source, tracks []media.Track) {
	if source != media.SourceFile || len(tracks) == 0 || tracks[0].Album == nil {
		return
	}
	s.editAlbumItem(m, media.Album{ID: tracks[0].Album.ID, Source: source})
}

// editAlbumItem is Edit album… in the menu of an album on this computer.
func (s *shell) editAlbumItem(m *ui.Menu, a media.Album) {
	if s.mini.win != nil || a.Source != media.SourceFile {
		return
	}
	if m.Item("Edit album…").Chosen() {
		s.editAlbum(a.ID)
	}
}

// editInfo reads the tags of the file of t, beside the main thread, and
// opens Edit info… with them, or tells why they can't be changed.
func (s *shell) editInfo(t media.Track) {
	go func() {
		ft, err := s.svc.LocalFiles.EditableTags(t.ID)
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			s.tagDialogs.seq++
			form := formOf(ft.Tags)
			from := ""
			if t.Album != nil {
				from = t.Album.ID
			}
			s.tagDialogs.edit = tagEditor{open: true, ids: []string{t.ID}, name: t.Title, from: from, form: form, was: form,
				cover: ft.Tags.Cover, coverURL: ft.CoverURL, seq: s.tagDialogs.seq}
		})
	}()
}

// editAlbum reads the album of id and the tags its songs share, beside
// the main thread, and opens Edit album… with them, or tells why its
// songs can't be changed.
func (s *shell) editAlbum(id string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		a, err := s.svc.Catalog.Album(ctx, id)
		var ids []string
		for _, t := range a.Tracks {
			ids = append(ids, t.ID)
		}
		if err == nil && len(ids) == 0 {
			err = errors.New("the album has no songs")
		}
		if err == nil {
			err = s.svc.LocalFiles.Editable(ids)
		}
		var tags tagwrite.Tags
		url := ""
		if err == nil {
			ft, ferr := s.svc.LocalFiles.AlbumTags(a)
			tags, url, err = ft.Tags, ft.CoverURL, ferr
		}
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			s.tagDialogs.seq++
			form := formOf(tags)
			s.tagDialogs.edit = tagEditor{open: true, album: true, ids: ids, name: a.Title, from: id, form: form, was: form,
				cover: tags.Cover, coverURL: url, seq: s.tagDialogs.seq}
		})
	}()
}

// formOf is the form of tags: several artists apart by "; ".
func formOf(t tagwrite.Tags) tagForm {
	number := func(n int) string {
		if n == 0 {
			return ""
		}
		return strconv.Itoa(n)
	}
	return tagForm{title: t.Title, artists: strings.Join(t.Artists, "; "), album: t.Album,
		albumArtists: strings.Join(t.AlbumArtists, "; "), track: number(t.Track), disc: number(t.Disc),
		year: number(t.Year), genre: t.Genre}
}

// tags reads the form back: the tags of the fields that differ from was,
// which a save writes, or why it can't be saved.
func (f tagForm) tags(was tagForm) (tagwrite.Tags, tagwrite.Fields, error) {
	var t tagwrite.Tags
	var fields tagwrite.Fields
	differs := func(a, b string) bool { return strings.TrimSpace(a) != strings.TrimSpace(b) }
	number := func(s, what string, least, most int) (int, error) {
		s = strings.TrimSpace(s)
		if s == "" {
			return 0, nil
		}
		n, err := strconv.Atoi(s)
		if err != nil || n < least || n > most {
			return 0, fmt.Errorf("%s must be a number from %d to %d", what, least, most)
		}
		return n, nil
	}
	if differs(f.title, was.title) {
		t.Title, fields = strings.TrimSpace(f.title), fields|tagwrite.Title
	}
	if differs(f.artists, was.artists) {
		t.Artists, fields = splitNames(f.artists), fields|tagwrite.Artists
	}
	if differs(f.album, was.album) {
		t.Album, fields = strings.TrimSpace(f.album), fields|tagwrite.Album
	}
	if differs(f.albumArtists, was.albumArtists) {
		t.AlbumArtists, fields = splitNames(f.albumArtists), fields|tagwrite.AlbumArtists
	}
	var err error
	if differs(f.track, was.track) {
		if t.Track, err = number(f.track, "The track", 1, 9999); err != nil {
			return t, 0, err
		}
		fields |= tagwrite.Track
	}
	if differs(f.disc, was.disc) {
		if t.Disc, err = number(f.disc, "The disc", 1, 9999); err != nil {
			return t, 0, err
		}
		fields |= tagwrite.Disc
	}
	if differs(f.year, was.year) {
		if t.Year, err = number(f.year, "The year", 1000, 9999); err != nil {
			return t, 0, err
		}
		fields |= tagwrite.Year
	}
	if differs(f.genre, was.genre) {
		t.Genre, fields = strings.TrimSpace(f.genre), fields|tagwrite.Genre
	}
	return t, fields, nil
}

// splitNames are the names of a field of several, apart by ";".
func splitNames(s string) []string {
	var out []string
	for _, n := range strings.Split(s, ";") {
		if n = strings.TrimSpace(n); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// tagDialog shows Edit info… or Edit album…, over the window.
func (s *shell) tagDialog(c *ui.Context) {
	e := &s.tagDialogs.edit
	if !e.open {
		return
	}
	k := &s.lk
	title := "edit info"
	if e.album {
		title = "edit album"
	}
	submitted := false
	s.dialog(c, &e.open, dialogOptions{key: "tags", title: title, width: 560, alert: e.saving}, func() {
		about := "The tags of " + e.name + ", which MixMusic writes into its file."
		if e.album {
			about = "What the " + plural(len(e.ids), "song") + " of " + e.name + " share, which MixMusic writes into their files."
		}
		ui.Text(c, about).FontSize(k.bodySize()).TextColor(k.muted)
		ui.Column(c).Gap(12).Children(func() {
			field := func(key, label string, value *string, width float32, focus bool) {
				ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
					ui.Text(c, label).FontSize(k.smallSize()).TextColor(k.muted).Width(112).Shrink(0)
					in := ui.TextInput(c.Key("tags-"+key), value).Label(label).Width(width)
					if focus {
						in.AutoFocus()
					}
					submitted = submitted || in.Submitted()
				})
			}
			if !e.album {
				field("title", "Title", &e.form.title, 360, true)
				field("artists", "Artists", &e.form.artists, 360, false)
			}
			field("album", "Album", &e.form.album, 360, e.album)
			field("album-artists", "Album artist", &e.form.albumArtists, 360, false)
			ui.Text(c, "Separate several artists with ;").FontSize(k.captionSize()).TextColor(k.muted).Margin(0, 0, 0, 128)
			if !e.album {
				field("track", "Track", &e.form.track, 96, false)
				field("disc", "Disc", &e.form.disc, 96, false)
			}
			field("year", "Year", &e.form.year, 96, false)
			if !e.album {
				field("genre", "Genre", &e.form.genre, 360, false)
			}
			s.tagCover(c, e)
		})
		if e.problem != "" {
			ui.Text(c, e.problem).FontSize(k.smallSize()).TextColor(k.danger)
		}
	}, func() {
		if s.dialogButton(c, "tags-cancel", "cancel", false).Disabled(e.saving).Clicked() {
			e.open = false
		}
		label := "save"
		if e.saving {
			label = "saving…"
			if len(e.ids) > 1 {
				label = fmt.Sprintf("saving %d of %d…", e.done+1, len(e.ids))
			}
		}
		if (s.dialogButton(c, "tags-save", label, true).Disabled(e.saving).Clicked() || submitted) && !e.saving {
			s.saveTags()
		}
	})
}

// tagCover is the cover of the dialog of e: the one the files are to
// hold, a way to choose another, and one to take it away.
func (s *shell) tagCover(c *ui.Context, e *tagEditor) {
	k := &s.lk
	ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "Cover").FontSize(k.smallSize()).TextColor(k.muted).Width(112).Shrink(0)
		if e.cover != nil {
			s.cover(c, "tags-cover-art", []media.Image{{URL: e.coverURL}}, 72, false)
		} else {
			ui.Text(c, "None").FontSize(k.smallSize()).TextColor(k.muted)
		}
		ui.Column(c).Gap(8).Children(func() {
			if s.dialogButton(c, "tags-cover-choose", "choose an image…", false).Disabled(e.saving).Clicked() {
				s.chooseTagCover(e.seq)
			}
			if e.cover != nil && s.dialogButton(c, "tags-cover-remove", "remove the cover", false).Disabled(e.saving).Clicked() {
				e.cover, e.coverURL, e.coverSet = nil, "", true
			}
		})
	})
}

// chooseTagCover asks for an image file, beside the main thread, which the
// dialog of seq then shows as the cover to write: a JPEG or PNG image of
// at most 10 MB.
func (s *shell) chooseTagCover(seq int) {
	win := s.win
	go func() {
		paths, err := openFileDialog(mygo.OpenDialogOptions{Parent: win, Title: "Choose a cover",
			Filters: []mygo.FileFilter{{Name: "Images", Extensions: []string{"jpg", "jpeg", "png"}}}})
		if err == nil && len(paths) == 0 {
			return
		}
		var pic tagwrite.Picture
		url := ""
		if err == nil {
			pic, url, err = s.svc.LocalFiles.CoverOf(paths[0])
		}
		s.update(func() {
			e := &s.tagDialogs.edit
			if !e.open || e.seq != seq {
				return
			}
			if err != nil {
				e.problem = "Couldn't use that image: " + err.Error()
				return
			}
			e.cover, e.coverURL, e.coverSet, e.problem = &pic, url, true, ""
		})
	}()
}

// saveTags writes what the dialog changed, beside the main thread: the
// fields that differ from those it opened with.
func (s *shell) saveTags() {
	e := &s.tagDialogs.edit
	t, fields, err := e.form.tags(e.was)
	if err != nil {
		e.problem = err.Error()
		return
	}
	if e.coverSet {
		t.Cover, fields = e.cover, fields|tagwrite.Cover
	}
	if fields == 0 {
		e.open = false
		return
	}
	e.saving, e.done, e.problem = true, 0, ""
	seq, ids, name, album, from := e.seq, slices.Clone(e.ids), e.name, e.album, e.from
	go func() {
		n, err := s.svc.LocalFiles.EditTags(ids, t, fields, func(done int) {
			s.update(func() {
				if e := &s.tagDialogs.edit; e.seq == seq {
					e.done = done
				}
			})
		})
		s.update(func() {
			e := &s.tagDialogs.edit
			mine := e.open && e.seq == seq
			if mine {
				e.saving = false
			}
			if n > 0 {
				s.retagged(ids, from)
			}
			switch {
			case err != nil && mine:
				e.problem = sentenceOf(err.Error())
				if n > 0 {
					e.problem = fmt.Sprintf("Saved %d of %d songs. %s", n, len(ids), e.problem)
				}
			case err != nil:
				s.notes = append(s.notes, noticeOf(err))
			case album:
				if mine {
					e.open = false
				}
				s.notes = append(s.notes, notice("Saved "+name+": "+plural(n, "song")))
			default:
				if mine {
					e.open = false
				}
				s.notes = append(s.notes, notice("Saved the tags of "+name))
			}
		})
	}()
}

// sentenceOf is text as a sentence: its first letter capital, and a full
// stop at its end.
func sentenceOf(text string) string {
	if text == "" {
		return text
	}
	text = strings.ToUpper(text[:1]) + text[1:]
	if !strings.HasSuffix(text, ".") {
		text += "."
	}
	return text
}

// retagged has the pages shown take in the songs of ids whose tags
// changed: the albums and artists of this computer ask the library again,
// and the page of the album from, which they were on, shows the album
// they're on now, should it have gone.
func (s *shell) retagged(ids []string, from string) {
	for key := range s.shown {
		if strings.HasPrefix(key, "album:"+string(media.SourceFile)+":") || strings.HasPrefix(key, "artist:"+string(media.SourceFile)+":") {
			s.refetch(key)
		}
	}
	if from == "" || s.router.Path() != albumPath(from) {
		return
	}
	if slices.ContainsFunc(s.svc.LocalFiles.Albums(), func(a media.Album) bool { return a.ID == from }) {
		return
	}
	for _, t := range s.svc.LocalFiles.Tracks() {
		if t.ID == ids[0] && t.Album != nil {
			s.navigate(albumPath(t.Album.ID))
			return
		}
	}
}

// moveToAlbum opens Move to album… of t, once MixMusic knows it can
// change its file, beside the main thread, or tells why it can't.
func (s *shell) moveToAlbum(t media.Track) {
	go func() {
		err := s.svc.LocalFiles.Editable([]string{t.ID})
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			s.tagDialogs.seq++
			s.tagDialogs.move = albumMover{open: true, track: t, seq: s.tagDialogs.seq}
		})
	}()
}

// moveFound returns the albums of the library the search of mv finds, but
// the song's own: searched again only once the search or the library
// changed, not in each frame.
func (s *shell) moveFound(mv *albumMover) []media.Album {
	v := s.svc.LocalFiles.Version()
	if mv.found != nil && mv.shownQ == mv.query && mv.version == v {
		return mv.found
	}
	mv.shownQ, mv.version, mv.found = mv.query, v, []media.Album{}
	words := strings.Fields(fold(mv.query))
	for _, a := range s.svc.LocalFiles.Albums() {
		if mv.track.Album != nil && a.ID == mv.track.Album.ID {
			continue
		}
		text := fold(a.Title + " " + names(a.Artists))
		if !slices.ContainsFunc(words, func(w string) bool { return !strings.Contains(text, w) }) {
			mv.found = append(mv.found, a)
			if len(mv.found) == moveChoices {
				break
			}
		}
	}
	if mv.chosen != "" && !slices.ContainsFunc(mv.found, func(a media.Album) bool { return a.ID == mv.chosen }) {
		mv.chosen, mv.note = "", ""
	}
	return mv.found
}

// moveDialog shows Move to album…: a search of the library's albums, those
// it finds, of which the user chooses one, or a new album the search
// names.
func (s *shell) moveDialog(c *ui.Context) {
	mv := &s.tagDialogs.move
	if !mv.open {
		return
	}
	k := &s.lk
	var in ui.Element
	query := strings.TrimSpace(mv.query)
	s.dialog(c, &mv.open, dialogOptions{key: "move-album", title: "move to album", width: 520, alert: mv.saving}, func() {
		ui.Text(c, "Choose an album of this computer for "+mv.track.Title+", or name a new one. MixMusic writes it into the song's file.").
			FontSize(k.bodySize()).TextColor(k.muted)
		in = ui.TextInput(c.Key("move-album-find"), &mv.query).AutoFocus().Label("Album").
			Placeholder(k.sentence("find an album, or name a new one")).Width(400)
		found := s.moveFound(mv)
		ui.Column(c).Gap(4).Children(func() {
			for i, a := range found {
				b := s.buttonBase(c.Key("move-album-"+count(i))).Radius(k.radiusSm).Padding(8, 12).Justify(ui.Start).
					Cursor(ui.CursorPointer).Label(a.Title+", "+names(a.Artists)).Border(2, ui.Color{})
				switch {
				case a.ID == mv.chosen:
					b.Background(k.surface2).Border(2, k.accent)
				case b.Hovered():
					b.Background(k.surface2)
				}
				if b.Clicked() && !mv.saving {
					mv.chosen, mv.note = a.ID, s.moveNote(mv.track, a)
				}
				b.Children(func() {
					ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
						s.cover(c, "move-album-art-"+count(i), a.Artwork, 40, false)
						ui.Column(c).Grow(1).MinWidth(0).Children(func() {
							ui.Text(c, a.Title).FontSize(k.bodySize()).MaxLines(1)
							ui.Text(c, names(a.Artists)).FontSize(k.smallSize()).TextColor(k.muted).MaxLines(1)
						})
					})
				})
			}
		})
		if len(found) == 0 && query != "" {
			ui.Text(c, "No album of this computer is called that: Move makes a new one.").FontSize(k.smallSize()).TextColor(k.muted)
		}
		if mv.note != "" {
			ui.Text(c, mv.note).FontSize(k.smallSize()).TextColor(k.muted)
		}
		if mv.problem != "" {
			ui.Text(c, mv.problem).FontSize(k.smallSize()).TextColor(k.danger)
		}
	}, func() {
		if s.dialogButton(c, "move-album-cancel", "cancel", false).Disabled(mv.saving).Clicked() {
			mv.open = false
		}
		label := "move"
		if mv.chosen == "" {
			label = "move to a new album"
		}
		if mv.saving {
			label = "moving…"
		}
		ready := !mv.saving && (mv.chosen != "" || query != "")
		if (s.dialogButton(c, "move-album-ok", label, true).Disabled(!ready).Clicked() || in.Submitted()) && ready {
			s.moveSong()
		}
	})
}

// moveNote tells what moving t to the album a writes besides its album:
// of an album its folder groups, whose songs have no album artist, from
// another folder, the album artist they all take.
func (s *shell) moveNote(t media.Track, a media.Album) string {
	plan, err := s.svc.LocalFiles.MovePlan(t.ID, a.ID)
	if err != nil || len(plan.Others) == 0 {
		return ""
	}
	return "The songs of " + a.Title + " have no album artist, and are in another folder: they all take " +
		strings.Join(plan.AlbumArtists, ", ") + " as theirs, so that they stay together."
}

// moveSong moves the song of the dialog, beside the main thread, to the
// album chosen, or to a new one of the search's name.
func (s *shell) moveSong() {
	mv := &s.tagDialogs.move
	mv.saving, mv.problem = true, ""
	seq, t, chosen, title := mv.seq, mv.track, mv.chosen, strings.TrimSpace(mv.query)
	for _, a := range mv.found {
		if a.ID == chosen {
			title = a.Title
		}
	}
	from := ""
	if t.Album != nil {
		from = t.Album.ID
	}
	go func() {
		var err error
		if chosen != "" {
			err = s.svc.LocalFiles.MoveToAlbum(t.ID, chosen)
		} else {
			err = s.svc.LocalFiles.MoveToNewAlbum(t.ID, title)
		}
		s.update(func() {
			mv := &s.tagDialogs.move
			mine := mv.open && mv.seq == seq
			if mine {
				mv.saving = false
			}
			switch {
			case err != nil && mine:
				mv.problem = sentenceOf(err.Error())
			case err != nil:
				s.notes = append(s.notes, noticeOf(err))
			default:
				if mine {
					mv.open = false
				}
				s.retagged([]string{t.ID}, from)
				s.notes = append(s.notes, notice("Moved "+t.Title+" to "+title))
			}
		})
	}()
}

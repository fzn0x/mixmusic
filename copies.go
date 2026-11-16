package main

// Copies of one song on this computer (#340): files of the same title and
// artists, a few seconds apart at most, show once in Library › On this
// computer and in the search, and play their best copy. A song's menu
// tells how many copies it has, which open a dialog that shows them side
// by side, the one that plays marked, to play one, take one out of the
// song, or link another; Play this copy plays one of them. A file without
// copies links one in the same dialog. Track info shows them too.
// internal/localfiles groups them, and internal/app has what was played,
// liked and mixed of them follow the song.

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
)

// copiesView is the dialog of a song's copies: the track it opened of, a
// song's or a file's; the search of the songs to link, and what it found
// of the index's version; and while a link or an unlink is under way, and
// why the last failed.
type copiesView struct {
	open    bool
	track   media.Track
	query   string
	shownQ  string
	version uint64
	found   []media.Track
	busy    bool
	problem string
}

// copyChoices is how many songs the dialog offers to link at once.
const copyChoices = 6

// copyItems are the items of the menu of a song on this computer that
// tell of its copies: Play this copy, of each, and how many it has, which
// opens their dialog; of a file without copies, Link a copy…. The mini
// player's menus have the first only: its window is too small for the
// dialog.
func (s *shell) copyItems(m *ui.Menu, t media.Track) {
	if t.Source != media.SourceFile || t.IsEpisode() {
		return
	}
	song, ok := s.svc.LocalFiles.Song(t.ID)
	if ok && len(song.Copies) > 1 {
		labels := copyLabels(song.Copies)
		m.Submenu(locales.CopiesPlayThis(), func(m *ui.Menu) {
			for i, cp := range song.Copies {
				if m.Item(labels[i]).Chosen() {
					s.p.PlayTracks([]media.Track{cp.Track}, 0)
				}
			}
		})
		if s.mini.win == nil && m.Item(copiesLabel(len(song.Copies))).Chosen() {
			s.openCopies(t)
		}
		return
	}
	if s.mini.win != nil || s.missingFile(t) {
		return
	}
	if _, file := localfiles.PathOf(t.ID); (file || ok) && m.Item(locales.CopiesLink()).Chosen() {
		s.openCopies(t)
	}
}

// copiesLabel is the item of a song's menu that opens its copies: "2
// copies…".
func copiesLabel(n int) string { return locales.CopiesCount(n) }

// openCopies opens the dialog of the copies of t.
func (s *shell) openCopies(t media.Track) {
	s.copies = copiesView{open: true, track: t}
}

// copyFormat tells what a copy's audio is: its kind, and of a lossless
// one its bit depth and sample rate, else its bitrate, as "FLAC · 24-bit
// · 96 kHz" or "MP3 · 320 kbit/s · 44.1 kHz".
func copyFormat(cp localfiles.Copy) string {
	parts := []string{codecLabel(cp.Codec)}
	if cp.Lossless() && cp.BitDepth > 0 {
		parts = append(parts, strconv.Itoa(cp.BitDepth)+"-bit")
	}
	if !cp.Lossless() && cp.Bitrate > 0 {
		parts = append(parts, strconv.Itoa(cp.Bitrate)+" kbit/s")
	}
	if cp.SampleRate > 0 {
		parts = append(parts, kilohertz(cp.SampleRate))
	}
	if cp.Lossless() && cp.BitDepth == 0 && cp.SampleRate == 0 && cp.Bitrate > 0 {
		parts = append(parts, locales.CopiesAboutBitrate(cp.Bitrate))
	}
	return strings.Join(parts, " · ")
}

// codecLabel names a codec as people know its files: the PCM of WAV as
// WAV.
func codecLabel(codec string) string {
	switch codec {
	case "PCM":
		return "WAV"
	case "":
		return locales.CopiesAudio()
	}
	return codec
}

// copyPlace tells where a copy is: its album, else its folder's name.
func copyPlace(cp localfiles.Copy) string {
	if cp.Track.Album != nil && cp.Track.Album.Name != "" {
		return cp.Track.Album.Name
	}
	return filepath.Base(filepath.Dir(cp.Path))
}

// copyLabels are the labels of copies in a menu, each its format and its
// place, and its file's name where two would read alike.
func copyLabels(copies []localfiles.Copy) []string {
	base := make([]string, len(copies))
	for i, cp := range copies {
		base[i] = copyFormat(cp) + " · " + copyPlace(cp)
	}
	out := slices.Clone(base)
	for i, l := range base {
		if slices.Index(base, l) != i || slices.Index(base[i+1:], l) >= 0 {
			out[i] = l + " · " + filepath.Base(copies[i].Path)
		}
	}
	return out
}

// copiesFound returns the songs of this computer the search of the dialog
// finds to link, but those of song: searched again only once the search
// or the library changed, not in each frame.
func (s *shell) copiesFound(v *copiesView, own []string) []media.Track {
	ver := s.svc.LocalFiles.Version()
	q := strings.TrimSpace(v.query)
	if v.found != nil && v.shownQ == q && v.version == ver {
		return v.found
	}
	v.shownQ, v.version, v.found = q, ver, []media.Track{}
	if q == "" {
		return v.found
	}
	for _, t := range s.svc.LocalFiles.Search(q, 50).Tracks {
		if slices.Contains(own, t.ID) {
			continue
		}
		v.found = append(v.found, t)
		if len(v.found) == copyChoices {
			break
		}
	}
	return v.found
}

// copiesDialog shows the copies of a song side by side, best first, the
// one that plays marked, each with Play and Unlink, and a search of the
// songs of this computer to link another.
func (s *shell) copiesDialog(c *ui.Context) {
	v := &s.copies
	if !v.open {
		return
	}
	k := &s.lk
	song, isSong := s.svc.LocalFiles.Song(v.track.ID)
	copies := song.Copies
	title, own := v.track.Title, []string{v.track.ID}
	if isSong {
		title = song.Track.Title
		own = append(own, song.Track.ID)
		for _, cp := range copies {
			own = append(own, cp.Track.ID)
		}
	} else if p, ok := localfiles.PathOf(v.track.ID); ok {
		copies = []localfiles.Copy{{Track: v.track, Path: p, Best: true}}
	}
	s.dialog(c, &v.open, dialogOptions{key: "copies", title: locales.CopiesTitle(title), width: 600}, func() {
		about := locales.CopiesAbout()
		ui.Text(c, about).FontSize(k.smallSize()).TextColor(k.muted)
		ui.Column(c).Gap(8).Children(func() {
			for i, cp := range copies {
				s.copyRow(c, i, cp, len(copies) > 1)
			}
		})
		ui.Text(c, k.sentence(locales.CopiesLinkAnother())).FontSize(k.bodySize()).FontWeight(k.leadWeight()).Margin(8, 0, 0)
		ui.TextInput(c.Key("copies-find"), &v.query).Label(locales.CopiesFindLabel()).
			Placeholder(k.sentence(locales.CopiesFindPlaceholder())).Width(400)
		found := s.copiesFound(v, own)
		ui.Column(c).Gap(4).Children(func() {
			for i, t := range found {
				s.copyChoice(c, i, t)
			}
		})
		if len(found) == 0 && strings.TrimSpace(v.query) != "" {
			ui.Text(c, locales.CopiesNoMatch()).FontSize(k.smallSize()).TextColor(k.muted)
		}
		if v.problem != "" {
			ui.Text(c, v.problem).FontSize(k.smallSize()).TextColor(k.danger)
		}
	}, func() {
		if s.dialogButton(c, "copies-close", locales.CopiesClose(), true).AutoFocus().Clicked() {
			v.open = false
		}
	})
}

// copyRow is a copy of the dialog: what its audio is and where it is,
// marked when it's the one that plays, with Play, and Unlink while its
// song has others.
func (s *shell) copyRow(c *ui.Context, i int, cp localfiles.Copy, unlink bool) {
	k := &s.lk
	v := &s.copies
	ui.Row(c.Key("copy-"+count(i))).Gap(12).Padding(8, 12).Radius(k.radiusSm).Background(k.surface2).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, copyFormat(cp)).FontSize(k.bodySize()).SingleLine().MinWidth(0)
				if cp.Best && unlink {
					ui.Text(c, locales.CopiesPlays()).FontSize(k.captionSize()).TextColor(k.accent).Padding(0, 6).
						Border(1, k.accent).Radius(k.radiusSm).Shrink(0)
				}
				if cp.Linked {
					ui.Text(c, locales.CopiesLinkedByHand()).FontSize(k.captionSize()).TextColor(k.muted).Shrink(0)
				}
			})
			ui.Text(c, copyPlace(cp)).FontSize(k.smallSize()).TextColor(k.muted).SingleLine()
			ui.Text(c, cp.Path).FontSize(k.captionSize()).TextColor(k.muted).Selectable()
		})
		if s.button(c, "copy-play-"+count(i), locales.CopiesPlayButton(), false).Label(locales.CopiesPlayThis()).Clicked() {
			s.p.PlayTracks([]media.Track{cp.Track}, 0)
		}
		if unlink && s.button(c, "copy-unlink-"+count(i), locales.CopiesUnlinkButton(), false).Disabled(v.busy).
			Label(locales.CopiesUnlinkThis()).Clicked() && !v.busy {
			s.copyChange(func() error { return s.svc.LocalFiles.Unlink(cp.Track.ID) }, locales.CopiesUnlinked(copyPlace(cp)))
		}
	})
}

// copyChoice is a song the search found to link as a copy.
func (s *shell) copyChoice(c *ui.Context, i int, t media.Track) {
	k := &s.lk
	v := &s.copies
	b := s.buttonBase(c.Key("copy-link-"+count(i))).Radius(k.radiusSm).Padding(8, 12).Justify(ui.Start).
		Cursor(ui.CursorPointer).Label(locales.CopiesLinkLabel(t.Title, names(t.Artists))).Disabled(v.busy)
	if b.Hovered() {
		b.Background(k.surface2)
	}
	if b.Clicked() && !v.busy {
		from := v.track.ID
		s.copyChange(func() error { return s.svc.LocalFiles.Link([]string{from, t.ID}) }, locales.CopiesLinked(t.Title))
	}
	b.Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			s.cover(c, "copy-link-art-"+count(i), t.Artwork, 40, false)
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, t.Title).FontSize(k.bodySize()).MaxLines(1)
				line := names(t.Artists)
				if t.Album != nil && t.Album.Name != "" {
					line += " · " + t.Album.Name
				}
				ui.Text(c, line).FontSize(k.smallSize()).TextColor(k.muted).MaxLines(1)
			})
		})
	})
}

// copyChange links or unlinks copies with change, beside the main thread,
// as the index keeps its links in the data folder, then tells done, or in
// the dialog why it failed.
func (s *shell) copyChange(change func() error, done string) {
	v := &s.copies
	v.busy, v.problem = true, ""
	go func() {
		err := change()
		s.update(func() {
			v := &s.copies
			v.busy = false
			if err != nil {
				v.problem = sentenceOf(err.Error())
				return
			}
			v.query, v.found = "", nil
			s.notes = append(s.notes, notice(done))
		})
	}()
}

// trackCopies shows, in Track info, the copies of a song on this computer
// side by side, the one that plays marked.
func (s *shell) trackCopies(c *ui.Context, t media.Track, _ app.TrackInfo) {
	if t.Source != media.SourceFile {
		return
	}
	song, ok := s.svc.LocalFiles.Song(t.ID)
	if !ok || len(song.Copies) < 2 {
		return
	}
	k := &s.lk
	ui.Text(c, k.sentence(locales.CopiesInfoTitle())).FontSize(k.smallSize()).FontWeight(k.leadWeight()).Margin(8, 0, 0)
	rows := make([]infoRow, len(song.Copies))
	for i, cp := range song.Copies {
		label := locales.CopiesCopyLabel()
		if cp.Best {
			label = locales.CopiesPlays()
		}
		rows[i] = infoRow{label, copyFormat(cp) + " · " + copyPlace(cp) + "\n" + cp.Path, false}
	}
	s.infoRows(c, rows)
}

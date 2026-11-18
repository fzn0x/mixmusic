package main

// Pick the lyrics of the track that plays (#598). Above the lyrics, in the
// lyrics panel and on the now playing page, beside synced and plain text,
// and under "No lyrics found for this track.", a button names where the
// lyrics come from and opens the menu of every source that is on, each
// with what it answered (internal/app's lyricspick.go): picking one that
// found lyrics shows them at once, and one not asked yet asks it then.
// Automatic goes back to the lookup's choice. Search by title and artist…
// asks every source for another title and artist, and Edit lyrics…
// opens the lyrics shown as LRC, to correct them. A pick is kept for the
// track, and every window, the mini player and the notch's too, looks the
// lyrics up again as it changes, with their translation and romanization.

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
	"github.com/fzn0x/mixmusic/lyrics"
)

// lyricsPickView is what the dialogs of the lyrics' sources show.
type lyricsPickView struct {
	search lyricsSearch
	edit   lyricsEdit
}

// lyricsSearch is the dialog of Search by title and artist…: of which
// track, the title and artist to ask for, and what the sources found,
// once they answered the latest search, seq.
type lyricsSearch struct {
	open          bool
	track         media.Track
	title, artist string
	seq           int
	searching     bool
	found         []app.LyricsChoice
}

// lyricsEdit is the dialog of Edit lyrics…: of which track, the lyrics it
// started from, which give the edit their credit, their text as it is
// edited, and why it couldn't be kept.
type lyricsEdit struct {
	open    bool
	track   media.Track
	from    lyrics.Lyrics
	text    string
	saving  bool
	problem string
}

// lyricsSourceItem is an item of the menu of the lyrics' sources: its ID
// and label, what its source answered under it, whether it's the one
// picked, and what choosing it picks.
type lyricsSourceItem struct {
	id, label, detail string
	on                bool
	pick              app.LyricsPick
}

// lyricsSourceItems are the items of the menu of the lyrics' sources ch
// tells of: Automatic, the edit or the result of a search picked, then
// each source that is on, in the user's order.
func lyricsSourceItems(ch app.LyricsChoices) []lyricsSourceItem {
	items := []lyricsSourceItem{{id: "automatic", label: locales.LyricsSourceAutomatic(), on: ch.Picked.IsZero()}}
	if ch.Picked.Edited != nil {
		items = append(items, lyricsSourceItem{id: "edited", label: locales.LyricsSourceEdited(), on: true, pick: ch.Picked})
	}
	if f := ch.Search; f != nil {
		items = append(items, lyricsSourceItem{id: "searched", on: true, pick: f.Pick, detail: lyricsStateText(*f),
			label: locales.LyricsSourceSearched(lyricsChoiceName(*f), f.Pick.Title, f.Pick.Artist)})
	}
	for _, f := range ch.Sources {
		items = append(items, lyricsSourceItem{id: "source-" + f.Pick.Source + "-" + f.Pick.Caption, label: lyricsChoiceName(f),
			detail: lyricsStateText(f), on: f.On, pick: f.Pick})
	}
	return items
}

// lyricsChoiceName names the source of a choice: a caption track by its
// language.
func lyricsChoiceName(f app.LyricsChoice) string {
	name := lyricsSourceText()[f.Pick.Source].name
	if f.Pick.Caption != "" {
		return locales.LyricsSourceCaptionTrack(name, captionLanguageName(f.Pick.Caption, f.CaptionName))
	}
	return name
}

// captionLanguageName is the name of the language of YouTube's code, as
// "pt-BR", in the language the window shows: that of the language, or of
// its own without its region, as Translate lyrics to names them; else the
// name YouTube gave its caption track, or else the code.
func captionLanguageName(code, youtube string) string {
	base, _, _ := strings.Cut(code, "-")
	for _, want := range []string{code, base} {
		for _, l := range translateLanguages {
			if strings.EqualFold(l.code, want) {
				return l.name()
			}
		}
	}
	if youtube != "" {
		return youtube
	}
	return code
}

// lyricsStateText says what the source of a choice answered.
func lyricsStateText(f app.LyricsChoice) string {
	switch f.State {
	case app.LyricsNotAsked:
		return locales.LyricsSourceNotAsked()
	case app.LyricsNotFound:
		return locales.LyricsSourceNotFound()
	case app.LyricsUnreachable:
		return locales.LyricsSourceUnreachable()
	}
	l := f.Lyrics
	if l.Instrumental {
		return locales.LyricsSourceInstrumental()
	}
	timing := "plain"
	switch {
	case l.WordTimed():
		timing = "words"
	case l.Synced:
		timing = "lines"
	}
	shown := "no"
	if f.Showing {
		shown = "yes"
	}
	return locales.LyricsSourceFound(shown, timing, wordedLines(l))
}

// wordedLines counts the lines of lyrics with words.
func wordedLines(l lyrics.Lyrics) int {
	n := 0
	for _, line := range l.Lines {
		if strings.TrimSpace(line.Text) != "" {
			n++
		}
	}
	return n
}

// lyricsPickedName names what the lyrics of a track show from, as ch
// tells: Automatic, the edit, or the source picked.
func lyricsPickedName(ch app.LyricsChoices) string {
	switch {
	case ch.Picked.IsZero():
		return locales.LyricsSourceAutomatic()
	case ch.Picked.Edited != nil:
		return locales.LyricsSourceEdited()
	case ch.Search != nil:
		return lyricsChoiceName(*ch.Search)
	}
	for _, f := range ch.Sources {
		if f.On {
			return lyricsChoiceName(f)
		}
	}
	return lyricsChoiceName(app.LyricsChoice{Pick: ch.Picked})
}

// lyricsSourceButton is the choice of where the lyrics of t come from,
// above them or under why there are none: its label, and a button naming
// the choice, which opens the menu of the sources. ly are the lyrics that
// show, nil for none, which Edit lyrics… starts from.
func (s *shell) lyricsSourceButton(c *ui.Context, t media.Track, ly *lyrics.Lyrics, where string) {
	k := &s.lk
	name := lyricsPickedName(s.svc.Catalog.LyricsChoices(t))
	ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
		ui.Text(c, k.sentence(locales.LyricsSource())).FontSize(k.smallSize())
		b := s.buttonBase(c.Key("lyrics-pick-" + where)).Height(34).PaddingStart(14).PaddingEnd(10).Gap(4).Radius(k.round(34)).
			Label(locales.LyricsSourceLabel(name)).Cursor(ui.CursorPointer)
		switch {
		case b.Pressed():
			b.Background(k.press)
		case b.Hovered():
			b.Background(k.hover)
		case k.tonal:
			b.Background(k.surface2)
		}
		if !k.tonal {
			b.Border(2, k.outline)
		}
		b.Children(func() {
			ui.Text(c, name).FontSize(k.smallSize()).NoWrap()
			ui.Icon(c, lineIcon(c, "chevronDown")).FontSize(14).TextColor(k.muted)
		})
		b.Menu(func(m *ui.Menu) { s.lyricsSourceMenu(m, t, ly) })
	})
}

// lyricsSourceMenu is the menu of the sources of the lyrics of t, which ly
// are when they show.
func (s *shell) lyricsSourceMenu(m *ui.Menu, t media.Track, ly *lyrics.Lyrics) {
	for _, it := range lyricsSourceItems(s.svc.Catalog.LyricsChoices(t)) {
		item := m.Item(it.label).ID(it.id).Radio(it.on)
		if it.detail != "" {
			item.Subtitle(it.detail)
		}
		if item.Chosen() && !it.on {
			s.pickLyrics(t, it.pick, nil)
		}
	}
	m.Separator()
	if m.Item(locales.LyricsSourceSearch()).ID("search").Chosen() {
		s.openLyricsSearch(t)
	}
	if m.Item(locales.LyricsSourceEdit()).ID("edit").Chosen() {
		var from lyrics.Lyrics
		if ly != nil {
			from = *ly
		}
		s.lyricsView.pick.edit = lyricsEdit{open: true, track: t, from: from, text: from.LRC()}
	}
}

// pickLyrics keeps pick for the lyrics of t, beside the main thread: the
// windows hear of it (followLyricsPicks). found are the lyrics the pick's
// source found, which then show at once.
func (s *shell) pickLyrics(t media.Track, pick app.LyricsPick, found *lyrics.Lyrics) {
	go func() {
		if err := s.svc.Catalog.PickLyrics(t, pick, found); err != nil {
			s.update(func() { s.notes = append(s.notes, noticeOf(err)) })
		}
	}()
}

// followLyricsPicks has the lyrics of a track looked up again as the
// listener picks them, in this window or another, until it closes.
func (s *shell) followLyricsPicks() func() {
	return s.a.OnLyricsPicked(func(id string, ready bool) { s.update(func() { s.lyricsRepicked(id, ready) }) })
}

// lyricsRepicked looks the lyrics of the track id up again, their
// translation and romanization too: those shown stay until the new ones
// come when they are ready, as lyrics a source found are. The lines
// chosen to share go.
func (s *shell) lyricsRepicked(id string, ready bool) {
	key := "lyrics:" + id
	if ready {
		s.refetch(key)
	} else {
		s.forget(key)
	}
	for k := range s.loads {
		if strings.HasPrefix(k, "lyrics-translation:"+id+":") || k == "lyrics-romaji:"+id || strings.HasPrefix(k, "lyrics-romanized:"+id+":") {
			delete(s.loads, k)
		}
	}
	if s.lyricPick.track == id {
		s.lyricPick = lyricsPick{}
	}
	if s.lyricsView.track == id {
		s.lyricsView.track = ""
	}
}

// openLyricsSearch opens Search by title and artist… for t, with the title
// and artist its lyrics are looked up by, which it asks for at once.
func (s *shell) openLyricsSearch(t media.Track) {
	v := &s.lyricsView.pick.search
	title, artist := s.svc.Catalog.LyricsSearchQuery(t)
	*v = lyricsSearch{open: true, track: t, title: title, artist: artist, seq: v.seq}
	s.searchLyrics()
}

// searchLyrics asks every source for the title and artist of the dialog,
// beside the main thread, unless one is missing.
func (s *shell) searchLyrics() {
	v := &s.lyricsView.pick.search
	if strings.TrimSpace(v.title) == "" || strings.TrimSpace(v.artist) == "" {
		return
	}
	v.seq++
	seq, t, title, artist := v.seq, v.track, v.title, v.artist
	v.searching, v.found = true, nil
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		found := s.svc.Catalog.SearchLyrics(ctx, t, title, artist)
		s.update(func() {
			if v := &s.lyricsView.pick.search; v.seq == seq {
				v.searching, v.found = false, found
			}
		})
	}()
}

// lyricsPickDialogs shows Search by title and artist… and Edit lyrics…
// while they are open.
func (s *shell) lyricsPickDialogs(c *ui.Context) {
	s.lyricsSearchDialog(c)
	s.lyricsEditDialog(c)
}

// lyricsSearchDialog shows Search by title and artist…: the title and
// artist to ask for, and every source's answer, each found with its
// credit, how it is timed, its lines and its first lines.
func (s *shell) lyricsSearchDialog(c *ui.Context) {
	v := &s.lyricsView.pick.search
	if !v.open {
		return
	}
	k := &s.lk
	submitted := false
	s.dialog(c, &v.open, dialogOptions{key: "lyrics-search", title: locales.LyricsSearchTitle(), width: 600}, func() {
		ui.Text(c, locales.LyricsSearchAbout()).FontSize(k.bodySize()).TextColor(k.muted)
		ui.Column(c).Gap(12).Children(func() {
			field := func(key, label string, value *string, focus bool) {
				ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
					ui.Text(c, label).FontSize(k.smallSize()).TextColor(k.muted).Width(96).Shrink(0)
					in := ui.TextInput(c.Key("lyrics-search-"+key), value).Label(label).Grow(1).MinWidth(0)
					if focus {
						in.AutoFocus()
					}
					submitted = submitted || in.Submitted()
				})
			}
			field("title", locales.LyricsSearchSongTitle(), &v.title, true)
			field("artist", locales.LyricsSearchArtist(), &v.artist, false)
		})
		if v.searching {
			ui.Text(c, locales.LyricsSearchLooking()).FontSize(k.smallSize()).TextColor(k.muted)
			return
		}
		ui.Column(c).Gap(8).Children(func() {
			for _, f := range v.found {
				s.lyricsFoundRow(c, f)
			}
		})
	}, func() {
		if s.dialogButton(c, "lyrics-search-cancel", locales.LibraryCancel(), false).Clicked() {
			v.open = false
		}
		ready := strings.TrimSpace(v.title) != "" && strings.TrimSpace(v.artist) != ""
		if s.dialogButton(c, "lyrics-search-go", locales.LyricsSearchGo(), true).Disabled(!ready).Clicked() {
			submitted = true
		}
	})
	if submitted {
		s.searchLyrics()
	}
}

// lyricsFoundRow is a source's answer to a search: its credit, as the
// lyrics panel writes it, how its lyrics are timed and their lines, and
// their first lines, with a button that shows them; or its name, and why
// it found none.
func (s *shell) lyricsFoundRow(c *ui.Context, f app.LyricsChoice) {
	k := &s.lk
	found := f.State == app.LyricsFound
	ui.Column(c.Key("lyrics-found-"+f.Pick.Source)).Gap(4).Padding(12, 16).Radius(k.radiusSm).Background(k.surface2).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				if found && f.Lyrics.Source != "" {
					ui.RichText(c).FontSize(k.bodySize()).Children(func() { s.creditParts(c, f.Lyrics) })
				} else {
					ui.Text(c, lyricsChoiceName(f)).FontSize(k.bodySize())
				}
				ui.Text(c, lyricsStateText(f)).FontSize(k.smallSize()).TextColor(k.muted)
			})
			if found && !f.Lyrics.Instrumental {
				use := s.dialogButton(c, "lyrics-found-use-"+f.Pick.Source, locales.LyricsSearchUse(), false).
					Label(locales.LyricsSearchUseFrom(lyricsChoiceName(f)))
				if use.Clicked() {
					s.lyricsView.pick.search.open = false
					l := f.Lyrics
					s.pickLyrics(s.lyricsView.pick.search.track, f.Pick, &l)
				}
			}
		})
		if !found {
			return
		}
		n := 0
		for _, line := range f.Lyrics.Lines {
			if n == 3 {
				break
			}
			if text := strings.TrimSpace(line.Text); text != "" {
				ui.Text(c, text).FontSize(k.smallSize()).TextColor(k.muted).SingleLine().MinWidth(0)
				n++
			}
		}
	})
}

// lyricsEditDialog shows Edit lyrics…: the lyrics as LRC, in a box to
// correct them, kept for the track with Save.
func (s *shell) lyricsEditDialog(c *ui.Context) {
	e := &s.lyricsView.pick.edit
	if !e.open {
		return
	}
	k := &s.lk
	s.dialog(c, &e.open, dialogOptions{key: "lyrics-edit", title: locales.LyricsEditTitle(), width: 640, alert: e.saving}, func() {
		ui.Text(c, locales.LyricsEditAbout()).FontSize(k.bodySize()).TextColor(k.muted)
		ui.TextArea(c.Key("lyrics-edit-text"), &e.text).Height(320).Label(locales.LyricsEditLabel()).AutoFocus()
		if e.problem != "" {
			ui.Text(c, e.problem).FontSize(k.smallSize()).TextColor(k.danger)
		}
	}, func() {
		if s.dialogButton(c, "lyrics-edit-cancel", locales.LibraryCancel(), false).Disabled(e.saving).Clicked() {
			e.open = false
		}
		if s.dialogButton(c, "lyrics-edit-save", locales.LibrarySave(), true).Disabled(e.saving).Clicked() {
			s.saveLyricsEdit()
		}
	})
}

// saveLyricsEdit keeps the lyrics edited for their track, beside the main
// thread, and closes the dialog; lyrics of no words stay in it, saying
// why.
func (s *shell) saveLyricsEdit() {
	e := &s.lyricsView.pick.edit
	e.saving, e.problem = true, ""
	t, text, from := e.track, e.text, e.from
	go func() {
		_, err := s.svc.Catalog.EditLyrics(t, text, from)
		s.update(func() {
			e := &s.lyricsView.pick.edit
			if e.track.ID != t.ID {
				return
			}
			e.saving = false
			switch {
			case errors.Is(err, app.ErrLyricsEmpty):
				e.problem = locales.LyricsEditEmpty()
			case err != nil:
				e.problem = errorText(err)
			default:
				e.open = false
			}
		})
	}()
}

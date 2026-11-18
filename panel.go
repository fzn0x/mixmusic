package main

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/locales"
	"github.com/fzn0x/mixmusic/lyrics"
)

// sidePanel is the panel at the end of the main pane, its right or, right to
// left, its left: the queue, or the lyrics.
func (s *shell) sidePanel(c *ui.Context) {
	k := &s.lk
	panel := ui.Column(c).Width(s.panelWidth(c)).Shrink(0).Background(k.surface)
	if k.floating {
		panel.Margin(0, 0, floatMargin).MarginEnd(floatMargin).Radius(k.radius).Clip()
	} else {
		panel.BorderStartWidth(1).BorderColor(k.border)
	}
	panel.Children(func() {
		if s.panel == "queue" {
			s.queuePanel(c)
		} else {
			s.lyricsPanel(c)
		}
	})
}

// panelHead titles the panel, with its actions and a close button.
func (s *shell) panelHead(c *ui.Context, title string, actions func()) {
	k := &s.lk
	title = k.words(title)
	size := k.pivotSize()
	if k.headings != headingsLight {
		size = 22 * k.scale
	}
	ui.Row(c).Padding(16, 16, 12).PaddingStart(24).Gap(8).Shrink(0).Children(func() {
		ui.Text(c, title).FontSize(size).FontWeight(k.titleWeight()).Font(k.display).LetterSpacing(k.tracking(-0.02, size)).
			SingleLine().Grow(1).MinWidth(0)
		if actions != nil {
			actions()
		}
		if s.iconButton(c, "close-panel", "close", locales.PanelClose(title), false, 16).Clicked() {
			s.showPanel("")
		}
	})
}

// queuePanel lists the track playing, and what plays next.
func (s *shell) queuePanel(c *ui.Context) {
	k := &s.lk
	q, at := s.st.Queue, s.st.Index
	upcoming := 0
	if at+1 < len(q) {
		upcoming = len(q) - at - 1
	}
	s.panelHead(c, locales.PanelUpNext(), func() {
		if upcoming > 0 {
			clear := s.buttonBase(c.Key("clear-queue")).Label(locales.PanelClearQueue()).Cursor(ui.CursorPointer)
			if clear.Clicked() {
				s.p.ClearUpcoming()
			}
			clear.Children(func() { ui.Text(c, k.words(locales.PanelClear())).FontSize(k.smallSize()).TextColor(k.accent) })
		}
	})
	ui.Scroll(c).Grow(1).MinHeight(0).Padding(0, 12, 24).Children(func() {
		if cur, ok := s.st.Current(); ok {
			ui.Text(c, k.words(locales.NowPlayingTitle())).FontSize(k.smallSize()).TextColor(k.accent).Padding(0, 8, 6)
			s.queueRow(c, at, cur, true)
		}
		if upcoming == 0 {
			text := locales.PanelQueueEmpty()
			if at < 0 {
				text = locales.PanelQueueStart()
			}
			ui.Text(c, text).FontSize(k.bodySize()).TextColor(k.muted).Padding(16, 8)
			return
		}
		// The queue of a long playlist holds thousands of entries: those
		// near the view are built, and the others take their room.
		list := ui.Column(c.Key("queue-upcoming")).Margin(12, 0, 0)
		from, to := s.nearRows(c, "queue-upcoming", list, upcoming, queueRowHeight)
		list.Children(func() {
			if from > 0 {
				ui.Box(c).Height(float32(from) * queueRowHeight)
			}
			for i := at + 1 + from; i < at+1+to; i++ {
				s.queueRow(c, i, q[i], false)
			}
			if to < upcoming {
				ui.Box(c).Height(float32(upcoming-to) * queueRowHeight)
			}
		})
	})
}

// queueRowHeight is the height of an entry of the queue.
const queueRowHeight = 56

// queueRow is an entry of the queue; the current one floods with the
// accent, and one the queue passes over is muted.
func (s *shell) queueRow(c *ui.Context, i int, it player.Item, current bool) {
	k := &s.lk
	t := it.Track
	row := ui.Row(c.Key("q-"+it.UID)).Height(queueRowHeight).Padding(0, 8).Gap(10).Radius(k.radiusSm).Cursor(ui.CursorDefault)
	ink, sub := k.text, k.muted
	skipped := it.Skip != "" && !current
	missing := s.missingFile(t)
	if skipped || missing {
		ink = k.muted
	}
	switch {
	case current:
		row.Background(k.selected)
		ink, sub = k.selectedText, k.selectedSub
	case row.Hovered():
		row.Background(k.hover)
	}
	row.ContextMenu(func(m *ui.Menu) { s.trackMenuAt(m, t, i) })
	if row.DoubleClicked() && !current {
		s.p.PlayAt(i)
	}
	if !current {
		s.reorderable(row, it.UID, i)
	}
	hovered := row.Hovered()
	row.Children(func() {
		art := s.buttonBase(c.Key("q-art-"+it.UID)).Size(40, 40).Shrink(0).Label(locales.PanelPlayTrack(t.Title)).Cursor(ui.CursorPointer)
		if art.Clicked() {
			if current {
				s.p.Toggle()
			} else {
				s.p.PlayAt(i)
			}
		}
		art.Children(func() {
			s.cover(c, "q-cover-"+it.UID, t.Artwork, 40, false)
			if hovered && !current {
				ui.Box(c).Absolute().Top(0).Left(0).Size(40, 40).Background(k.scrim).Radius(k.scrimRadius()).Center().Children(func() {
					ui.Icon(c, icon("play")).FontSize(14).TextColor(k.text)
				})
			}
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			s.queueTitle(c, t, skipped && it.Skip == player.SkipExplicit, ink, sub)
			ui.Row(c).Gap(6).MinWidth(0).Children(func() {
				s.sourceMark(c, t.Source, current)
				s.downloadMark(c, "q-dl-"+it.UID, t, current)
				line := names(t.Artists)
				if it.Radio {
					line = locales.PanelRadioLine(line)
				}
				switch {
				case missing:
					line = locales.PanelFileNotFound()
				case skipped && it.Skip == player.SkipMadeWithAI:
					// A click still plays it.
					line = locales.PanelMadeWithAI(line)
				case skipped && it.Skip == player.SkipExplicit:
					line = locales.PanelHiddenNote(line)
				case skipped:
					line = locales.PanelSkipped(line)
				}
				ui.Text(c, line).FontSize(k.smallSize()).TextColor(sub).SingleLine().MinWidth(0)
			})
		})
		if t.DurationMs > 0 {
			ui.Text(c, clockLength(ms(t.DurationMs))).FontSize(k.captionSize()).TextColor(sub).FontFeatures("tnum")
		}
		if !current && hovered {
			if s.iconButton(c, "q-remove-"+it.UID, "close", locales.PanelRemoveTrack(t.Title), false, 14).Clicked() {
				s.p.RemoveAt(i)
			}
		}
	})
}

// queueDrag is an entry of the queue dragged, by its UID: what plays may
// move it meanwhile.
type queueDrag struct{ uid string }

// reorderable lets the user drag the upcoming entry of uid, at i, onto
// another, which takes it in its place, as the web interface's queue does:
// a line of the accent over an entry shows where it would go.
func (s *shell) reorderable(row ui.Element, uid string, i int) {
	row.Drag(queueDrag{uid})
	if row.Dragging() {
		row.Opacity(0.4)
	}
	if _, over := ui.DragOver[queueDrag](row); over {
		row.InsetShadow(0, 2, 0, 0, s.lk.accent)
	}
	if d, ok := ui.Drop[queueDrag](row); ok {
		from := slices.IndexFunc(s.st.Queue, func(it player.Item) bool { return it.UID == d.uid })
		if from >= 0 && from != i {
			s.p.Move(from, i)
		}
	}
}

// lyricsPanel shows the lyrics of the track playing, or the details of an
// episode.
func (s *shell) lyricsPanel(c *ui.Context) {
	// An episode has details, not lyrics: nothing to show full screen.
	var actions func()
	if cur, ok := s.st.Current(); ok && !cur.Track.IsEpisode() {
		actions = func() {
			if s.iconButton(c, "lyrics-stage", "expand", locales.LyricsFullScreen(), false, 16).Clicked() {
				s.toggleStage()
			}
		}
	}
	s.panelHead(c, s.lyricsTitle(), actions)
	s.lyrics(c, false)
}

// lyricsState is the lyrics views' own: the track they show, and where
// the panel's and the now playing page's lyrics scrolled, which show at
// once. Whether synced lyrics show as plain text is a setting, as are
// their translation and romanization.
type lyricsState struct {
	track       string
	panel, page lyricsScroll
	// stage is the full-screen lyrics' (lyricsstage.go).
	stage lyricsScroll
	// previewAt is when the preview's made-up song started, in Settings
	// (lyricspreview.go), and previewBall what its line tells its ball.
	previewAt   time.Time
	previewBall ballRig
	// system is the system's language, and languages the choices of the
	// language lyrics translate to, once asked for.
	system    string
	languages []string
	// romaji is what they know of the romaji of Japanese lyrics.
	romaji romajiView
	// local is what they know of the lyrics romanized on the computer.
	local romanizeView
}

// lyricsScroll is where a lyrics view scrolled, the line it last scrolled
// to, and when the user last scrolled it.
type lyricsScroll struct {
	list     ui.ListState
	scrolled int
	userAt   time.Time
}

// lyricsPause is how long lyrics the user scrolled don't follow the song.
const lyricsPause = 4 * time.Second

// lyrics follows the track playing: synchronized lines light up as they
// are sung. large sets them at the size of the now playing page.
func (s *shell) lyrics(c *ui.Context, large bool) { s.lyricsWith(c, lyricsOptions{large: large}) }

// lyricsOptions say how lyricsWith shows the lyrics. Its zero value is the
// panel's.
type lyricsOptions struct {
	// large sets them at the size of the now playing page.
	large bool
	// stage, when set, shows them as the full-screen lyrics do
	// (lyricsstage.go), each line drawn in the style: with no switches,
	// share bar or note above them, and no way to choose lines to share.
	stage *lyricStyle
}

// lyricsWith is lyrics for the options o.
func (s *shell) lyricsWith(c *ui.Context, o lyricsOptions) {
	k := &s.lk
	ls := &s.lyricsView
	view, where := &ls.panel, "panel"
	switch {
	case o.stage != nil:
		view, where = &ls.stage, "stage"
	case o.large:
		view, where = &ls.page, "page"
	}
	cur, ok := s.st.Current()
	if !ok {
		s.lyricsEmpty(c, o, locales.LyricsPlayToSee())
		return
	}
	t := cur.Track
	if t.IsEpisode() {
		// An episode has its details in their place (episodedetails.go); the
		// full-screen lyrics close for it (lyricsstage.go).
		if o.stage == nil {
			s.episodeDetails(c, t, o.large)
		}
		return
	}
	l := fetch(s, "lyrics:"+t.ID, func(ctx context.Context) (lyrics.Lyrics, error) { return s.svc.Catalog.Lyrics(ctx, t) })
	switch {
	case !l.done:
		s.lyricsEmpty(c, o, locales.LyricsLooking())
		return
	case l.err != nil:
		s.lyricsEmpty(c, o, errorText(l.err))
		// A track without lyrics stays so; a failure may pass. The
		// full-screen lyrics refresh with the page (F5) instead.
		if !errors.Is(l.err, lyrics.ErrNotFound) && o.stage == nil {
			ui.Row(c).Padding(0, 24, 8).Children(func() {
				if s.button(c, "lyrics-retry-"+where, locales.WidgetsTryAgain(), false).Clicked() {
					s.forget("lyrics:" + t.ID)
				}
			})
		}
		return
	case l.value.Instrumental:
		s.lyricsEmpty(c, o, locales.LyricsInstrumental())
		return
	}
	ly := l.value
	if ls.track != t.ID {
		ls.track, ls.panel.scrolled, ls.page.scrolled, ls.stage.scrolled = t.ID, -1, -1, -1
	}
	synced := ly.Synced && !s.set.LyricsPlain
	x := s.lyricsChoicesWith(c, t, ly, where, lyricsChoiceOptions{quiet: o.stage != nil})
	if o.stage == nil {
		s.lyricsShareBar(c, t, ly, where)
		if ly.Note != "" {
			s.lyricsNote(c, ly.Note)
		}
	}
	active := -1
	if synced {
		pos := s.st.Position + lyricsLead(s.st.Speed)
		for i, line := range ly.Lines {
			if ms(line.TimeMs) <= pos {
				active = i
			} else {
				break
			}
		}
	}
	size, sub := k.bodySize(), k.smallSize()
	if o.large {
		size, sub = k.size(2), k.bodySize()
	}
	// The full-screen lyrics glide to the line sung when the song goes on to
	// it from a line near; from afar, or at first, they jump (lyricsstage.go).
	glide := false
	if d := active - view.scrolled; o.stage != nil && active >= 0 && view.scrolled >= 0 && d != 0 && d >= -4 && d <= 4 && time.Since(view.userAt) > lyricsPause {
		glide = true
	}
	// The ball, and the room each line keeps for it, are for synced lyrics with
	// words timed: the others have no ball to keep room for.
	ball := o.stage != nil && s.set.LyricsBall && synced && slices.ContainsFunc(ly.Lines, func(l lyrics.Line) bool { return len(l.Words) > 0 })
	n := len(ly.Lines) + 1
	list := ui.List(c.Key("lyrics-"+where+"-"+t.ID), &view.list, n, func(i int) {
		if i == len(ly.Lines) {
			if o.stage != nil {
				s.stageCredit(c, ly)
			} else {
				s.lyricsCredit(c, ly)
			}
			return
		}
		line := ly.Lines[i]
		text, roman, meaning := x.line(i, line.Text)
		if text == "" {
			text = "♪"
		}
		ink := k.text
		switch {
		case synced && i == active:
			ink = k.accent
		case synced && i < active:
			ink = k.muted
		}
		// The line sung now lights word by word, when its words have times
		// (lyricswords.go), and so does its romanization made on the
		// computer (lyricsromanize.go).
		var words, romanWords []lyrics.Word
		if synced && i == active {
			words, romanWords = x.words(i, line, text, roman)
		}
		if o.stage != nil {
			// The ball goes on from the line sung to the next, if its words have
			// times.
			var next ballNext
			if ball && synced && i == active && i+1 < len(ly.Lines) {
				nl := ly.Lines[i+1]
				ntext, nroman, _ := x.line(i+1, nl.Text)
				if ntext == "" {
					ntext = "♪"
				}
				if shown, _ := x.words(i+1, nl, ntext, nroman); len(shown) > 0 {
					// The ball lands on the word that starts first: a second voice
					// may be listed after the main one's words, and start before them.
					first := shown[0]
					for _, w := range shown[1:] {
						if w.StartMs < first.StartMs {
							first = w
						}
					}
					next = ballNext{ok: true, text: ntext, word: first}
				}
			}
			s.stageLine(c, stageLine{style: *o.stage, index: i, active: active, line: line, text: text, roman: roman, meaning: meaning,
				words: words, synced: synced, reading: time.Since(view.userAt) <= lyricsPause, glide: glide, ball: ball, next: next})
			return
		}
		var p ui.Element
		if roman == "" && meaning == "" {
			p = s.lyricsText(c, text, size, ink, words, 24, 4)
		} else {
			// The line, then in Latin letters, then its translation, which
			// screen readers read in this order too.
			p = ui.Column(c).Padding(4, 24).Gap(2).Children(func() {
				s.lyricsText(c, text, size, ink, words, 0, 0)
				if roman != "" {
					s.lyricsRoman(c, roman, sub, romanWords)
				}
				if meaning != "" {
					ui.Text(c, meaning).FontSize(sub).TextColor(k.muted)
				}
			})
		}
		// A line plays the song from there, once it plays: a song that
		// loads isn't moved. While lines are chosen to share, it is chosen.
		if s.lyricPick.picking(t.ID) {
			s.pickLine(p, i, text)
		} else if synced && !s.st.Loading {
			p.Cursor(ui.CursorPointer)
			if p.Clicked() {
				s.p.Seek(ms(line.TimeMs))
			}
		}
	}).Grow(1).MinHeight(0)
	if o.stage != nil {
		// Room above the first line and below the last, so that the line sung
		// can sit in the middle of the window even at the song's ends.
		_, wh := c.Size()
		list.Padding(0.38*wh, 0)
	}
	// Lyrics the user scrolls, to read back say, stop following the song
	// for a while, and never pull the page they're on.
	list.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind == ui.InputScroll || ev.Kind == ui.InputPointerMove && ev.Button >= 0 {
			view.userAt = time.Now()
		}
		return s.scrollsInside(ev)
	})
	if synced && active >= 0 && active != view.scrolled && time.Since(view.userAt) > lyricsPause {
		view.scrolled = active
		// The line sung asked the full-screen lyrics to glide to it: they
		// don't jump there too.
		if !s.stage.glided {
			view.list.ScrollTo(active, ui.Center)
		}
	}
	s.stage.glided = false
}

// lyricsLead is how far ahead of the position synced lyrics light up, at
// the speed the track plays at: the position comes every 250 ms, in which
// the track moves on 250 ms at its speed, so a line lights up no later
// than it starts.
func lyricsLead(speed float64) time.Duration {
	if speed <= 0 {
		speed = 1
	}
	return time.Duration(float64(250*time.Millisecond) * speed)
}

// lyricsCredit names where lyrics come from, and who gave them to it, as
// creditParts says.
func (s *shell) lyricsCredit(c *ui.Context, ly lyrics.Lyrics) {
	k := &s.lk
	ui.RichText(c).FontSize(k.captionSize()).TextColor(k.muted).Padding(16, 24).Children(func() {
		s.creditParts(c, ly)
	})
}

// creditParts writes the credit of the lyrics ly into a rich text: where
// they come from, linked to it for sources whose licence asks for the
// link, and who gave them to it, linked to their page, for sources that
// ask for that too, as Unison does: "lyrics from Unison · submitted by
// <name>".
func (s *shell) creditParts(c *ui.Context, ly lyrics.Lyrics) {
	k := &s.lk
	source := lyricsSourceName(ly.Source)
	credit := locales.LyricsCreditFrom(mark(0, source))
	if ly.By != "" {
		credit = locales.LyricsCreditFromBy(mark(0, source), mark(1, ly.By))
	}
	// The source and the one who gave them link to their pages, where the
	// source asks for it.
	linked := func(url string) func(text string) {
		return func(text string) {
			if url == "" {
				ui.Text(c, text)
			} else {
				s.linkOf(c, text, url)
			}
		}
	}
	inlineMessage(c, k.sentence(credit), linked(ly.SourceURL), linked(ly.ByURL))
}

func (s *shell) emptyIn(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(s.lk.bodySize()).TextColor(s.lk.muted).Padding(8, 24)
}

// trackMenu is the menu of a track: play it, queue it, start its radio,
// like it, keep it in a mix, go to its artist or album, open it on its
// service.
func (s *shell) trackMenu(m *ui.Menu, t media.Track) { s.trackMenuAt(m, t, -1) }

// trackMenuAt is the menu of a track of the queue, the entry at queued, or
// of another track when queued is -1: in the queue, Play plays that entry,
// and the entry can be taken out, but the current one.
func (s *shell) trackMenuAt(m *ui.Menu, t media.Track, queued int) {
	one := []media.Track{t}
	switch {
	case queued >= 0:
		if m.Item(locales.PlayerPlay()).Chosen() {
			s.p.PlayAt(queued)
		}
		if queued != s.st.Index && m.Item(locales.PanelPlayNext()).Chosen() {
			s.p.PlayNextAt(queued)
			s.notes = append(s.notes, notice(locales.PanelPlaysNext(t.Title)))
		}
		if queued != s.st.Index && m.Item(locales.PanelRemoveFromQueue()).Chosen() {
			s.p.RemoveAt(queued)
		}
	default:
		if m.Item(locales.PlayerPlay()).Chosen() {
			s.p.PlayTracks(one, 0)
		}
		if m.Item(locales.PanelPlayNext()).Chosen() {
			s.playNextTrack(t)
		}
		if m.Item(locales.PanelAddToQueue()).Chosen() {
			s.queueTrack(t)
		}
		s.roomSongItem(m, t)
	}
	if !t.IsEpisode() && t.Source != media.SourceFile && m.Item(locales.PanelStartRadio()).Chosen() {
		s.startRadio(t)
	}
	m.Separator()
	liked := s.a.IsLiked(t.ID)
	label := locales.PanelLike()
	if liked {
		label = locales.PanelUnlike()
	}
	if m.Item(label).Chosen() {
		s.setLiked(t, !liked)
	}
	disliked := s.a.IsDisliked(t.ID)
	label = locales.PlayerDislike()
	if disliked {
		label = locales.PanelRemoveDislike()
	}
	if m.Item(label).Chosen() {
		s.setDisliked(t, !disliked)
	}
	s.librarySongItem(m, t)
	s.addToMenus(m, one)
	s.addToSpotifyMenu(m, one)
	if !t.IsEpisode() {
		s.downloadItems(m, t)
	}
	s.localItems(m, t)
	s.copyItems(m, t)
	s.exportItem(m, t)
	s.trackInfoItem(m, t)
	m.Separator()
	for _, a := range t.Artists {
		if a.ID != "" {
			if m.Item(locales.PanelGoToArtist(a.Name)).Chosen() {
				s.navigate(artistPath(a.ID))
			}
			break
		}
	}
	if t.Album != nil && t.Album.ID != "" {
		if m.Item(locales.PanelGoToAlbum()).Chosen() {
			s.navigate(albumPath(t.Album.ID))
		}
	}
	s.episodeItems(m, t)
	if t.URL != "" {
		u := t.URL
		if m.Item(locales.PanelOpenIn(sourceName(t.Source))).Chosen() {
			go s.svc.System.OpenExternal(u)
		}
	}
	s.copySongItems(m, t)
	s.shareItem(m, s.songShare(t))
	if t.Source == media.SourceSpotify && !t.IsEpisode() {
		if m.Item(locales.PanelFindMatch()).Chosen() {
			if err := s.svc.Player.ForgetMatch(t.ID); err != nil {
				s.notes = append(s.notes, noticeOf(err))
			} else {
				s.notes = append(s.notes, notice(locales.PanelMatchForgotten()))
			}
		}
	}
	s.movingCoverItem(m, t)
	// What plugins add to the menu of every track.
	if actions := s.plugins.TrackActions(); len(actions) > 0 {
		m.Separator()
		for _, a := range actions {
			if m.Item(a.Label).Chosen() {
				a.Run(t)
			}
		}
	}
}

// copyLinkItem is a menu's way to share what has a page on its service, at
// rawURL: it copies that link, which anyone can open. Without a link, the
// menu has no such item.
func (s *shell) copyLinkItem(m *ui.Menu, rawURL string) {
	if rawURL != "" && m.Item(locales.PanelCopyLink()).Chosen() {
		copyText(rawURL)
		s.notes = append(s.notes, notice(locales.PanelLinkCopied()))
	}
}

// startRadio plays track, then tracks like it.
func (s *shell) startRadio(t media.Track) {
	if t.Source == media.SourceFile {
		s.notes = append(s.notes, notice(locales.PanelRadioSources()))
		return
	}
	s.notes = append(s.notes, notice(locales.PanelStartingRadio(t.Title)))
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		found, err := s.svc.Catalog.Radio(ctx, t)
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
				return
			}
			s.p.PlayRadio(t, found)
		})
	}()
}

// lyricsSourceName is the name of where lyrics came from, as the window
// says it: the services' own names, and YouTube's captions in its
// language.
func lyricsSourceName(source string) string {
	if source == app.CaptionsSource {
		return locales.LyricsSourceCaptionsName()
	}
	return source
}

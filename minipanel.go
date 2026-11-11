package main

import (
	"cmp"
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/lyrics"
)

// The mini player's panel (#371): buttons beside the artists open a panel
// with the queue, the lyrics, the output device or, after a track failed,
// why, one at a time, which the window grows for (miniwindow.go). The
// settings keep the panel open, but the error's, for the next time the
// mini player opens. Escape closes it.

// miniError is the panel of a track that couldn't play, which the settings
// don't keep.
const miniError = "error"

// miniState is what the mini player keeps from frame to frame.
type miniState struct {
	// panel is the panel chosen, "" for none; shown is the one the window
	// shows, which stays as it closes, until the window shrank back.
	panel, shown string
	// win is the window the panel grows, nil in tests that give none.
	win miniWindow
	// begun is set once the mini player's first frame took in the panel
	// the settings keep, and the tracks that failed before it opened.
	begun bool
	// below is set while the panel shows below the player, else above.
	below bool
	// part is the player's height beside the panel, at 100%: its height as
	// the panel opened.
	part float32
	// move is the window's move as the panel opens or closes, nil once
	// it's done.
	move *miniMove
	// failed is the failure, of the player's, the Error button tells of
	// since: one that came before the mini player opened, or that the
	// user saw and closed.
	failed int
	// hidden are the buttons the transport row had no room for in the
	// frame before, which the menu holds.
	hidden miniOptional
	// refocus is the button of the panel Escape closed, which takes the
	// focus back.
	refocus string
	// queue is where the queue's list scrolled.
	queue ui.ListState
}

// beginMini takes in, in the mini player's first frame with its window,
// the tracks that failed before it opened, which its Error button doesn't
// tell of, and opens the panel the settings keep.
func (s *shell) beginMini(c *ui.Context) {
	m := &s.mini
	if m.begun || m.win == nil {
		return
	}
	m.begun = true
	s.sawFailure()
	if p := s.set.MiniPanel; p != "" {
		_, h := c.Size()
		s.openMini(p, min(max(h, miniMinHeight), miniArt))
	}
}

// openMini shows the panel p, from the player part high, at 100%,
// growing the window for it unless another shows, whose place it takes.
func (s *shell) openMini(p string, part float32) {
	m := &s.mini
	switch p {
	case settings.MiniOutput:
		// The devices are asked for again as the panel opens, as the
		// player bar's buttons do as they're clicked.
		s.refetch(outputDevicesKey)
		s.svc.Player.FindCastDevices()
		s.forget("devices")
	}
	was := m.panel
	if was == miniError {
		s.sawFailure()
	}
	m.panel, m.shown = p, p
	if was == "" {
		s.growMini(part)
	}
}

// sawFailure has the Error button tell only of the tracks that fail after
// the latest, which the user saw, or which failed before the mini player
// opened.
func (s *shell) sawFailure() {
	if f := s.p.Failures(); len(f) > 0 {
		s.mini.failed = f[0].ID
	}
}

// closeMini closes the panel, and the window shrinks back to the player.
// The panel of a failure, closed, leaves the Error button until another
// track fails.
func (s *shell) closeMini() {
	m := &s.mini
	if m.panel == "" {
		return
	}
	if m.panel == miniError {
		s.sawFailure()
	}
	m.panel = ""
	s.shrinkMini()
}

// toggleMini opens the panel p, or closes it while it shows, and keeps the
// choice for the next time the mini player opens, the error's but.
func (s *shell) toggleMini(c *ui.Context, p string) {
	if s.mini.panel == p {
		s.closeMini()
	} else {
		_, h := c.Size()
		part, _ := s.miniHeights(h)
		s.openMini(p, part)
	}
	if p != miniError || s.mini.panel == "" {
		s.keepMini()
	}
}

// keepMini keeps the panel shown in the settings, for the next time the
// mini player opens: none for the error's.
func (s *shell) keepMini() {
	keep := s.mini.panel
	if keep == miniError {
		keep = ""
	}
	if s.set.MiniPanel != keep {
		s.set.MiniPanel = keep
		s.save()
	}
}

// miniFailure is the latest track that couldn't play, while the Error
// button tells of it: since the mini player opened, and until the user
// closed its panel.
func (s *shell) miniFailure() (player.Failure, bool) {
	f := s.p.Failures()
	if len(f) == 0 || (s.mini.panel != miniError && f[0].ID <= s.mini.failed) {
		return player.Failure{}, false
	}
	return f[0], true
}

// miniPanelButtons are the buttons of the panels, beside the artists: the
// queue, the lyrics, the output device and, after a track failed, the
// error, in the accent while their panel shows. The button whose panel
// Escape closed takes the focus back.
func (s *shell) miniPanelButtons(c *ui.Context) {
	m := &s.mini
	button := func(key, name, label string) {
		var b ui.Element
		if key == miniError {
			b = s.miniAlert(c, label)
		} else {
			b = s.miniButton(c, "mini-"+key, name, label, m.panel == key, 24, 15)
		}
		b.Expanded(m.panel == key)
		if m.refocus == key {
			m.refocus = ""
			b.Focus()
		}
		if b.Clicked() {
			s.toggleMini(c, key)
		}
	}
	ui.Row(c).Shrink(0).Children(func() {
		button(settings.MiniQueue, "queue", "Queue")
		button(settings.MiniLyrics, s.lyricsIcon(), s.lyricsTitle())
		button(settings.MiniOutput, "speaker", "Output device")
		if _, failed := s.miniFailure(); failed {
			button(miniError, "alert", "Why it couldn't play")
		}
	})
}

// miniAlert is the button of the panel of a track that couldn't play: its
// icon in the colour of errors.
func (s *shell) miniAlert(c *ui.Context, label string) ui.Element {
	k := &s.lk
	b := s.buttonBase(c.Key("mini-"+miniError)).Size(24, 24).Radius(k.round(24)).Label(label).Tooltip(label).
		Cursor(ui.CursorPointer)
	if b.Pressed() {
		b.Background(k.press)
	} else if b.Hovered() || s.mini.panel == miniError {
		b.Background(k.hover)
	}
	s.pressMotion(b, 0.9)
	b.Children(func() { ui.Icon(c, icon("alert")).FontSize(15).TextColor(k.danger) })
	return b
}

// miniEscape closes the panel on Escape, and gives the focus to its
// button.
func (s *shell) miniEscape(c *ui.Context) {
	if s.mini.panel != "" && c.Shortcut(0, ui.KeyEscape) {
		s.mini.refocus = s.mini.panel
		s.closeMini()
		s.keepMini()
	}
}

// miniPages opens, in the main window, the page a menu of the mini player
// went to, as Go to album: the mini player shows no pages.
func (s *shell) miniPages() {
	if s.mini.win == nil {
		// A test's shell, which the main window shares.
		return
	}
	if at := s.router.Location(); at != "/" {
		s.router.Replace("/")
		s.svc.System.OpenMainWindow(at)
	}
}

// followMiniPanel redraws the mini player as the cast devices found
// change, and has it list the output devices again as one comes or goes.
// Neither tells of it in a toast, which would cover the player.
func (s *shell) followMiniPanel() []func() {
	return []func(){
		s.a.OnCast(func(string) { s.update(func() {}) }),
		s.a.OnOutputDevice(func(string) { s.update(func() { s.refetch(outputDevicesKey) }) }),
	}
}

// miniPanel is the panel shown, room high in a window w wide, against the
// player: the panel's head, with its title and a button that closes it,
// and its body. While the window grows, the panel slides out from the
// player.
func (s *shell) miniPanel(c *ui.Context, w, room float32) {
	k := &s.lk
	m := &s.mini
	high := max(room, miniPanelHeight)
	top := float32(0)
	if !m.below {
		top = room - high
	}
	ui.Box(c.Key("mini-panel")).Width(w).Height(room).Shrink(0).Clip().Children(func() {
		panel := ui.Column(c.Key("mini-panel-"+m.shown)).Absolute().Left(0).Top(top).Size(w, high).
			Background(k.surface).BorderColor(k.border).Role(ui.RoleRegion).Label(s.miniTitle(m.shown))
		if m.below {
			panel.BorderWidth(1, 0, 0, 0)
		} else {
			panel.BorderWidth(0, 0, 1, 0)
		}
		panel.Children(func() {
			s.miniHead(c)
			switch m.shown {
			case settings.MiniQueue:
				s.miniQueue(c)
			case settings.MiniLyrics:
				s.miniLyrics(c)
			case settings.MiniOutput:
				s.miniOutput(c)
			case miniError:
				s.miniErrorPanel(c)
			}
		})
	})
}

// miniTitle is the title of the panel p.
func (s *shell) miniTitle(p string) string {
	switch p {
	case settings.MiniQueue:
		return "Up next"
	case settings.MiniLyrics:
		return s.lyricsTitle()
	case settings.MiniOutput:
		return "Output device"
	}
	return "Why it couldn't play"
}

// miniHead is the head of the panel: its title, the queue's place in it,
// and a button that closes it.
func (s *shell) miniHead(c *ui.Context) {
	k := &s.lk
	m := &s.mini
	title := s.miniTitle(m.shown)
	ui.Row(c).Shrink(0).Padding(6, 6, 2, 12).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, k.words(title)).FontSize(k.bodySize()).FontWeight(k.titleWeight()).SingleLine().MinWidth(0).Shrink(1)
		if m.shown == settings.MiniQueue && s.st.Index >= 0 && len(s.st.Queue) > 0 {
			ui.Text(c, queuePlace(s.st.Index, len(s.st.Queue))).FontSize(k.smallSize()).TextColor(k.muted).
				FontFeatures("tnum").SingleLine().Shrink(0)
		}
		ui.Box(c).Grow(1)
		if s.miniButton(c, "mini-panel-close", "close", miniCloseLabel(m.shown), false, 24, 14).Clicked() {
			s.closeMini()
			s.keepMini()
		}
	})
}

// miniCloseLabel is the label of the button that closes the panel p.
func miniCloseLabel(p string) string {
	switch p {
	case settings.MiniQueue:
		return "Close the queue"
	case settings.MiniLyrics:
		return "Close the lyrics"
	case settings.MiniOutput:
		return "Close the output devices"
	}
	return "Close the details of the error"
}

// queuePlace is the place of the entry at, of n, as the queue's panel
// tells it: "3 of 24".
func queuePlace(at, n int) string { return strconv.Itoa(at+1) + " of " + strconv.Itoa(n) }

// miniQueueRow is the height of an entry of the mini player's queue.
const miniQueueRow = 44

// miniQueue lists the track playing and what plays next, from it: a click
// on an entry plays it, the current one plays or pauses, and each has its
// menu, as in the main window's queue.
func (s *shell) miniQueue(c *ui.Context) {
	k := &s.lk
	q, at := s.st.Queue, s.st.Index
	if at < 0 || at >= len(q) {
		ui.Text(c, "Play something and the queue shows here.").FontSize(k.smallSize()).TextColor(k.muted).Padding(8, 12)
		return
	}
	s.mini.queue.Key = func(row int) any {
		if at+row < len(q) {
			return q[at+row].UID
		}
		return row
	}
	ui.List(c.Key("mini-queue"), &s.mini.queue, len(q)-at, func(row int) {
		s.miniQueueEntry(c, at+row, q[at+row], row == 0)
	}).Grow(1).MinHeight(0).Padding(0, 4, 6).Label("Up next")
}

// miniQueueEntry is the entry i of the queue, it, the current one flooded
// with the accent: its cover, title and artists, and its length.
func (s *shell) miniQueueEntry(c *ui.Context, i int, it player.Item, current bool) {
	k := &s.lk
	t := it.Track
	label := "Play " + t.Title
	if current {
		label = t.Title + ", playing"
	}
	row := s.buttonBase(c.Key("mq-"+it.UID)).Height(miniQueueRow).Padding(0, 8).Gap(10).Justify(ui.Start).
		Radius(k.radiusSm).Label(label).Cursor(ui.CursorPointer)
	ink, sub := k.text, k.muted
	switch {
	case current:
		row.Background(k.selected)
		ink, sub = k.selectedText, k.selectedSub
	case row.Hovered():
		row.Background(k.hover)
	}
	if it.Skip != "" && !current {
		ink = k.muted
	}
	row.ContextMenu(func(m *ui.Menu) { s.trackMenuAt(m, t, i) })
	if row.Clicked() {
		if current {
			s.p.Toggle()
		} else {
			s.p.PlayAt(i)
		}
	}
	row.Children(func() {
		s.cover(c, "mq-cover-"+it.UID, t.Artwork, 32, false)
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			ui.Text(c, t.Title).FontSize(k.smallSize()).TextColor(ink).SingleLine().MinWidth(0)
			ui.Text(c, names(t.Artists)).FontSize(k.captionSize()).TextColor(sub).SingleLine().MinWidth(0)
		})
		if t.DurationMs > 0 {
			ui.Text(c, clock(ms(t.DurationMs))).FontSize(k.captionSize()).TextColor(sub).FontFeatures("tnum").Shrink(0)
		}
	})
}

// miniNote is a line of the panel that tells why it shows nothing more.
func (s *shell) miniNote(c *ui.Context, text string) {
	ui.Text(c, text).FontSize(s.lk.smallSize()).TextColor(s.lk.muted).Padding(8, 12)
}

// miniLyrics shows the line of the lyrics sung now, word by word where
// the lyrics have the words' times, and the line after it, following the
// music, in Latin letters as the lyrics panel shows them; plain lyrics
// show whole, and lyrics that aren't there say why. An episode shows its
// chapter in their place.
func (s *shell) miniLyrics(c *ui.Context) {
	k := &s.lk
	cur, ok := s.st.Current()
	if !ok {
		s.miniNote(c, "Play a track to see its lyrics.")
		return
	}
	t := cur.Track
	if t.IsEpisode() {
		s.miniChapters(c, t)
		return
	}
	l := fetch(s, "lyrics:"+t.ID, func(ctx context.Context) (lyrics.Lyrics, error) { return s.svc.Catalog.Lyrics(ctx, t) })
	switch {
	case !l.done:
		s.miniNote(c, "Looking for the lyrics…")
		return
	case l.err != nil:
		s.miniNote(c, l.err.Error())
		// A track without lyrics stays so; a failure may pass.
		if !errors.Is(l.err, lyrics.ErrNotFound) {
			ui.Row(c).Padding(0, 12).Children(func() {
				if s.link(c, "mini-lyrics-retry", "try again") {
					s.forget("lyrics:" + t.ID)
				}
			})
		}
		return
	case l.value.Instrumental:
		s.miniNote(c, "This track is instrumental.")
		return
	}
	ly := l.value
	if !ly.Synced || len(ly.Lines) == 0 {
		ui.Scroll(c).Grow(1).MinHeight(0).Padding(4, 12, 12).Children(func() {
			for _, line := range ly.Lines {
				ui.Text(c, cmp.Or(line.Text, "♪")).FontSize(k.smallSize())
			}
			s.miniCredit(c, ly)
		})
		return
	}
	lines := ly.Lines
	pos := s.st.Position + lyricsLead(s.st.Speed)
	at := sort.Search(len(lines), func(i int) bool { return ms(lines[i].TimeMs) > pos }) - 1
	latin, how := s.latinLines(t, ly)
	line := func(i int) string { return notchLine(lines, latin, how, i) }
	ui.Column(c).Grow(1).MinHeight(0).Padding(4, 12, 8).Gap(6).Justify(ui.Center).Children(func() {
		text := line(at)
		var words []lyrics.Word
		if at >= 0 && text == lines[at].Text {
			words = lines[at].Words
		}
		s.lyricsText(c, text, k.size(1.25), k.accent, words, 0, 0).MinWidth(0)
		if under := notchUnder(latin, how, at); under != "" {
			ui.Text(c, under).FontSize(k.smallSize()).TextColor(k.muted).MinWidth(0)
		}
		if at+1 < len(lines) {
			ui.Text(c, line(at+1)).FontSize(k.bodySize()).TextColor(k.muted).MaxLines(2).MinWidth(0)
		}
	})
	s.miniCredit(c, ly)
}

// miniCredit names where the lyrics come from, and who gave them to it,
// as the lyrics panel does, smaller.
func (s *shell) miniCredit(c *ui.Context, ly lyrics.Lyrics) {
	k := &s.lk
	if ly.Source == "" {
		return
	}
	ui.RichText(c).FontSize(k.captionSize()).TextColor(k.muted).Padding(0, 12, 8).Shrink(0).Children(func() {
		s.creditParts(c, ly)
	})
}

// miniChapters shows the chapter of the episode t that plays now, and the
// one after it, as the lyrics show the line sung now.
func (s *shell) miniChapters(c *ui.Context, t media.Track) {
	k := &s.lk
	key, l := s.episodeDetailsOf(t)
	switch {
	case !l.done:
		s.miniNote(c, "Looking for the details…")
		return
	case l.err != nil:
		s.miniNote(c, l.err.Error())
		if !errors.Is(l.err, app.ErrNoEpisodeDetails) {
			ui.Row(c).Padding(0, 12).Children(func() {
				if s.link(c, "mini-details-retry", "try again") {
					s.forget(key)
				}
			})
		}
		return
	}
	chapters, limit := l.value.Chapters, s.seekLimit(t, l.value)
	in := chapterAt(chapters, max(limit, 0), limit) + 1
	if in == 0 {
		s.miniNote(c, "No chapters.")
		return
	}
	at := max(chapterAt(chapters[:in], s.st.Position+lyricsLead(s.st.Speed), limit), 0)
	ui.Column(c).Grow(1).MinHeight(0).Padding(4, 12, 8).Gap(6).Justify(ui.Center).Children(func() {
		ui.Text(c, chapters[at].Title).FontSize(k.size(1.25)).FontWeight(k.leadWeight()).TextColor(k.accent).MaxLines(2).MinWidth(0)
		if at+1 < in {
			ui.Text(c, chapters[at+1].Title).FontSize(k.bodySize()).TextColor(k.muted).MaxLines(2).MinWidth(0)
		}
	})
}

// miniOutput lists where what plays can play: the output devices of this
// computer, then the cast devices on the network and the Spotify Connect
// devices, as the player bar's buttons list them where it shows them.
func (s *shell) miniOutput(c *ui.Context) {
	ui.Scroll(c).Grow(1).MinHeight(0).Padding(0, 4, 8).Children(func() {
		shown := false
		if settings.NativeAudio {
			shown = true
			s.miniOutputDevices(c)
		}
		if s.castShown() {
			shown = true
			s.miniCastDevices(c)
		}
		if s.set.SpotifyPlayback == settings.SpotifyPlaybackConnect {
			shown = true
			s.miniConnectDevices(c)
		}
		if !shown {
			s.miniNote(c, "MixMusic plays on the system's default device.")
		}
	})
}

// miniSection is a group of the output panel's choices, titled.
func (s *shell) miniSection(c *ui.Context, key, title string, choices func()) {
	k := &s.lk
	ui.Column(c.Key(key)).Role(ui.RoleRadioGroup).Label(title).Padding(6, 0, 0).Children(func() {
		ui.Text(c, k.words(title)).FontSize(k.captionSize()).FontWeight(k.leadWeight()).TextColor(k.muted).Padding(2, 8)
		choices()
	})
}

// miniChoice is a choice of the output panel, which a click chooses: its
// label, what tells more of it, and a check while it's the one chosen.
func (s *shell) miniChoice(c *ui.Context, key, label, detail string, on, disabled bool) bool {
	k := &s.lk
	name := label
	if detail != "" {
		name += ", " + detail
	}
	b := s.buttonBase(c.Key(key)).Justify(ui.Start).Gap(8).Padding(5, 8).MinHeight(34).Radius(k.radiusSm).
		Role(ui.RoleRadio).Checked(on).Label(name).Cursor(ui.CursorPointer).Disabled(disabled)
	if b.Hovered() && !disabled {
		b.Background(k.hover)
	}
	if disabled {
		b.Opacity(0.5)
	}
	b.Children(func() {
		ui.Box(c).Size(16, 16).Shrink(0).Center().Children(func() {
			if on {
				ui.Icon(c, icon("check")).FontSize(16).TextColor(k.accent)
			}
		})
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ink := k.text
			if on {
				ink = k.accent
			}
			ui.Text(c, label).FontSize(k.smallSize()).TextColor(ink).SingleLine().MinWidth(0)
			if detail != "" {
				ui.Text(c, detail).FontSize(k.captionSize()).TextColor(k.muted).SingleLine().MinWidth(0)
			}
		})
	})
	return b.Clicked()
}

// miniOutputDevices are the output devices MixMusic's own player can play
// on (outputChoices), and why what plays now doesn't play there, if it
// doesn't.
func (s *shell) miniOutputDevices(c *ui.Context) {
	devices := s.outputDevices()
	s.miniSection(c, "mini-output-devices", "On this computer", func() {
		if aside := s.outputAside(); aside != "" {
			s.miniNote(c, aside)
		}
		for _, o := range outputChoices(devices.value, devices.done && devices.err == nil, s.set.OutputDevice) {
			if s.miniChoice(c, "mini-output-"+o.Device.ID, o.Label, o.Detail, o.Chosen, false) {
				s.chooseOutput(o.Device)
			}
		}
		switch {
		case !devices.done:
			s.miniNote(c, "Looking for devices…")
		case devices.err != nil:
			s.miniNote(c, devices.err.Error())
		}
	})
}

// miniCastDevices are the cast devices on the network, beside this
// computer, as the cast button lists them.
func (s *shell) miniCastDevices(c *ui.Context) {
	casting, on := s.svc.Player.CastingTo()
	starting, connecting := s.svc.Player.CastStarting()
	s.miniSection(c, "mini-cast", "Cast", func() {
		if s.miniChoice(c, "mini-cast-here", "This computer", "", !on, connecting) && on {
			go s.svc.Player.StopCasting()
		}
		for _, d := range s.svc.Player.CastDevices() {
			detail := d.Model
			if connecting && starting.ID == d.ID {
				detail = strings.TrimPrefix(detail+" · connecting…", " · ")
			}
			if s.miniChoice(c, "mini-cast-"+d.ID, d.Name, detail, on && casting.ID == d.ID, connecting) {
				s.castTo(d)
			}
		}
	})
}

// miniConnectDevices are the Spotify Connect devices that can play Spotify
// tracks, as the player bar's devices button lists them.
func (s *shell) miniConnectDevices(c *ui.Context) {
	chosen := s.set.SpotifyDevice
	devices := fetch(s, "devices", s.svc.Player.Devices)
	s.miniSection(c, "mini-connect", "Spotify Connect", func() {
		if s.miniChoice(c, "mini-connect-active", "The active device", "", chosen == "", false) {
			s.setDevice("")
		}
		switch {
		case !devices.done:
			s.miniNote(c, "Looking for devices…")
		case devices.err != nil:
			s.miniNote(c, devices.err.Error())
		case len(devices.value) == 0:
			s.miniNote(c, "Open Spotify on a device to see it here")
		}
		for _, d := range devices.value {
			detail := ""
			if d.Active {
				detail = "active"
			}
			if s.miniChoice(c, "mini-connect-"+d.ID, d.Name, detail, chosen == d.ID, d.Restricted) {
				s.setDevice(d.ID)
			}
		}
	})
}

// miniErrorPanel shows the details of the latest track that couldn't
// play, with Copy (failureDetails).
func (s *shell) miniErrorPanel(c *ui.Context) {
	f := s.p.Failures()
	if len(f) == 0 {
		s.miniNote(c, "No track failed to play since MixMusic started.")
		return
	}
	ui.Scroll(c).Grow(1).MinHeight(0).Padding(4, 12, 12).Children(func() { s.failureDetails(c, f[0]) })
}

package main

// Selecting tracks (#341): Ctrl-click (⌘-click on macOS) a track's row to
// select it or let it go, Shift-click another to select the rows from the
// one clicked last to it, or choose Select in a track's menu. Once a list
// has tracks selected, its rows show check boxes, and a click on a row
// selects it or lets it go. A bar above the player bar tells how many are
// selected, and acts on them all, in their list's order. Escape, the bar's
// close button or another page lets them go.
//
// The bar's actions register from init() with addPickAction, each in its
// feature's file, so that a new one changes no list here.

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/downloads"
	"github.com/fzn0x/mixmusic/internal/media"
)

// trackPicks are the tracks selected in a track list: of the list of key
// list, on the page at page (its path and query), in the order they were
// selected, and by their IDs. anchor is the track Shift-click selects
// from: the one clicked last. shown is the list's tracks as its last frame
// built them, which Select all takes and the actions' order follows. seq
// changes with what's selected and with shown, which the memos follow.
type trackPicks struct {
	list, page string
	tracks     []media.Track
	ids        map[string]bool
	anchor     string
	shown      []media.Track
	seq        uint64
	// ordered are the tracks selected in their list's order, made for
	// orderedAt; todo those of them that download, for todoAt and the
	// downloads' version.
	ordered          []media.Track
	orderedAt        uint64
	inShown          int
	todo             []media.Track
	todoAt, todoKept uint64
}

// pickBar is the height of the bar of the tracks selected, and pickGap the
// room under it, above the player bar.
const (
	pickBar = 56
	pickGap = 12
)

// pickAction is a button of the bar of the tracks selected: build builds
// it for tracks, the tracks selected in their list's order, and does what
// it does once it's clicked.
type pickAction struct {
	order int
	build func(s *shell, c *ui.Context, tracks []media.Track)
}

// pickActions are the bar's buttons, by their orders.
var pickActions []pickAction

// addPickAction adds a button to the bar of the tracks selected, from
// init() in its feature's file, at order among the others. Orders go up in
// tens, so that a new one slots in between; two at the same order stop
// MixMusic at start-up, so that the tests catch it.
func addPickAction(order int, build func(*shell, *ui.Context, []media.Track)) {
	i, taken := slices.BinarySearchFunc(pickActions, order, func(a pickAction, order int) int { return cmp.Compare(a.order, order) })
	if taken {
		panic(fmt.Sprintf("two actions of the selected tracks at %d", order))
	}
	pickActions = slices.Insert(pickActions, i, pickAction{order, build})
}

// picking reports whether the list of key, on the page being built, has
// tracks selected.
func (s *shell) picking(key string) bool {
	p := &s.picks
	return len(p.ids) > 0 && p.list == key && p.page == s.router.Location() && s.pagePath == s.router.Path()
}

// pickIn starts a selection in the list of key, on the page shown, unless
// that list's is the one under way: another's goes.
func (s *shell) pickIn(key string) {
	p := &s.picks
	if p.list == key && p.page == s.router.Location() {
		return
	}
	shown := p.shown
	if p.list != key {
		shown = nil
	}
	*p = trackPicks{list: key, page: s.router.Location(), ids: map[string]bool{}, shown: shown, seq: p.seq + 1}
}

// set selects t, or lets it go.
func (p *trackPicks) set(t media.Track, on bool) {
	if p.ids[t.ID] == on {
		return
	}
	p.seq++
	if on {
		p.ids[t.ID] = true
		p.tracks = append(p.tracks, t)
		return
	}
	delete(p.ids, t.ID)
	p.tracks = slices.DeleteFunc(p.tracks, func(x media.Track) bool { return x.ID == t.ID })
}

// pickClick does what a click on the row of t, at i of the list of key
// whose frame built tracks, does with mods held: Cmd, or Ctrl, selects it
// or lets it go, and Shift selects the rows from the anchor to it. Without
// either, it selects or lets go once the list has tracks selected, and
// does nothing otherwise.
func (s *shell) pickClick(key string, i int, t media.Track, tracks []media.Track, mods ui.Modifiers) {
	p := &s.picks
	switch {
	case mods&ui.Shift != 0:
		s.pickIn(key)
		s.pickRange(tracks, i)
	case mods&ui.Cmd != 0:
		s.pickIn(key)
		p.set(t, !p.ids[t.ID])
		p.anchor = t.ID
	case s.picking(key):
		p.set(t, !p.ids[t.ID])
		p.anchor = t.ID
	}
}

// pickRange selects the tracks of tracks from the anchor to the one at i,
// as Shift-click does, or that one alone when the list holds no anchor;
// the anchor stays, so that another Shift-click ranges from it again.
func (s *shell) pickRange(tracks []media.Track, i int) {
	p := &s.picks
	from := slices.IndexFunc(tracks, func(x media.Track) bool { return x.ID == p.anchor })
	if from < 0 {
		from, p.anchor = i, tracks[i].ID
	}
	for _, t := range tracks[min(from, i) : max(from, i)+1] {
		p.set(t, true)
	}
}

// pickList takes the list of key as its frame builds it, of tracks: what
// Select all takes, and the order the actions follow. While it has tracks
// selected, it ends with the room the bar takes, so that its last row can
// be scrolled out from under the bar.
func (s *shell) pickList(list ui.Element, key string, tracks []media.Track) {
	p := &s.picks
	if p.list != key {
		return
	}
	if len(p.shown) != len(tracks) || len(tracks) > 0 && &p.shown[0] != &tracks[0] {
		p.shown = tracks
		p.seq++
	}
	if s.picking(key) {
		list.Padding(0, 0, pickBar+pickGap)
	}
}

// pickRow makes row, of the track t at i of the list of key whose frame
// built tracks, select as pickClick tells, and show it selected: tinted,
// but as the track that plays, which the accent floods.
func (s *shell) pickRow(row ui.Element, key string, i int, t media.Track, tracks []media.Track, current bool) {
	if row.Clicked() {
		s.pickClick(key, i, t, tracks, row.ClickModifiers())
	}
	if current || !s.picking(key) || !s.picks.ids[t.ID] {
		return
	}
	k := &s.lk
	if row.Hovered() {
		row.Background(k.accent.Alpha(0.24))
	} else {
		row.Background(k.accent.Alpha(0.16))
	}
}

// pickBox is the check box at the start of the row of t, at i of the list
// of key whose frame built tracks, while the list has tracks selected:
// checked when t is. A click selects t or lets it go, and with Shift
// selects the rows from the anchor to it.
func (s *shell) pickBox(c *ui.Context, key string, i int, t media.Track, tracks []media.Track) {
	if !s.picking(key) {
		return
	}
	k := &s.lk
	p := &s.picks
	on := p.ids[t.ID]
	box := s.pointerKeys(ui.CheckboxBase(c.Key(key+"-pick-"+count(i)), &on)).Label("Select "+t.Title).
		Size(24, 24).Center().Cursor(ui.CursorPointer)
	if box.Changed() {
		if box.ClickModifiers()&ui.Shift != 0 {
			s.pickRange(tracks, i)
		} else {
			p.set(t, on)
			p.anchor = t.ID
		}
	}
	box.Children(func() {
		face := ui.Box(c).Size(18, 18).Radius(4).Center()
		if p.ids[t.ID] {
			face.Background(k.accent).Children(func() {
				ui.Icon(c, icon("check")).FontSize(14).TextColor(k.accentText)
			})
		} else {
			face.Border(2, k.outline)
		}
		if box.FocusVisible() {
			face.Shadow(0, 0, 0, 2, k.accent)
		}
	})
}

// pickItem is the item of the menu of the row of t that selects it, or
// lets it go: the way to start a selection without the keys.
func (s *shell) pickItem(m *ui.Menu, key string, t media.Track) {
	m.Separator()
	if s.picking(key) && s.picks.ids[t.ID] {
		if m.Item("Deselect").Chosen() {
			s.picks.set(t, false)
		}
		return
	}
	if m.Item("Select").Chosen() {
		s.pickIn(key)
		s.picks.set(t, true)
		s.picks.anchor = t.ID
	}
}

// inOrder returns the tracks selected in their list's order, those the
// list no longer shows, as a search left out, after them, and counts those
// it shows in inShown: made again once the selection or the list changed,
// not in each frame.
func (p *trackPicks) inOrder() []media.Track {
	if p.orderedAt == p.seq && p.ordered != nil {
		return p.ordered
	}
	out := make([]media.Track, 0, len(p.tracks))
	in := map[string]bool{}
	for _, t := range p.shown {
		if p.ids[t.ID] && !in[t.ID] {
			in[t.ID] = true
			out = append(out, t)
		}
	}
	p.inShown = 0
	for _, t := range p.shown {
		if p.ids[t.ID] {
			p.inShown++
		}
	}
	for _, t := range p.tracks {
		if !in[t.ID] {
			out = append(out, t)
		}
	}
	p.ordered, p.orderedAt = out, p.seq
	return out
}

// clearPicks lets the tracks selected go.
func (s *shell) clearPicks() { s.picks = trackPicks{seq: s.picks.seq + 1} }

// picksBar is the bar of the tracks selected, above the player bar: how
// many, Select all while the list shows more, the actions, and a button
// that lets them go, as Escape does. Another page lets them go too.
func (s *shell) picksBar(c *ui.Context) {
	p := &s.picks
	if len(p.ids) == 0 {
		return
	}
	if p.page != s.router.Location() {
		s.clearPicks()
		return
	}
	if c.Shortcut(0, ui.KeyEscape) {
		s.clearPicks()
		return
	}
	k := &s.lk
	tracks := p.inOrder()
	// Over the main pane, between the rail and the side panel.
	right := float32(0)
	if s.panel != "" {
		right = s.panelWidth(c)
		if k.floating {
			right += floatMargin
		}
	}
	ui.Row(c.Key("picks")).Absolute().Left(s.railWidth()).Right(right).Bottom(k.barRoom()+pickGap).Padding(0, 16).
		Justify(ui.Center).PassThrough().Children(func() {
		bar := ui.Row(c.Key("picks-bar")).MinHeight(pickBar).Padding(8, 8, 8, 20).Gap(8).Wrap().AlignItems(ui.Center).
			Justify(ui.Center).Radius(k.round(pickBar)).Background(k.raised).Border(1, k.border).
			Shadow(0, 6, 20, -6, ui.RGBA(0, 0, 0, 0.3)).Shrink(1).MinWidth(0).Label("Selected tracks")
		bar.Children(func() {
			ui.Text(c, k.sentence(fmt.Sprintf("%d selected", len(tracks)))).FontSize(k.bodySize()).FontFeatures("tnum").
				SingleLine().Shrink(0).Margin(0, 8, 0, 0)
			if p.inShown < len(p.shown) && s.link(c, "picks-all", "select all") {
				for _, t := range p.shown {
					p.set(t, true)
				}
			}
			for _, a := range pickActions {
				a.build(s, c, tracks)
			}
			if s.iconButton(c, "picks-clear", "close", "Clear the selection", false, 16).Clicked() {
				s.clearPicks()
			}
		})
	})
}

func init() { addPickAction(10, (*shell).queuePicked) }

// queuePicked is the bar's Add to queue.
func (s *shell) queuePicked(c *ui.Context, tracks []media.Track) {
	if s.button(c, "picks-queue", "add to queue", false).Label("Add the selected tracks to the queue").Clicked() {
		s.p.Enqueue(tracks)
		s.notes = append(s.notes, notice("Added "+plural(len(tracks), "track")+" to the queue"))
	}
}

func init() { addPickAction(20, (*shell).mixPicked) }

// mixPicked is the bar's Add to mix: a new mix of the tracks, or one of
// the user's.
func (s *shell) mixPicked(c *ui.Context, tracks []media.Track) {
	s.button(c, "picks-mix", "add to mix", false).Label("Add the selected tracks to a mix").Menu(func(m *ui.Menu) { s.mixItems(m, tracks) })
}

func init() { addPickAction(30, (*shell).downloadPicked) }

// downloadPicked is the bar's Download, while some of the tracks download
// and aren't downloaded, nor waiting to be.
func (s *shell) downloadPicked(c *ui.Context, tracks []media.Track) {
	p := &s.picks
	if v := s.kept.Version(); p.todoAt != p.seq || p.todoKept != v || p.todo == nil {
		p.todo = slices.DeleteFunc(s.downloadableOf(tracks), func(t media.Track) bool {
			return s.kept.State(t.ID).Kind != downloads.None
		})
		p.todoAt, p.todoKept = p.seq, v
	}
	if len(p.todo) > 0 && s.button(c, "picks-download", "download", false).Label("Download the selected tracks").Clicked() {
		s.download(p.todo)
	}
}

package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
)

// What the full-screen lyrics (lyricsstage.go) show at their sides: at the
// left of the cover, the buttons that play and pause, and go to the next and
// the previous track; at the right, beside the lyrics' scroll bar, the covers
// of the tracks next in the queue, which a click plays.

const (
	// stageControlsW is the width of the column of buttons at the left of the
	// cover.
	stageControlsW = 52
	// stageQueueW is the width of the strip of the next tracks' covers. The
	// stage keeps it whether or not there are tracks next, so that the lyrics
	// don't change width, and wrap anew, as the queue runs out.
	stageQueueW = 68
	// stageQueueCovers is the most covers the strip shows.
	stageQueueCovers = 10
)

// stageButton is a round button of the stage's ink, size across with a glyph
// of the size given: filled, if main, and outlined by its hover otherwise.
// It tells whether it was pressed. The stage's ink is white, whatever the
// theme, as the ground is dark.
func (s *shell) stageButton(c *ui.Context, key, name, label string, size, glyph float32, main bool) bool {
	b := s.buttonBase(c.Key(key)).Size(size, size).Shrink(0).Radius(size / 2).Label(label).Tooltip(label).Cursor(ui.CursorPointer)
	ink := stageInk
	switch {
	case main:
		bg := stageInk
		if b.Hovered() {
			bg = stageInk.Alpha(0.86)
		}
		b.Background(bg)
		ink = ui.Color{A: 255}
	case b.Pressed():
		b.Background(stageInk.Alpha(0.28))
	case b.Hovered():
		b.Background(stageInk.Alpha(0.16))
	}
	s.pressMotion(b, 0.9)
	b.Children(func() { ui.Icon(c, icon(name)).FontSize(glyph).TextColor(ink) })
	return b.Clicked()
}

// stageControls are the buttons at the left of the cover, in a column as tall
// as the cover, so that they lie at its middle: the previous track, play or
// pause, and the next track.
func (s *shell) stageControls(c *ui.Context, cover float32) {
	name, label := toggleOf(s.st)
	ui.Column(c.Key("stage-controls")).Width(stageControlsW).Height(cover).Shrink(0).AlignItems(ui.Center).Justify(ui.Center).Gap(14).
		Label("Player controls").Children(func() {
		if s.stageButton(c, "stage-previous", "previous", "Previous track", 40, 18, false) {
			s.p.Previous()
		}
		if s.stageButton(c, "stage-toggle", name, label, 52, 22, true) {
			s.p.Toggle()
		}
		if s.stageButton(c, "stage-next", "next", "Next track", 40, 18, false) {
			s.p.Next()
		}
	})
}

// stageQueue is the strip of the covers of the tracks next in the queue, h
// high, as many as fit between the close button and the foot: the queue
// panel's upcoming entries (s.st.Queue after the current one), each of which
// plays when it is clicked. The strip takes its room even when it is empty.
func (s *shell) stageQueue(c *ui.Context, h float32) {
	const size, gap = 52, 12
	q, at := s.st.Queue, s.st.Index
	room := min(max(int((h-2*80+gap)/(size+gap)), 0), stageQueueCovers)
	ui.Column(c.Key("stage-queue")).Width(stageQueueW).Height(h).Shrink(0).AlignItems(ui.Center).Justify(ui.Center).Gap(gap).
		Label("Up next").Children(func() {
		for rank, i := 0, at+1; i < len(q) && rank < room; rank, i = rank+1, i+1 {
			s.stageQueueCover(c, i, q[i], rank, size)
		}
	})
}

// stageQueueCover is the cover of the entry i of the queue, which plays that
// entry when it is clicked. The nearer to playing it lies, the plainer it
// shows, and an entry the queue passes over shows dim.
func (s *shell) stageQueueCover(c *ui.Context, i int, it player.Item, rank int, size float32) {
	t := it.Track
	b := s.buttonBase(c.Key("stage-q-"+it.UID)).Size(size, size).Shrink(0).Radius(10).Clip().
		Label("Play " + t.Title).Tooltip(t.Title + " · " + names(t.Artists)).Cursor(ui.CursorPointer)
	op := max(1-0.09*float32(rank), 0.45)
	if it.Skip != "" {
		op = min(op, 0.35)
	}
	if b.Hovered() {
		op = 1
		b.Border(2, stageInk)
	}
	b.Opacity(op)
	b.Children(func() { s.cover(c, "stage-q-cover-"+it.UID, t.Artwork, size, false) })
	if b.Clicked() {
		s.p.PlayAt(i)
	}
}

package main

// Right to left (#564): in a language written right to left, MyGo lays the
// window out from the right, as the window's direction says, so the rail is
// on the right, rows start at the right and the side panel is on the left.
// MyGo mirrors neither physical edges nor what the app draws: the views use
// the start and end forms of edges, and what follows here.

import "github.com/egoist/mygo/ui"

// barInsets are the room the window's buttons take, under a hidden title
// bar, at the side lines start and at the other: TitleBar tells them by
// the window's left and right, which the start and the end swap right to
// left.
func barInsets(c *ui.Context) (start, end float32) {
	bar := c.TitleBar()
	if c.Direction() == ui.RightToLeft {
		return bar.Right, bar.Left
	}
	return bar.Left, bar.Right
}

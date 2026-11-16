package main

// Right to left (#564): in a language written right to left, MyGo lays the
// window out from the right, as the window's direction says, so the rail is
// on the right, rows start at the right and the side panel is on the left.
// MyGo mirrors neither physical edges nor what the app draws: the views use
// the start and end forms of edges, and what follows here. The media
// controls stay left to right in every language (Direction(ui.LeftToRight)
// where they are built), as playback runs one way: the seek bars and their
// times, previous, play and next, the volume and the equalizer's bands.

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

// lineIcon is the icon name, turned to point the way the line reads: the
// arrows of history and of shelves point back to the right, and the sidebar
// is on the right, right to left. The other icons show a shape, which reads
// alike either way, or a direction of playback, which doesn't turn.
func lineIcon(c *ui.Context, name string) *ui.SVG {
	if svg, ok := mirroredIcons[name]; ok && c.Direction() == ui.RightToLeft {
		return svg
	}
	return icon(name)
}

// mirroredIcons are the icons that point along the line, mirrored, drawn
// as icons draws their lines.
var mirroredIcons = func() map[string]*ui.SVG {
	out := map[string]*ui.SVG{}
	for _, name := range []string{"back", "forward", "sidebar"} {
		out[name] = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="square" stroke-linejoin="miter"><path transform="matrix(-1 0 0 1 24 0)" d="` + iconPaths[name] + `"/></svg>`))
	}
	return out
}()

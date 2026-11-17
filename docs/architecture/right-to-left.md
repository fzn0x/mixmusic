# Right to left

In a language written right to left, as Arabic, Hebrew, Persian and Urdu
are, the windows are laid out mirrored (#564): the rail on the right, rows
from the right, the side panel on the left, and back arrows pointing right.
MyGo lays a window out in the direction of the app's language, and mirrors
rows, columns, the start and end forms of edges and its own controls
([its guide](https://github.com/egoist/mygo/blob/v0.8.1/docs/ui/right-to-left.md)).
It mirrors neither physical edges nor what an app draws, so the views
follow a few rules:

- **Edges by their side of the line.** A padding, margin, border or
  position on one side is `PaddingStart`, `MarginEnd`, `BorderStartWidth`,
  `Start` or `End`, not `Padding(top, right, bottom, left)`, `Left` or
  `Right`. Physical edges stay for what is the same both ways, as a box
  that fills its parent, and for what pictures something that doesn't
  mirror, as Discord's card in Settings.
- **The window's buttons.** `TitleBar` tells the room the window's
  buttons take at its left and its right: `barInsets` (`rtl.go`) gives it
  at the start and the end, for the rail and the top bar.
- **Icons.** An icon that points along the line, as the arrows of history
  and of shelves and the sidebar's, turns right to left (`lineIcon`); an
  icon that shows a shape doesn't. What MixMusic draws itself reads
  `Context.Direction`, as the disclosure arrow of Stats does.
- **Media controls stay left to right**, in `Direction(ui.LeftToRight)`
  given to the element they are built in, as playback runs one way in
  every language: the seek bars and their times, previous, play and next,
  the volume, and the equalizer's bands, in the player bar, on the now
  playing page, in the mini player, around the notch and on another
  computer's page. Their bars fill from the left. A widget reads the
  direction as it is built, so the direction goes on the row or box around
  it, not on the slider. The chart of Stats reads left to right too.
- **Titles and lyrics** read in the direction of their own text, as MyGo
  sets a paragraph's direction by its first strong character: a Latin
  title keeps its order in a window laid out right to left. They align to
  the start of the line, as the rest of the window.

A window lays out right to left with `MYGO_DIRECTION=rtl`, and in MyGo's
pseudo-language `ar-XB`, English words written right to left (`mygo dev
-lang ar-XB`), whatever MixMusic's languages. The tests in `rtl_test.go`
lay the main screens out right to left on `ui.Tester` and under `ar-XB`;
with `MIXMUSIC_RENDER=<folder>`, `TestRenderRightToLeft` draws them each
way, for a look.

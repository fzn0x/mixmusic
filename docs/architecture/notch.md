# The player around the notch

On a MacBook whose screen has a notch, MixMusic widens the camera housing
with what plays, and a panel grows down from it with the player and the
lyrics (#46). It is a window of its own, as the mini player is: `notch.go`
opens it with a `shell` of its own, and keeps its state in a `notch`.

- **When it shows** (`internal/app/notch.go`): while `notchPlayer` is on,
  the main window is open, in the tray too, and a track is loaded.
  `Options.Notch` opens it, and returns nil where no screen has a housing:
  other Macs, external displays, a closed lid, Windows and Linux. The app
  follows the player's events, the settings (from settings.json too) and
  `mygo.Screen.OnDisplaysChanged`, which closes it and opens it again at
  the housing's new place. It closes with the main window, so it alone
  never keeps MixMusic running. `System.OpenMainWindow` shows the main
  window, at a page through `navigateTo`, and leaves the other windows
  open.
- **The housing** (`notchplace.go`, `notch_darwin.go`): each `NSScreen`
  that answers `auxiliaryTopLeftArea` gives the widths of the menu bar's
  parts beside the housing, their height and the gap between them;
  `housingOf` matches it to the `mygo.Display` of the same
  `NSScreenNumber` that is `Internal`, and places it in MyGo's
  coordinates. The compact window and the open one are sized from it, in
  points: they don't follow the zoom.
- **The window**: MyGo's `Panel`, `Frameless`, `Transparent`, without a
  shadow, shown on every Space with `ShowInactive`. AppKit's part, through
  purego, sets the nonactivating panel's style alone, as `Frameless` keeps
  a titled one that AppKit holds below the menu bar; `NSStatusWindowLevel`;
  `canHide` off; no animation of its own; and `becomesKeyOnlyIfNeeded`
  while compact, so that a click on it leaves the keyboard where it was.
  MyGo tracks the pointer only while the app is active, which the panel
  never makes MixMusic: `notch_darwin.go` adds a tracking area that is
  always on, for hover and the close countdown.
- **Motion**: the window keeps the open size while the panel moves, and
  the root's `Draw` paints the black, whose size `AnimateTo` moves; the
  contents fade with `AnimateColor`. Under Reduce Motion the size goes at
  once, so the panel fades in place instead, at its open size. Once both
  settle, the window takes the compact size, if the panel wasn't opened
  since. The bars are painted from `p.Now()` at 30 frames a second, and
  rest while paused or under Reduce Motion.
- **Closing and the keyboard** (`notchclose.go`): a click on either side,
  Escape, or the pointer away two seconds (`countdown`), which a drag of
  the progress bar or the keyboard's focus on a control holds off. Open,
  the window takes the keyboard (`makeKeyWindow`); closing gives it back
  by ordering the window out and straight back in, as `resignKeyWindow`
  is only a notification. Monitors of presses give it back on a click
  outside, which goes on to what it was on, and count the presses the
  window gets: `keyHandBack` waits for the last one's release, as giving
  the keyboard back between a press and its release cancels MyGo's click.
- **Lyrics** come from `Catalog.Lyrics`, which keeps them, through
  `fetch`: the line sung now, with those before and after it.
- **The setting**'s row is `settings_notch.go`, in Settings › On the
  desktop, only where a housing was found (`hasNotch`, found again as the
  displays change).

The tests draw it with a made-up housing (`notch_test.go`); what only a
Mac with a notch shows, its place over the menu bar, the keyboard going
back and the items beside it, is checked by hand.

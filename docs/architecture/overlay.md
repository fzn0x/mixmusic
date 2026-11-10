# Now playing for overlays

Settings › Desktop › Now playing for overlays serves what plays to the
overlays of streams (#344), on `127.0.0.1` only.

- **`internal/overlay`** is the server. `Apply(on, port)` has it listen,
  stop, or listen on another port; it keeps the status the settings page
  shows, with the problem of a port that another program holds, and tries
  again at the next `Apply`. Its routes are `GET /now-playing`, `/cover`,
  `/events` and `/overlay` (`page.go`), and a request of another method,
  or to a host other than the computer's own, is refused, so that a web
  page can't reach it through a name of its own (DNS rebinding).
- **What plays** comes from the player's report, where the system's media
  controls and Discord get it: `Player.Report` calls `reportOverlay`
  (`internal/app/overlay.go`), which gives the server a `overlay.Info`.
  The server never polls the windows. `Update` keeps the latest state and
  wakes the clients of the event stream only at a change of track, play or
  pause, a seek (a position that is more than two seconds from where the
  track should be), or a new cover; the position that moves on as it
  should tells nothing.
- **Slow and gone clients.** Each client of `/events` has a one-slot
  wake channel and no queue: a client that reads slowly skips the events
  between, and gets the latest state, so that `Update` never waits on
  one. Each write has a deadline of five seconds, which ends the stream of
  a client that doesn't read, and a comment every fifteen seconds finds a
  client that left. Stopping the server, or a client leaving, ends the
  handler's goroutine; `TestSlowAndGoneClients` checks that none stay.
- **The cover** is fetched on `/cover` from what the system's media
  controls use (`App.coverBytes`), not kept by the server.
- **Settings.** `settings.Overlay` has `overlayServer` (off at first) and
  `overlayPort` (8284; a value outside 1024 to 65535 goes back to it).
  `settings_overlay.go` registers the rows with `addSettings`, and reads
  the status from `System.OverlayStatus`. `Options.OverlayListen` lets
  tests listen on a port of `127.0.0.1` that the system picks.

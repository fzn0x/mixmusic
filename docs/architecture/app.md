# The app and the desktop

- **`internal/app`** gives the windows the services (`App.Services`), tells
  them when the settings, the accounts, the add-ons or the updates change
  (`App.Watch`), makes the player, and puts MixMusic on the desktop: the
  tray, media keys, global shortcuts, deep links, notifications, updates.
- **`internal/nowplaying`** shows what the engine plays in the system's media
  controls, and passes on what they ask: Now Playing on macOS, through
  MediaPlayer, and an MPRIS player on Linux, served by GIO's GDBus, both
  called with purego, and the SystemMediaTransportControls of a hidden
  window on Windows, reached through WinRT without cgo. It downloads
  nothing: `internal/app` hands it each cover from the artwork store
  (`controls_cover.go`), as its bytes on macOS and Windows, and on Linux
  as a file in the cache folder's `now-playing/`, whose address MPRIS
  gives the desktop. The notification of a track that starts shows the
  same cover (`notecover.go`), its centre square at most 512 pixels
  across (`artwork.Square`), as the bytes MyGo's notifications take; it
  waits 2 seconds at most for it.
- **`internal/portal`** asks the desktop's Background portal, in a
  Flatpak, to let MixMusic run without a window and to start it at login:
  the sandbox's own autostart folder is one the desktop never reads.
  `internal/app` asks it for Start at login, and for close to the tray.
  [packaging/flatpak](../../packaging/flatpak) holds the Flatpak itself,
  which may read, read-only, the profiles of the browsers installed from
  Flathub: the import of a YouTube Music sign-in lists each apart from the
  same browser installed otherwise (`youtube_cookies_flatpak.go`).
  [packaging/flathub](../../packaging/flathub) builds MixMusic from source
  for Flathub, whose build leaves a mark that `flathub.go` tells it by:
  Flathub updates it, so Settings › About offers no download, and it runs
  the yt-dlp of its manifest rather than download one.
- **`internal/taskbar`** puts previous, play or pause, and next on the
  thumbnail of the main window's taskbar button on Windows, through the
  shell's ITaskbarList3, and takes their clicks in a window procedure in
  front of MyGo's.
- **`internal/keypad`** takes Ctrl and the keypad's +, which MyGo gives its
  windows as no key, so that they zoom in as Ctrl+= does (#251): in a
  keyboard hook of the window's thread on Windows, and on Linux in a
  handler of the GTK window's key presses, which GTK runs before the
  surface's. On macOS, View › Zoom In's ⌘+ takes them (`zoomInKey`).
- **`internal/diag`** writes the log, `mixmusic.log` in the logs folder,
  rotated at a megabyte and without the cookies, hashes and tokens a line
  may hold, and has Go write a crash to `crash.log` there, which the next
  start keeps as `crash-<time>.log` and offers to report.

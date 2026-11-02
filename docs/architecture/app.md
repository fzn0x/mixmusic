# The app and the desktop

- **`internal/app`** gives the windows the services (`App.Services`), tells
  them when the settings, the accounts, the add-ons or the updates change
  (`App.Watch`), makes the player, and puts MixMusic on the desktop: the
  tray, media keys, global shortcuts, deep links, notifications, updates.
- **`internal/nowplaying`** shows what the engine plays in the system's media
  controls, and passes on what they ask: Now Playing on macOS, through
  MediaPlayer, and an MPRIS player on Linux, served by GIO's GDBus, both
  called with purego, and the SystemMediaTransportControls of a hidden
  window on Windows, reached through WinRT without cgo.
- **`internal/portal`** asks the desktop's Background portal, in a
  Flatpak, to let MixMusic run without a window and to start it at login:
  the sandbox's own autostart folder is one the desktop never reads.
  `internal/app` asks it for Start at login, and for close to the tray.
  [packaging/flatpak](../../packaging/flatpak) holds the Flatpak itself.
- **`internal/taskbar`** puts previous, play or pause, and next on the
  thumbnail of the main window's taskbar button on Windows, through the
  shell's ITaskbarList3, and takes their clicks in a window procedure in
  front of MyGo's.
- **`internal/diag`** writes the log, `mixmusic.log` in the logs folder,
  rotated at a megabyte and without the cookies, hashes and tokens a line
  may hold, and has Go write a crash to `crash.log` there, which the next
  start keeps as `crash-<time>.log` and offers to report.

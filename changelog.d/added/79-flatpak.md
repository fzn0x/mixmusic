- A Flatpak for Linux on x64, `mixmusic-<version>-linux-amd64.flatpak`,
  which brings GTK, WebKitGTK, the tray's library and Node.js for yt-dlp
  with it: `flatpak install ./mixmusic-<version>-linux-amd64.flatpak`
  installs it, and fetches the GNOME runtime from Flathub the first time.
  Settings › About calls it a Flatpak and offers each new version's
  file. Inside it, MixMusic shows in the desktop's media controls under
  its Flatpak's name, starts at login through the desktop's Background
  portal, and asks that portal to play on with close to the tray on. Its
  settings and library are its own, in
  `~/.var/app/io.github.fzn0x.mixmusic`, apart from a .deb or AppImage
  copy's: the README tells how to bring them over. Report a problem now
  says how MixMusic was installed (#79).

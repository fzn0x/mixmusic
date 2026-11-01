# Troubleshooting

MixMusic writes what goes wrong to `mixmusic.log`, without your sign-ins:
Settings › About › open logs folder shows it, beside the reports of any
crash. Settings › About › report a problem opens a new issue on GitHub with
your version, system, tools, theme and plugins filled in; attach the log
after reading it. After a crash, MixMusic offers to report it the next
time it starts.

## Common problems

- **A YouTube Music track doesn't play.** It says why: some are not
  available in your country, or only to signed-in adults on YouTube's own
  website. The tracks MixMusic can't find the audio of itself, such as those
  made for children, need yt-dlp: check Settings › Playback that it is
  installed, and a JavaScript runtime found. MixMusic updates the yt-dlp it
  installed every day, as an old one stops finding streams; one of your
  own, chosen there, you keep up to date yourself (`yt-dlp -U`, or your
  package manager).
- **The JavaScript runtime is not found.** Opened from the Finder, the Dock
  or a desktop menu, MixMusic does not get the `PATH` of your terminal. It
  also looks where Homebrew, Deno, Bun, Volta, Scoop and WinGet install;
  elsewhere, choose the runtime in Settings › Playback.
- **YouTube Music signed you out.** Settings › Accounts says so when YouTube
  ends the session, as it does when you sign out of Google everywhere or
  change your password. Sign out there, then sign in again.
- **Google refuses the sign-in window** with "This browser or app may not
  be secure". Google decides this; try again later, or from another
  network. Or sign in with a browser instead, in Settings › Accounts:
  import the session of a browser you signed in to YouTube Music with, or
  paste the Cookie header of a request of a signed-in music.youtube.com
  tab.
- **Importing a browser's session fails.** Close the browser first: while
  it runs, it may hold its cookies, or not have saved a new sign-in yet.
  Recent Chrome on Windows seals its cookies against other programs:
  import from Firefox there, or paste a cookie header. On macOS, reading
  Safari's cookies takes Full Disk Access for MixMusic, in System
  Settings › Privacy & Security. Zen and Helium are read from their
  profile folders: give the folder when MixMusic doesn't find it.
- **Spotify plays from YouTube Music, not in MixMusic.** Spotify plays in
  MixMusic on Windows, with Premium, and with a sign-in made since MixMusic
  played Spotify itself: Settings › Playback asks you to sign in again when
  yours is older.
- **Spotify refuses the sign-in.** A Spotify app in development mode serves
  only while its owner has Premium, and only the accounts added under Users
  Management in its settings, by their Spotify email. Settings › Accounts
  says which it was: see
  [Spotify](spotify.md#when-spotify-refuses-the-sign-in).
- **No tray icon on Linux.** The tray needs `libayatana-appindicator3` and a
  desktop that shows AppIndicator icons; GNOME needs its AppIndicator
  extension. Without it, closing the window quits MixMusic.
- **Linux does not start it.** Install GTK 3, as the
  [README](https://github.com/fzn0x/mixmusic#the-first-launch) says.
- **macOS cannot check the app,** as it is not notarized yet: open it from
  System Settings › Privacy & Security › Open Anyway.

## Known issues

- MixMusic reads YouTube Music through the interfaces of its own website,
  and plays it through YouTube's player API and yt-dlp. When YouTube
  changes them, searching, playing or signing in can stop working until
  MixMusic or yt-dlp catches up.
- YouTube Music has no sign-in for other apps: MixMusic signs in with a
  browser session, which can do with your account what YouTube Music can.
  Treat it like a password; sign out in Settings › Accounts to delete it.
- Spotify plays in MixMusic itself only on Windows, whose webview,
  WebView2, plays Spotify's protected audio: WebKitGTK on Linux does not,
  and macOS's is not known to. Elsewhere, Spotify Connect plays it.
- Spotify allows apps in development mode up to five users, while their
  owner has Premium, and lets them read less: search shows ten results of
  each kind, and the tracks of playlists you do not own stay hidden.
- Tracks YouTube restricts to some countries, to signed-in adults or to a
  channel's members do not play, as streams are found signed out.
- The macOS app is not notarized and the Windows installer is not signed
  yet, so the first launch asks you to allow it.
- The portable zip does not register `mixmusic://` links, and Windows on Arm
  is less tested than the rest.

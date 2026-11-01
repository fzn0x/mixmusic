# Changelog

All notable changes to MixMusic are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- The main window's title names the track that plays, for the taskbar,
  the window switcher and screen readers, and starts with "Paused:" while
  it's paused. Turn off "Show what plays in the window title" in
  Settings › Desktop to keep it to MixMusic's name (#105).

### Changed

- A new icon: two pink rings and a play button on black, in the app, the
  tray, the website and the README.
- MixMusic is dual-licensed: use it under the MIT license, as before, or
  under the GNU General Public License, version 3, whichever you prefer.
  Settings › About says so.
- Your Discord profile shows what plays without setting anything up:
  MixMusic has its own Discord application now, so you no longer create
  one and paste its ID. It is on unless you turn off Show what plays on
  Discord in Settings › Desktop, where you can still paste the ID of an
  application of yours to show its name. If you had set up your own
  application and turned it off, it stays off.
- Discord's member list says "Listening to MixMusic" rather than the
  artists, for Spotify and YouTube Music tracks alike; your profile still
  shows the title, the artists and the artwork. Discord's own Spotify
  status comes from your Discord account's link to Spotify: to show only
  MixMusic, turn off Display Spotify as your status in Discord's
  Connections.
- The Windows installer offers to close a running MixMusic before it
  upgrades it, and stops one that keeps running in the tray.

### Fixed

- On macOS 26 and 27, macOS no longer runs an AutoFill helper for
  MixMusic's windows, which took CPU and could stall scrolling (#85).
- On macOS, Cmd+Q, the Dock's Quit and the tray menu's Quit end MixMusic
  even when a restart, as after an update, left it running without a
  window (#86). The restart that installs this version still runs the old
  version's code.
- On Windows, a frame the screen never takes no longer holds up MixMusic
  for a second (#87).
- On macOS, closing the window in full screen with "Close to the tray"
  on no longer leaves an empty, black Space: the window leaves full
  screen, then hides in the tray, and comes back in full screen (#84).
- After a restart, a queue of more than 500 tracks comes back on the
  track that was playing, where it was, with what comes up after it.
  MixMusic kept the queue's first 500 tracks, and when the track playing
  was past them it came back on the 500th, at the other track's place;
  it now keeps the 500 tracks around the one playing (#88).
- Changes to the queue made while a song is paused, or before anything
  plays after a start, are kept when MixMusic quits, as are the changes
  made right before quitting: the queue, where the song was, the volume
  and the colours of Settings › Appearance. This holds when Windows
  shuts down, restarts or signs out too (#89).
- Shuffle mixes in the tracks before the one playing that haven't played
  yet, as when you start an album or a playlist from one of its tracks:
  it left them out, and with Repeat all they played in the list's order.
  The track playing then comes first, so Previous starts it again. Your
  own tracks stay ahead of the radio's when you shuffle, and turning
  shuffle off puts the tracks you added with Play next or Add to queue
  back where you added them, after a restart too (#95).
- After a restart, the radio's tracks keep their "radio" mark in up
  next, and Add to queue still puts tracks before them (#95).
- On Windows, a sign-in the Credential Manager can't keep is kept
  encrypted for your Windows account, so you stay signed in after a
  restart. Sign-in files in plain text found in MixMusic's data folder,
  such as from a copy made on Linux, move into the system's credential
  store and are deleted, on Linux too once a keyring is there (#104).

## [0.5.5] - 2026-10-28

Pages scroll smoothly on every system, even with thousands of liked songs
(#59). A song that loads plays rather than pausing, and Exit in the tray
menu quits MixMusic with "Close to the tray" on. MixMusic 0.3.0 and later
offer this version in Settings › About.

### Fixed

- Pages scroll smoothly (#59): liked songs, playlists, the history and
  the libraries of Spotify and YouTube Music, and the shelves of Home,
  Explore and the artists' pages. Every frame built every row and tile of
  the page, so that a list of a thousand songs scrolled at a few frames a
  second; MixMusic now builds only what is near the window, as fast for
  five thousand songs as for a hundred, and sorts and searches a list
  when it changes, not in every frame.
- A song that loads can no longer be paused, from the play button, the
  media keys or the tray: it plays once it loaded. Choosing another song
  still leaves it. The play button shows that the song loads.
- Exit, in the tray icon's menu, quits MixMusic again with "Close to the
  tray" on: it closed the window to the tray instead, and MixMusic kept
  running. Quit (Cmd+Q) on macOS did the same.

## [0.5.4] - 2026-10-24

On Linux, signing in to YouTube Music no longer closes MixMusic (#51).
MixMusic 0.3.0 and later offer this version in Settings › About.

### Fixed

- On Linux, signing in to YouTube Music no longer closes MixMusic as the
  sign-in window opens (#51). MixMusic now asks whether WebKitGTK is
  there on the thread that opens windows, where WebKit has to start.

## [0.5.3] - 2026-10-23

Signed in to Spotify, MixMusic no longer flickers between signed in and
signed out, and when Spotify refuses the sign-in, MixMusic says why
rather than that you signed in. On Linux, MixMusic no longer damages its
own memory as its first window opens. Signing in to YouTube Music can
still close MixMusic on Linux (#51): that fix is planned for 0.5.4.
MixMusic 0.3.0 and later offer this version in Settings › About.

### Fixed

- After signing in to Spotify, the library, Settings › Accounts and
  Settings › Playback no longer flicker between signed in and signed out
  (#50). When Spotify refuses the sign-in, Settings › Accounts now says
  why and offers to sign out, and MixMusic asks Spotify again a minute
  later, not several times a second.
- A Spotify sign-in that Spotify refuses, as it refuses every app whose
  owner has no Premium, no longer says "Signed in to Spotify": it tells
  Spotify's reason instead.
- On Linux, on computers with a GPU, MixMusic no longer damages its own
  memory as its first window opens, which could crash it later: it now
  uses MyGo 0.6.2.

## [0.5.2] - 2026-10-22

The arrow keys seek and change the volume with nothing to set first. On
Windows the music no longer stutters while a game or a browser keeps the
computer busy, and on Linux closing the mini player no longer crashes
MixMusic. MixMusic 0.3.0 and later offer this version in Settings › About.

### Changed

- The arrow keys seek and change the volume with no shortcut to set first:
  left and right go back and forward 10 seconds, up and down turn the
  volume up and down. They took Shift and Ctrl (⌘ on macOS) before, and a
  shortcut you set in Settings › Shortcuts stays yours. The search box
  keeps the arrows for its cursor, and sliders, lists and the stats page's
  chart keep them while they have the focus. Up and down no longer scroll
  a page line by line: the mouse wheel, Page Up and Page Down, and Home
  and End do. The mini player takes no shortcuts (#38).

### Fixed

- On Linux, closing the mini player, with its close button or with Open
  MixMusic, no longer crashes MixMusic (#37).
- The settings icon in the sidebar is a gear again, centred and as large as
  the icons beside it, where it looked like a light bulb (#40).
- On Windows, the music no longer stutters while a game or a browser keeps
  the computer busy: MixMusic's audio now goes ahead of other programs, as
  the audio of browsers and other players does.

## [0.5.1] - 2026-10-22

A track no longer fails now and then with "YouTube refused to send its
audio", only to play when you try it again, and a copy built on a Mac for
its own processor installs new versions again. MixMusic 0.3.0 and later
offer this version in Settings › About.

### Fixed

- A track no longer fails now and then with "YouTube refused to send its
  audio". YouTube sends a track's audio only a few seconds after it gave
  out its address, and MixMusic asked at once, then again at a new
  address, refused as well. It now waits until the moment yt-dlp tells, so
  a track MixMusic hadn't looked up ahead starts a few seconds later
  instead.
- A copy of MixMusic built on a Mac for that Mac's processor, as
  `bun run build` makes it there, installs new versions again: it asked for
  an update file that releases did not publish, and failed with a 404.
  Releases publish it now, and check before they publish that every app
  finds its update (#33, #34).

## [0.5.0] - 2026-10-22

MixMusic draws its own interface, and plays through an audio engine of its
own, without the webview it ran in: it takes far less memory. Spotify plays
in MixMusic itself on Windows, with no Spotify app open, plugins move to a
new API, and a new version asks to install in a pop-up. Spotify Connect no
longer shows a track paused while your device plays it, and goes on to the
next track.

Four new themes join the others, a search says how many times each song
played, and on Windows the media flyout and the taskbar's thumbnail control
what plays. MixMusic 0.3.0 and later offer this version in Settings ›
About. The queue of 0.4.0, which its webview kept, doesn't come along: the
player starts on an empty queue once, and your library, mixes and settings
stay.

### Added

- MixMusic draws its interface itself, on the GPU: every page, the player
  bar, the queue and lyrics, the mini player and the command palette, in
  every theme. The built-in themes' effects come along: Mix's tile
  textures, Aurora's drifting lights and frosted glass, Sakura's petals,
  and Terminal's scanlines, glow and phosphor-green artwork (#4).
- MixMusic's own audio engine: it decodes the audio of YouTube Music itself
  and plays it on your audio device, with the sound the webview gave it:
  the equalizer, crossfades, volume levelling and the visualizer work as
  before. It shows what plays in the system's media controls, which the
  media keys and headphones reach as before: Windows' media flyout, with
  the artwork, the buttons and the seek bar, Now Playing on macOS, and the
  desktop's player controls on Linux.
- On Windows, the thumbnail of MixMusic's taskbar button has previous, play
  or pause, and next buttons, as other music players have (#13).
- Spotify plays in MixMusic itself, on Windows: choose in mixmusic, from
  spotify in Settings › Playback. MixMusic plays at Spotify's own quality,
  with no Spotify app open, and shows among your Spotify devices while it
  plays. It needs Spotify Premium; new installs on Windows play Spotify
  tracks this way. Sign in to Spotify again once, for Spotify to let
  MixMusic play.
- Plugins of a new API, 2: each runs in a JavaScript engine of MixMusic's
  own, and adds pages, widgets, sidebar items, track actions and commands,
  made of elements MixMusic draws in your theme and of canvases the plugin
  draws on. The community's sleep timer and spectrum are updated for it,
  and docs/plugins.md tells how to move a plugin.
- Settings › About shows the licenses of the code and the faces MixMusic
  includes: libopus, Chromium, Selawik and Quicksand.
- A new version asks to install in a pop-up, unless MixMusic installs
  updates without asking. Later asks again at the next start, skip this
  version at the next version. Copies that cannot replace themselves, the
  .deb, the .rpm and the AppImage, offer the new version's download there.
- Copy link in the menus of albums, playlists and artists, on their pages
  and their tiles, as tracks have it.
- Four new themes. Greenroom: charcoal and one jade light, its ground
  washed down from the top in the colour of the cover playing (#8).
  Glasshouse: white glass and a rose-red accent, the cover's colours
  glowing under translucent panels (#7). Anime: summer-sky blues and cel
  colours, speed lines that burst as each track starts, and manga
  screentone (#7). Mochi: kawaii candy pastels in a round face, with
  sparkles drifting up (#7).
- A search's songs say how many times each played on YouTube Music, or
  how many views a video has: "1.2B plays", "905M views", so that the
  original stands out of the covers. Spotify gives apps no such count.

### Changed

- Themes set the colors, fonts and corners MixMusic draws everything with.
  Themes made for the web interface still load, without their stylesheet
  and options, which nothing draws now. The labels of the sidebar take a
  theme's own face too, as Terminal's monospaced one.
- On Linux, MixMusic plays through PulseAudio, which PipeWire serves too,
  or ALSA, and no longer needs GStreamer; WebKitGTK is needed only to sign
  in to YouTube Music.
- MixMusic looks for a new version at start and every few hours, no longer
  once a day. Copies that cannot replace themselves look too, and Settings ›
  About turns that off.
- A Spotify track that plays from YouTube Music instead says why once, not
  at every track.
- On Windows and Linux, the main window has the system's own title bar and
  buttons, as asked in #6. On macOS, the traffic lights stay over the
  window.

### Removed

- The web interface. MixMusic opens a webview only for Google's sign-in to
  YouTube Music and, on Windows, for Spotify's player.
- Your own CSS, in Settings › Appearance.
- Plugins made for the web interface: they no longer run, as Settings ›
  Plugins says, until their authors move them to the new API.
- A track YouTube serves without Opus audio, which is rare, no longer plays:
  the webview played those.

### Fixed

- MixMusic no longer opens on a black window for up to 40 seconds when
  Spotify or YouTube Music is slow to answer: it shows at once, and the
  names of your accounts follow.
- On Spotify Connect, a track no longer shows paused while your device plays
  it: MixMusic took Spotify's late reports, and the copy of a track Spotify
  plays in your country, for someone else playing on the device.
- On Spotify Connect, the next track plays when one ends, where MixMusic
  waited on the track that ended.
- A Spotify device that takes a track but waits, paused, is told once more to
  play it.
- Skipping a track in its last seconds on Spotify Connect no longer skips the
  next one too.
- Moving through a Spotify track before it plays, as after a restart, no
  longer starts it on your device behind MixMusic's back.
- The title bar and its buttons are light or dark as the theme you wear
  is: on a light theme while the computer was dark, the buttons showed
  white on white.
- On a screen smaller than the window, as a laptop's, the window opens
  fitting it, where its title bar and buttons were above the screen, out
  of reach (#6). A window the last run left there comes back within it.
- A link you copy is no longer MixMusic's own address, as
  mygo://localhost/…, which opens nothing: Copy link copies the link on
  YouTube Music or Spotify, which anyone can open (#15).
- Choosing another track stops the one that plays at once. It played on
  under the new track's title, artwork and lyrics while MixMusic found the
  new track's audio, which can take a while on macOS, and for good when
  that failed.
- When Spotify refuses the account after you sign in, Settings › Accounts
  says why and what to do, where it showed only "spotify: HTTP 403": the
  account isn't one of your app's users, to add under Users Management in
  its settings, or the app's owner has no Spotify Premium, which Spotify
  asks of apps in development mode since February 2026. The steps to
  connect Spotify ask for both beforehand, and a refused sign-in can be
  signed out of.
- The visualizer of the now playing page, and the analyser plugins read,
  as the spectrum, move for Spotify tracks too, on Windows: MixMusic hears
  Spotify where it plays on this computer, in MixMusic or in the Spotify
  app on Spotify Connect, through Windows' process loopback, which hears
  one app's audio and never records it. Where MixMusic can't hear the
  track, as on another device, the visualizer shows nothing, not a flat
  line.

## [0.4.0] - 2026-10-20

MixMusic tells you when a yt-dlp you installed yourself is out of date,
how to update it, and offers its own, which stays up to date. MixMusic
0.3.0 and later offer this version in Settings › About.

### Added

- When the yt-dlp MixMusic runs is your own, as from winget, Homebrew or
  pip, and older than the latest release, Home and Settings › Playback say
  so. They give the command that updates it, when its folder tells how you
  installed it, and offer MixMusic's own yt-dlp, which stays up to date.
  A report of a problem says when yt-dlp is out of date.

## [0.3.2] - 2026-10-20

A track whose audio fails to load says why, and stops the queue only when
three in a row cannot play. MixMusic 0.3.0 and 0.3.1 offer this version in
Settings › About.

### Fixed

- A track whose audio fails to load says why, such as YouTube refusing it
  or its servers out of reach, instead of "the audio format is not
  supported", and the log keeps the reason. It counts as one track that
  could not play, not two or three, so the queue no longer stops at the
  first.

## [0.3.1] - 2026-10-20

Discord shows what you play on Windows, which it never did. MixMusic 0.3.0
offers this version in Settings › About.

### Fixed

- On Windows, your Discord profile shows what plays once you turn it on and
  paste your application ID: MixMusic looked for Discord in the wrong place,
  found it not running, and said nothing.

## [0.3.0] - 2026-10-20

Playlists of your own and a brand channel on YouTube Music, an About page
that shows the newest version and installs it, and downloads for more
systems. Signing in to YouTube Music no longer freezes MixMusic, and the
name of every page fits the window, with all of its sections. Copies of
0.2.0 from an installer or install.sh offer to update to this version;
install the .deb by hand.

### Added

- Make playlists on YouTube Music, from the library or from a track's or a
  list's menu, add tracks of either service to them, and take tracks out of
  your own playlists.
- Use YouTube Music as a brand channel of your account, with its own home
  feed, library and likes, chosen in Settings › Accounts.
- What you play joins your YouTube Music history, which its home and its
  radios are made from; a switch in Settings › Accounts turns it off.
- A log of what goes wrong, without your sign-ins, and the report of a
  crash, which the next start offers to send. Settings › About opens the
  logs folder, and reports a problem with the version and setup filled in.
- Going back or forward returns to where the page was scrolled.
- Downloads for more systems: a portable zip for Windows, an RPM package
  for Fedora and openSUSE, and an AppImage for any Linux.
- Settings › About shows the newest version. A copy that updates itself
  installs it there, with its progress, and restarts into it; the `.deb`,
  the `.rpm` and the AppImage download their own file of it.

### Changed

- The name of a page and its sections shrink to fit the window, and a long
  name, such as a search, an album or a playlist, wraps, instead of running
  off the edge: every section of Settings shows, About included. Tiles,
  empty pages and settings follow the room the side panels leave too.
- Tracks play signed out also when you signed in to YouTube Music: with your
  session, yt-dlp found fewer streams, often none it could play. No file
  keeps a copy of the session for it any more, and the audio quality of
  your account no longer applies.
- MixMusic updates the yt-dlp it installed, once a day.
- Discord's member list shows the artists you listen to, instead of
  MixMusic.
- The Debian package installs the libraries of the tray icon and of the
  keyring.

### Fixed

- Signing in to YouTube Music no longer leaves a blank window and freezes
  MixMusic, its search included, once Google sends the window back to
  YouTube Music, which it does at once when it remembers you.
- Home shows all of its shelves, not only those YouTube sends first.
- Your saved playlists, albums and artists of YouTube Music show in full,
  not only their first page.
- Opened from the Finder, the Dock or a desktop menu, MixMusic finds yt-dlp
  and Deno, Node.js or Bun installed with Homebrew, or by Deno's and Bun's
  own installers.
- When YouTube Music ends your session, MixMusic says so and asks you to
  sign in again, instead of showing an empty library.
- A track that cannot play says why in plain words, such as being not
  available in your country, instead of yt-dlp's message.
- The radio skips other uploads, and the music videos, of songs already in
  the queue.
- After three tracks in a row that cannot play, the queue stops instead of
  skipping through all of it, and a last track that cannot play still
  starts the radio.
- On macOS, a click on the Dock icon shows the window closed to the tray.
- Discord shows what plays from its Canary build, and from Vesktop,
  installed with Flatpak.

## [0.2.0] - 2026-10-19

Your own YouTube Music in MixMusic, an explore page, Last.fm and Discord, and
updates that install themselves. MixMusic 0.1.0 cannot update itself, so
install this version by hand once; the versions after it come to the app.

### Added

- Sign in to YouTube Music, in a window showing Google's own page: your home
  feed and library (liked songs, playlists, albums, artists), likes that reach
  YouTube Music, and the audio quality of your account.
- Save albums and playlists, and follow artists, on their service, YouTube
  Music or Spotify, from their pages.
- Explore: new albums and singles, trending songs, the charts, and moods and
  genres, each with its playlists.
- Related songs, playlists and artists at the foot of the now playing page.
- Scrobbling to Last.fm, with your own API account; plays sent later when
  offline, and a switch to pause it.
- What plays on your Discord profile, with the artwork and the time left.
- Updates: MixMusic checks the latest release once a day and installs it when
  you agree, or by itself if you choose.
- Change the keys of every command, plugins' too, in Settings › Shortcuts.
- Follow the computer's light or dark mode with a theme for each.
- The minutes listened day by day on the stats page, also as a table.
- Search inside a playlist, a mix or the liked songs, and sort them by title,
  artist or length.
- Show synced lyrics as plain text.

### Changed

- Home shows YouTube Music's home feed; what was its explore page moved to
  Explore.

## [0.1.0] - 2026-10-19

The first release of **MixMusic**: Spotify and YouTube Music in one desktop
player, for macOS, Windows and Linux.

### Added

- Search YouTube Music and Spotify at once, with search suggestions.
- One queue for tracks of both services: shuffle, repeat, crossfade, and a
  radio carrying on when the queue runs out.
- Spotify tracks on Spotify Connect, or from their recording on YouTube
  Music, without Premium.
- Home shelves of both services and of your own listening; album, artist and
  playlist pages.
- Liked songs and mixes holding tracks of every service; import of a
  playlist into a mix; history and listening stats.
- Synced lyrics, a visualizer, a 10-band equalizer, volume levelling and an
  offline cache of played tracks.
- The tray, media keys, notifications, a mini player, start at login and
  `mixmusic://` links.
- Six themes, theme options, an accent following the artwork, your own CSS,
  and plugins with a typed API.
- A marketplace of community themes and plugins.
- Apps for macOS 12 and later (Apple silicon and Intel), Windows 10 and 11,
  and Linux, on x64 and Arm64.

[Unreleased]: https://github.com/fzn0x/mixmusic/compare/v0.5.5...HEAD
[0.5.5]: https://github.com/fzn0x/mixmusic/compare/v0.5.4...v0.5.5
[0.5.4]: https://github.com/fzn0x/mixmusic/compare/v0.5.3...v0.5.4
[0.5.3]: https://github.com/fzn0x/mixmusic/compare/v0.5.2...v0.5.3
[0.5.2]: https://github.com/fzn0x/mixmusic/compare/v0.5.1...v0.5.2
[0.5.1]: https://github.com/fzn0x/mixmusic/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/fzn0x/mixmusic/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/fzn0x/mixmusic/compare/v0.3.2...v0.4.0
[0.3.2]: https://github.com/fzn0x/mixmusic/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/fzn0x/mixmusic/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/fzn0x/mixmusic/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/fzn0x/mixmusic/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/fzn0x/mixmusic/releases/tag/v0.1.0

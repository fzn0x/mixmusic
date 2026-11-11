# How MixMusic works
<!-- docs: group=inside order=10 -->

MixMusic is a native desktop player for YouTube Music and Spotify, written in Go. Here is how its engine, streams, Spotify, services and updates fit together.

This page is the big picture for people who use MixMusic and want to know
what happens behind a click. The pages after it go deeper: the
[audio engine](audio-engine.md), [where the audio comes from](streams.md),
[how Spotify tracks play](spotify-playback.md) and
[your data and privacy](data-and-privacy.md). For the code itself, read
[the architecture](architecture/README.md).

## One program, drawn by itself

MixMusic is one Go program. [MyGo](https://github.com/egoist/mygo), the
desktop framework it is built with, draws every window in Go, on the GPU:
the pages, the player bar, the queue, the lyrics, the mini player, the
command palette and, on a MacBook with a notch, the player around the
camera. There is no browser engine inside MixMusic drawing its interface,
as there is in an Electron app, and no web page behind the windows.

A web view of the system (WebView2 on Windows, WebKit on macOS, WebKitGTK
on Linux) opens only for three things, and only while it's needed:

- **Spotify's player**, on Windows, when Spotify tracks play in MixMusic
  itself. It opens at the first Spotify track that plays there.
- **Google's sign-in page**, when you sign in to YouTube Music.
- **The picture of music videos**, when "Show the video of music videos"
  is on in Settings › Playback. It closes a minute after the last picture.

MixMusic builds without a C compiler: it calls the system's libraries
while it runs, for the audio output, the media controls, the tray and the
credential store, partly through
[purego](https://github.com/ebitengine/purego).

## From a click to your speakers

When you play a YouTube Music track, this happens:

1. **The player takes the track.** One part of MixMusic owns the queue and
   what plays; the network calls run beside it, so the window never waits
   on them.
2. **MixMusic finds the track's audio.** It asks YouTube's player API
   itself, as one of YouTube's own apps does, signed out. For the few
   tracks YouTube won't give it that way, such as those made for
   children, it runs [yt-dlp](https://github.com/yt-dlp/yt-dlp).
   [Where the audio comes from](streams.md) tells the details.
3. **MixMusic's stream proxy fetches the audio.** It serves a download or
   the offline cache first, and otherwise fetches the stream
   from YouTube in chunks of 4 MB, in the background, so that seeking
   back costs nothing.
4. **The audio engine plays it.** MixMusic decodes the audio itself, in
   Go, converts it to 48 kHz, turns a loud track down to an even
   loudness, applies the speed, the equalizer, volume levelling and the
   volume, and plays it on your system's output.
   [The audio engine](audio-engine.md) tells the details.
5. **The next track gets ready.** A second into a track, MixMusic finds
   the next one's stream; ten seconds in, it fetches its audio, so that
   it starts at once when its turn comes.

A Spotify track plays one of three ways, which you choose in Settings ›
Playback › Spotify tracks: in MixMusic through Spotify's own player, on a
Spotify Connect device, or from its recording on YouTube Music through
MixMusic's own engine. [How Spotify tracks play](spotify-playback.md)
compares them.

## YouTube Music

MixMusic reads YouTube Music through the API YouTube Music's own website
uses. It needs no account and no API key:

- **Signed out**, it reads search and its suggestions, the home and
  explore pages, the charts, moods and genres, albums, artists,
  playlists, radios, podcasts and plain lyrics.
- **Signed in**, with the cookies of a Google session, it signs its
  requests as the website does, and reads and changes your library:
  liked songs, playlists, albums, subscriptions, likes, dislikes and
  saved shows. It can act as one of your brand channels.

Audio is a separate matter: MixMusic always finds streams signed out, even
while you're signed in, as YouTube gives an account's requests fewer
streams.

## Spotify

MixMusic reads Spotify through the Spotify Web API: search, albums,
artists, playlists, your library and your Spotify Connect devices. You
sign in with a Spotify app of your own, as Spotify allows a development
app only five users and only while its owner has Premium:
[Spotify](spotify.md) walks you through it. The sign-in happens in your
browser, with PKCE, and comes back to MixMusic at
`http://127.0.0.1:47823/callback` on your own computer.

## The services it talks to

MixMusic has no server of its own. Apart from YouTube and Spotify, it
talks to:

- **Lyrics:** [Better Lyrics](https://betterlyrics.org), for lyrics timed
  word by word, first, then [Unison](https://unison.boidu.dev), an open,
  crowdsourced database of lyrics, then [LRCLIB](https://lrclib.net), then
  YouTube Music's own lyrics. Unison also translates lyrics, through
  Google, and romanizes Arabic, but only while you turn those on.
- **Latin letters for lyrics** are made on your computer for every other
  script. Japanese romaji reads kanji with a dictionary MixMusic downloads
  from GitHub the first time you turn romaji on (11.7 MB); the readings of
  Chinese are built in.
- **Last.fm**, once you connect it: what you play, to scrobble.
- **Discord**, through the Discord app on your computer, over its local
  pipe or socket: what plays, for your profile.
- **Cast devices** on your network, such as Chromecasts and Nest
  speakers, which MixMusic finds and speaks to itself, in Go.
- **GitHub**, for new versions of MixMusic, for yt-dlp, for the community
  marketplace, and for the kanji dictionary.

To tell whether the network answers, MixMusic asks YouTube's and Spotify's
addresses over HTTPS. [Your data and privacy](data-and-privacy.md) lists
what each of these gets.

## Themes, plugins and the marketplace

- **Themes** are a `theme.json` file of colours, fonts and corners.
  MixMusic draws every theme in the layout you choose in Settings ›
  Appearance: Blend (the default), Expressive, Crate or Tiles. It can
  also take its colours from a file that pywal or matugen write, and
  follows the file as it changes. [Themes](themes.md) lists every token.
- **Plugins** are JavaScript modules, usually compiled from TypeScript.
  Each plugin you enable runs in a JavaScript engine of its own,
  [Sobek](https://github.com/grafana/sobek), written in Go, with
  `console`, timers, `fetch` and MixMusic's plugin API. A plugin's pages
  and widgets are elements that MixMusic draws in your theme, and its
  canvases are drawing calls that MixMusic paints. The permissions a
  plugin lists are shown to you, not enforced, so MixMusic asks you to
  trust a plugin before it enables it. [Plugins](plugins.md) documents the
  API.
- **The marketplace** reads the community's registry from this
  repository on GitHub when you open it, and installs an add-on from the
  repository's registry branch, where the project's CI publishes each
  add-on once its pull request is merged.

## Updates

### MixMusic itself

- **When it looks.** MixMusic asks GitHub for its newest release 10
  seconds after it starts, then every 6 hours, and when the computer
  wakes after a check was due. A check that fails is tried again an hour
  later. Settings › About › Updates has the switches: Check for updates,
  and Install updates without asking. A development copy, run with
  `bun run dev`, never looks.
- **What it offers.** A new version asks to install in a pop-up, unless
  you chose to install updates without asking. Later asks again at the
  next start, or after a day; skip this version waits for the version
  after it.
- **How it installs.** The installers, the portable zip, the macOS app and
  the Linux archive replace themselves. They download the update file of
  the release made for their system, or a smaller delta from the version
  before. Each is signed with MixMusic's key, whose public half is built
  into the app: a file whose signature doesn't match is never installed.
  The new version runs once MixMusic restarts.
- **Copies that can't replace their own files**, the `.deb`, the `.rpm`,
  the AppImage and the Flatpak, still look for new versions, and offer the
  new version's file for you to install.

### yt-dlp and the JavaScript runtime

MixMusic needs yt-dlp for some tracks, and yt-dlp needs a JavaScript
runtime to solve the challenges YouTube sends. Settings › Playback ›
Tools shows both.

- **MixMusic's own yt-dlp** is the one you install from Settings ›
  Playback or Home. MixMusic downloads it from yt-dlp's releases on
  GitHub, checks it against the checksums the release publishes, and
  keeps it in the `bin` folder of its data folder. On macOS it installs
  yt-dlp's unpacked build, which starts in 0.2 seconds rather than 8. A
  minute after each start, and then once a day, MixMusic replaces it when
  a newer release is out.
- **A yt-dlp of your own**, from a package manager, pip, Homebrew or
  winget, or the one you name under Own yt-dlp, is never changed.
  MixMusic asks GitHub for the latest release at most every 6 hours, and
  when yours is older, Home and Settings › Playback say so, with the
  command that updates it when its folder tells how you installed it.
  They also offer MixMusic's own instead.
- **The JavaScript runtime** is yours: MixMusic installs none. It looks
  for Deno, Node.js, Bun and QuickJS, in that order, in its `bin` folder,
  on your `PATH`, and where Homebrew, Deno, Bun, Volta, Scoop and WinGet
  install them, or takes the one you choose under JavaScript runtime. You
  update it the way you installed it. The Flatpak brings Node.js, which
  updates with the Flatpak.

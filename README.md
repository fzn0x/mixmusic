<p align="center">
  <img src="resources/icon.png" width="112" height="112" alt="MixMusic's icon: two pink rings and a play button on black" />
</p>

<h1 align="center">MixMusic</h1>

<p align="center"><strong>One player for YouTube Music and Spotify.</strong></p>

<p align="center">YouTube Music and Spotify client for macOS, Windows and Linux. YouTube Music works with no account. One search, one queue, mixed playlists, synced lyrics, themes and plugins.</p>

<p align="center">
  <a href="https://github.com/fzn0x/mixmusic/actions/workflows/ci.yml"><img src="https://github.com/fzn0x/mixmusic/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="https://github.com/fzn0x/mixmusic/actions/workflows/pages.yml"><img src="https://github.com/fzn0x/mixmusic/actions/workflows/pages.yml/badge.svg" alt="Website" /></a>
  <a href="https://github.com/fzn0x/mixmusic/releases/latest"><img src="https://img.shields.io/github/v/release/fzn0x/mixmusic" alt="Latest release" /></a>
  <a href="https://github.com/fzn0x/mixmusic/releases"><img src="https://img.shields.io/github/downloads/fzn0x/mixmusic/total" alt="Downloads" /></a>
  <a href="https://pkg.go.dev/github.com/fzn0x/mixmusic"><img src="https://pkg.go.dev/badge/github.com/fzn0x/mixmusic.svg" alt="Go Reference" /></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/fzn0x/mixmusic" alt="Go version" /></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey" alt="macOS, Windows and Linux" />
  <a href="#license"><img src="https://img.shields.io/badge/license-MIT%20or%20GPL--3.0-blue" alt="MIT or GPL-3.0" /></a>
</p>

<p align="center">
  <a href="https://fzn0x.github.io/mixmusic/">Website</a> ·
  <a href="https://github.com/fzn0x/mixmusic/releases/latest">Download</a> ·
  <a href="https://fzn0x.github.io/mixmusic/docs/">Docs</a> ·
  <a href="CHANGELOG.md">Changelog</a>
</p>

<p align="center">
  <img src="docs/banner.webp" alt="MixMusic's pages in the Mochi theme, standing upright on a dark stage and facing you: now playing アイドル by YOASOBI in the middle, outlined in pink, with the mini player in front of it, and home, search, the library, a mix, explore, stats, settings and the marketplace on both sides" />
</p>

## Why MixMusic

If your music is split between YouTube Music and Spotify, you usually need
two apps, with two searches, two queues and two libraries. MixMusic puts
both services in one player:

- **One search.** It looks through YouTube Music and Spotify at once, and
  marks each result with the service it comes from.
- **One queue and one library.** A Spotify track can play right after a
  YouTube Music one, and a playlist can hold tracks from both.
- **No account needed.** YouTube Music plays as soon as you open the app.
  Sign in only if you want your own library. Spotify is optional.
- **A small program.** MixMusic is one Go program that draws its own
  windows on the GPU. There is no browser inside it to download or keep in
  memory.
- **Yours to change.** Ten themes come with it, the community adds more
  themes and plugins, and any command can have a key of your own.
- **Private.** MixMusic has no account, server, analytics or telemetry of
  its own.
- **Free and open source,** under the MIT license or the GPL 3.0.

## Features

### New in 0.5.6

- **Zoom.** <kbd>Ctrl</kbd> <kbd>=</kbd> and <kbd>Ctrl</kbd> <kbd>-</kbd>
  make everything larger or smaller, from 80% to 200%, and
  <kbd>Ctrl</kbd> <kbd>0</kbd> goes back to 100% (<kbd>⌘</kbd> on macOS).
  The mini player zooms too.
- **Even loudness.** Loud YouTube Music tracks are turned down to one level,
  from YouTube's own measure of each track, so no song jumps out. It is on
  by default, in Settings › Playback.
- **AAC.** Tracks that YouTube Music has only in AAC now play. The audio
  engine decodes AAC itself, as it does Opus.
- **An even volume slider.** Each tenth of the slider is 6 dB, so the quiet
  end no longer jumps.
- **Covers offline.** The covers you saw are kept on disk, up to 256 MB, and
  show at once at the next start, even without a connection.
- **Steadier playback.** YouTube Music tracks start in a moment. When the
  network drops, the song waits and then plays on from where it stopped.
  MixMusic pauses when the computer goes to sleep.
- **Discord with no setup.** Your profile shows what plays, as "Listening
  to MixMusic".
- **The track in the window title,** for the taskbar and window switchers.
- **A new icon:** two pink rings and a play button on black.
- **Plugins in TypeScript.** The plugin API and the community's plugins
  are written in TypeScript and checked against the SDK's types.
- **Two licenses.** Use MixMusic under the MIT license or the GPL 3.0.

### Search and play

- **One search for both services.** Results from YouTube Music and Spotify
  side by side, each marked with its service. YouTube Music results show
  how many times each track was played, so the original stands out from
  covers.
- **One queue.** Mix tracks of both services, shuffle and repeat across
  them, and let a radio carry on when the queue runs out.
- **Explore.** New releases, trending songs, the charts, moods and genres,
  and related songs for what plays.
- **Synced lyrics** that follow the song, or plain text if you prefer.
- **A mini player** that stays on top of your other windows.

### Your library

- **Mixes:** playlists that hold tracks from every service. Import a
  service's playlist into a mix, and search and sort inside any of them.
- **Liked songs** from both services in one list.
- **History and listening stats,** day by day.
- **Your accounts.** Sign in to YouTube Music on Google's own page for your
  home feed, liked songs, playlists, albums and artists, as your account or
  one of its brand channels. Sign in to Spotify for your library there.
  Likes, saved albums and playlists, and followed artists reach the service
  too. What you play joins your YouTube Music history, which its
  recommendations come from.

### Spotify, three ways

- **In MixMusic itself,** at Spotify's quality, with no Spotify app open
  (Premium, on Windows).
- **On Spotify Connect:** your Spotify app, your phone or a speaker.
- **From YouTube Music:** MixMusic plays the Spotify track's recording on
  YouTube Music, which needs no Premium to play.

### Sound

- Crossfade, even loudness and volume levelling.
- A ten-band equalizer with presets.
- A visualizer.
- An offline cache of the tracks you played.

### On your desktop

- Media keys and the system's media controls on Windows, macOS and Linux.
- The player's buttons on the taskbar thumbnail on Windows.
- Tray controls, notifications of new tracks, and start at login.
- `mixmusic://` links.
- **Last.fm:** every track you hear joins your profile, and scrobbles made
  offline are sent once you are back online.
- **Discord:** your profile shows what plays, with its artwork and the time
  left.
- **Updates:** MixMusic checks for a new release at start and every few
  hours, checks its signature, and installs it when you say so, or by
  itself if you choose.

### Make it yours

- **Themes.** Ten built in, with effects you can tune, one theme for light
  mode and one for dark mode, and an accent color of your own or taken
  from the artwork that plays.
- **Community themes and plugins,** one click away in the marketplace.
- **Plugins** add pages, commands, track actions and widgets through a
  typed API, drawn in your theme.
- **Keyboard first.** Shortcuts for everything common, a key of your own for
  any command, plugins' commands included, and a command palette
  (<kbd>Ctrl</kbd> <kbd>K</kbd>) for the rest.

## A look around

https://github.com/user-attachments/assets/a803dedd-46ac-4219-966f-2b64aff14b5e

<p align="center"><sub>A minute in MixMusic, page by page, in the Mochi theme, with sound: home, now playing, search, an artist and an album, liked songs, mixes, explore, stats, the command palette, the themes, the equalizer, the marketplace and a plugin.</sub></p>

> [!NOTE]
> The music you hear is royalty-free, by Kevin MacLeod
> ([incompetech.com](https://incompetech.com), licensed under
> [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/)), because we
> can't use copyrighted songs on our website or in our README.

<p align="center">
  <img src="docs/look-listen.webp" alt="MixMusic's pages in the Mochi theme, standing upright on a dark stage and facing you: now playing アイドル by YOASOBI in the middle, outlined in pink, with the mini player in front of it; home, search, YOASOBI's artist page and the album THE BOOK 3 on the left; the lyrics panel, explore, liked songs and the library's mixes on the right" />
</p>

<p align="center"><sub><b>Listen.</b> In the middle, now playing アイドル by YOASOBI with its synced lyrics, and the mini player in front of it. On the left, home, one search across both services, an artist and an album. On the right, the lyrics panel, explore, your liked songs and your mixes.</sub></p>

<p align="center">
  <img src="docs/look-yours.webp" alt="More of MixMusic in the Mochi theme, standing upright on a dark stage and facing you: Settings › Appearance with its built-in themes in the middle, outlined in pink; a mix, listening stats, the command palette and the history on the left; the marketplace's community themes, the Sleep Timer plugin, the equalizer and the shortcuts on the right" />
</p>

<p align="center"><sub><b>Make it yours.</b> In the middle, the ten built-in themes. On the left, a mix, your listening stats, the command palette (<kbd>Ctrl</kbd> <kbd>K</kbd>) and your history. On the right, community themes in the marketplace, the Sleep Timer plugin, the ten-band equalizer and shortcuts you can change.</sub></p>

## Install

Download the file for your system from the
[latest release](https://github.com/fzn0x/mixmusic/releases/latest):

| System | Download |
|---|---|
| macOS 12 and later (Apple silicon and Intel) | [`mixmusic-<version>-macos-universal.dmg`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Windows 10 and 11 (x64, Arm), installer | [`mixmusic-<version>-windows-<arch>-setup.exe`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Windows, portable, no installation | [`mixmusic-<version>-windows-<arch>-portable.zip`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Debian, Ubuntu and derivatives | [`mixmusic-<version>-linux-<arch>.deb`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Fedora, openSUSE and derivatives | [`mixmusic-<version>-linux-<arch>.rpm`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Any Linux, one file | [`mixmusic-<version>-linux-<arch>.AppImage`](https://github.com/fzn0x/mixmusic/releases/latest) |
| Any Linux, in your home folder | [`mixmusic-<version>-linux-<arch>.tar.gz`](https://github.com/fzn0x/mixmusic/releases/latest), with `install.sh` |

`<arch>` is `amd64` for x64 and `arm64` for Arm. Each release lists
`SHA256SUMS` to check the downloads with. You don't need the files named
`update-…`, `….delta`, `mixmusic-<version>-macos-…` or
`-windows-<arch>.tar.gz`: installed apps update from them.

The installers, the portable zip and the Linux archive update themselves.
The `.deb`, the `.rpm` and the AppImage can't replace their own files, so
MixMusic tells you when a new version is out and downloads it for you. For
the portable zip, unpack it anywhere and open `MixMusic.exe`. For the
AppImage, make it executable (`chmod +x`) and open it.

### The first launch

- **macOS:** the app isn't notarized by Apple yet, so the first time you
  open it, macOS says it can't check it. Open it anyway from System
  Settings › Privacy & Security › Open Anyway, or run this once in
  Terminal:

  ```sh
  xattr -dr com.apple.quarantine /Applications/MixMusic.app
  ```
- **Windows:** the installer isn't signed yet, so SmartScreen may warn
  about it: choose More info, then Run anyway. Signing in to YouTube Music
  and playing Spotify in MixMusic need the WebView2 Runtime, which Windows
  11 includes.
- **Linux:** MixMusic needs GTK 3, and PulseAudio (PipeWire serves it too)
  or ALSA. Signing in to YouTube Music needs WebKitGTK 4.1, the tray icon
  needs `libayatana-appindicator3`, and keeping your sign-ins in the
  system's keyring needs `libsecret`. The `.deb` and the `.rpm` install
  them; for the AppImage and the archive:

  ```sh
  # Debian and Ubuntu
  sudo apt install libgtk-3-0 libpulse0 libwebkit2gtk-4.1-0 libayatana-appindicator3-1 libsecret-1-0
  # Fedora
  sudo dnf install gtk3 pulseaudio-libs webkit2gtk4.1 libayatana-appindicator-gtk3 libsecret
  ```
- **Every system:** MixMusic finds the audio of most YouTube Music tracks
  itself. For the rest, such as tracks made for children, it uses
  [yt-dlp](https://github.com/yt-dlp/yt-dlp), which it installs for you
  from Settings › Playback and keeps up to date. yt-dlp needs a JavaScript
  runtime: [Deno](https://deno.com), Node.js, Bun or QuickJS.

### Accounts (all optional)

- **YouTube Music:** choose Sign in to YouTube Music in Settings › Accounts
  and sign in to Google in the window that opens. What you type goes to
  Google only. MixMusic keeps the session encrypted, with a key in your
  system's keyring where there is one.
- **Spotify:** Spotify lets an app serve up to five users, and only while
  its owner has Premium, so you sign in with a Spotify app of your own. It
  takes two minutes: [docs/spotify.md](docs/spotify.md) walks you through
  it.
- **Last.fm:** create an API account at
  [last.fm/api/account/create](https://www.last.fm/api/account/create)
  (any name, no callback URL), paste its API key and shared secret in
  Settings › Accounts, and approve MixMusic in your browser.
- **Discord:** while the Discord app runs, your profile shows what you
  listen to. Turn it off in Settings › Desktop.

Something not working? [docs/troubleshooting.md](docs/troubleshooting.md)
covers the common problems and the known issues.

## Build from source

You need [Go](https://go.dev/dl/) 1.27.3 or later, [Bun](https://bun.sh)
1.2 or later, which runs MyGo's command line, and Git. MyGo needs no C
compiler, so any system builds MixMusic for every system.

### Windows

```powershell
winget install GoLang.Go
winget install Oven-sh.Bun
winget install Git.Git
```

Open a new terminal, then:

```powershell
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

Signing in and Spotify playback need the WebView2 Runtime, which Windows 11
includes. On Windows 10, install it from
[Microsoft](https://developer.microsoft.com/microsoft-edge/webview2/).

### macOS

With [Homebrew](https://brew.sh):

```sh
brew install go oven-sh/bun/bun
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

### Linux

Install Go from [go.dev/dl](https://go.dev/dl/) (distribution packages are
often older than 1.27.3), then Bun and the libraries MixMusic runs with:

```sh
curl -fsSL https://bun.sh/install | bash
# Debian and Ubuntu
sudo apt install git libgtk-3-0 libpulse0 libwebkit2gtk-4.1-0 libayatana-appindicator3-1 libsecret-1-0
# Fedora
sudo dnf install git gtk3 pulseaudio-libs webkit2gtk4.1 libayatana-appindicator-gtk3 libsecret
```

Then:

```sh
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

### Nix

With [Nix](https://nixos.org), `nix develop` gives you the tools releases
are built with, at the versions `flake.lock` pins: Go, Bun, NSIS,
GoReleaser and Zola. `nix build` builds MixMusic for Linux from source,
into `result/`.

### Develop

`bun run dev` builds the app, starts it, and rebuilds it as you edit. Its
data stays apart from an installed MixMusic, under the name
`MixMusic Dev`.

```sh
bun run build:plugins          # the plugins' JavaScript, from their TypeScript
go test ./...                  # the app's tests
bun run typecheck              # the plugin SDK and the community's plugins
go run ./tools/registry -check # the community registry
bun run build                  # the packaged app for this system, in build/
bun run build -- -platform windows/amd64,linux/amd64,linux/arm64
```

The [website](https://fzn0x.github.io/mixmusic/) is in `site/`, built with
[Zola](https://www.getzola.org), and the guides in `docs/` are its
`/docs/` pages. `go run ./tools/sitedocs && zola --root site serve`
previews it. Releases are built in the Nix shell and published by
[GoReleaser](https://goreleaser.com) (`.goreleaser.yaml`).

[docs/architecture.md](docs/architecture.md) explains how the parts fit
together.

### Layout

The app is the root package, with the packages of `internal/` under it. The
packages outside `internal/` are for anyone to import, and documented on
[pkg.go.dev](https://pkg.go.dev/github.com/fzn0x/mixmusic).

```
*.go                 the app and its windows: the pages, the player bar, the
                     mini player, the tray, media keys, deep links
fonts/               the typefaces the windows draw with
internal/app         the services the windows call, and the desktop integration
internal/player      the queue, played by the audio engine or on Spotify's devices
internal/audio       MixMusic's audio engine, and the system's audio output
internal/plugins     plugins, each in a JavaScript engine of its own
internal/youtube     YouTube Music's catalog, and your account there
internal/spotify     the Spotify Web API, its sign-in and Spotify Connect
internal/stream      audio streams, from YouTube or yt-dlp, opened for the engine
internal/match       Spotify tracks matched to their YouTube Music recordings
internal/library     liked songs, mixes, history and stats
internal/lastfm      scrobbling to Last.fm
internal/discord     what plays, on your Discord profile
internal/diag        the log, without secrets, and the reports of crashes
internal/addons      themes and plugins, and the community registry
builtin/themes       the themes MixMusic ships with
public/              the page of Spotify's player, on Windows
sdk/                 the types of the plugin API
community/           community themes and plugins, and their registry
```

The packages you can import:

```
opus/                an Opus decoder in pure Go, a port of libopus's
aac/                 an AAC-LC decoder in pure Go
ac3/                 an AC-3 decoder in pure Go, written from ATSC A/52
webm/                the audio of WebM files: packets, times and seeking
mp4/                 the audio of MP4 files: packets, times and seeking
resample/            sample rate conversion, with Kaiser-windowed sincs
lyrics/              synced lyrics from LRCLIB, and the LRC format
resources/           MixMusic's logo, for code that draws it
```

## Community

Anyone can install community themes and plugins from the marketplace, the
store icon in the sidebar, and you can add yours.

### Make a theme

A theme is a `theme.json` file that sets colors, fonts and corners.
[docs/themes.md](docs/themes.md) lists every token and shows a complete
theme. To try yours, put its folder in the themes folder (Settings ›
Appearance › open themes folder) and choose it in Settings › Appearance.

### Make a plugin

A plugin is a small module, written in TypeScript or JavaScript, that adds
pages, commands, track actions and widgets.
[docs/plugins.md](docs/plugins.md) documents the API, with its
[types](sdk/index.d.ts) for your editor. The community's
[sleep timer](community/plugins/sleep-timer) and
[spectrum](community/plugins/spectrum) are working examples.

### Share it

1. Fork this repository.
2. Add one folder: `community/themes/<id>/` for a theme, or
   `community/plugins/<id>/` for a plugin, with its `plugin.json` and its
   `index.ts`.
3. Run `bun run build:plugins`, `go run ./tools/registry` and
   `go run ./tools/registry -check`.
4. Open a pull request.

Once it is merged, CI publishes it to the
[registry branch](https://github.com/fzn0x/mixmusic/tree/registry), and it
shows up in everyone's marketplace. [community/README.md](community/README.md)
has the details.

## Privacy

MixMusic has no account, server, analytics or telemetry of its own. It
talks to:

- **YouTube and Google:** YouTube Music's catalog, its streams and how loud
  each track is, asked for while signed out, and the artwork; some streams
  through yt-dlp. Signed in, your library there, and, unless you turn it
  off in Settings › Accounts, the tracks you play, which join your YouTube
  Music history.
- **Spotify,** once you sign in: your library, your Spotify Connect devices,
  and Spotify's player, loaded from `sdk.scdn.co`, when Spotify plays in
  MixMusic.
- **LRCLIB:** the title, artists, album and length of the track that plays,
  to find its lyrics.
- **GitHub:** a check for a new version at start and every few hours,
  which you can turn off in Settings › About; the community registry when
  you open the marketplace; and the downloads of yt-dlp.
- **Last.fm,** once you connect it: what you play.
- **Discord,** unless you turn it off: what plays, sent to the Discord app
  on your computer, which shows it on your profile.

What it keeps on your computer:

- Your settings and library (liked songs, mixes, history and recent
  searches), in the folder Settings › About names.
- The tracks you played, in the cache, as much as you allow in Settings ›
  Playback, and how loud YouTube measured them.
- The covers you saw, up to 256 MB.
- Your sign-ins, the YouTube Music session included, in the system's
  keyring. On Windows, when the Credential Manager can't keep one, it goes
  in a file encrypted for your Windows account; on Linux without a
  keyring, in files only your user can read.
- The log, without your sign-ins.

The library's history page clears what you played, Settings › Playback
clears the cache and the covers, and signing out of a service deletes its
sign-in.

On Windows, the visualizer hears the audio of Spotify playing on this
computer, in MixMusic or in the Spotify app, through Windows' process
loopback, which hears only that app. It analyzes the audio in memory and
never records or sends it.

## Credits

MixMusic is built on these projects:

- **[MyGo](https://github.com/egoist/mygo)**, the desktop framework MixMusic
  is built with. MixMusic uses MyGo's native UI: MyGo draws every window
  itself, in Go, without a webview, from the pages and the player bar to the
  mini player. Its command line packages the app, with its installers and
  the signed updates it updates itself from.
- **[Go](https://go.dev)**, the language of the app, with
  [purego](https://github.com/ebitengine/purego), which calls the system's
  libraries without cgo, and [golang.org/x/image](https://pkg.go.dev/golang.org/x/image)
  and [golang.org/x/text](https://pkg.go.dev/golang.org/x/text).
- **[Sobek](https://github.com/grafana/sobek)**, the JavaScript engine each
  plugin runs in.
- **[Nix](https://nixos.org)**, which pins the tools releases are built
  with, and builds MixMusic for Linux from source.
- **[GoReleaser](https://goreleaser.com)**, which publishes the releases, and
  its [nFPM](https://nfpm.goreleaser.com), which makes the RPM packages.
- **[Bun](https://bun.sh)**, which runs the build, and
  **[TypeScript](https://www.typescriptlang.org)**, which compiles the
  plugins and checks the plugin SDK's types.
- **[Zola](https://www.getzola.org)**, which builds the website.
- **[NSIS](https://nsis.sourceforge.io)**, which makes the Windows
  installers, and [appimagetool](https://github.com/AppImage/appimagetool),
  the AppImages.
- **[yt-dlp](https://github.com/yt-dlp/yt-dlp)**, which finds the audio of
  the tracks MixMusic can't find itself.
- **[LRCLIB](https://lrclib.net)**, the open database of synced lyrics.
- **[libopus](https://opus-codec.org)**, whose decoder the Opus decoder is a
  port of, and **[SharpJaad](https://github.com/jimm98y/SharpJaad)**, the C#
  port of [JAAD](https://sourceforge.net/projects/jaadec/) the AAC decoder
  derives from.
- **[FFmpeg](https://ffmpeg.org)**, which the decoders' tests compare with.
- **[Selawik](https://github.com/microsoft/Selawik)** and
  **[Quicksand](https://github.com/andrew-paglinawan/QuicksandFamily)**, the
  typefaces of the windows.
- **[Kevin MacLeod](https://incompetech.com)**, whose music, under
  [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), plays in the
  walkthrough and on the website.

## Contributing

Fixes, features, themes, plugins and bug reports are all welcome.

- **Report a problem** from Settings › About › report a problem. It opens a
  new GitHub issue with your version, system, theme and plugins filled in.
  Or [open an issue](https://github.com/fzn0x/mixmusic/issues/new) yourself.
- **Send a change** as a pull request. [CONTRIBUTING.md](CONTRIBUTING.md)
  explains the checks to run, the commit style, and the changelog entry
  that a change people will notice needs.
- **Share a theme or a plugin:** see [Community](#community).

## License

MixMusic is dual-licensed: use it under the [MIT license](LICENSE-MIT) or
the [GNU General Public License, version 3](LICENSE-GPL), whichever you
prefer. Contributions are accepted under both. The typefaces in `fonts` are
under the SIL Open Font License: [Selawik's](fonts/OFL.txt) and
[Quicksand's](fonts/quicksand-OFL.txt). The Opus decoder is a port of
libopus, under [its BSD license](opus/LICENSE.libopus), and the AAC decoder
derives from SharpJaad, under [its MIT license](aac/LICENSE.sharpjaad).

MixMusic is not affiliated with, endorsed by or sponsored by Spotify or
Google. It plays what these services offer their users, through the
services' own interfaces; use it as their terms allow. Spotify and YouTube
Music are trademarks of their owners.

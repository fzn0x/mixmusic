# Architecture

MixMusic is one Go program. [MyGo](https://github.com/egoist/mygo) draws its
windows in Go, with its `ui` package: immediate-mode views, which the GPU
draws, without a webview. The windows show the catalogs, the accounts and
the library of the services of `internal/app`, and the player in Go plays
the queue through MixMusic's own audio engine, or on Spotify's devices.

```
┌──────────────────── the windows (internal/native) ──────────────────┐
│ pages, shell, player bar, queue, lyrics, mini player, themes,       │
│ plugins' pages and widgets                                          │
└──────┬──────────────────────────┬────────────────────────┬──────────┘
       │ App.Services             │ the queue              │ plugins
┌──────┴──────────────────┐ ┌─────┴──────────────┐ ┌───────┴──────────┐
│ internal/app: Catalog,  │ │ internal/player    │ │ internal/plugins │
│ Collection, Accounts,   │ │ ├ internal/audio   │ │ (sobek, a JS     │
│ Preferences, Addons,    │ │ └ Spotify devices  │ │ engine in Go)    │
│ System; the desktop     │ └────────────────────┘ └──────────────────┘
├─────────┬─────────┬────────┬───────┬────────┬─────────┬─────────────┤
│ youtube │ spotify │ stream │ match │ lyrics │ library │ addons  ... │
└─────────┴─────────┴────────┴───────┴────────┴─────────┴─────────────┘
```

## The Go side

- **`internal/media`** defines what every service shares: tracks, albums,
  artists, playlists, shelves. IDs carry their service: `yt:<video id>`,
  `sp:<track id>`, `mx:<mix id>`.
- **`internal/youtube`** reads YouTube Music through the API its website uses:
  search, suggestions, the home and explore pages, charts, moods and genres,
  albums, artists, playlists, related tracks, radios and plain lyrics.
  Signed in with the cookies of the user's session, which it signs its
  requests with as the website does, it reads and changes their library:
  liked songs, playlists, albums, subscriptions, likes and saves. Its
  parsers are tested against recorded responses in `testdata/`, and against
  the live service with `MIXMUSIC_LIVE=1 go test ./internal/youtube -run
  Live` (`MIXMUSIC_YT_COOKIES` adds a signed-in account).
- **`internal/spotify`** signs in with PKCE on a loopback callback, refreshes
  tokens, reads the Web API (search, albums, artists, playlists, the
  library), and controls Spotify Connect devices.
- **`internal/stream`** finds the audio of a YouTube video through YouTube's
  player API, asked signed out as YouTube's visionOS app (`Direct`), else
  with yt-dlp, keeps it until its URL expires, and opens it for the audio
  engine (`Proxy.Open`): chunked fetches from YouTube in the background, a
  new URL when YouTube refuses the old one, and an on-disk cache of tracks
  played to their end. The player API's answers are tested against
  recorded ones in `testdata/`, and against YouTube with
  `MIXMUSIC_LIVE=1 go test ./internal/stream -run Live`.
- **`internal/webm`** reads the audio of WebM files, as YouTube serves it:
  the packets of the first audio track with their times, and seeking by the
  Cues, reading a file played from its start straight through.
- **`internal/opus`** decodes Opus: a pure Go port of libopus 1.6.1's decoder
  (`celt`, `silk` and the range decoder `rangedec`), which decodes the test
  vectors of RFC 8251 as libopus does, bit for bit.
- **`internal/audio`** is MixMusic's audio engine: two decks, each a
  goroutine that reads a track with `internal/webm` and decodes it with
  `internal/opus` into a ring the audio thread plays from, and the preamp,
  equalizer, levelling compressor, volume and analyser of the web player
  MixMusic had, ported from Chromium's Web Audio, which they match sample
  for sample. `internal/audio/output` plays it on the system's output,
  without cgo: through WASAPI on Windows, an Audio Queue on macOS, and
  PulseAudio, which PipeWire serves too, or ALSA on Linux. On Windows it
  also hears what a process and its children play, through WASAPI's
  process loopback: `internal/app`'s listener hears Spotify there, in
  MixMusic's webview or the Spotify app, into a `Tap`, the analyser of
  the engine for audio outside it, which the visualizer and the plugins'
  analyser read while Spotify plays.
- **`internal/nowplaying`** shows what the engine plays in the system's media
  controls, and passes on what they ask: Now Playing on macOS, through
  MediaPlayer, and an MPRIS player on Linux, served by GIO's GDBus, both
  called with purego, and the SystemMediaTransportControls of a hidden
  window on Windows, reached through WinRT without cgo.
- **`internal/taskbar`** puts previous, play or pause, and next on the
  thumbnail of the main window's taskbar button on Windows, through the
  shell's ITaskbarList3, and takes their clicks in a window procedure in
  front of MyGo's.
- **`internal/player`** plays the queue: one goroutine owns the queue and
  what plays, and the network calls run beside it. It plays YouTube Music,
  and Spotify tracks matched to their recording, through the engine, and
  Spotify tracks on a Spotify device: a Spotify Connect device, or MixMusic
  itself, as below. It shuffles, repeats, crossfades, starts radios when the
  queue runs out, counts plays and keeps up to 500 tracks of the queue,
  around the one playing, in `queue.json`, as it changes and as MixMusic
  quits.
- **`internal/tools`** finds yt-dlp and a JavaScript runtime, and installs
  yt-dlp from its releases, checked against their published checksums.
- **`internal/match`** finds a Spotify track's recording on YouTube Music, by
  title, artists and duration, for playback without a Spotify device.
- **`internal/lyrics`** finds synced lyrics on LRCLIB and reads LRC.
- **`internal/library`** keeps liked songs, mixes, the history and stats in
  `library.json`.
- **`internal/lastfm`** signs in to Last.fm with the user's API account and
  scrobbles by Last.fm's rules, keeping what fails offline for later.
- **`internal/discord`** shows what plays on the user's Discord profile, over
  Discord's local pipe or socket.
- **`internal/diag`** writes the log, `mixmusic.log` in the logs folder,
  rotated at a megabyte and without the cookies, hashes and tokens a line
  may hold, and has Go write a crash to `crash.log` there, which the next
  start keeps as `crash-<time>.log` and offers to report.
- **`internal/addons`** validates theme and plugin manifests, lists the
  built-in and installed add-ons, and installs add-ons from the community
  registry.
- **`internal/plugins`** runs the plugins the user enabled, each in a
  JavaScript runtime of its own, [sobek](https://github.com/grafana/sobek)
  (a fork of goja with ES modules), on a goroutine of its own: their
  modules, `console`, timers and `fetch`, and the API of
  [plugins.md](plugins.md), built in `api.js` over Go natives. What they add
  to the interface the windows show: pages and widgets as views of
  elements, which the windows draw in their own styles, and canvases, whose
  drawing calls the plugin records and the window paints.
- **`internal/store`** writes JSON files atomically; **`internal/settings`**
  defines the settings and keeps them in range.
- **`internal/app`** gives the windows the services (`App.Services`), tells
  them when the settings, the accounts, the add-ons or the updates change
  (`App.Watch`), makes the player, and puts MixMusic on the desktop: the
  tray, media keys, deep links, notifications, updates.

## The windows

`internal/native` draws the main window and the mini player. Pages fetch
what they show beside the main thread (`fetch`), and a window's state
belongs to its main thread: what finishes elsewhere comes back through
`update`.

- **Artwork** downloads only for what shows in the window, scaled to the
  size it shows at, and the images shown most recently are kept, up to 48
  MB of pixels (`artwork.go`). An image that couldn't load tries again
  after 1 to 2 seconds, then less often, up to every 2 minutes, and at once
  when another of its host loads; one whose address gives no image waits
  half an hour. Both windows, the accent and Terminal's tint read the
  images through `artwork.Store` (`internal/artwork`), which downloads each
  once, keeps it on disk as it came, up to 256 MB, and logs a host's
  failures at most once a minute, without the addresses.
- **Themes** give their tokens: colours, fonts, radius (`theme.go`), and
  the accent follows the artwork of what plays. The windows draw the
  effects of the built-in themes, as their options say (`effects.go`):
  Mix's textures, Aurora's drifting lights and frosted dialogs, Sakura's
  petals, Terminal's scanlines, glow and phosphor artwork.
- **Deep links** and second launches open their pages in the main window
  (`App.OnNavigate`).

## Webviews

Two windows still show a web page, and only while they're needed:

- **Spotify in MixMusic itself** (`internal/app/spotifyhere.go`): Spotify's
  Web Playback SDK, which decrypts Spotify's protected audio, in the hidden
  window of `public/spotify.html`, the only page of the app, which it embeds.
  It opens at the first Spotify track that plays in MixMusic, and makes
  MixMusic a Spotify device of its own, which the player plays tracks on
  through the Web API and follows by asking the page (`Page.Eval`); the page
  gets its token from `SpotifyPage.Token`. Only WebView2, on Windows,
  decrypts Spotify's audio.
- **Google's sign-in to YouTube Music**, whose cookies MixMusic keeps.

## Data

Everything lives in the user's data folder (`%AppData%\MixMusic` on
Windows, `~/.config/MixMusic` on Linux): `settings.json`, `library.json`,
`queue.json`, `addons/`, `bin/` (yt-dlp), the YouTube session
(`youtube-session.bin`, encrypted with a key of the system's credential
store, which also keeps the Spotify token and the Last.fm connection), the
scrobbles still to send, and `updater.json`, the update preferences. The
cache folder holds audio, the covers shown (`artwork/`), matches and
yt-dlp's cache, and the logs folder
the log and the crash reports. `MIXMUSIC_DATA_DIR` moves all three into one
folder, for a portable copy or a test run. `MIXMUSIC_YTDLP_ONLY=1` has
yt-dlp find every stream, as before MixMusic asked YouTube's player API,
to time one against the other or tell which fails: the log tells which
streams took over a second to find, and which tracks over two to start.

## Capture mode

`MIXMUSIC_CAPTURE=<dir>` screenshots pages of the main window, and
`MIXMUSIC_CAPTURE_QUIT=1` quits once they are saved:

```sh
MIXMUSIC_CAPTURE=shots MIXMUSIC_CAPTURE_QUIT=1 MIXMUSIC_CAPTURE_ROUTES="/|/search?q=jazz|/settings" MIXMUSIC_CAPTURE_SIZE=1440x900 ./MixMusic
```

`MIXMUSIC_CAPTURE_WAIT` sets how long each step gets (`5s`). Steps that
aren't pages act: `act:shot:<name>` saves the window as it is,
`act:mute` mutes the player, `act:play:<search>` plays what a search
finds, `act:resume` plays the queue kept in the data folder,
`act:wait:<duration>` waits, `act:palette` opens the command palette,
`act:panel:<queue|lyrics|none>` shows a side panel, `act:scroll:<y|end>`
scrolls the page, `act:theme:<id>` wears a theme, `act:mini` opens the
mini player, which saves `mini.png`, `act:spotify` logs whether MixMusic's
own Spotify device could play, `act:title` logs the window's title, which
the screenshots leave out, and `act:memory` and `act:heap` log the Go
heap and save a profile of it. Nothing plays unless the player is muted,
or `MIXMUSIC_SILENT=1` has the audio engine play into no device, which
keeps the visualizer and the lyrics moving without a sound.
`MIXMUSIC_CAPTURE_COMMUNITY=<dir>` has the marketplace read the community
registry from a copy of `community/`, for add-ons not on GitHub yet. The
site's screenshots are made this way, from a throwaway profile, at
1536x900 (`MIXMUSIC_CAPTURE_SIZE=1552x909`: the size includes the
window's invisible borders). `go test ./internal/native -run Render` with
`MIXMUSIC_RENDER=<dir>` draws the pages and themes without a window.

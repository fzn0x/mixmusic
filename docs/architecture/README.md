# Architecture

MixMusic is one Go program. [MyGo](https://github.com/egoist/mygo) draws its
windows in Go, with its `ui` package: immediate-mode views, which the GPU
draws, without a webview. The windows show the catalogs, the accounts and
the library of the services of `internal/app`, and the player in Go plays
the queue through MixMusic's own audio engine, or on Spotify's devices.

```
┌────────────────── the windows (the root package) ───────────────────┐
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

Each part has a file of its own, so that changes to different parts don't
edit the same file. The Go side's packages come first, then the windows:

1. [The services](services.md): what every service shares, YouTube Music,
   Spotify, matching, lyrics, Last.fm and Discord.
2. [The streams](stream.md): how the audio of a YouTube video is found and
   fetched, live streams and video pictures too, and yt-dlp.
3. [Containers and codecs](codecs.md): WebM, Opus, MP4, AAC and AC-3, and
   the resampler.
4. [The audio engine](audio.md): the decks, the effects, speed, and the
   system's output.
5. [The player](player.md): the queue, what plays, what it skips, and
   whether the network answers.
6. [The library and downloads](library.md): liked songs, the history, and
   the tracks kept to play offline.
7. [The app and the desktop](app.md): the services the windows use, the
   media controls, the Flatpak's portal, the taskbar and the log.
8. [Add-ons](addons.md): theme and plugin manifests, colour files, and the
   plugins' runtime.
9. [Settings](settings.md): the settings, and how their file is read and
   written.
10. [The windows](windows.md): refresh, artwork, themes and deep links.
11. [Webviews](webviews.md): the three web pages that still show.
12. [Data](data.md): what MixMusic keeps where, on each system.
13. [Capture mode](capture.md): screenshots of the pages, for the site and
    for tests.

A new part gets a file of its own here, and a line in this list.

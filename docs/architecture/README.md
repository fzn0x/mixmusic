# Architecture
<!-- docs: group=project order=60 -->

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

Each part has a file of its own in this folder, named after it, such as
`player.md` or `audio.md`. Nothing lists them by hand, so that two changes
never add a line at the same place: GitHub shows the folder's files above
this page, and the site shows each part after it, by name, with their
headings in the page's contents.

A new part gets a file of its own here, and needs a line nowhere else.

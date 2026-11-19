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

## Packages anyone can import

Some of the code is in Go packages at the module root,
`github.com/fzn0x/mixmusic`, documented on
[pkg.go.dev](https://pkg.go.dev/github.com/fzn0x/mixmusic). The decoders
are `opus`, `aac`, `ac3`, `flac`, `ogg`, `webm`, `mp4` and `resample`, told
in `codecs.md`; `lyrics` finds lyrics on Better Lyrics, Unison and LRCLIB
and reads TTML and LRC; `brand` holds MixMusic's logo, the pink rings with
a play button, and the Discord status badges. The logo isn't in
`resources/`: `mygo build` ships every file there beside the app, and it
holds only `darwin/` now.

## Build from source

The project's README has the steps for each system. With
[Nix](https://nixos.org), `nix build` builds MixMusic for Linux from
source into `result/` (x86_64 and aarch64, `nix/package.nix`),
and `nix run github:fzn0x/mixmusic` runs it. `nix develop`, on Linux and
macOS, gives Go 1.27, Bun, GoReleaser, jq, zip and Zola, and, on Linux,
NSIS, osslsigncode and binutils, at the versions `flake.lock` pins.
Releases are built in that shell, as are CI's builds and the website, and
published with GoReleaser (`.goreleaser.yaml`). In the shell, build Linux
downloads with `.github/scripts/build.sh linux/amd64,linux/arm64`: a plain
`bun run build` there makes a program that asks for Nix's loader and starts
only on NixOS. `go run ./tools/elfloader build/linux-amd64` checks a build.


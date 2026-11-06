# MixMusic on Flathub

This folder builds MixMusic from source as Flathub builds apps: in the
GNOME SDK, with Go's and Node.js's SDK extensions, without the network.
The [flathub workflow](../../.github/workflows/flathub.yml) builds it on
x86_64 and aarch64 the way Flathub's builder does, runs Flathub's linter on
the manifest and on what it built, then installs the bundle and starts
MixMusic in it.

| File | What it is |
|---|---|
| `io.github.fzn0x.mixmusic.yml` | The manifest. Its MixMusic is this checkout |
| `go-modules.json` | The Go modules of `go.sum`, vendored from the Go module proxy, written by `go run ./tools/flathub` |
| `build.sh` | What `mygo build` does for Linux, as the sandbox runs it |
| `io.github.fzn0x.mixmusic.desktop` | The desktop entry |
| `io.github.fzn0x.mixmusic.metainfo.xml` | The AppStream metadata; `build.sh` adds the releases of CHANGELOG.md |

## Submitting

Flathub's [Generative AI policy](https://docs.flathub.org/docs/for-app-authors/requirements#generative-ai-policy)
rules out three things this folder would otherwise be for:

- A manifest submitted to Flathub must contain no AI-generated or
  AI-assisted content, and disclosing it doesn't exempt it. The manifest
  here was written with an AI assistant, so it's MixMusic's own build and
  check of what Flathub needs, not the one to submit.
- AI tools must not open or automate the submission pull request, or write
  its description, its commit messages or the replies to reviewers. This
  folder has no text for that pull request, and no workflow that opens
  pull requests on Flathub.
- AI-generated code and packaging in the app must be disclosed, with the
  parts it covers and roughly how much, and reviewers may turn an app down
  for how much of it is generated.

Flathub also turns down apps that have "only existed for a very short
period of time" ([development history](https://docs.flathub.org/docs/for-app-authors/requirements#insufficient-development-history)),
and its pull request template asks for a video of the Flatpak running on
Linux.

What the maintainer does, by hand:

1. Release a version that has this folder and Flathub's install kind
   (0.6.0 or later).
2. Write the manifest for Flathub. What it needs is listed under
   [What the build needs](#what-the-build-needs). Run it through
   `flatpak run --command=flathub-build org.flatpak.Builder --install <manifest>`
   and `flatpak run --command=flatpak-builder-lint org.flatpak.Builder manifest <manifest>`,
   then `... repo repo`.
3. Fork [flathub/flathub](https://github.com/flathub/flathub) with all of
   its branches, branch off `new-pr`, add the manifest, its `go-modules.json`
   and Flathub's shared modules
   (`git submodule add https://github.com/flathub/shared-modules.git`),
   and open a pull request to `new-pr` titled `Add io.github.fzn0x.mixmusic`,
   filling in Flathub's template yourself. Then comment `bot, build`.
4. Answer the reviewers, and accept the invitation to
   `flathub/io.github.fzn0x.mixmusic` within a week of the merge, with
   two-factor authentication on.
5. Verify the app on flathub.org: the `io.github.fzn0x` ID is checked
   through the `fzn0x` GitHub account.

## What the build needs

- **The GNOME 51 runtime and SDK**, with `org.freedesktop.Sdk.Extension.golang`,
  whose Go (1.27.3 in its 26.08 branch) must be at least `go.mod`'s, as the
  build runs with `GOTOOLCHAIN=local`; and `org.freedesktop.Sdk.Extension.node24`.
  The runtime moves with the release's Flatpak's (`packaging/flatpak`).
- **The Go modules** as sources, unpacked into `vendor/` with
  `vendor/modules.txt`, and `go build -mod=vendor`. `go-modules.json` holds
  them, each the proxy's zip of the module, which `tools/flathub` checks
  against `go.sum`. The community's
  [flatpak-go-mod](https://github.com/dennwc/flatpak-go-mod), which
  flatpak-builder-tools points to, writes the same layout.
- **What `mygo build` does for Linux**, which needs Bun or Node.js only to
  read `mygo.config.ts`, and downloads nothing for MixMusic: `go build` of
  the root package with `-trimpath`, the `mygo_noinspector` tag, `public/`
  embedded through a generated file, and the app's name, version,
  identifier and URL scheme linked in with `-X`; no update feed. `build.sh`
  does this; `tools/flathub`'s tests keep it in step with `mygo.config.ts`.
  Nothing is built in Nix, so the release's loader fix (`tools/elfloader`,
  `system-loader.py`) has no part in it.
- **The app's files**: the executable in `/app/lib/mixmusic`, the release
  Flatpak's launcher (`packaging/flatpak/mixmusic.sh`, which keeps the
  tray's icon where the desktop sees it) as `/app/bin/mixmusic`, the desktop
  entry, the SVG icon, the metadata, and the licenses of MixMusic and of
  each Go module.
- **The mark** `/app/share/mixmusic/flathub`, by which MixMusic tells it is
  Flathub's build (`internal/app/flathub.go`).
- **yt-dlp and Node.js**, below.

### Updates

MixMusic doesn't update a Flathub copy itself. In Flathub's build,
Settings › About names the build "Flathub", offers no `.flatpak`, and says
a new version comes from Flathub through the software center, or
`flatpak update`. It still tells when a new version is out on GitHub;
Flathub has it once the update of the app's Flathub repository is merged
and built.

### yt-dlp

Flathub's build has yt-dlp as a module of its manifest: the pinned wheels of
`yt-dlp` and `yt-dlp-ejs`, its solver of YouTube's challenges, which runs on
Node.js from the SDK's extension. MixMusic runs it, unless the user chose
another in Settings › Playback, and downloads none. This is what other
downloaders on Flathub do, such as Video Downloader and Parabolic, whose
`x-checker-data` has Flathub's bot propose each new yt-dlp: that matters
here, as an old yt-dlp stops finding YouTube's streams, so its updates are
worth merging quickly. A copy of the release's `.flatpak` keeps the yt-dlp
it downloaded in the same data folder; Flathub's build leaves it alone.

Downloading yt-dlp into the app's data, as the other builds do, isn't
forbidden by Flathub's requirements, but it runs code that no review saw
and that Flathub doesn't update, which the reviewers of a new app would
question.

### Permissions

Each `finish-args` entry and what it's for:

| Permission | Why |
|---|---|
| `--share=network` | YouTube Music, Spotify, lyrics, covers, casting |
| `--socket=wayland`, `--socket=fallback-x11`, `--share=ipc` | The windows |
| `--device=dri` | MyGo draws on the GPU |
| `--socket=pulseaudio` | Playback, through PulseAudio or PipeWire |
| `--talk-name=org.freedesktop.Notifications` | The notifications of what plays, and of tracks liked from the tray or the media keys. MyGo talks to the notification server directly; reviewers may ask for the notification portal instead, which is MyGo's to adopt |
| `--talk-name=org.kde.StatusNotifierWatcher` | The tray icon, and close to the tray |
| `--talk-name=org.freedesktop.secrets` | The keyring, for the YouTube Music, Spotify and Last.fm sign-ins, where the desktop has no Secret portal; libsecret uses the portal where there is one |
| `--filesystem=xdg-run/discord-ipc-0`, `xdg-run/app/com.discordapp.Discord:create`, `…DiscordCanary:create`, `xdg-run/app/dev.vencord.Vesktop:create` | Discord's socket, for showing what plays on the user's profile when they turn it on |

The MPRIS player and the Background portal need no permission.

**Left out: the profiles of Flatpak browsers.** The release's `.flatpak`
may read, read-only, the profiles of Firefox, Chrome, Chromium, Edge, Brave
and Zen installed from Flathub (`~/.var/app/<browser>/…`), from which
Settings › Accounts imports a YouTube Music sign-in (#199). That is
reading other apps' cookies, which Flathub's reviewers are the most likely
to refuse, so Flathub's build doesn't ask for it. Its users sign in with
Google's sign-in in MixMusic's own window, as everyone can; the import
lists no browser. Someone who wants it grants one browser's folder:

```sh
flatpak override --user --filesystem=~/.var/app/org.mozilla.firefox/.mozilla/firefox:ro io.github.fzn0x.mixmusic
```

## Keeping it up to date

- **go.sum.** After a change of the Go modules, `go run ./tools/flathub`
  writes `go-modules.json` again; the workflow fails until it does
  (`go run ./tools/flathub -check`).
- **Releases.** Flathub's bot (`x-checker-data` of type `git` on MixMusic's
  source) proposes each new tag in a pull request on the app's Flathub
  repository. When the release changed `go.sum`, that pull request also
  needs the release's `go-modules.json`, which `go run ./tools/flathub -o
  <dir> -tag vX.Y.Z` writes with the rest of the folder; the bot's test
  build fails until it has it.
- **yt-dlp.** The bot proposes each new `yt-dlp` and `yt-dlp-ejs` wheel.
  A new yt-dlp may ask for a new `yt-dlp-ejs`: merge them together.
- **The runtime.** GNOME runtimes get about a year of support: move
  `runtime-version` with the release's Flatpak's, and the Node.js
  extension's branch with it.

Updates go to the app's Flathub repository as pull requests, which Flathub
builds as a test before they're merged; a merge publishes within a couple
of hours.

## Building it here

On Linux, with [Flatpak](https://flatpak.org/setup/) and
`flatpak install flathub org.flatpak.Builder`:

```sh
git submodule update --init packaging/flatpak/shared-modules
go run ./tools/flathub -o build/flathub
flatpak run --command=flathub-build org.flatpak.Builder --install build/flathub/io.github.fzn0x.mixmusic.yml
flatpak run io.github.fzn0x.mixmusic
```

`-o` writes the folder of a Flathub repository: the manifest, `go-modules.json`
and Flathub's shared modules. Without `-commit` or `-tag`, its MixMusic is
this checkout; with them, a commit of GitHub's repository, as the workflow
builds.

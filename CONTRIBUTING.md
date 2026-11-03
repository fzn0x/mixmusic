# Contributing

Thank you for helping. There are three ways in: themes and plugins for the
community registry, fixes and features of the app, and reports of what does
not work.

## Themes and plugins

[community/README.md](community/README.md) tells how to share a theme or a
plugin: a pull request adding one folder.

## The app

You need [Go](https://go.dev/dl/) 1.27.3 or later and [Bun](https://bun.sh) 1.2 or later.

```sh
git clone https://github.com/fzn0x/mixmusic
cd mixmusic
bun install
bun run dev
```

`bun run dev` builds the app, starts it, and rebuilds it as you edit it.
Development builds keep their data apart from an installed MixMusic, under
the name `MixMusic Dev`.

Before you open a pull request:

```sh
gofmt -l .                      # prints nothing
go vet ./... && GOOS=linux go vet ./... && GOOS=darwin go vet ./...
bun run build:plugins           # the community plugins' JavaScript, which tests run
go test ./...
bun run typecheck
go run ./tools/registry -check
```

[docs/architecture](docs/architecture/README.md) tells where things live. A few
rules keep the project healthy:

- **Every system.** Changes must build for macOS, Windows and Linux; `GOOS=linux go vet ./...` and `GOOS=darwin go vet ./...` check them from Windows.
- **The theme contract is public.** Community themes set the tokens of
  [docs/themes.md](docs/themes.md): colors, fonts and a radius. Rename or
  remove one only with a plan for the themes that set it, and document new
  ones there.
- **The plugin API is public.** Change `internal/plugins/api.js`,
  `sdk/index.d.ts` and [docs/plugins.md](docs/plugins.md) together; the
  typecheck checks the community's plugins against the types.
- **Tests do not need the network.** Parsers are tested on recorded
  responses in `testdata/`; live tests run only with `MIXMUSIC_LIVE=1`.
  The Opus decoder's conformance test runs when `OPUS_TESTVECTORS` names
  the folder of the
  [RFC 8251 test vectors](https://opus-codec.org/static/testvectors/opus_testvectors-rfc8251.tar.gz),
  which CI downloads. The tests that open the audio device, silently, run
  only with `MIXMUSIC_AUDIO=1`, which CI sets on Linux and macOS, and those that compare the native engine's
  sound with Chromium's Web Audio when `MIXMUSIC_BROWSER` names a Chromium
  browser, as `msedge.exe`.
- **Honest errors.** When a service refuses something, say what and how to
  get past it, in the words of the interface.

## Keeping changes apart

Many pull requests are open at once, and one that merges shouldn't make the
others conflict. So a feature lives in files of its own: adding one touches
those files and a few new ones, not a shared list or a shared struct.

Each pull request branches off main, and only main merges into it. One
built on another's branch carries the other's changes a second time once
that one is squashed into main, and conflicts: a pull request that needs
another waits for it to merge. CI's Overlap check merges each pull request
with every other open one ([tools/overlap](tools/overlap/main.go)). When it
fails, the pull request pushed last changes so that the two don't touch the
same lines, as below, or waits for the other to merge.

- **Settings.** A setting lives in its section's struct in
  `internal/settings`, in that section's file, such as
  `internal/settings/playback.go`, with its default and its clean-up beside
  it. Each field has a doc comment of its own, so a new field never makes
  gofmt re-align the ones beside it. `settings.json` stays flat: the
  sections are embedded.
- **The settings page.** A setting's row lives in its feature's file, and
  registers itself from `init()` with an order number, spaced by 10.
  Nothing edits a central list or switch.
- **Options, not arguments.** When a function or an interface method would
  take one more argument, give it an options struct whose zero value keeps
  what it does today, and add a field. Don't add bool or positional
  arguments to interfaces that tests fake.
- **Test fakes.** Fakes embed the do-nothing doubles of
  `internal/player/playertest`, and override only what the test needs.
- **Tests.** A feature's tests go in a test file of their own, named after
  the feature, such as `speed_test.go`, not at the end of a shared one.
- **Docs.** Each part's docs are a file of their own in
  [docs/architecture](docs/architecture/README.md).
- **The changelog.** A change people will notice adds a file of its own to
  changelog.d, as [Commits](#commits) tells.
- **Small files.** Keep files small, and name them after what they hold.
  Split a file when two features that have nothing to do with each other
  keep editing it.

## Commits

Commits follow [Conventional Commits](https://www.conventionalcommits.org),
one change per commit:

```
feat(player): crossfade between tracks
fix(youtube): read albums without a year
docs: explain the plugin storage
```

Types: `feat`, `fix`, `perf`, `refactor`, `docs`, `test`, `build`, `ci`,
`chore`. Scopes name the part changed: `app`, `player`, `ui`, `theme`,
`themes`, `plugins`, `youtube`, `spotify`, `stream`, `match`, `lyrics`,
`library`, `addons`, `community`, `sdk`, `tools`.

A change people will notice also gets an entry in the changelog: a file in
[changelog.d](changelog.d/README.md), in the folder of its section as
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) has them (`added`,
`changed`, `removed`, `fixed` and so on), named after its issue, such as
`changelog.d/fixed/101-stream-retry.md`. It holds the entry as
[CHANGELOG.md](CHANGELOG.md) has them, a Markdown list item. Pull requests
don't edit CHANGELOG.md itself, so they never conflict over it;
`go run ./tools/changelog -print` shows the entries of the next release.

## Releases

1. Run `go run ./tools/changelog`, which folds the entries of changelog.d
   into `## [Unreleased]` in CHANGELOG.md and deletes them. Rename
   `## [Unreleased]` to `## [x.y.z] - YYYY-MM-DD`, add an empty
   `## [Unreleased]` above it, and update the links at the foot of the file.
2. Set `version` in package.json to `x.y.z`, and commit both as
   `chore: release vx.y.z`.
3. Tag the commit and push it with the tag:

   ```sh
   git tag -a vx.y.z -m "MixMusic vx.y.z"
   git push origin main vx.y.z
   ```

The tag runs [the release workflow](.github/workflows/release.yml). It
checks that the tag, package.json and CHANGELOG.md agree, builds the apps of
every system, and publishes the release with the version's section of
CHANGELOG.md, the downloads of
[.github/release-notes.md](.github/release-notes.md) and `SHA256SUMS`. To
build a tag again, run the workflow from the Actions tab with the tag.

The [website](site/templates/index.html)'s download buttons open the latest
release, and the page reads the latest version from GitHub's API. Once the
release is published, set the two `data-version` spans to it, for when the
API does not answer.

The release has a Flatpak too, which the workflow packs from the Linux
build's x64 archive (see [The Flatpak](#the-flatpak)). Its releases come
from CHANGELOG.md's headings, so it needs no step of its own; when its job
fails, the release goes out without it, and running the failed jobs again
adds it.

Installed apps update themselves from the latest release. Its update files are
signed with the key whose public half is `updates.publicKey` in
[mygo.config.ts](mygo.config.ts); the secret half is the repository's
`MYGO_UPDATER_PRIVATE_KEY` secret, which the workflow signs with. Without it, the
release has no update files and apps stay on the version they have.

### Signing and Homebrew

The workflow signs and notarizes the macOS app, signs the Windows apps and
installers, and updates the Homebrew cask, each once its secrets are in the
repository; without them it says so in a notice and skips that part, and
the release goes out as before. Some of the macOS secrets without the
others stop the build. `mygo build` signs, before it makes the update
archives, so that updates install the signed app too: see the comments of
[mygo.config.ts](mygo.config.ts) and of the workflow.

| Secret | What it holds |
|---|---|
| `MACOS_CERTIFICATE` | the Developer ID Application certificate and its private key, as a `.p12`, in base64 |
| `MACOS_CERTIFICATE_PASSWORD` | the password of the `.p12` |
| `APPLE_API_KEY` | an App Store Connect API key, the `AuthKey_<key ID>.p8` file as it is, which notarizes |
| `APPLE_API_KEY_ID`, `APPLE_API_ISSUER` | its key ID and its issuer ID |
| `WINDOWS_CERTIFICATE` | a code signing certificate and its private key, as a `.pfx`, in base64 |
| `WINDOWS_CERTIFICATE_PASSWORD` | its password, if it has one |
| `HOMEBREW_TAP_TOKEN` | a token that can write to `fzn0x/homebrew-tap` |

What only the maintainer can do, once:

1. **macOS.** In the [Apple Developer Program](https://developer.apple.com/programs/),
   create a Developer ID Application certificate, and export it with its
   key from Keychain Access as a `.p12` with a password. In App Store
   Connect › Users and Access › Integrations, create a team API key with
   the Developer role. Then:

   ```sh
   base64 -i developer-id.p12 | gh secret set MACOS_CERTIFICATE
   gh secret set MACOS_CERTIFICATE_PASSWORD
   gh secret set APPLE_API_KEY < AuthKey_XXXXXXXXXX.p8
   gh secret set APPLE_API_KEY_ID
   gh secret set APPLE_API_ISSUER
   ```
2. **Windows.** Get a code signing certificate that comes as a `.pfx`,
   then:

   ```sh
   base64 -w0 code-signing.pfx | gh secret set WINDOWS_CERTIFICATE
   gh secret set WINDOWS_CERTIFICATE_PASSWORD
   ```

   Most certificates sold today keep their key on a hardware token or in a
   signing service, and never come as a `.pfx`: those sign with MyGo's
   `windows.signCommand` instead, which takes a step of its own. SmartScreen
   may still warn about a new certificate until it has a reputation.
3. **Homebrew.** Create the public repository `fzn0x/homebrew-tap`, and a
   fine-grained personal access token for it alone with Contents: Read and
   write, then `gh secret set HOMEBREW_TAP_TOKEN`. The next release writes
   `Casks/mixmusic.rb` there, for `brew install --cask fzn0x/tap/mixmusic`.

The first signed release: check on a Mac and on Windows that it opens
without a warning, and that `brew install`, `brew upgrade` and
`brew uninstall` work. Then take "not notarized yet" and "not signed yet"
out of the README's first launch, docs/troubleshooting.md and the
website, and "once its tap is up" out of the README's Homebrew line. The
release notes and the cask's caveats leave them out by themselves.

## The website

The [website](site/templates/index.html) and the docs' pages are built with
[Zola](https://www.getzola.org) from `site/` and `docs/` (see
[site/zola.toml](site/zola.toml)), and go out to two places whenever what
they're built from changes on main:

- [pages.yml](.github/workflows/pages.yml) publishes them to GitHub Pages,
  at https://fzn0x.github.io/mixmusic/.
- [cloudflare.yml](.github/workflows/cloudflare.yml) builds them again with
  `--base-url https://mixmusic.functionx.dev` and deploys them to that
  domain as the Worker of [site/wrangler.jsonc](site/wrangler.jsonc), which
  serves only the built files. It needs two secrets in the repository: a
  Cloudflare API token as `CLOUDFLARE_API_TOKEN`, made from the "Edit
  Cloudflare Workers" template with the functionx.dev zone in it, and the
  account's ID as `CLOUDFLARE_ACCOUNT_ID`. Pull requests that change the
  site check the Worker without deploying it, and keep the built site as
  their `site-cloudflare` artifact.

## The Flatpak

[packaging/flatpak](packaging/flatpak) packs the app of the Linux build
into a Flatpak, `io.github.fzn0x.mixmusic`, on the GNOME runtime, with the
tray's library from Flathub's shared modules (a git submodule) and Node.js
from the SDK's extension. CI builds it on every pull request, installs it
and starts MixMusic in it; its `flatpak` artifact is the bundle. To build
it on Linux, with [Flatpak](https://flatpak.org/setup/) and
`flatpak install flathub org.flatpak.Builder`:

```sh
git submodule update --init
bun run build -- -platform linux/amd64
mkdir -p packaging/flatpak/dist
cp build/linux-amd64/mixmusic-*-linux-amd64.tar.gz packaging/flatpak/dist/mixmusic-linux-amd64.tar.gz
go run ./tools/metainfo -o packaging/flatpak/dist/io.github.fzn0x.mixmusic.metainfo.xml
flatpak run org.flatpak.Builder --user --install --install-deps-from=flathub --force-clean \
  build/flatpak packaging/flatpak/io.github.fzn0x.mixmusic.yml
flatpak run io.github.fzn0x.mixmusic
```

`flatpak run --log-session-bus io.github.fzn0x.mixmusic` shows what the
sandbox refuses on the session bus. GNOME runtimes get about a year of
support: the manifest's `runtime-version`, the `gnome-<version>` image of
ci.yml and release.yml, and the Node.js extension's branch move together.

## Translations of the README

[README.zh-CN.md](README.zh-CN.md) (Simplified Chinese) and
[README.id.md](README.id.md) (Indonesian) translate
[README.md](README.md), which stays the one that's current. A pull request
that changes the README needn't change them: say in it which sections
changed, so that a translator can follow. Corrections and new languages are
welcome, as `README.<language code>.md` with a link in each README's
language line.

## Reports

Open an [issue](https://github.com/fzn0x/mixmusic/issues) with what you did,
what happened, and what you expected, with your system and MixMusic's
version (Settings › About). For security problems, see
[SECURITY.md](SECURITY.md) instead.

## Conduct

Everyone taking part follows the [code of conduct](CODE_OF_CONDUCT.md).

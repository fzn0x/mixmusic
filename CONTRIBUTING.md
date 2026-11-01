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

[docs/architecture.md](docs/architecture.md) tells where things live. A few
rules keep the project healthy:

- **Every system.** Changes must build for macOS, Windows and Linux; `GOOS=linux go vet ./...` and `GOOS=darwin go vet ./...` check them from Windows.
- **The theme contract is public.** Community themes set the tokens of
  [docs/themes.md](docs/themes.md): colors, fonts, a radius and layout
  tokens, whose defaults keep Mix's look. Rename or
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

Installed apps update themselves from the latest release. Its update files are
signed with the key whose public half is `updates.publicKey` in
[mygo.config.ts](mygo.config.ts); the secret half is the repository's
`MYGO_UPDATER_PRIVATE_KEY` secret, which the workflow signs with. Without it, the
release has no update files and apps stay on the version they have.

## Reports

Open an [issue](https://github.com/fzn0x/mixmusic/issues) with what you did,
what happened, and what you expected, with your system and MixMusic's
version (Settings › About). For security problems, see
[SECURITY.md](SECURITY.md) instead.

## Conduct

Everyone taking part follows the [code of conduct](CODE_OF_CONDUCT.md).

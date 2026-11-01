# MixMusic's community add-ons, as apps download them

This branch holds the files MixMusic's marketplace downloads: the themes and
plugins of `community/` on main, with the plugins' JavaScript compiled from
their TypeScript. The Registry workflow (`.github/workflows/registry.yml`)
publishes it on every change to `community/` on main: don't edit it by hand.

- `plugins/` and `themes/`: the add-ons of `community/registry-2.json`.
- `web/`: the add-ons of `community/registry.json`, for MixMusic 0.4 and
  before, as they were when MixMusic moved to its own interface.

To add or change an add-on, open a pull request against main: see
`community/README.md`.

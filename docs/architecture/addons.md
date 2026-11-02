# Add-ons

- **`internal/addons`** validates theme and plugin manifests, lists the
  built-in and installed add-ons, and installs add-ons from the community
  registry.
- **`internal/colorfile`** reads the colours of pywal's `colors.json`,
  matugen's JSON, MixMusic's matugen template and a `theme.json`, telling
  them apart by what they hold; `internal/app/colorfile.go` follows the
  file the settings name with MyGo's file watch, and keeps the colours it
  read last while the file is half written.
- **`internal/plugins`** runs the plugins the user enabled, each in a
  JavaScript runtime of its own, [sobek](https://github.com/grafana/sobek)
  (a fork of goja with ES modules), on a goroutine of its own: their
  modules, `console`, timers and `fetch`, and the API of
  [plugins.md](../plugins.md), built in `api.js` over Go natives. What they add
  to the interface the windows show: pages and widgets as views of
  elements, which the windows draw in their own styles, and canvases, whose
  drawing calls the plugin records and the window paints.

# Settings

- **`internal/store`** writes JSON files atomically. A file the user may
  edit while MixMusic runs, `settings.json`, it reads again when asked
  (`File.Reload`), and before each write, so that it never saves over an
  edit it hasn't read: it merges the edit in, by top-level keys. While the
  file doesn't decode, it leaves it as it is and holds its writes.
  **`internal/settings`** defines the settings, keeps them in range, and
  reads `settings.json` (`Decode`), at start and on each edit alike.
- **A struct for each section.** `Settings` embeds one struct for each
  part of the settings, such as `Appearance`, `Playback` and `Desktop`, each
  in a file of its own with its defaults and its clean-up. JSON flattens
  embedded structs, so `settings.json` keeps one flat object, in the
  order the sections are embedded. `testdata/default.json` and
  `testdata/full.json` pin how it writes.
- **Paths.** encoding/json writes text as UTF-8: a path whose bytes
  aren't, as a file's name on Linux may be, would read back with
  replacement characters, and name no file. `store.EncodePath` keeps such
  a path in base64, after `base64:`, which no absolute path starts with;
  one that is UTF-8 stays as it is. `Settings.MarshalJSON` writes
  `colorFile`, `downloadsFolder`, `musicFolders` and `ytdlpPath` so, and
  `UnmarshalJSON` reads them back, which the store's copies through JSON
  go through too; `local-files.json` keeps its paths the same way.
- **The settings page** builds each tab from the parts that registered for
  it with `addSettings`, from their features' files, in the order they
  asked for.

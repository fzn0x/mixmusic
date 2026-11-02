# Settings

- **`internal/store`** writes JSON files atomically. A file the user may
  edit while MixMusic runs, `settings.json`, it reads again when asked
  (`File.Reload`), and before each write, so that it never saves over an
  edit it hasn't read: it merges the edit in, by top-level keys. While the
  file doesn't decode, it leaves it as it is and holds its writes.
  **`internal/settings`** defines the settings, keeps them in range, and
  reads `settings.json` (`Decode`), at start and on each edit alike.

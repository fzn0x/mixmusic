# Backups

Back up and restore (#592) live in `internal/backup`, which knows the
files, and in `internal/app/backup.go` and the root's `backup.go`, which
ask for them.

- **A backup** is a zip of the files of the data folder of the parts
  ticked (`backup.Part`): Library (`library.json`, `plays.jsonl`,
  `local-copies.json`, `mix-covers/`), Settings (`settings.json`),
  Add-ons (`addons/`), Queue (`queue.json`, `positions.json`), and
  Downloads, a `downloads.json` made of `downloads.Manager.Listing`, not
  a file of the folder. Each part names its files, so that nothing else,
  as a sign-in or a file added later, ever goes in. `manifest.json` tells
  the format (`backup.Format`), the version, the time, the system, the
  folders of music and downloads, and each entry's size and SHA-256.
  `backup.Write` writes the zip beside its file, and renames it over.
- **Opening one** (`backup.Open`) refuses, with a `*backup.Error`, a file
  that isn't a backup, is damaged (its CRC, size or hash), is of a newer
  format, is past 512 MB, unpacks past 2 GB, or has an entry outside the
  data folder or of no part. It unpacks it into the data folder's
  `restore/open-*`, for the preview to read, and brings an older format
  up to date (`upgrades`).
- **A restore** (`Backup.Stage`) moves what is ticked into
  `restore/next/files`, then makes it fit this computer: the songs of
  music files move to the folder here that matches theirs
  (`localfiles.MoveID`, `Library.Move`, `localfiles.MoveLinks`), the
  library is merged with this one's for Add to my library
  (`library.MergeData`, `library.MergePlays`), and the settings keep this
  computer's folders, tools and devices, with no plugin on
  (`backup.KeepLocal`). `restore/next/plan.json`, written last, lists the
  paths it replaces. MixMusic restarts, and `main` calls `backup.Apply`
  before `app.New` opens anything: each path moves to `restore/aside`,
  and the staged one into its place; should one fail, all move back.
  What was set aside becomes `restore/undo`, which `backup.StageUndo`
  stages to swap back.
- **Automatic backups** (`App.autoBackup`) look a moment after the start
  and then each hour whether one is due (`backup.Due`), and run in
  `App.flush` as MixMusic quits for those of each quit. `backup.Sum`, a
  fingerprint of the entries, leaves out one where nothing changed since
  the last, which `backups.json` keeps. `backup.Auto` writes it to the
  folder and keeps the last few (`backup.Prune`), deleting only the names
  `backup.AutoName` makes. A folder's problem is a `*backup.FolderError`,
  which the windows tell, and the next start tries again.

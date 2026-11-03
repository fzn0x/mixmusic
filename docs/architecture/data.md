# Data

Everything lives in the user's data folder (`%AppData%\MixMusic` on
Windows, `~/Library/Application Support/MixMusic` on macOS,
`~/.config/MixMusic` on Linux): `settings.json`, `library.json`,
`queue.json`, `addons/`, `bin/` (yt-dlp), the YouTube session
(`youtube-session.bin`, encrypted with a key of the system's credential
store, which also keeps the Spotify token and the Last.fm connection), the
scrobbles still to send, and `updater.json`, the update preferences. On
Windows, a secret the Credential Manager can't keep, as in a logon session
without credentials, is sealed for the user with DPAPI in
`spotify-token.dpapi`, `lastfm.dpapi` or `youtube-session-key.dpapi`; on
Linux without a Secret Service, the secrets and the session are in plain
files only the user can read (`spotify-token.json`, `lastfm.json`,
`youtube-session.json`). `secretStore` (`internal/app/secrets.go`) moves a
secret found in a file into the credential store once it can keep it.
`settings.json` may be edited while MixMusic runs: `App.watchSettings`
watches it (`mygo.WatchFiles`, or a look every two seconds where it can't),
and `App.ReloadSettings` applies an edit as Settings applies a change, then
tells the windows (`App.Watch`). A file with a mistake is told of after a
second, and kept as `settings.json.broken` if MixMusic quits before it is
fixed; a `settings.json` that is a link stays one. The
cache folder holds audio, the covers shown (`artwork/`), on Linux the
cover the desktop's media controls show (`now-playing/`), matches, how loud
YouTube measured the tracks played (`loudness.json`), the videos YouTube
labels as made with AI, for 30 days (`ai-labels.json`), the dictionary of
the readings of kanji romaji downloads (`romaji/`), and yt-dlp's cache,
and the logs folder the log and the crash reports. The downloads are in a
folder of their own, which the user can move in Settings › Playback
(`downloadsFolder` in `settings.json`): by default
`%LocalAppData%\MixMusic\downloads` on Windows,
`~/Library/Application Support/MixMusic/downloads` on macOS, and
`$XDG_DATA_HOME/MixMusic/downloads` (`~/.local/share/MixMusic/downloads`)
on Linux: never a folder the system treats as a cache (`~/Library/Caches`,
`~/.cache`), whose files cleaning tools delete, nor the Windows folder
that roams. `MIXMUSIC_DATA_DIR` moves all four into one folder,
for a portable copy or a test run.
`MIXMUSIC_YTDLP_ONLY=1` has yt-dlp find every stream, as before MixMusic
asked YouTube's player API, to time one against the other or tell which
fails: the log tells which streams took over a second to find, and which
tracks over two to start.

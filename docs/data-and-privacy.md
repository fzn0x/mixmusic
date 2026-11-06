# Your data and privacy
<!-- docs: group=inside order=50 -->

MixMusic has no account, server, analytics or telemetry of its own. This page tells where it keeps your data and what each feature sends where.

Everything MixMusic knows of you stays in folders on your computer and in
your system's credential store, apart from what you keep on YouTube Music,
Spotify and Last.fm themselves. For the code, read
[the data's part of the architecture](architecture/data.md).

## Where MixMusic keeps its data

MixMusic keeps its data in four folders, and your sign-ins in the
system's credential store. Settings › About names the data folder, and
opens the logs folder. A development copy, run with `bun run dev`, uses
`MixMusic Dev` in place of `MixMusic` in these names, so it never touches
an installed copy's data.

### Windows

- Data (settings, library, add-ons, yt-dlp): `%AppData%\MixMusic`
- Cache: `%LocalAppData%\MixMusic`
- Logs: `%AppData%\MixMusic\logs`
- Downloads: `%LocalAppData%\MixMusic\downloads`
- Sign-ins: the Credential Manager

### macOS

- Data: `~/Library/Application Support/MixMusic`
- Cache: `~/Library/Caches/MixMusic`
- Logs: `~/Library/Logs/MixMusic`
- Downloads: `~/Library/Application Support/MixMusic/downloads`
- Sign-ins: the login Keychain

### Linux

- Data: `~/.config/MixMusic`
- Cache: `~/.cache/MixMusic`
- Logs: `~/.config/MixMusic/logs`
- Downloads: `~/.local/share/MixMusic/downloads`
- Sign-ins: the Secret Service, as GNOME Keyring, KWallet or KeePassXC

`$XDG_CONFIG_HOME`, `$XDG_CACHE_HOME` and `$XDG_DATA_HOME` take the place
of `~/.config`, `~/.cache` and `~/.local/share` when you set them. The
Flatpak keeps all of it under `~/.var/app/io.github.fzn0x.mixmusic`: the
data in `config/MixMusic`, the cache in `cache/MixMusic`, and the
downloads in `data/MixMusic/downloads`, apart from a `.deb`, `.rpm`,
AppImage or archive copy's.

### The data folder

- `settings.json`: your settings. You can edit it while MixMusic runs, and
  what you save applies at once.
- `library.json`: your liked and disliked songs, your mixes, the last
  1,000 tracks you played, which the history and the stats read, your
  last 20 searches, the shows you follow, and the artists you follow
  in MixMusic.
- `local-files.json`: the index of the [music on this computer](local-music.md):
  the path, size, time of change and tags of each music file of the
  folders you added. The folders themselves are `musicFolders` in
  `settings.json`.
- `queue.json`: up to 500 tracks of the queue, around the one playing,
  and where it was.
- `positions.json`: where you left the latest 500 podcast episodes.
- `lastfm-queue.json`: the scrobbles still to send, as while offline.
- `updater.json`: your choices of updates.
- `addons/`: the themes and plugins you installed, and what plugins keep
  in `addons/.storage`.
- `bin/`: MixMusic's own yt-dlp.
- `youtube-session.bin`: your YouTube Music session, encrypted with a key
  that the credential store keeps.
- `matugen.json`, when you follow the colours that matugen writes there.

### The cache folder

- `audio/`: the offline cache, the tracks you played, as much as
  Settings › Playback › Offline cache allows (1 GB by default).
- `artwork/`: the covers you saw, up to 256 MB.
- `local-art/`: a copy of the covers of the music on this computer, which
  a file holds or that is beside it, by its hash.
- `now-playing/`, on Linux: the cover of what plays, for the desktop's
  media controls.
- `matches.json`: the YouTube Music recordings found for Spotify tracks.
- `loudness.json`: how loud YouTube measured the tracks you played.
- `ai-labels.json`: the videos YouTube labels as made with AI, kept 30
  days, while you skip them.
- `releases.json`: the releases of the last year of the artists you
  follow, for [New from artists you follow](library.md#new-releases).
- `lyrics-translations/`: the translations of lyrics you turned on, by
  track and language.
- `romaji/`: the dictionary of the readings of kanji, once you turn on
  romaji (11.7 MB).
- `yt-dlp/`: what yt-dlp keeps of YouTube's player.

### The logs folder

- `mixmusic.log`: what went wrong, started anew at a megabyte, with the
  three before it kept. It leaves out the cookies, hashes, keys and
  tokens a line may hold, and the addresses of streams, which carry your
  IP address.
- `crash-<time>.log`: the report of a crash, the last five kept. The next
  start offers to report it, in a new GitHub issue that you read and send
  yourself.

### Sign-ins

Your Spotify sign-in, your Last.fm connection and the key of your YouTube
Music session are in the system's credential store, which encrypts them
for your account. MixMusic never sees your passwords: you sign in on
Google's, Spotify's and Last.fm's own pages.

Where the credential store can't keep a secret:

- **On Windows**, as in a session without credentials, the secret is
  encrypted for your Windows account with DPAPI, in
  `spotify-token.dpapi`, `lastfm.dpapi` or `youtube-session-key.dpapi` in
  the data folder.
- **On Linux without a Secret Service**, the secrets are in files only
  your user can read: `spotify-token.json`, `lastfm.json` and
  `youtube-session.json`.
- Once the store can keep them, MixMusic moves them into it, and deletes
  the files.

## What each feature sends where

| Feature | Goes to | What it sends |
|---|---|---|
| Browsing YouTube Music: search, home, explore, pages | YouTube Music (`music.youtube.com`) | what you search and open, with a visitor ID YouTube gave; signed in, your session's cookies |
| Your YouTube Music library, once you sign in | YouTube Music | the likes, dislikes, saves, playlists and follows you make |
| New releases of the artists you follow | YouTube Music, and Spotify once you sign in | the artists' IDs, to read their pages or lists of albums, at most every 12 hours each |
| The audio of a track | YouTube's player API (`www.youtube.com`) and media servers; yt-dlp, which asks YouTube too | the video's ID, always signed out |
| Your YouTube Music history, signed in, while "Add what you play to your YouTube Music history" is on | YouTube Music | the tracks you play, once they played a while, and how far you got in episodes |
| Skip songs made with AI, while it's on | YouTube (`www.youtube.com`), signed out | the video's ID, once a video |
| Covers | the image servers of YouTube, Google and Spotify | the image asked for |
| Spotify, once you sign in | Spotify (`accounts.spotify.com`, `api.spotify.com`) | what you search and open, your library's changes, and the commands of what plays |
| Spotify in MixMusic | Spotify's player, loaded from `sdk.scdn.co` | what Spotify's own player sends |
| Lyrics | Unison (`unison.boidu.dev`) | the video ID of the track that plays, or, without one, its title, artists, album and length |
| Lyrics, when Unison has none | LRCLIB (`lrclib.net`) | the title, artists, album and length of the track that plays |
| Lyrics of a track on this computer | Unison, then LRCLIB | its title, artists, album and length, as its tags tell, unless they name no artist; never its file |
| Translate, or romanized Arabic, while it's on beside the lyrics | Unison, which has Google translate them | the lyrics of the track that plays, with its video ID |
| Romaji, the first time you turn it on | GitHub | a download of the dictionary of kanji readings; the lyrics stay on your computer |
| Last.fm, once you connect it | Last.fm | the title, artists, album and length of what you play, and when |
| Discord, unless you turn it off in Settings › Desktop | the Discord app on your computer | the title, artists, album and cover address of what plays, and its times |
| Casting | the cast device you choose, on your network | the title, artist and cover of what plays; the device fetches the audio from MixMusic while you cast |
| Checks for updates, unless you turn them off in Settings › About | GitHub | a request for the newest release, and its download when you install it |
| yt-dlp, when you install or update it | GitHub | a download of yt-dlp's latest release |
| The marketplace, when you open it | GitHub | a download of the registry, and of the add-ons you install |
| Knowing whether the network answers | YouTube and Spotify | a request over HTTPS for an empty answer |

- **Plugins** can send what they like: a plugin's permissions are shown
  to you, not enforced, which is why MixMusic asks you to trust a plugin
  before it enables it.
- **Report a problem**, in Settings › About, opens a new GitHub issue in
  your browser, with your version, system, tools, theme and plugins
  filled in. Nothing goes until you send it yourself.

## What stays on your computer

- Your settings, your library, your history and your searches.
- The [music on this computer](local-music.md#privacy): MixMusic plays it
  from its files, and sends no file anywhere. Last.fm and Discord hear of
  what plays as of any track, as their settings say.
- Latin letters for lyrics, Japanese romaji included: MixMusic makes them
  itself, and sends no lyrics for them. Only Arabic's come from Unison,
  while romanize is on.
- The audio: the offline cache, the downloads, and on Windows what the
  visualizer hears of Spotify through Windows' process loopback, which
  hears only Spotify's app. MixMusic analyzes that audio in memory, and
  never records or sends it.
- The log and the crash reports, until you choose to send one.

## Clear, move or reset

- **Your history:** the library's history page clears what you played.
- **The cache:** Settings › Playback › Offline cache › Clear tracks and
  Clear covers.
- **A sign-in:** signing out of a service in Settings › Accounts deletes
  its sign-in.
- **Your settings:** Settings › Desktop › Reset settings puts back the
  settings of a new install. Your accounts, library and add-ons stay.
- **The downloads:** Settings › Playback › Downloads › Folder moves them
  to another folder, with what they hold.
- **Everything:** sign out of each service, quit MixMusic, then delete
  the data, cache, logs and downloads folders above.

**To bring your data to another copy**, as to another computer or into
the Flatpak, quit both copies, copy `settings.json`, `library.json`,
`queue.json`, `positions.json` and the `addons` folder into the other
copy's data folder, and sign in to your accounts again there: sign-ins
stay in the credential store of the computer they were made on.

**To keep everything in one folder**, as for a portable copy or a test
run, start MixMusic with the environment variable
`MIXMUSIC_DATA_DIR` set to a folder: it keeps its data, cache, logs and
downloads in the `data`, `cache`, `logs` and `downloads` folders inside.

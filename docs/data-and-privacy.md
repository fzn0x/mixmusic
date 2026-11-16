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
  1,000 tracks you played, which the history reads, your
  last 20 searches, the shows you follow, and the artists you follow
  in MixMusic.
- `plays.jsonl`: every track you played, once it counted, a line each,
  which the stats, Your year and the Plays sort read: when, and the
  track's ID, service, title, artists, album and length, without covers.
  It grows by about 1 MB a year at 10 plays a day. Clear history empties
  it.
- `mix-covers/`: the covers you chose for your mixes, cut to a square.
- A GIF in `mix-covers/`, beside the still of a mix's cover, for one that
  moves.
- `local-files.json`: the index of the [music on this computer](local-music.md):
  the path, size, time of change and tags of each music file of the
  folders you added. The folders themselves are `musicFolders` in
  `settings.json`.
- `local-copies.json`: the [copies of one song](local-music.md#copies-of-one-song)
  among those files, the ones you linked or unlinked by hand, and what
  each copy's audio is.
- `queue.json`: up to 500 tracks of the queue, around the one playing,
  and where it was; while you're in a room of
  [Listen with friends](listen-with-friends.md), yours as it was as you
  joined. A room keeps nothing else.
- `positions.json`: where you left the latest 500 podcast episodes.
- `lastfm-queue.json`: the scrobbles still to send, as while offline.
- `listenbrainz-queue.json`: the same for ListenBrainz, and
  `listenbrainz-recordings.json` the IDs ListenBrainz gave the recordings
  of the latest 5,000 tracks that played, which likes send their loves by.
  Disconnecting deletes both.
- `updater.json`: your choices of updates.
- `remote.json`: whether [remote control](remote-control.md) is on, and
  the computers paired with this one, by their names and their keys'
  fingerprints. The key of this computer is in the credential store.
- `mcp.json`, while [AI assistants](ai-assistants.md) may control
  MixMusic: the port on `127.0.0.1` the command they start connects to,
  and the key it gives, new each time it turns on.
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
- `moving-covers/`: the clips of albums' moving covers that played, as
  much as Settings › Playback › Offline cache allows (256 MB by default),
  and which albums have one, once a source of them is chosen.
- `local-art/`: a copy of the covers of the music on this computer, which
  a file holds or that is beside it, by its hash.
- `now-playing/`, on Linux: the cover of what plays, for the desktop's
  media controls.
- `matches.json`: the YouTube Music recordings found for Spotify tracks.
- `loudness.json`: how loud YouTube measured the tracks you played.
- `ai-labels.json`: the videos YouTube labels as made with AI, kept 30
  days, while you skip them.
- `sponsorblock.json`: the parts SponsorBlock marks of the videos you
  played, 5,000 videos at most, those you downloaded first, while you skip
  them: asked again after 7 days, and used however old offline.
- `releases.json`: the releases of the last year of the artists you
  follow, for [New from artists you follow](library.md#new-releases).
- `lyrics-translations/`: the translations of lyrics you turned on, by
  track and language.
- `lyrics-timing.json`: where the songs of the music videos you played
  start in them, for [their lyrics](lyrics.md#music-videos), kept 180
  days, and the videos whose song has no clear match, kept 7, 400 videos
  at most.
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
- `memory.log` and `heap-<time>.pprof`, only while you
  [record memory use](settings.md#about): a line each minute of how much
  memory MixMusic uses, its Go heap's and the whole app's as the system
  tells it, and profiles of the Go heap, the last five kept, which tell
  where in MixMusic's code its memory was taken, and how much. They hold
  only how MixMusic uses its memory: nothing of your music, your
  library or your accounts. Report a problem tells of them, for you to
  attach.

### Sign-ins

Your Spotify sign-in, your Last.fm and ListenBrainz connections and the key
of your YouTube Music session are in the system's credential store, which encrypts them
for your account. MixMusic never sees your passwords: you sign in on
Google's, Spotify's and Last.fm's own pages. Signing in to YouTube Music
[with your browser](accounts.md#sign-in-with-your-browser) opens it with
a profile made for that sign-in alone, which MixMusic deletes once it took
the session; it never reads your own profiles.

Where the credential store can't keep a secret:

- **On Windows**, as in a session without credentials, the secret is
  encrypted for your Windows account with DPAPI, in
  `spotify-token.dpapi`, `lastfm.dpapi`, `listenbrainz.dpapi` or
  `youtube-session-key.dpapi` in the data folder.
- **On Linux without a Secret Service**, the secrets are in files only
  your user can read: `spotify-token.json`, `lastfm.json`,
  `listenbrainz.json` and `youtube-session.json`.
- Once the store can keep them, MixMusic moves them into it, and deletes
  the files.

## What each feature sends where

| Feature | Goes to | What it sends |
|---|---|---|
| Browsing YouTube Music: search, home, explore, pages | YouTube Music (`music.youtube.com`) | what you search and open, with a visitor ID YouTube gave; signed in, your session's cookies |
| Your YouTube Music library, once you sign in | YouTube Music | the likes, dislikes, saves, playlists and follows you make |
| New releases of the artists you follow | YouTube Music, and Spotify once you sign in | the artists' IDs, to read their pages or lists of albums, at most every 12 hours each |
| The audio of a track | YouTube's player API (`www.youtube.com`) and media servers; yt-dlp, which asks YouTube too | the video's ID, signed out |
| The audio of a track, while "Play audio as your account" is on | yt-dlp, which asks YouTube (`www.youtube.com`, `music.youtube.com`) | the video's ID, with your YouTube Music session's cookies, in a file of yours alone that goes once yt-dlp ran |
| Your YouTube Music history, signed in, while "Add what you play to your YouTube Music history" is on | YouTube Music | the tracks you play, once they played a while, and how far you got in episodes |
| Skip songs made with AI, while it's on | YouTube (`www.youtube.com`), signed out | the video's ID, once a video |
| Skip the parts SponsorBlock marks, while it's on | SponsorBlock (`sponsor.ajay.app`), or the mirror you name | only the first 4 characters of the SHA-256 hash of the video's ID, once a video, never the ID: SponsorBlock answers with the parts of all the videos whose hash starts so, some 65,000 of them, and MixMusic keeps the video's. Nothing is sent of what you skip |
| Covers | the image servers of YouTube, Google and Spotify | the image asked for |
| Spotify, once you sign in | Spotify (`accounts.spotify.com`, `api.spotify.com`) | what you search and open, your library's changes, and the commands of what plays |
| Spotify in MixMusic | Spotify's player, loaded from `sdk.scdn.co` | what Spotify's own player sends |
| Lyrics, from the [sources](lyrics.md#choose-and-order-the-sources) on in Settings › Playback › Lyrics, one at a time in their order, until one has synced lyrics | Better Lyrics (`api.betterlyrics.org`) | the title, first artist, album and length of the track that plays, once it has an album and a length |
| Lyrics, the same way | Unison (`unison.betterlyrics.org`) | the video ID of the track that plays, or, without one, its title, artists, album and length |
| Lyrics, the same way | LRCLIB (`lrclib.net`) | the title, artists, album and length of the track that plays; of a music video, its song's |
| [Lyrics timed to a music video](lyrics.md#music-videos), while "Time lyrics to music videos" is on | YouTube's player API (`www.youtube.com`) and media servers, signed out | the video IDs of the music video and of its song, once a video, for stretches of their audio: of the song, only those compared. No play is reported |
| Lyrics, the same way | YouTube Music | the video ID of the track that plays, or of its recording there |
| Lyrics, the same way, and the transcript of a podcast episode | YouTube (`www.youtube.com`), signed out | the video ID of the track that plays, or of its recording there, for its captions |
| Lyrics of a track on this computer | the sources on, but YouTube captions | its title, artists, album and length, as its tags tell, unless they name no artist; never its file |
| Translate, or romanized Arabic, while it's on beside the lyrics | Unison, which has Google translate them | the lyrics of the track that plays, with its video ID |
| Romaji, the first time you turn it on | GitHub | a download of the dictionary of kanji readings; the lyrics stay on your computer |
| Last.fm, once you connect it | Last.fm | the title, artists, album and length of what you play, and when; with Send likes as loves on, the songs you like and unlike |
| ListenBrainz, once you connect it | ListenBrainz (`api.listenbrainz.org`), or the server you name | the title, first artist, album and length of what you play, and when, that MixMusic sent it, and for YouTube Music and Spotify tracks the service, the track's page, and Spotify's address and the ISRC where known; with Send likes as loves on, the songs you like and unlike, and the title, artist and album of a song looked up for its love |
| Discord, unless you turn it off in Settings › Desktop | the Discord app on your computer | the title, artists, album and cover address of what plays, and its times, as you choose them; with the Listen button, the track's link on its service |
| Casting | the cast device you choose, on your network | the title, artist and cover of what plays; the device fetches the audio from MixMusic while you cast |
| AI assistants, while you let them control MixMusic | the assistant on your computer, and from it the AI model it uses, which may be on the internet | what plays and the queue, the results of its searches, and the names of your playlists and mixes, as it asks for them |
| Remote control, with a computer paired | that computer's MixMusic, on your network, encrypted | what plays, the volume and the next 20 tracks, and the commands; when you move what plays to it, the queue, with the tags of your music files, never their paths |
| Listen with friends, while you host a room | each guest's MixMusic, on your network, or over the internet while you let friends outside it join, encrypted, once it proved the code or the link | your name in rooms, what plays, where it is and its volume, and up to 1,000 tracks of the queue, with the tags of your music files, never their paths or their audio; before a guest joins, who hosts, what plays and how many listen; the room's announcement tells your name in rooms to the whole network |
| Listen with friends, while you're in a room | the host's MixMusic, on your network, or over the internet at the address its link names, encrypted | your name in rooms, and when each track is ready |
| Listen with friends, while you let friends outside your network join | your router | a request to forward one TCP port, the room's, to your computer, by UPnP, NAT-PMP or PCP, and to let it in at your IPv6 address, renewed while the room lasts and removed as it ends; your invite link, which anyone it reaches can read, carries your router's internet address and the room's port, and your computer's IPv6 address, a temporary one, never one made of its hardware address, or the address you typed for invites in their place |
| Checks for updates, unless you turn them off in Settings › About | GitHub | a request for the newest release, and its download when you install it |
| yt-dlp, when you install or update it | GitHub | a request for the version of yt-dlp's latest release, from its page or GitHub's API, and a download of it |
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
  from its files, and sends no file anywhere. Last.fm, ListenBrainz and
  Discord hear of what plays as of any track, as their settings say.
- Latin letters for lyrics, Japanese romaji included: MixMusic makes them
  itself, and sends no lyrics for them. Only Arabic's come from Unison,
  while romanize is on.
- The audio: the offline cache, the downloads, and on Windows what the
  visualizer hears of Spotify through Windows' process loopback, which
  hears only Spotify's app. On macOS it hears Spotify in MixMusic through
  a process tap of MixMusic's own web views, which macOS lets it do once
  you allowed it to record what other apps play. MixMusic analyzes that
  audio in memory, and never records or sends it.
- The log and the crash reports, until you choose to send one.

## Clear, move or reset

- **Your history:** the library's history page clears what you played,
  and the plays the stats count.
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
`queue.json`, `positions.json` and the `addons` and `mix-covers` folders
into the other copy's data folder, and sign in to your accounts again
there: sign-ins stay in the credential store of the computer they were
made on. Both must be quit, because a running MixMusic keeps
`library.json` and most other files in memory and writes over a copied
one at its next save. The cache folder is safe to leave out, as is `bin/`
(MixMusic downloads its yt-dlp again). `remote.json` is no use on the
other computer, as its key stays in the credential store, and `mcp.json`
is made anew each time the assistants' port turns on.

**To keep everything in one folder**, as for a portable copy or a test
run, start MixMusic with the environment variable
`MIXMUSIC_DATA_DIR` set to a folder: it keeps its data, cache, logs and
downloads in the `data`, `cache`, `logs` and `downloads` folders inside.

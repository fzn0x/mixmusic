# The services

- **`internal/media`** defines what every service shares: tracks, albums,
  artists, playlists, shelves. IDs carry their service: `yt:<video id>`,
  `sp:<track id>`, `mx:<mix id>`. The services give a playlist with its
  first page of tracks, and `More`, the token of the next;
  `Playlist.ReadOn` reads on a page at a time, asks again after 1, 2 and
  4 seconds for a page that fails, and stops at `MaxTracks`, 20,000, past
  which the playlist is `Cut`. `Catalog.Playlist` reads one whole, for
  what takes all of it, as a download or a plugin.
- **`internal/youtube`** reads YouTube Music through the API its website uses:
  search, suggestions (searches, then songs, albums, artists and
  playlists), the home and explore pages, charts, moods and genres, albums,
  artists, playlists, related tracks, radios and plain lyrics. Below a
  playlist the user owns, YouTube Music suggests songs to add, five at a
  time: the playlist's `Suggest` is the token of the first, and each
  answer has the token of a refresh, for the next (`PlaylistSuggestions`).
  Signed in with the cookies of the user's session, which it signs its
  requests with as the website does, it reads and changes their library:
  liked songs, playlists, albums, subscriptions, likes, dislikes and
  saves, songs saved to the library apart from likes, by the feedback
  tokens of their menus in the queue of their "next", and it deletes the playlists the user made, which it tells from
  those they saved by the menus YouTube sends with them. An add of a
  video the playlist holds already YouTube refuses, offering in its
  answer the same add with its check skipped (`DEDUPE_OPTION_SKIP`):
  that, not the answer's words, which are in the account's language,
  tells it from other refusals (`ErrAlreadyInPlaylist`), and
  `AddToPlaylistAnyway` sends it. Collaboration on a playlist of the
  user's goes through the same `browse/edit_playlist`: an action that
  turns it on answers with an invite link (`InviteCollaborators`),
  another closes the playlist to contributions (`StopCollaboration`), and
  a private playlist, which collaborators can't open, is made unlisted
  first (`SetPlaylistPrivacy`). YouTube refuses the invite with 400
  ("Precondition check failed") until that change reaches it, 18 s in a
  live run, so `InviteCollaboratorsOnceUnlisted` asks again every 3 s for
  up to 45 s. A playlist page's header tells who may
  open a playlist of the user's, in its tools, and whether its avatars
  are its collaborators', by the panel they open: `Mine`, `Privacy` and
  `Collaborative` of `media.Playlist`. Its tests read hand-written
  answers of the shapes documented, which
  `MIXMUSIC_LIVE=1 MIXMUSIC_YT_WRITE=1 go test ./internal/youtube -run LiveCollaboration`
  records, with a second account in `MIXMUSIC_YT_COOKIES2` to join as a
  collaborator. It lists the
  Google accounts of the session and their brand channels, from YouTube's
  website, and uses a brand channel by its page ID: one whose page ID the
  list doesn't tell is listed as `Unswitchable`, as without it the account
  itself would be used. Once the app switches to a brand channel, it goes
  back if YouTube Music answers as the Google account itself, as the list
  names it; any other answer keeps the switch. It also reads
  whether YouTube labels a video as made with AI, from the answer of
  YouTube's own website, asked signed out with a field mask
  (`MadeWithAI`): nothing of YouTube Music carries the label. Its parsers
  are tested against recorded responses in `testdata/`, and against the
  live service with `MIXMUSIC_LIVE=1 go test ./internal/youtube -run Live`
  (`MIXMUSIC_YT_COOKIES` adds a signed-in account, and
  `MIXMUSIC_YT_WRITE=1` lets it make and delete a playlist there, and
  dislike a song and take the dislike back).
- **`internal/spotify`** signs in with PKCE on a loopback callback, refreshes
  tokens, reads the Web API (search, albums, artists, playlists, the
  library), and controls Spotify Connect devices. It changes the
  playlists the user may: adds tracks, moves one by its place in the
  playlist, which only a playlist whose items MixMusic all shows has, and
  makes one of the user's collaborative, off their profile, as Spotify
  asks. A playlist is `Mine` when its owner is the user signed in. A 403
  of such a change that isn't one of development mode's refusals is
  `ErrPlaylistRefused`, which says who may change what. An app in
  development mode may make all these changes: on 2026-11-11
  `MIXMUSIC_LIVE=1 MIXMUSIC_SPOTIFY_WRITE=1 MIXMUSIC_SPOTIFY_TOKEN=… go test ./internal/spotify -run LiveCollaboration`
  made a private playlist (`POST /me/playlists`), added to it, moved a
  track, made it collaborative and back, and took it out of the library
  with `DELETE /playlists/{id}/followers`. Spotify lists that route as
  removed in February 2026, with `DELETE /me/library` in its place; it
  still answered for that app's Client ID, made before then.
- **`internal/sysproxy`** picks the proxy of every request MixMusic makes,
  as the `Proxy` of `http.DefaultTransport`, which the clients use or
  clone: the environment's (`HTTPS_PROXY` and the like) when it names
  one; on Windows, failing that, the one of Windows' settings, which
  browsers, WebView2 and yt-dlp use too. Automatic setup comes first,
  detection or a setup script, which WinHTTP answers for each site and
  which is remembered for 5 minutes, failures too; then the proxy
  server, unless the bypass list names the address. Loopback addresses
  always connect directly. It reads the settings again when their
  registry key changes, as VPN and proxy apps turn them on and off,
  logs what MixMusic connects through, without user names or passwords,
  and gives yt-dlp a `--proxy` where yt-dlp wouldn't find that proxy
  itself. A package that clones `http.DefaultTransport` as it
  initializes imports it, as `internal/online` does. macOS and Linux
  keep the environment's proxy alone.
- **`internal/match`** finds a Spotify track's recording on YouTube Music, by
  title, artists and duration, for playback without a Spotify device. It
  also finds the song of a music video, by its title and artists, which
  the video's radio starts from: YouTube Music makes the radio of a video
  out of videos.
- **`lyrics`** finds synced lyrics on Unison and LRCLIB, reads TTML and LRC,
  and translates and romanizes lyrics on Unison.
- **`internal/romaji`** writes Japanese lyrics in Hepburn romaji on the
  computer, as Unison gives none for Japanese: kana from a table, and kanji
  by the readings that kagome's tokenizer finds in its IPA dictionary,
  word by word. The dictionary isn't built in, which would grow the app by
  11.7 MB: `Dictionary` downloads it from kagome-dict's module on GitHub
  the first time romaji is on for Japanese lyrics, checks it against the
  SHA-256 written in the package, keeps it in the cache's `romaji/`, and
  checks it again each time it loads. A download that fails or isn't the
  dictionary goes, and is tried again after 30 seconds, twice as long
  after each failure in a row, up to 10 minutes. Until it is there, the
  lines' kana are in romaji and their kanji stay as they are. Loaded, the
  dictionary takes some 89 MB of memory: it loads in about 0.4 s when
  lines need it, and goes 5 minutes after they last did. The windows make
  each track's romaji once, with its lyrics. Its tests read the
  dictionary from the Go module cache, where
  `go mod download github.com/ikawaha/kagome-dict/ipa@v1.2.6` puts it.
- **`internal/romanize`** writes lyrics in the other scripts in Latin
  letters on the computer, each run of a script of a line by the standard
  of its language, as the package's doc lists them: the Revised
  Romanization for Korean, with its sound changes; pinyin for Chinese;
  the systems of each language of Cyrillic, told by its letters when the
  lyrics' language isn't known; ELOT 743 for Greek; Hunterian for the
  Brahmic scripts, by one engine over their aligned blocks; RTGS for
  Thai, by a syllabifier of rules; BGN/PCGN for Lao and Burmese and the
  UN's system for Khmer, each parted into syllables by its script's own
  structure (`syllables.go` places them); EWTS for Tibetan; and the vowel
  points of Hebrew and Arabic where they're written. `Words` gives each
  word of a line synced word by word its part of the line in Latin
  letters. Chinese reads `pinyin.bin`, 177 KB built in, which
  `tools/pinyindata` makes from Unihan and mozillazg's pinyin-data and
  phrase-pinyin-data: each character's reading, and only the words read
  otherwise than their characters, 8,799 of 47,152. It is read the first
  time Chinese is romanized, in 4 ms, and holds 1.3 MB. The windows make
  each track's romanization once, with its lyrics (`lyricsromanize.go`),
  and ask Unison for none, but Arabic's, whose romanization by Google
  has the vowels Arabic doesn't write.
- **`internal/lastfm`** signs in to Last.fm with the user's API account and
  scrobbles by Last.fm's rules, keeping what fails offline for later.
- **`internal/discord`** shows what plays on the user's Discord profile, over
  Discord's local pipe or socket: an activity's lines, times, picture, the
  small image at its corner and up to two buttons, fitted to Discord's
  limits. What the activity holds is the app's choice
  (`internal/app/discordstatus.go`): `DiscordActivity` makes it of what
  plays and the settings, as a pure function that the settings page's
  preview draws too, and `presence` sends it again whenever the track, the
  play or pause, the position or those settings change. The badges of
  the services and of a pause are `brand/discord`'s PNGs, drawn by
  `tools/discordbadges`, which Discord reads from GitHub by their
  addresses, whatever the application.

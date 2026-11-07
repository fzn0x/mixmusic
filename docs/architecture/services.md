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
  artists, playlists, related tracks, radios and plain lyrics.
  Signed in with the cookies of the user's session, which it signs its
  requests with as the website does, it reads and changes their library:
  liked songs, playlists, albums, subscriptions, likes, dislikes and
  saves, songs saved to the library apart from likes, by the feedback
  tokens of their menus in the queue of their "next", and it deletes the playlists the user made, which it tells from
  those they saved by the menus YouTube sends with them. It lists the
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
  library), and controls Spotify Connect devices.
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
  Discord's local pipe or socket.

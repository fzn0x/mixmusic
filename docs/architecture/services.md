# The services

- **`internal/media`** defines what every service shares: tracks, albums,
  artists, playlists, shelves. IDs carry their service: `yt:<video id>`,
  `sp:<track id>`, `mx:<mix id>`. The services give a playlist with its
  first page of tracks, and `More`, the token of the next;
  `Playlist.ReadOn` reads on a page at a time, asks again after 1, 2 and
  4 seconds for a page that fails, and stops at `MaxTracks`, 20,000, past
  which the playlist is `Cut`. `Catalog.Playlist` reads one whole, for
  what takes all of it, as a download or a plugin. A service that leaves
  items out of a page, as Spotify's local files, tells how many by kind
  in `Playlist.Unplayable` (`ReadOnCounting` adds the pages up), and
  `playlistgap.go` shows them under the header with the explicit songs
  the settings hide, for a playlist of any service.
- **All of an artist's songs** (#543). YouTube Music lists them in a
  playlist, which the title of an artist's Top songs and its Show all
  open as a `VL…` browse ID of a playlist page (`songsList`):
  `media.Artist.SongsID` keeps it. The playlist reads whole without the
  link's params, with which YouTube answers not found; live, in November
  2026, it held at most 150 songs. `youtube.Client.SongList` reads it a
  page at a time, each tried again on `media.PageWaits`, and gives `each`
  the new songs of each page, a song the list holds twice once; a page
  past the first that still fails returns what came with a
  `media.ShortList`. `ArtistSongs` finds the list from the artist's ID
  first, in the shape of Spotify's, and `Catalog.YouTubeArtistSongs`
  routes to it. `Catalog.PlayArtist` (`internal/app/artistplay.go`)
  plays the first songs that come into a `player.List` and adds the rest
  as they come, none the settings hide, until the queue no longer holds
  the list. A read that came in part tells how many songs came. An
  artist without a list, or whose list's first page failed, plays their
  top songs: for an artist of the music on this computer, all of theirs.
  An artist's tile and its menu read the artist first (`Catalog.PlayArtistID`),
  then play as the page does; the assistants' `play_result` reads the songs
  whole (`Catalog.AllSongs`) and plays or queues them like an album.
- **`internal/youtube`** reads YouTube Music through the API its website uses:
  search, suggestions (searches, then songs, albums, artists and
  playlists), the home and explore pages, charts, moods and genres, albums,
  artists, playlists, related tracks, radios and plain lyrics. A page of
  a list that has more ends in an item with the token of the next: a
  playlist sorted by Top voted wraps it in a `commandExecutorCommand`,
  beside a command that refreshes the votes, and `continuationToken`
  reads either (#626). Below a
  playlist the user owns, YouTube Music suggests songs to add, five at a
  time: the playlist's `Suggest` is the token of the first, and each
  answer has the token of a refresh, for the next (`PlaylistSuggestions`).
  Signed in with the cookies of the user's session, which it signs its
  requests with as the website does, it reads and changes their library:
  liked songs, playlists, albums, subscriptions, likes, dislikes and
  saves, songs saved to the library apart from likes, by the feedback
  tokens of their menus in the queue of their "next", and it deletes the playlists the user made, which it tells from
  those they saved by the menus YouTube sends with them. The lists of
  the library, its playlists, albums, artists, songs and podcasts, are
  read a page at a time by their continuation tokens, up to 1,000 pages
  (`libraryList`, `maxLibraryPages`); a page past the first that fails
  is tried again on `media.PageWaits` (`media.TryPage`), and each item
  comes once, by its ID. A list that stops short, at a page that still
  fails, at a token that names a page read before, or at the bound
  (`Cut`), comes with what was read and a `media.ShortList` with no
  `Total`, as YouTube Music counts none of these lists; `shortlist.go`
  shows it as "Showing the first 1,200 songs", with Try again. The app
  gives these lists 20 minutes to come rather than fetch's minute
  (`wholeList`), and while Songs in your library isn't whole, a song's
  menu asks `SongSaved` of the song as it opens (`savedApart`). An add of a
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
  `Collaborative` of `media.Playlist`. A collaborator's page has none of
  the owner's tools, as anyone's, and their library tells a playlist they
  joined from one they saved by nothing: `PlaylistsToAddTo`, the
  playlists YouTube Music offers to save Liked Music to
  (`playlist/get_add_to_playlist`), holds those they collaborate on, so
  the app marks them `Editable` and `Collaborative`. A collaborator adds,
  moves any track, and takes out the tracks they added alone, whose menus
  offer it (`Removable` of `media.Track`). Its tests read hand-written
  answers of the shapes documented, and answers that
  `MIXMUSIC_LIVE=1 MIXMUSIC_YT_WRITE=1 go test ./internal/youtube -run LiveCollaboration`
  recorded, with a second account in `MIXMUSIC_YT_COOKIES2` that joins as
  a collaborator and tries each edit. It lists the
  Google accounts of the session and their brand channels, from YouTube's
  website, and uses a brand channel by its page ID: one whose page ID the
  list doesn't tell is listed as `Unswitchable`, as without it the account
  itself would be used. Once the app switches to a brand channel, it goes
  back if YouTube Music answers as the Google account itself, as the list
  names it; any other answer keeps the switch. Each answer to a signed-in
  request tells whether the account has YouTube Music Premium, in its
  GFEEDBACK tracking parameters (`has_unlimited_entitlement`, else
  `premium_membership`): the client keeps what the latest that told said,
  for the session and account it was told of (`Client.Premium`,
  `premium.go`), and the app offers Premium's higher bitrate from it
  (#471). Its fixtures are hand-written from the parameters of a recorded
  answer; `go test ./internal/youtube -run LivePremium`, signed in,
  records an account's own. It also reads
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
  library), and controls Spotify Connect devices. `Client.ArtistSongs`
  (`artistsongs.go`) reads the songs of all of an artist's albums and
  singles: every page of the releases, then each release with `Album`, 3
  at a time, each page or release tried again on `media.PageWaits`, a
  song on two releases once by title, artists and length in seconds, and
  `each` given the new songs of each release in order. A release that
  fails leaves a `media.ShortList` of releases. `app.Catalog.ArtistSongs`
  routes by service, YouTube Music's artists to `YouTubeArtistSongs`. The
  artist's Play and Shuffle (`internal/app/artistplay.go`) play it as they
  do YouTube Music's: the first release plays at once and the later ones
  join the queue. The lists of the
  user's library are read whole (`librarylists.go`): their playlists,
  saved albums and shows by offset, the first page, then the pages its
  count tells of, 3 at a time, as a 429 is waited out once, and on while
  the last names a next, up to the offset bound of 100,000; the artists
  they follow by cursor, a page at a time. A page past the first that
  fails is tried again on `media.PageWaits` (`media.TryPage`), and each
  item comes once, by its ID. A list that stops short, at a page that
  fails, at a page of nothing or of nothing new that names a next, or at
  the bound, comes with what was read and a `media.ShortList`, which
  tells how many of how many came; `shortlist.go` shows it, as "Showing
  200 of 250 playlists" with Try again, in the library and its menus, for
  any service's list. Home reads one page of the playlists and of the
  albums, so as not to wait. It changes the
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
  `FollowPlaylist` saves and removes through `PUT` and `DELETE /me/library`
  with the playlist's URI, as `Save` and `Remove` do for the other items,
  and falls back to `/playlists/{id}/followers` when the new route
  answers 403 or 404; both failing is one error naming both answers.
  `LiveCollaboration` logs which route answered.
  A 429 waits for its Retry-After once, if it is 5 seconds or less. A
  background read of a long list (`media.WithPageWait`, as the playlist
  page does) waits up to `media.MaxPageWait`, 60 seconds, once for each
  page, tells the time left, and stops with its context. `APIError.RetryAfter`
  keeps what Spotify asked. A 403 in plain text is a refusal, never a wait.
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
- **`internal/sponsorblock`** asks SponsorBlock which parts of a video it
  marks, as sponsor reads, intros and the parts of music videos without
  music, and its highlight (#548): by the first 4 hex characters of the
  SHA-256 of the video's ID, keeping the video's parts of the answer, so
  that SponsorBlock never learns which video plays. A lookup reads 2 MB
  at most, gives up after 8 seconds, goes through the system's proxy, and
  asks the mirror the settings name in place of SponsorBlock's server; it
  never sends parts or votes. `internal/app/sponsorblock.go` keeps the
  answers as the labels of AI are kept: the videos with parts in
  `sponsorblock.json`, 5,000 at most, those downloaded first, for 7 days,
  and offline however old; those without, in memory for a day; a failed
  lookup not at all. It asks of the recording a stream plays, a Spotify
  track's too, never of a file on this computer.
- **`lyrics`** finds synced lyrics on Better Lyrics, Unison and LRCLIB,
  reads TTML and LRC, and LRCLIB's Lyricsfile, the YAML whose words are
  timed (`readLyricsfile`), and translates and romanizes lyrics on Unison.
  `Catalog.Lyrics` (`internal/app/lyricssources.go`) asks the sources one
  at a time, in the order of `settings.Lyrics.SourceOrder`, those on, and
  keeps the best answer: synced over plain, word by word over line by
  line while `LyricsPreferWords` is on, the longer of two plain answers.
  Each source is a `lyricsLookup` of `lyricsLookups`, by its key in
  `settings.LyricsSourceKeys`; YouTube captions come through
  `stream.Direct.Captions` (`lyricscaptions.go`), as does an episode's
  transcript (`Catalog.Transcript`). The lyrics kept of a track are kept
  by its sources' setup (`lyricsSetup`), so that a change looks it up
  again. The music video's timing (`lrclibLyrics`, `keepsTime`) stays in
  `catalog.go`, apart from the sources, and `videoLyrics`
  (`lyricsvideo.go`, #565) times a video's song's lyrics to the video:
  once `internal/align` found where the song starts in it, the lines move
  by that shift (`Lyrics.Shifted`). The first lookup starts the
  measurement (`videoTiming`), one at a time, which reads both
  recordings through the proxy a block at a time (`Proxy.OpenRanges`)
  and decodes their stretches (`audio.Stretches`); meanwhile the lyrics
  show as the song has them, with a note, and aren't kept, and as it ends
  the windows look them up again (`App.OnLyricsTimed`). A shift is kept
  in `lyrics-timing.json` in the cache folder for 180 days, no clear match
  for 7, 400 videos at most; a failure keeps nothing, and holds the video
  off for 10 minutes. YouTube Music's own map of a video and its song,
  the `counterpart` and `segmentMap` of the "next" answer signed in,
  isn't read yet: no recorded answer holds them.
- **`internal/align`** finds where a stretch of one recording's audio is
  in another's, on the computer: the envelopes of both, the level of
  each third of an octave from 100 Hz to 3.2 kHz every 10 ms (`Levels`),
  compared by normalized correlation, each band's mean taken away, at
  every shift of the plan (`NewPlan`): three windows of the song, at a
  fifth, half and four fifths of it, searched 120 s either way of the
  lengths' difference. `Plan.Shift` takes a shift when two windows or
  more match clearly and agree within 100 ms, at a speed within 1%.
  `internal/align/aligntest` makes audio from seeds, drums, a bass and a
  tune, to test it on, and `MIXMUSIC_LIVE=1 go test ./internal/app -run
  LiveLyricsTiming -v` measures music videos on YouTube.
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
- **`internal/scrobble`** is what every scrobbling service shares (#568).
  `Scrobbler` decides from the player's reports which plays count, by
  Last.fm's rules: a track over 30 seconds, once half of it or 4 minutes
  played, each play once. It keeps those not taken yet in a queue, up to
  5000, the oldest dropped first, and sends them to a `Service` with its
  now-playing updates, `Options.Batch` at a time. A service's errors tell
  what becomes of the plays: `ErrKeyRefused` keeps them, and the plays
  heard meanwhile, until `SetSession` gives a new key; `Rejected` drops
  them, once the batch has gone again in halves until the play at fault
  went alone; any other error keeps them. `Run` sends beside the app: at
  once when something waits, after a failure again after 15 seconds,
  twice as long each time up to 5 minutes, or after the service's
  `RetryAfter`, and at once on `Retry`. `internal/app/scrobbles.go` feeds
  every connected service the same `scrobbleTrack`, starts each one's
  `Run` with the first play it hears of, calls `Retry` when
  `online.Watch` tells the network came back, stops it on disconnect and
  in `App.Close` before its files go, and has the windows told how many
  plays wait only while sends fail, and once none do (`waitingTold`).
- **`internal/lastfm`** signs in to Last.fm with the user's API account,
  and is a `scrobble.Service`: now-playing updates and scrobbles, and
  loves (`track.love`, `track.unlove`). The app saves its queue in
  `lastfm-queue.json`; disconnecting is the only thing that deletes the
  file.
- **`internal/listenbrainz`** calls ListenBrainz's API, or a server that
  speaks it, as Maloja at `/apis/listenbrainz`: `validate-token`;
  `submit-listens`, `playing_now` with `return_msid`, `single` for one
  listen and `import` for more, up to 1,000 a request; recording feedback,
  and the metadata lookup. Each listen tells, in `additional_info`,
  MixMusic as its client and player, the service and the track's page,
  its length, and Spotify's address and the ISRC where known. It makes a
  call a second at most (`Client.Gap`) and holds every call after a 429
  for its `X-RateLimit-Reset-In`; a 401 is `scrobble.ErrKeyRefused`, a 400
  rejects the listens, and any other answer keeps them, as a server of the
  user's that is down or moved. `ParseServer` reads the address the user
  writes, and `Local` allows plain http to a host of the local network
  alone: loopback, private, link-local and unique local addresses, a name
  of one label or under `.local`, `.lan`, `.home.arpa` or `.internal`,
  or a name all of whose addresses are of those. The app
  (`internal/app/listenbrainz.go`) keeps the server, the token and the
  user's name in the credential store, the queue in
  `listenbrainz-queue.json`, and the MessyBrainz IDs of the recordings of
  the latest 5,000 tracks announced in `listenbrainz-recordings.json`
  (`recordingBook`), which `loves.go` sends a like's feedback by, or else
  by the MusicBrainz ID the lookup finds.
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

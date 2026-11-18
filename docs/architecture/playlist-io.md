# Importing and exporting playlists

Import a playlist brings in lists of songs from files and text, and from
the data downloads of Spotify and Google; Export the list and Export the
library write any list to a file other players read (#613).

- **Reading and writing** (`internal/playlistio`). It knows no service:
  `ReadFile`, `Read` and `ReadText` give a `Source`, of `List`s of
  `Entry`s, what each song's file says: title, artists, album, length,
  ISRC, a YouTube video's ID, a Spotify track's ID, MixMusic's own ID, a
  link, a path. By the extension, else by what the file holds, it reads
  CSV and TSV (`table.go`: the separator of the first line, the header
  among the first 10 rows by the names of `fieldNames`, a table whose
  names it doesn't know left to the user as a `Table`), M3U and M3U8
  (`m3u.go`: `ResolvePath` reads Windows' paths on every system),
  lines of text (`lines.go`), JSON (`jsonfile.go`: MixMusic's own, the
  files of Spotify's data download, or an array of objects read as a
  table) and the ZIP archives of data downloads (`archive.go`). `Decode`
  reads UTF-8, UTF-16 and the code page of Windows. `Write` writes CSV,
  M3U8 and JSON, which mark themselves as MixMusic's own, so that their
  songs come back as they were. The bounds of an import are its
  constants: 100 files, 20,000 songs, 50 MB of text, and archives of
  100 MB and 1,000 files.
- **Finding a song** (`internal/match`). `Judge` tells `Sure`, `ToCheck`
  or `NotFound` of a candidate: sure when `TitleKey`s, the credited
  artists and the version words agree, and the lengths are within 3
  seconds or 2%. `Matcher.Find` judges the library's candidates first,
  then searches YouTube Music's songs and videos by the title and every
  artist, then by the first artist, and stops at the first sure one, or
  at a search that fails.
- **The import** (`internal/app/playlistimport.go`). `Imports` keeps one
  import, in `import.json`: its step (`ImportColumns`, `ImportPick`,
  `ImportMatching`, `ImportHalted`, `ImportReview`), its lists and their
  rows, and the mix each source's lists went to, so that one imported
  again adds what's new. `Finish` adds the songs ticked to new mixes,
  which `library.CreatePlaylist` puts where new mixes go, to a mix, or to
  Liked songs (`library.LikeAll`, then the likes of the services, as
  `SetLiked` does).
- **The looking** (`internal/app/playlistimport_match.go`). A run takes
  the songs not looked for yet, 4 workers at once, through a `pacer`
  that starts a request 100 ms after the one before at least. A song of
  MixMusic's own lists, a file, a video's ID, and a Spotify track's ID
  with its title become tracks with no search; the rest go to the cache
  of `import-matches.json`, then `Find`. A 429 of either service, or 3
  failures in a row, halts the run; a quit keeps it at `ImportMatching`,
  which `resumeImport` takes up as the app starts. `ImportServices` let
  tests look for songs without the network.
- **The windows** (`playlistimport.go`, `playlistimport_review.go`,
  `playlistexport.go`). The page at `/import` shows the step the import
  is at, and follows it with `Imports.Watch`. Dropped files of the
  extensions of `importExtensions` import, the others play. The review
  lists only the rows near the view (`nearRows`).

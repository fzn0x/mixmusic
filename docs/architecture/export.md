# Export

Export… writes copies of tracks to a folder the user chooses, to take
them to a phone or another player (#341): named "Artist - Title", with
their tags and cover in each file. The downloads and the user's files
stay as they are.

- **What exports** (`internal/app/export.go`). A track the downloads
  hold, by its ID, or, of a recording on YouTube Music, by its video's
  file (`Snapshot.VideoFile`); and a song on this computer, whose file is
  there, of a song with copies its best copy (`FileOf`). `Exportable`
  tells the windows which, and `Export` writes them in turn, telling its
  progress, and returns the files written and the tracks that failed,
  with why.
- **What a copy tells.** A download's copy gets the title, artists and
  album of its track as it was asked for, so a Spotify track's, its
  ISRC, which Spotify's whole tracks give (`media.Track.ISRC`), and the
  cover the downloads keep (`ArtworkFile`), as a JPEG when it's another
  kind (`export.Cover`). The page of its album (`ExportOptions.Album`,
  the catalog's `Album` in the windows) gives the year, the album's
  artists and the track's number there, by its ID, else its title: each
  album is asked for once, for at most 10 seconds, and once two in a row
  failed, as without the network, the others aren't. A song on this
  computer goes as it is, its tags and cover the user's.
- **Writing a copy** (`internal/export`). `Name` makes the name: the
  characters Windows refuses give way to ones that read alike or go, a
  dot first and dots and spaces last go, the names Windows keeps for its
  devices (CON, COM1, LPT¹ and the like) get an underscore, and a long
  name is cut at a character's end, at 200 bytes. `claim` creates the
  file with `O_EXCL`, numbered "(2)", "(3)" and on when the name is
  taken, so that two tracks of one name, and what the folder held, keep
  their files. `store.Replace` then writes the copy beside it and renames
  it over, so that a failure leaves nothing. An M4A, MP3, FLAC or Ogg
  file gets its tags through `tagwrite.Rewrite`; a WAV file, whose tags
  MixMusic doesn't write, goes as it is.
- **Opus in WebM** becomes Ogg Opus (`tagwrite.RemuxWebM`): the packets
  as they were, the OpusHead the track's CodecPrivate, then OpusTags with
  the tags and the cover, then pages of about a second of audio. A page's
  granule position counts the samples at 48 kHz decoded by the end of
  the last packet that ends on it, the pre-skip among them, and the last
  page's leaves out what the last block's DiscardPadding drops, so that
  the copy decodes to the same samples as the download.
- **The windows** (`export.go`): Export… in the menu of a track that
  exports, beside its other local items, and export in the bar of the
  tracks selected, registered with `addPickAction`. The system's dialog
  asks for the folder, the Music folder first, and a toast tells how the
  export went, with show, which shows the first copy in its folder, and
  why the others failed.

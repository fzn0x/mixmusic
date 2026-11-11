# Editing tags

Edit info…, Edit album… and Move to album… change the tags of the music
on this computer (#339). MixMusic changes a file only then.

- **`internal/tagwrite`** writes tags, as `internal/tags` reads them: the
  title, artists, album, album artists, track and disc numbers, year,
  genre, ISRC and cover. `Rewrite` writes a file anew to a writer, the
  fields a `Fields` mask names, and keeps the rest; `Write` changes a
  file on the disk, and `Writable` tells first why it can't: a file
  read-only, a folder that can't take a new file, or a kind it doesn't
  write (`ErrUnsupported`, as WAV). Each kind has its file:
  - `id3.go`: ID3v2 at the start of an MP3, of the version the file had,
    2.3 or 2.4, else 2.4; a 2.2 tag becomes 2.4, with the frames 2.4 has
    names for. Text of 2.4 is UTF-8, of 2.3 ISO-8859-1 or UTF-16, several
    values between NULs. Unsynchronisation is undone, an extended header
    dropped, and frames that ask to go once the tag changes go. An ID3v1
    tag at the end takes the new values as far as it holds them.
  - `flac.go`: the VORBIS_COMMENT block, and a PICTURE block for the
    cover; STREAMINFO stays first, the other blocks in their order, and
    padding fills the room the old metadata took when the new fits, so
    that the audio doesn't move. An ID3v2 tag before `fLaC` goes, what
    it told written into the comments.
  - `ogg.go`: the comment header of an Ogg Opus or Ogg Vorbis stream,
    laid out anew on pages after the first, with Vorbis's setup header
    as it was; the audio's pages follow as they were, but for their
    sequence numbers and checksums, and a damaged page stops the write
    then. A file whose headers share a page with its audio is refused.
  - `mp4.go`: the items of the ilst of the moov's udta's meta, and of a
    QuickTime moov's meta. When the moov changes size before the audio,
    a free box after the ilst or after the moov takes up the difference
    where it can; else the chunk offsets (`stco`, `co64`), the
    fragments' base offsets (`tfhd`) and the random access index (`tfra`)
    move with the audio.
  - `vorbis.go`: Vorbis comments, of FLAC, Opus and Vorbis. New values
    come first everywhere, so that a reader that takes the first of a
    key finds them.
  - `webm.go`: `RemuxWebM` writes the Opus of a WebM file, which other
    players read no tags of, as Ogg Opus with its tags, for an export
    (export.md).
- **Safe writes** go through `store.Replace`: the new file is written
  beside the old, read back (`tags.Read`, which must give what was
  written) and opened as the engine opens files (`audio.Probe`), put on
  the disk, and renamed over the old one, through `os.Root`'s rename,
  which has POSIX semantics on NTFS: the engine, which opens local files
  with `store.OpenShared` (sharing their deletion on Windows), goes on
  playing the old file. A failure leaves the old file, and no new one.
- **`internal/app`'s `LocalFiles`** reads a song's tags for the dialog
  (`EditableTags`, `AlbumTags`), writes them (`EditTags`), and moves a
  song (`MoveToAlbum`, `MoveToNewAlbum`), after `localfiles.MoveTo` tells
  what joining an album takes as the index groups albums: its title and
  album artists, or, for an album grouped by its folder that the song
  isn't in, the album artist it shows written to all its songs. Then
  `Index.Reread` reads the files written again at once, and
  `Player.Retagged` gives the queue's entries their new tags.
- **The dialogs** (`localtags.go`) open once `Editable` found nothing in
  the way, write only the fields that changed, beside the main thread,
  and keep what failed in the dialog; the pages of local albums and
  artists shown ask the library again, and an album page whose album
  went shows the one its songs went to. The mini player's menus have
  none of them.

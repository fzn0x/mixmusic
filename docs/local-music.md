# Music on this computer
<!-- docs: group=using order=25 -->

MixMusic plays the music files on your computer beside Spotify and YouTube Music: in one library, one search and one queue. It plays them from their files, and uploads nothing.

Your own recordings, the albums you bought, a folder of live sets: add the
folders they're in, or open the files, and they play as any track does,
with the equalizer, crossfade, speed and the visualizer of
[the audio engine](audio-engine.md).

## Add a folder

Settings › Playback › Music on this computer, or the add a folder button of
Library › On this computer, opens your system's folder picker. MixMusic
reads the folder and the folders in it, and keeps what it finds:

- **Folders** lists each folder with how many tracks it holds, how many
  files it skipped, files of kinds it can't play or broken, and how many
  folders in it can't be read, as one MixMusic isn't allowed to read.
- **Remove** takes a folder out of the library. Its files stay where they
  are, and its tracks stay in your mixes, greyed.
- **Rescan** reads the folders again now. MixMusic also reads them as it
  starts, quietly, and when you add or remove one.

A scan reads only what changed: a file new to it, or one whose size or
time of change differs from the last scan. A library of thousands of
files is read whole once, then in moments.

While MixMusic runs, it watches the folders: a file you add, change,
rename or remove in them shows in the library within a few seconds,
without a scan, and a folder you copy in is read once, as the copy
settles. Where it can't watch a folder, Settings says why, under the
folders: one on a network drive, whose server may not tell of what other
computers change; on Windows, one on a removable drive, which a watch
would keep from being ejected; on Linux, one past the system's limit on
watched folders, `fs.inotify.max_user_watches`. For such a folder,
rescan, or start MixMusic again, to see what
you added. Hidden folders, whose names start with a dot, are passed
over. Files and folders play whatever their names hold, even names that
aren't valid UTF-8, as on some Linux drives: MixMusic keeps their paths
in base64, so that they name the same files after a restart.

## Open files without a folder

Open music files…, in the [command palette](settings.md#the-command-palette)
or with Ctrl+O (⌘O on macOS), and open files in Library › On this
computer, choose files to play now, as a queue. Drop files on the window,
or a folder, and they play the same way, in the order of their names.
MixMusic keeps none of them in the library: they play as long as their
files are where they were, from the queue, your mixes or your liked
songs.

## Library › On this computer

The [Library page](library.md#the-library-page)'s On this computer pivot
shows how many tracks your folders hold, and:

- **Songs:** every track, with Play, Shuffle and open files, a box that
  searches the list, and a Sort menu: the list's own order, by album and
  track number, Title, Artist, Length or Plays.
- **Albums:** the albums your files' tags name, with their covers. An
  album is grouped by its title and its album artist, or, without an
  album artist, by the folder its files are in, as an album's files
  usually are; one of several artists without an album artist is by
  Various artists. Its page plays it.
- **Artists:** the artists of your files, whose page lists their songs
  and albums.

From N folders, beside the sections, tells where they come from, and
manage opens Settings.

## Search

The search finds the music on this computer too, in its own section, On
this computer, above the results of the services, with both services
chosen. It finds tracks by their titles, artists and albums, regardless
of case and accents, and shows even without the network.

## What plays

| Kind | Files | Notes |
|---|---|---|
| MP3 | `.mp3` | MPEG-1, 2 and 2.5 Layer III; without the silence LAME's tag tells of at their start and end, so that an album's tracks follow each other without gaps; of a variable bitrate, as long as its Xing or VBRI header tells |
| FLAC | `.flac` | up to 8 channels and 32 bits |
| WAV | `.wav` | PCM of 8 to 32 bits, or floats of 32 or 64 |
| AAC | `.m4a`, `.mp4` | AAC-LC; HE-AAC and Apple Lossless (ALAC) aren't played |
| Opus | `.opus`, `.ogg` | in Ogg, mono or stereo |
| Vorbis | `.ogg`, `.oga` | in Ogg |

Files of more than two channels play mixed down to stereo. Files of
other kinds, as `.wma` or `.aac` alone, aren't read. A file MixMusic can't
play counts as skipped in Settings, and never stops a scan.

## Tags and covers

MixMusic reads the title, artists, album, album artists, track and disc
numbers, year and genre your files' tags tell: ID3v2 and ID3v1 of MP3,
MP4's tags, the Vorbis comments of FLAC, Opus and Vorbis, and WAV's INFO
list.

- **A file without tags** shows by its file's name, by Unknown artist.
- **Covers:** the cover a file holds, the front cover when it holds
  several, else an image beside it named cover, folder, front or album
  (`.jpg`, `.png` or `.webp`). MixMusic keeps a copy of each in its cache
  folder, never beside your music.

MixMusic changes a file only when you edit its tags, as below.

## Edit tags

A wrong title, artist, album or year, a missing cover, or a song filed
under the wrong album can be put right in MixMusic:

- **Edit info…**, in the menu of a song on this computer, opens its
  title, artists, album, album artist, track and disc numbers, year,
  genre and cover. Separate several artists with `;`. Choose an image
  sets the cover, a JPEG or PNG of at most 10 MB, and Remove the cover
  takes it away. Save writes what you changed into the file.
- **Edit album…**, in the menu of an album on this computer and of its
  page, changes what its songs share in one go: the album's title, its
  album artist, its year and its cover.
- **Move to album…**, in the menu of a song, files it under another
  album of your library, which you find by its title, or under a new one
  you name, by writing its album's tags. An album without an album
  artist is grouped by its folder: a song from another folder joins it
  once all its songs have the album artist it shows, which the dialog
  says it writes to them too.

The mini player's menus leave them out: its window is too small for
their dialogs. The library shows the new tags at once, and so do the queue and the
player. Your mixes and liked songs keep the title they had until you add
the song again.

MixMusic writes ID3v2 tags to MP3 files (2.3 or 2.4, as the file had,
else 2.4, and the ID3v1 tag too where the file has one), Vorbis comments
and the cover's picture to FLAC files, and the comment header of Opus and
Vorbis files, and the tags of M4A files. WAV files can't be edited.

- **Only the tags change.** The audio stays as it was, byte for byte, as
  do the tags you didn't change, as a comment or ReplayGain.
- **Safe writes.** MixMusic writes the new file beside the old one,
  reads its tags back, opens it as the player would, and only then
  renames it over the old one. Whatever fails, as a full disk, leaves the
  file as it was, and the dialog says why. A song can be edited while it
  plays.
- **What can't be changed says so first:** a file that is read-only, a
  folder MixMusic can't write in, or a WAV file, before any dialog
  opens. On Windows, a file another program holds open says so on
  Save; close it there, and save again.
- **A new cover takes the place of the pictures** the file holds, as a
  back cover, so that it's the one every player shows.

## When a file is gone

A track whose file moved, was deleted, or is on a drive that isn't
plugged in shows greyed, with File not found in place of its artists.
Playing it moves on to the next track, with a short notice. So does a
track whose file goes, or can't be read, while it plays, after one more
try at once, also while you're offline.

- **A rescan** drops the files that are gone from the library. Those of a
  folder that can't be read stay until a scan reads it again: those of a
  folder of music on a drive unplugged, even partway through the scan,
  greyed until it's back, and those of a folder in it that can't be
  read, as one MixMusic isn't allowed to read, as they were, greyed only
  if their own files can't be reached.
- **Your mixes and liked songs** keep the track, greyed, and it plays
  again once its file is back where it was.

## What they do and don't do

Tracks on this computer join the queue, your [mixes](library.md#mixes)
and your liked songs, and you can dislike them, as any track. What needs
a service doesn't apply to them:

- No radio starts from them, Autoplay doesn't carry on after one, and no
  related tracks show under it.
- No YouTube Music match is looked for, and they don't go to Spotify or to
  YouTube Music playlists.
- They don't download: they're on your computer already.
- They don't play on [cast devices](casting.md), which can't reach your
  files.
- Show in Explorer, in a track's menu (Show in Finder on macOS), opens the
  folder of its file.
- [Track info](playing-music.md#track-info), in a track's menu, tells what
  its file holds, its format, bitrate, sample rate, bit depth, channels
  and length, and its path, size and tags.
- [Edit info… and Move to album…](#edit-tags), in a track's menu, change
  its file's tags, and Edit album…, in an album's, those of its songs.
- The system's [media controls](desktop.md)
  show it with its cover, as any track.

## Privacy

Nothing of your music files leaves your computer. MixMusic keeps an index
of what your folders hold, their paths and tags, in `local-files.json` in
your [data folder](data-and-privacy.md#the-data-folder), and their covers
in the cache folder's `local-art`.

A track on this computer is treated as any other by the features that send
what plays, as their settings say:

- **[Last.fm](accounts.md)**, once you connect it, scrobbles its title,
  artists, album and length. A track of no artist isn't scrobbled.
- **Discord**, unless you turn it off in Settings › Desktop, shows its
  title and artists, or [the lines you chose](desktop.md#what-your-discord-status-shows),
  without a cover or a Listen button: with the service's badge on, a
  record's badge takes the cover's place.
- **[Lyrics](lyrics.md)**, when you show them, are looked for on Unison
  and LRCLIB by its title, artists, album and length.

A file that names no artist, as a voice memo, shows by its file's name,
which may be anything: Last.fm and the lyrics hear nothing of it.

[Your data and privacy](data-and-privacy.md) lists every feature's.

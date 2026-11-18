# Import and export playlists
<!-- docs: group=using order=22 -->

MixMusic brings in playlists from files and text, and from the data Spotify and Google let you download, so you can bring your playlists without signing in to the service they came from. It saves any list of yours to a file other players read, too.

## Import a playlist

Open **Import a playlist** with its tile beside New mix in Library › Mixes, or with Import a playlist… in the [command palette](settings.md#the-command-palette). Then:

- **Choose files…** opens up to 100 files at once: CSV and TSV, M3U and M3U8, JSON, text, and the ZIP archives of [data downloads](#data-downloads). Dropping them on the window does the same, wherever you are; music files dropped with them play as before.
- **Paste** links, or "Artist - Title" lines, one a line, into the box, then **Import the text**.

An import reads 20,000 songs and 50 MB of text at most; past that, the page says only the first 20,000 were read. Starting an import while one is under way asks first, since the new one replaces it.

### CSV and TSV

- **Separators:** commas, semicolons or tabs, whichever the first line has most of.
- **Encodings:** UTF-8, with a byte order mark or without; UTF-16; or, as spreadsheets save CSV files on Windows, the code page of Windows your computer uses (Windows-1252 off Windows).
- **Columns by name:** the title (Title, Track, Track name, Name, Song, Song title), the artists (Artist, Artists, Artist name, Artist names, Performer, Creator), the album, the length (Duration, Length, Time, in seconds, milliseconds, or as 3:45), the ISRC, a YouTube video ID, a link, a Spotify URI, and the playlist each song is in, which makes a list of each. Several artist columns, as Artist name 1 and Artist name 2, are all read.
- **Columns by hand:** when no column has a name MixMusic knows, the page shows the first 5 rows, however many columns there are, with a menu above each to say what it holds, and **The first row names the columns**. Continue once one holds the titles, links or IDs; **Skip this file** leaves it out.

### M3U, M3U8 and text

- **M3U and M3U8** lists read their `#EXTINF` lines, "Artist - Title" and the length, `#EXTALB` and `#PLAYLIST`, then each song's link or file.
- **Files** may be relative to the list's folder, absolute, or `file://` URLs. Windows' paths read on every system: their backslashes part the folders, and a path of another computer is found by its file's name among your [music on this computer](local-music.md), once Unicode normalizes it, as macOS keeps names decomposed. A file MixMusic can't reach, as one outside the folders the Flatpak may read, is found by its words instead.
- **Text** holds a song a line: a link, or "Artist - Title". Empty lines, and lines that start with `#` or `//`, are skipped.

### Which track each song becomes

- **Links and IDs first:** a YouTube video's ID or link, a Spotify track's URI or link, and a file that's there become that track without a search. A Spotify track named only by its link needs you [signed in](accounts.md) to Spotify, to read its title.
- **Then your library,** liked songs, mixes and the music on this computer, by the title, or the ISRC; **then YouTube Music,** its songs, then its videos, searched by the title and the artists, and again by the title and the first artist alone when that found nothing sure.
- **Found** means the titles, the credited artists and the version agree, and the lengths are within 3 seconds or 2%, whichever is more, or one is unknown. The version is told by words as live, remix, acoustic, instrumental, karaoke, cover, demo, extended, unplugged, orchestral, piano, slowed, sped, nightcore, a cappella, reprise, rework, bootleg and symphonic: a live take is never found for the studio song.
- **To check** is the closest candidate when none is found for sure, which you check; **Not found** when nothing is close.

### Pace

MixMusic searches 4 songs at once at most, each starting 100 ms after the one before at least, so that playback, which asks the same services, isn't held back. The page shows "Matching 120 of 480…", with **Stop and review**, which shows what was found so far, and Cancel.

- **It stops** when YouTube Music asks it to slow down, or after 3 failures in a row, and says why, with Try again and Review what was found.
- **What searches found** is kept 90 days, 20,000 songs at most, in `import-matches.json` in your [cache folder](data-and-privacy.md#the-cache-folder): importing the same songs again asks nothing.
- **An import under way** is kept in `import.json` in your [data folder](data-and-privacy.md#the-data-folder), and goes on after a restart.

### Review and where the songs go

The page counts the songs found, to check, in your library already and not found, and lists them, each with what its file says and what was found of it. All, Found, To check and Not found show those alone.

- **Ticks:** the songs found are ticked, those to check aren't. **Tick all found** and **Untick all** do every song at once.
- **Replace…** on any song searches your library and YouTube Music for another track, by its title and artists at first, and the one you choose replaces it.
- **Save the songs not found…** saves them as a CSV file, which you can fix and import again.
- **A new mix** of the list, named as the file or its `#PLAYLIST` line, which you can change, in the list's order; of several lists, a new mix of each. **A mix of yours** gets the songs it doesn't hold already. **Liked songs** likes them, first, in their order, and on their service too where you're [signed in](accounts.md), as any like is.
- **Importing a list again** adds only what's new, to the mix it went to the first time, while that mix is there.

## Data downloads

MixMusic reads the ZIP archive of each service's data download, up to 100 MB and 1,000 files, and lists what it holds for you to pick.

### Spotify

1. Open your [account's privacy settings](https://www.spotify.com/account/privacy/), and, under Download your data, ask for your **Account data**, and your **Extended streaming history** if you want it. Spotify's [Data rights and privacy settings](https://support.spotify.com/us/article/data-rights-and-privacy-settings/) tell more.
2. Spotify emails a link once the data is ready, which takes days.
3. Import the ZIP it gives, `my_spotify_data.zip`, as it is.

Its playlists come with their songs, files of your own among them, Your Library with its songs, and **Most played on Spotify**: the songs your streaming history heard for 30 seconds or more, the most played first. Spotify's data names each song's Spotify URI, title and artist, so its songs need no search.

### YouTube Music

1. Open [Google Takeout](https://takeout.google.com/), choose **Deselect all**, then tick **YouTube and YouTube Music**.
2. Under All YouTube data included, keep **playlists** and **music-library-songs**, and leave out videos, which make the archive far bigger than 100 MB.
3. Export once, as a ZIP, and import it as it is.

Its playlists, and the songs of your YouTube Music library as Your Library, name each song by its video's ID, which needs no search. The archive's folders are named in your account's language: MixMusic finds the files by what they hold.

## Export a list

**Export the list…**, in the More menu of a mix, a playlist or an album, and on Liked songs, saves its songs to a file; **Export the library…**, in the command palette, saves every mix and Liked songs in one. The dialog chooses the format:

- **CSV, for spreadsheets:** UTF-8 with a byte order mark and Windows' line ends, in 9 columns: Playlist, Title, Artists, Album, Duration, ISRC, Service, MixMusic ID and URL. A cell a spreadsheet would take for a formula, which starts with `=`, `+`, `-`, `@`, a tab or a carriage return, starts with an apostrophe.
- **M3U8:** `#EXTM3U`, then each list's `#PLAYLIST` and its songs, an `#EXTINF` line each, then the song's file, or its link on its service.
- **JSON:** the same 9 fields for each song.

MixMusic imports its own files back as they were, every song in its list and its order, without a search.

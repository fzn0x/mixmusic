# Lyrics
<!-- docs: group=using order=30 -->

MixMusic shows synced and word-by-word lyrics for Spotify and YouTube Music tracks, and can translate them or show them in Latin letters.

Lyrics come from Better Lyrics, Unison, lrc.red, Spotify, LRCLIB, QQ Music, YouTube Music, the video's captions on YouTube, and YouTube video descriptions and comments, for the track that plays, whichever service it's from, and from [the file of a song on this computer](#songs-on-this-computer) or a .lrc file beside it. Nothing needs setting up (Spotify's own lyrics ask only when you're signed in with a Spotify web session), and Settings › Playback › Lyrics chooses and orders the sources. The lyrics themselves go to Unison, for their translation or for the romanization of Arabic, only while you turn those on.

## Where lyrics show

- **The lyrics panel** opens at the right of the window, beside the page you're on, from the Lyrics button by the player (beside Queue), or with Ctrl+Shift+L (⇧⌘L on macOS). The same keys or button close it.
- **The now playing page** shows them larger, under the player: open it from the cover in the player bar, the card at the foot of the sidebar, or Alt+4 (⌥4).
- **The [full-screen lyrics](#full-screen-lyrics)** cover the window with the lyrics over the cover of the track, blurred, in a style you choose: from the full-screen button of the lyrics panel, or of the now playing page, or with Ctrl+Shift+F (⇧⌘F on macOS).
- **The [notch player](desktop.md#the-player-around-the-notch),** on a MacBook with a notch, shows the line sung now, with the line before and the line after, in Latin letters when you show the lyrics so.

![The lyrics panel at the right of the window, beside Library › Liked: its Synced and Plain text choice, the Translate and Romaji switches, and the lyrics of アイドル by YOASOBI, the line sung now lighting up in the accent colour](shots/lyrics.webp)

The panel and the page show the same lyrics, with the same switches above them, and each keeps where it was scrolled. Synced lyrics follow the song; scroll them to read back, and they stay where you put them for a few seconds before they follow again. Click a line of synced lyrics to play the song from there, once it plays: a song you paused stays paused. To find a line by its words, use [Find in the lyrics](#find-in-the-lyrics).

The panel says what it found:

- "Looking for the lyrics…" while it looks;
- "No lyrics found for this track." when none of the sources has any, with try again when some of them couldn't be reached, and the [source](#pick-another-source) to pick another;
- "Lyrics can't be loaded right now. Check your connection, or try again in a moment.", with try again, when no source could be reached;
- "This track is instrumental." for a track whose lyrics source says so.

Refresh this page (F5, or ⇧⌘R on macOS) asks again. For a podcast episode, the panel says Details, and shows the episode's description and chapters in place of lyrics: see [Podcasts](podcasts.md#episode-details-and-chapters).

## Where they come from

MixMusic asks the sources one at a time, in this order unless you [change it](#choose-and-order-the-sources), and stops at the first that has synced lyrics (or word-by-word lyrics while **Prefer word-by-word lyrics** is on):

1. **[Better Lyrics](https://betterlyrics.org),** whose service keeps lyrics timed word by word, by the track's title, artists, album and length: only for lyrics timed word by word, and only for tracks that have an album and a length, which it needs to find a song. It keeps the songs people have played with Better Lyrics, so it has some that Unison doesn't. When it has none, is busy or can't be reached, MixMusic goes on after a wait of 3 seconds at most.
2. **[Unison](https://unison.betterlyrics.org),** an open, crowdsourced database of lyrics, by the track's video ID, or by its title, artists, album and length. Unison has many lyrics timed word by word. While it's out of reach, MixMusic goes on after a wait of 3 seconds at most.
3. **[lrc.red](https://lrc.red),** serving word- and line-synced TTML lyrics by the track's ISRC when known, or by its title and first artist. When it has none, is busy or can't be reached, MixMusic goes on after a wait of 3 seconds at most.
4. **Spotify,** Spotify's own synced and plain lyrics by the track's Spotify ID, only while you're signed in with a [Spotify web session (`sp_dc`)](accounts.md#spotify). While it's out of reach, MixMusic goes on after a wait of 3 seconds at most.
5. **[LRCLIB](https://lrclib.net),** the open database of synced lyrics, by the track's title, artists, album and length; for a music video, by its song's, [timed to the video](#music-videos). Where LRCLIB has the words of a song timed, they light one by one, as Unison's do.
6. **QQ Music,** synced lyrics and embedded translations by the track's title and first artist. While it's out of reach, MixMusic goes on after a wait of 3 seconds at most.
7. **YouTube Music:** its own lyrics, as plain text.
8. **YouTube captions:** the timed captions of the video that plays, for live versions and uploads no lyrics service has. Captions YouTube makes from speech are left out. A Spotify track takes those of its recording on YouTube Music only when the two are as long, give or take 3 seconds, and a file on your computer never asks. MixMusic waits 3 seconds at most for them.
9. **YouTube video & comments:** plain lyrics extracted from the video description or pinned creator comment of the video (or matching official and lyric uploads found on YouTube, signed out), waiting 5 seconds at most.

Synced lyrics beat plain ones, whichever source has them. Of two sources with plain lyrics, the longer lyrics win, and the source higher in the list wins otherwise. Lyrics of one timed line count as plain.

A Spotify track is looked up by the video ID of its [recording on YouTube Music](playing-music.md#how-spotify-tracks-play) once MixMusic has matched one, and by its title, artists, album and length before. The line at the end of the lyrics names where they came from: "lyrics from Better Lyrics", "lyrics from Unison", "lyrics from YouTube captions" and so on, and "lyrics from Unison · submitted by <name>" for Unison's lyrics that name who submitted them. No address is printed: where the source has a page, its name is the link, Better Lyrics to its site and lrc.red and Unison to the song's page there, and the submitter's name to theirs. Spotify, LRCLIB, QQ Music, YouTube Music, YouTube captions and a file's lyrics have no link.

**A source that answers "Too Many Requests"** (HTTP 429), LRCLIB, Better Lyrics, Unison, lrc.red, Spotify or QQ Music, is left alone: MixMusic doesn't ask it again until the wait it names has passed, an hour at most, with up to 30% more at random, or for a minute when it names none. Meanwhile that source counts as out of reach, for a refresh or try again too, and the others answer as usual. A restart of MixMusic ends the wait. LRCLIB's cached lookup, which asks only its own database, is tried when its usual lookup fails in any other way, as with the 503 it answers when busy, never after a 429.

### Songs on this computer

A song of [the music on this computer](local-music.md) shows the lyrics its file holds at once, offline too, and when its file names no artist. MixMusic looks, in this order:

1. **A .lrc file beside it,** of its name, as `Quiet Field.lrc` beside `Quiet Field.flac`, in any case: LRC, with its offset and lines of several times, or plain text, in UTF-8, UTF-16 or ISO-8859-1, up to 1 MB. Where enhanced LRC times the words of a line, as `<00:12.50>`, they light one by one.
2. **The synced lyrics of its tags:** an MP3's or WAV's SYLT frame, its syllables joined into lines and lit one by one, or a SYNCEDLYRICS or SYNCLYRICS tag. A file with several SYLT frames shows the one of lyrics, else of a transcription, else of other text; frames of chords, events or links are never lyrics.
3. **The plain lyrics of its tags:** an MP3's or WAV's USLT frame or LYRICS tag, the LYRICS, UNSYNCEDLYRICS or UNSYNCED LYRICS comment of a FLAC, Ogg or Opus file, or an M4A's lyrics. A file with several keeps to the format's own place first (USLT, the LYRICS comment, the M4A's lyrics item), then the tags in the order named here, not the order in the file. Timed LRC or TTML written there counts as synced.

The first synced lyrics found show, else the first plain ones. Of a song with [copies](local-music.md#copies-of-one-song), the copy that plays comes first, then the others.

- **Synced lyrics of the file** are final: no source is asked.
- **Plain lyrics of the file** give way only to synced lyrics from the sources, which are asked as for any track. They stay when the sources have none, or only plain ones, or can't be reached.
- **The line at the end** names the file: "lyrics from Quiet Field.lrc".
- **A .lrc added, or tags changed,** show the next time the lyrics are looked up, as when you refresh the page (F5, or ⇧⌘R on macOS).
- **Lyrics from a file go nowhere,** but to Unison while you turn on Translate, or Romanize for Arabic, as any lyrics do.

### Choose and order the sources

Settings › Playback › Lyrics lists the sources under **Lyrics sources**, in the order MixMusic asks them, a row each with a switch and what MixMusic sends that source.

- **Switch a source off,** and MixMusic never asks it, nor sends it anything. YouTube Music and the captions switch off too. The last source on can't be switched off.
- **Move a source** by dragging its row onto another, or with Alt+Up and Alt+Down while its row has the focus.
- **Reset to default** puts them back in their own order, every one on.
- **A change** looks the lyrics of the track that plays up again.
- **Prefer word-by-word lyrics,** off by default: while it's on, lyrics timed word by word from a source lower in the list beat lyrics timed line by line from one above it, so MixMusic asks more sources for the track.
- **Time lyrics to music videos,** on by default: a music video's lyrics that are its song's move to where the song starts in the video, as [Music videos](#music-videos) tells. Off, they keep the song's times.
- **Caption language:** the language of the lyrics from YouTube captions. **Original**, the default, is the video's own captions; any language [Translate lyrics to](#translate) offers takes the video's captions in it when it has them, and its own else.

## Pick another source

When the lyrics are wrong, belong to another version of the song, or are plain while another source has them timed, pick another. Above the lyrics, in the panel and on the now playing page, beside Synced and Plain text, **source** names where they come from (shown as **Automatic (LRCLIB)** and the like while Automatic is chosen), and opens a menu of every source that is on in [Lyrics sources](#choose-and-order-the-sources), in their order. It shows under "No lyrics found for this track." too.

- **What each source answered** shows under its name: Showing, for the lyrics that show; Found, with how they're timed (word by word, by line, or not timed) and their number of lines; Not found; Can't be reached; or Not asked yet, for a source MixMusic didn't need, as one before it had synced lyrics.
- **A video** lists each caption track its uploader wrote, by its language, in place of YouTube captions, once the captions were asked.
- **Pick a source that found lyrics,** and they show at once. **Pick one not asked yet,** and MixMusic asks it then: no source gets the track unless the lookup or you ask it. One that couldn't be reached is asked again.
- **The pick is kept** for the track in `lyrics-picks/`, in the [data folder](data-and-privacy.md#the-data-folder), for 10,000 tracks, the least used going past that. A source you switch off since shows MixMusic's own choice until you switch it on again.
- **Automatic,** first in the menu, goes back to MixMusic's own choice.
- **The mini player, the notch player, shared cards, translation and romanization** follow the pick.

**Search by title and artist…,** in the same menu, opens a dialog with the track's title and artist, which you can change, and asks every source that is on for them at once. Providers that return multiple results (such as lrc.red, LRCLIB and QQ Music) show a scrollable list of all matching versions with their title, artist, album and length. Filter pills at the top let you view all results or focus on a single provider, **preview** loads or expands lines before choosing, and **show these** picks that exact version and keeps it for the track. Spotify, YouTube Music and YouTube captions look up the track that plays, not a typed title.

**Edit lyrics…,** in the same menu, opens the lyrics shown as text, with their times as LRC has them: `[01:02.50]` before a line, and `<01:02.50>` before a word timed word by word. Correct a word or a line, and Save keeps your edit for the track, with the credit of the lyrics it started from. It shows before a file's lyrics and every source, and nothing is sent for it, until you pick another source or Automatic. An edit with no times shows as plain lyrics.

## Music videos

A music video often opens with a scene before its song, or ends with one after it, so lyrics timed for the song would run early in the video. Unison may have lyrics of the video itself, by its video ID, timed for it. When the sources before LRCLIB have none, MixMusic looks for the video's song on YouTube Music, and asks LRCLIB for the song's lyrics, by its title, artists, album and length. Those are timed for the song, so MixMusic times them to the video:

- **It compares the two recordings' audio** on your computer: three stretches of the song, each up to 10 seconds, at a fifth, half and four fifths of it, against the video's audio, at every shift to within 10 ms, up to 2 minutes either way of the shifts that start the song at the video's start and end it at the video's end. Both come from YouTube, signed out, a block at a time as they're read: of the song, only the stretches, and nothing goes to another service. Reading them reports no play and adds nothing to your history.
- **It takes a shift** only when the song matches clearly: two stretches or more each match the video at a correlation of 0.4 or more, standing clear of their next best match and not at the edge of the shifts searched.
  - **When all three match,** they fit a line, of where the song starts in the video and how fast the video plays it, each within 100 ms, and that speed is the song's own within 1%. Some videos play their song a fraction of a percent slower or faster than the song's own recording.
  - **When only two match,** they agree within 100 ms, and the song plays at its own speed: a line would fit any two.
- **The lines move to where the video plays them:** the shift, plus each line's time in the song at the video's speed, so lines late in the song land as well as early ones. A line it moves before 0:00 starts at 0:00. Every view of the lyrics follows, and so does clicking a line, and the plugins, which get the same lyrics.
- **The first play.** While MixMusic times them, in a few seconds, the lyrics show as below, with the note "Timing these lyrics to the video…", and once it has, they switch in place, with a note such as "Timed to this video (+29.1 s)", or "Timed to this video (+2.9 s, 0.26% slower)" when the video plays the song off its speed. It times one video at a time, and gives up after 30 seconds.
- **It remembers** each video and song in `lyrics-timing.json`, in the [cache folder](data-and-privacy.md#the-cache-folder): a shift and a speed for 180 days, and that the song has no clear match for 7, 400 videos at most. A download that fails, or a timing that takes too long, keeps nothing: the lyrics show as below for 10 minutes, and the next time they're looked up after that, MixMusic tries again.

When the song has no clear match, as for a live take, a video that cuts a verse or plays the song 1% faster or slower or more, or a song of one loop over and over, the song's synced lyrics show with their times while the video is as long as the song, give or take 3 seconds, and without them, with the note "Not synced: the song's synced lyrics don't match this video.", when it's longer or shorter. So do they while Time lyrics to music videos is off. An upload YouTube Music has no song of, and a Spotify track that plays through a video, are looked up as any track is, with nothing to time against.

## Synced and plain lyrics

Synced lyrics have a time for each line. The line sung now lights in the accent colour, the lines sung before it dim, and the view keeps it in the middle.

- **Synced or Plain text:** above synced lyrics, this choice shows them as plain text instead, without following the song. MixMusic remembers it for every track.
- **When "synced" is offered:** only for lyrics whose lines can be followed in time. Lyrics of one timed line, or whose lines all have one time, count as plain, and another source's lyrics take their place when it has more of them.
- **At another [speed](playing-music.md#speed),** synced lyrics keep the track's own time, so they still light as the words are sung.

Plain lyrics show as they are, with only their source above them.

## Word by word

Lyrics timed word by word, as Unison has many, light each word of the line sung now as it's sung, in the panel and on the now playing page:

- each word fills with the accent from its start to its end;
- words sung stay lit, and words to come keep the colour of the lines to come;
- background vocals light the same way.

Lines shown in Latin letters made on your computer light word by word too. Romaji, and Arabic's romanization from Unison, don't: shown in place of the line, they light a whole line at a time, as lyrics timed line by line do. While your system asks for [less motion](look-and-feel.md#reduced-motion), each word lights whole as it starts.

## Timing

Some lyrics are timed a little early or late for the song that plays, and some headphones, as Bluetooth ones, sound late in ways your system doesn't tell MixMusic. Move the lyrics earlier or later, and keep it.

- **This song:** above synced lyrics, in the panel and on the now playing page, beside Synced and Plain text, two arrows move them: ← (Earlier) shows the lyrics 0.05 s earlier, and → (Later) 0.05 s later, or 1 s with Shift. The line sung now changes at once. Between the arrows, the timing shows in words, as "0.35 s earlier" or "as timed"; click it, and type it in milliseconds: above 0 is earlier, below 0 later, and Enter or leaving it keeps it. **Reset** shows while the lyrics are moved. The arrows show only while the lyrics show synced.
- **Kept for these lyrics:** the timing is kept for the track, for the lyrics of the source it was set for. When another source's lyrics show, as when one can't be reached, they show as timed. Each caption language of a video is lyrics of its own. MixMusic keeps the timings of 10,000 tracks in `lyrics-offsets/`, in the [data folder](data-and-privacy.md#the-data-folder), the least used going past that, and writes a change a few seconds after the last press, or as it quits.
- **Every song:** **Lyrics timing**, in Settings › Playback › Lyrics, moves the lyrics of every song, for an output that sounds late: the slider goes 10 s either way, earlier to the left and later to the right, in steps of 0.05 s, and its field takes one typed in milliseconds as far as 120 s, which the slider then shows at its end. **Reset** shows while it isn't 0. It adds to each song's timing. As it is a delay of the sound, it grows with the [speed](playing-music.md#speed): 0.2 s later is 0.3 s of the song at 1.5×. A song's own timing doesn't.
- **As far as 120 s,** earlier or later, every control stops. A timing typed stays as it is, between the steps too.
- **Everywhere the line sung shows:** the panel, the now playing page, the [full-screen lyrics](#full-screen-lyrics), the mini player and the [notch player](desktop.md#the-player-around-the-notch) follow the timing, and so do the words lit [word by word](#word-by-word). Clicking a line plays the song from its time less the timing, never before the start, so that it is the line sung then.
- **Under the timing:** an LRC file's own `[offset:]` still applies, and the timing adds to how far MixMusic times a [music video's lyrics](#music-videos) to it. Lyrics that show without their times, as plain lyrics, have no timing.
- **Plugins** get the timing with the lyrics, as `offsetMs`.

## Full-screen lyrics

The full-screen lyrics cover the window with the lyrics of the track playing. Its cover, blurred and darkened, is the ground. At the left are the buttons of the player (the previous track, play or pause, and the next track), the cover, and under it the title and the artist. In the middle are the lyrics: the line sung now large and white, lit word by word where its words have times, with a ball hopping over its words, and the lines away from it dimmer and, in most styles, blurrier the farther they lie. At the right, beside the lyrics' scroll bar, are the covers of the tracks next in the queue. The ink is white whatever your theme, as the ground is dark.

- **Open them** with the full-screen button of the lyrics panel's head, beside its search and close buttons, or of the now playing page, with Ctrl+Shift+F (⇧⌘F on macOS), with "Show or hide the full-screen lyrics" in the [command palette](settings.md#the-command-palette), or with **show** in Settings › Appearance › Lyrics style. Some input methods take Ctrl+Shift+F (Microsoft Pinyin on Windows, for one): [Settings › Shortcuts](settings.md#change-a-shortcut) gives the command other keys, as it does any command. Escape, the close button at the top right, or the same shortcut closes them. A podcast episode has no lyrics, so they don't open for one, and close when one starts.
- **The queue.** The covers at the right are those of the tracks after the one playing, as the queue panel lists them: click one to play it. In a window narrower than 760 pixels they give way to the lyrics, and, narrower than 600, so do the buttons.
- **The style, the size and the ball** are Settings', and the full-screen lyrics have no controls for them. Settings › Appearance › Lyrics style has the styles (Spotlight, the default, and five others, below), **Lyrics text size**, from 60% to 180% in steps of 10%, the **Bouncing ball** switch, **Ball between lines** (below), and a small preview that moves as you change them, so that you see what each does. "Lyrics style: …" in the [command palette](settings.md#the-command-palette) chooses a style too.
- **The bouncing ball.** For lyrics timed word by word, a ball hops from word to word over the line sung. It waits on a word until the last moments before the next one starts, hops, and lands as it starts, with a squash and a ring that spreads over the word: it touches the tops of the letters, as high as the tall ones or as low as the small ones of the word. Lines timed only line by line have no ball. The switch is on at first. While your system asks for [less motion](look-and-feel.md#reduced-motion) the ball doesn't hop: it sits on the word being sung.
- **Ball between lines.** From the last word of a line the ball goes to the first word of the next, landing on it as it starts, in a motion Settings › Appearance › Lyrics style chooses while the ball is on: **Mix** (the default), a different one for each line, in turn, and the next that fits where the pause is short; **Hop**, one arc; **Boomerang**, thrown out past the end of the line and curved back to the next; **Loop**, a loop over the last word and a swoop down; **Dive**, a small rise and a drop; or **Ricochet**, off the right edge of the lyrics and their floor and up to the next line. A motion needs time, which the pause between two lines gives: where it is too short for the one chosen, the ball hops, and where there is no time for a hop it waits for the next line. The fast motions leave a short trail. The ball flies over the lyrics, past their edges, and is the same in the preview.
- **The motion.** As the song goes on to the next line, the lyrics glide up to it slowly, and the lines fade and blur to where their distance puts them, rather than jump. A new style or size centers the line sung again at once. While your system asks for [less motion](look-and-feel.md#reduced-motion), they jump.
- **What they show** is what you chose in the lyrics panel: synced or plain, translated, in Latin letters. The switches are in the panel, not here. A click on a line of synced lyrics plays the song from there. Scroll them to read back, and they stay where you put them for a few seconds, with nothing blurred, before they follow again.
- **The keys.** The full-screen lyrics have none of their own: Space, the arrows, the volume, <kbd>Ctrl</kbd> <kbd>K</kbd> and the other shortcuts work as they do elsewhere. Reduce Motion lights each word whole, as it does in the panel.

| Style | Set in | What it looks like |
|---|---|---|
| **Spotlight**, the default | Selawik Bold | Very big letters. Only the line sung is sharp: the others blur. |
| **Glow** | Quicksand Bold | Round, bold letters with a soft glow on the words as they are sung. The other lines blur. |
| **Focus** | Roboto Flex Bold | Bold, even letters. The farther a line is from the one sung, the more it fades and blurs. |
| **Simple** | Selawik Semibold | Plain letters, as in the rest of MixMusic. No blur: the other lines only fade. |
| **Bold** | Archivo SemiCondensed ExtraBold | Big, heavy, tight letters, like a poster. |
| **Calm** | Roboto Flex Medium | Small, quiet letters, with a light fade and almost no blur, for a relaxed mood. |

The styles use the faces MixMusic brings, so they look the same on every computer. They set the type and the blur; the theme and the [layout](look-and-feel.md#layouts) don't change them.

## Translate

Lyrics in another language can show their translation: turn on Translate above them, in the panel or on the now playing page. Each line then shows its translation under it, smaller, moving with it in synced lyrics.

- **When the switch shows:** for lyrics in another language than the one they translate to, or when the lyrics themselves already carry an embedded translation (such as bilingual TTML, LRC, or YouTube video descriptions and comments). That's your system's language by default; "Translate lyrics to", in [Settings › Playback](settings.md#playback) › Lyrics, chooses another, among 47.
- **Placement:** when lyrics carry an embedded translation and Translate is on, choose **under** (each translated line under the original), **above** (above the original), or **only** (in place of the original). Embedded translations stay on your computer and send nothing to Unison.
- **It's remembered** for the next song in the same language: turned on for Korean lyrics, it stays on for Korean lyrics.
- **What is sent:** while Translate is on, the lyrics of the track playing and its video ID go to Unison, which has Google translate them. Nothing is sent while it's off, as it is by default. [Your data and privacy](data-and-privacy.md#what-each-feature-sends-where) lists what every feature sends.
- **Once per song:** each translation is kept on disk, by track and language, so a song is translated once.
- **While it comes,** the panel says "Translation is on its way…". When the service is busy, the lyrics show untranslated, with "Translation isn't available right now." and try again.

With romaji or Latin letters on too, each line shows the original, then its Latin letters, then its translation, as screen readers read them:

```
今日はいい天気
kyō wa ii tenki
The weather is nice today.
```

![The now playing page of アイドル with Translate and Romaji on, and Romaji under chosen: each line of its lyrics with its romaji under it, then its English translation](shots/lyrics-translate.webp)

## Japanese lyrics in romaji

Japanese lyrics can show in romaji, Japanese in Latin letters by the Hepburn system: turn on Romaji above them, then choose:

- **Romaji under:** each line with its romaji under it;
- **Romaji only:** the romaji in place of the line.

For 今日はいい天気, the romaji is "kyō wa ii tenki".

- **Made on your computer.** MixMusic reads the kanji with kagome, a Japanese morphological analyser, and its IPA dictionary. The lyrics aren't sent anywhere for it.
- **The dictionary.** The first time you turn romaji on, MixMusic downloads the dictionary of the readings of kanji, 11.7 MB, from GitHub, and keeps it in its cache folder. Until it's there, or while it can't be downloaded, kana show in romaji and kanji as they are: the panel says "The readings of kanji are downloading: kanji show as they are until then.", or that they couldn't be downloaded, with try again.
- **Memory.** While Japanese lyrics show in romaji, MixMusic uses some 90 MB more memory, which it gives back a few minutes after.
- **It's remembered** for the next Japanese song.

## Other scripts in Latin letters

Lyrics in the other scripts can show in Latin letters too: turn on Romanize above them, then choose Romanized under, each line with its romanization under it, or Romanized only, in place of the line. The switch shows as soon as the lyrics show, and the choice is remembered for the next song in the same language.

These are romanized on your computer, and nothing of them is sent, each by the official or the most used standard of its language:

| Language or script | How it's written in Latin letters |
|---|---|
| Korean | The Revised Romanization of Korean (2000), with its sound changes within a word: 한국어 is "hangugeo" |
| Chinese, simplified and traditional | Hanyu Pinyin with tone marks, a character of several readings read by its word: 银行 is "yín háng" |
| Russian | BGN/PCGN |
| Ukrainian | Ukraine's national system of 2010 |
| Belarusian | Belarus's national system of 2007 |
| Bulgarian | The Streamlined System |
| Serbian | Gaj's Latin alphabet |
| Macedonian | The official system of 2008 |
| Kazakh and Kyrgyz | BGN/PCGN (1979) |
| Mongolian | MNS 5217:2012 |
| Greek | ELOT 743, as the UN adopted it |
| Armenian | BGN/PCGN (1981) |
| Georgian | Georgia's national system of 2002 |
| Hindi, Bengali, Punjabi, Gujarati, Odia, Tamil, Telugu, Kannada, Malayalam, Sinhala, and the other languages of their scripts | Hunterian, India's national system, with its long vowels marked |
| Thai | The Royal Thai General System, a syllable apart |
| Lao | BGN/PCGN (1966), a syllable apart: ຫຼວງພະບາງ is "louang pha bang" |
| Khmer | The UN's system of 1972, its vowels read by their consonants, a syllable apart: ភ្នំពេញ is "phnum pénh" |
| Burmese | BGN/PCGN (1970), its sounds voiced as said, a syllable apart: မင်္ဂလာပါ is "min ga la ba" |
| Tibetan | Extended Wylie |
| Hebrew | The rules of the Academy of the Hebrew Language (2006) |
| Persian and Urdu | The plain form of ALA-LC's romanization |
| Amharic | BGN/PCGN (1967), for its Ethiopic script |

- **The Latin letters of a line,** as the English in a K-pop song, stay as they are.
- **Lyrics said to be in another language,** as English lyrics with lines in Hangul, are romanized as the language of their script.

### Arabic

Arabic doesn't write its short vowels, which no rule can tell, so Arabic's romanization comes from Unison, which has Google romanize the lyrics with them. While Romanize is on, the lyrics of the track playing and its video ID go to Unison. Until its romanization comes, and when it can't, Arabic shows in Latin letters made on your computer, by ALA-LC's plain form.

For lyrics in another script that MixMusic doesn't romanize itself, the Romanize switch shows once you turn on Translate and their translation from Unison comes with Latin letters.

## Share lines

Share lines, above the lyrics, saves lines you choose as an image card. Click Share lines, then click each line to choose it: it shows tinted, and a second click lets it go. Up to 12 lines fit a card. While you choose, a click on a line doesn't play the song from there. Save image saves a square card and Save for a story a tall one, as a PNG, where the system's dialog says. The card sets the lines large, with the song's cover, title and artist at its foot, in the colours of your theme, and ends with where the lyrics come from, as their licence asks. MixMusic's logo and name sit at its bottom right. Done stops choosing. What a line shows, in Latin letters or translated, is what the card holds: the line as the lyrics show it, with no line under it.

## Find in the lyrics

Find a line by its words, and play the song from it.

- **Open the box** with the search button of the lyrics panel's head, beside its close button, or beside Share lines on the now playing page; the box takes the keyboard's focus. **Find in the lyrics**, in the [command palette](settings.md#the-command-palette), opens the box of the now playing page while it shows, and else the lyrics panel with its box. It has no keys of its own: Ctrl+F (⌘F on macOS) stays the music search, and [Settings › Shortcuts](settings.md#change-a-shortcut) can give it some. The mini player, the notch player and the full-screen lyrics have no box, and the palette's command closes the full-screen lyrics.
- **Type,** and the lines that hold what you typed show tinted, from the first letter: one kanji or one Hangul syllable finds its lines. Case doesn't count, curly and straight quotes are the same, and so are Latin, Greek and Cyrillic letters with and without accents: "cafe" finds "café", and "ежик" finds "ёжик". In other scripts a mark makes another letter, so か doesn't find が, and Cyrillic й stays apart from и. Arabic and Hebrew lines are found without their vowel signs, and Arabic's alef without its hamza. A line is found by its words as written, and by its [Latin letters](#other-scripts-in-latin-letters) and its [translation](#translate) while they show.
- **The line found** is tinted more than the others: at first, the first at or after the line sung now, and the first line of plain lyrics. Beside the box, "2 of 5" says which it is, or "No line holds these words". Screen readers hear how many lines hold the words once you stop typing, and the line each arrow goes to. No line dims while the box holds words, and synced lyrics stop following the song, so the line found stays in view. Once the box is empty, they follow it again.
- **↑ and ↓** in the box go to the line found before and after, and wrap around.
- **Enter** plays the song from the line found, as a click on it does: at its time less the [timing](#timing), not while the track loads, and a song you paused stays paused. While you choose lines to [share](#share-lines), Enter chooses it. Plain lyrics, and synced ones shown as Plain text, are found line by line too, but there Enter only scrolls to the line, as the box says.
- **Escape** empties the box, and a second Escape closes it, leaving the panel open. The search button closes it too.
- **Another track,** other lyrics, their translation or their Latin letters coming find the lines again, with the same words.

## The notch player's line

On a MacBook with a notch, the [notch player's](desktop.md#the-player-around-the-notch) panel shows the lyrics beside the player: the line sung now, larger, the line before and the line after it, following the music. It shows them in Latin letters as you chose for their language in the panel, with the line in Latin letters under the line sung now when you show them under. Plain lyrics say "No timed lyrics.", and a podcast episode shows its chapter instead, with those before and after it. Under the lines of timed lyrics, a muted credit line names the source as the lyrics panel does, "lyrics from <source>", or "lyrics from Unison · submitted by <name>", with the names linked where the source has a page; plain lyrics show none.

## Credits

Lyrics from [Better Lyrics](https://betterlyrics.org), whose service keeps lyrics timed word by word, and from [Unison](https://unison.betterlyrics.org), an open, crowdsourced database of lyrics, whose data is under the [Unison Community License](https://github.com/better-lyrics/unison/blob/main/LICENSE-DATA.md): MixMusic, an open-source project, may use it, and credits Unison and the person who submitted the lyrics wherever they show, as the licence asks: [the line at the end of the lyrics](#where-they-come-from) and the notch player's link both, and a lyrics card names both, without links. Synced lyrics also come from [LRCLIB](https://lrclib.net), the open database of synced lyrics, and from the captions of the videos on YouTube. Romaji reads kanji with [kagome](https://github.com/ikawaha/kagome) and the IPA dictionary of [kagome-dict](https://github.com/ikawaha/kagome-dict), made from mecab-ipadic.

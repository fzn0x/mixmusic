# Changelog

All notable changes to MixMusic are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.5.10] - 2026-11-08

Play the music files on your computer beside Spotify and YouTube Music
(#317). On macOS, MixMusic plays Spotify itself, as on Windows, and the
visualizer hears it (#295, #304). Follow artists without an account,
and see their new releases on Home (#310). The search finds YouTube's
videos, such as DJ sets (#331), its suggestions play songs and open
albums, artists and playlists (#327), and a YouTube Music song can join
your library without a like (#328). Notifications show the track's
cover (#102), and a Flatpak comes for Linux on Arm64 (#294). MixMusic
takes about half the memory it took (#309), Aurora no longer makes the
app lag (#325), and a second start no longer crashes it (#305).
MixMusic 0.3.0 and later offer this version in Settings › About, except
the Linux x64 copies of 0.5.6, which don't start: install it from the
release page.

### Added

- The notification of a track that starts shows its cover, a track on
  this computer's too, cut to its centre square as the player bar shows
  it: in place of the tray icon on Windows, attached on macOS, where it
  grows as the notification expands, and as the notification's image on
  Linux. A cover that hasn't downloaded within 2 seconds leaves the
  notification without one, as before (#102).
- MixMusic builds from source as Flathub builds apps, and CI checks that
  build with every change. MixMusic isn't on Flathub for now. A copy
  built this way calls itself Flathub in Settings › About, offers no
  `.flatpak`, and runs the yt-dlp that comes with it rather than
  downloading one (#293).
- A Flatpak for Linux on Arm64, `mixmusic-<version>-linux-arm64.flatpak`,
  beside the x64 one, for the Raspberry Pi 5, Asahi Linux and Snapdragon
  laptops: `flatpak install ./mixmusic-<version>-linux-arm64.flatpak`.
  In an Arm64 Flatpak, Settings › About offers each new version's Arm64
  bundle (#294).
- On macOS, MixMusic plays Spotify tracks itself, with no Spotify app
  open, as on Windows: In MixMusic, from Spotify in Settings › Playback,
  the default of a new install. Spotify's own player plays them, at
  Spotify's quality, with FairPlay, macOS's own protection for Spotify's
  audio, and MixMusic shows among your Spotify devices. It needs Spotify
  Premium. An install that chose a way before keeps it (#295).
- On macOS 14.2 and later, the visualizer and the plugins' analyser hear
  the Spotify tracks MixMusic plays itself, as on Windows. macOS asks
  once, the first time the visualizer would hear one, whether MixMusic may
  record what other apps play; MixMusic only analyzes that audio, and
  never records or sends it. It hears Spotify only while the visualizer or
  a plugin shows it (#304).
- Follow artists without an account: signed out of their service,
  Follow on an artist's page keeps them in MixMusic, and signed in, it
  follows them on YouTube Music or Spotify, as before. Follow and
  Unfollow are in the page's More menu and in the menus of artists'
  tiles too. Library › Artists lists the artists you follow, in MixMusic
  and on each service, and Home's "New from artists you follow" shows
  their albums and singles of the last eight weeks, or of this year where
  YouTube Music tells only the year. MixMusic asks for an artist's
  releases at most every 12 hours, and never follows on a service on its
  own (#310).
- The system's media controls, Windows' media flyout, Now Playing on
  macOS and MPRIS on Linux, show the cover of a track on this computer,
  as they show an online track's (#317).
- Play the music files on your computer beside Spotify and YouTube Music.
  Add folders of music in Settings › Playback › Music on this computer,
  and Library › On this computer lists their songs, albums and artists,
  with the tags and covers your files hold; the search finds them too, in
  a section of their own. Open files with Open music files… (Ctrl+O, ⌘O
  on macOS), or drop them on the window, and they play at once. MixMusic
  plays MP3, without gaps, FLAC, WAV, M4A, and Opus and Vorbis in Ogg,
  reads a folder again only where files changed, and uploads nothing.
  Tracks on this computer join the queue, your mixes and your liked songs;
  one whose file is gone shows greyed, as File not found, and is passed
  over (#317).
- The search box suggests songs, albums, artists and playlists from
  YouTube Music below the searches it suggests, each with its cover and
  what it is and by whom. A song plays at once, with its radio after it
  in the queue when Autoplay is on; an album, artist or playlist opens
  its page. The arrow keys move through them all, and in a short window
  the suggestions scroll (#327).
- Keep a YouTube Music song without liking it: Add to library in a
  song's menu saves it to your library on YouTube Music, as its own Save
  to library does, so it stays out of your likes, which shape what
  YouTube Music recommends. Library › YouTube Music lists them in Songs
  in your library, whose menus take them out again (#328).
- The search finds YouTube's videos too, such as DJ sets, live
  recordings and fans' uploads, which aren't songs of YouTube Music's
  catalog: All shows the first five under the songs, and a new Videos
  pivot lists every one (#331).

### Changed

- On Windows 11, the tray icon's menu opens above the taskbar, centred on
  the icon, as the system's own menus do. On Windows and Linux, the
  sidebar's labels are in Selawik Semilight, as on macOS, rather than in
  Light (#102).

### Fixed

- No notification repeats what the mini player shows while it is in
  front, as none does for the main window: neither the track that starts
  nor a like or a dislike, as from a global shortcut. On a MacBook, the
  track's notification doesn't show either while the player around the
  notch is open (#289).
- The synced lyrics of a music video longer than its song, as one that
  opens with a scene before the song, no longer light up each line too
  early: LRCLIB's lyrics, timed for the song, show without their times,
  with a note saying so. Lyrics Unison has of the video itself stay
  synced (#290).
- The downloads no longer ship `resources.go`, a file of MixMusic's
  source code, nor copies of its icons that it never reads beside the
  app: the Linux archive holds `mixmusic`, `mixmusic.desktop` and
  `mixmusic.png` alone, and the `.deb`, the `.rpm`, the AppImage, the
  macOS app, the Windows installer and the portable zip lose them too
  (#303).
- MixMusic no longer crashes when it's started again while it's still
  starting, as by a second double-click, or by opening it while it starts
  at login. It finishes starting, then shows its window. A click on its
  Dock icon on macOS during the start no longer crashes it either (#305).
- MixMusic takes about half the memory it took while it plays: on a Mac,
  about 200 MB rather than about 400 MB. The artwork in the sidebar, which
  turns over now and then to show what plays next, left behind what it
  drew for each turn (#309).
- A Spotify artist's page asks for their albums and singles ten at a
  time, the most Spotify now allows, so they show again where Spotify
  refused the larger page (#319).
- Aurora no longer makes the whole app lag. Its northern lights had the
  graphics card paint the window over and over in every frame: on a Mac,
  pages scrolled at about 16 frames a second, and the lights alone kept
  the graphics card 80% busy. They look the same, pages scroll at 60
  frames a second as in the other themes, and the drifting lights take
  about 6% of it. Glasshouse's lights, drawn the same way, scroll
  smoothly again too (#325).
- On a MacBook, the black of the player around the notch is square
  against the top of the screen, as meant: its rounded corners reached a
  few pixels down into it, which left its top corners slightly
  see-through (#335).

## [0.5.9] - 2026-11-06

Podcasts from YouTube Music and Spotify, which start where you left them
on another device (#10, #231). Layouts apart from the theme: MixMusic
opens in Blend, the new default, after the update, and Tiles keeps the
look you had (#68); four new themes come with them, and the colours of
pywal, matugen or a file of your own (#68, #74). New too: downloads
that play without the network (#78), dislikes (#77), global shortcuts
(#80), signing in to YouTube Music from a browser (#96), skipping songs
made with AI (#97), speed (#99, #198), the picture of music videos
(#103), live streams (#123), 5.1 surround (#124), casting (#201) and a
player around a MacBook's notch (#46). Lyrics come from Unison first,
can be translated, show in Latin letters, and light word by word
(#135, #136, #137, #197, #238, #243). MixMusic connects through
Windows' proxy (#228), reads playlists of up to 20,000 tracks (#195),
reaches your YouTube Music history again (#261), and plays from another
YouTube server when one doesn't answer (#262). MixMusic 0.3.0 and later
offer this version in Settings › About, except the Linux x64 copies of
0.5.6, which don't start: install it from the release page.

### Added

- Podcasts from YouTube Music and Spotify: a podcasts pivot in the search
  finds shows and episodes, a show's page lists its episodes newest first,
  with their days and lengths, and Library › Podcasts keeps the episodes
  you're listening to and the shows you follow, which are saved on their
  service too when you're signed in. An episode plays from where you left
  it, without a crossfade, and the queue that ends on one stops rather than
  start a radio; episodes don't scrobble, and the stats leave them out.
  Spotify's episodes play only from Spotify, in MixMusic or on Spotify
  Connect, and signing in to Spotify again lets MixMusic start them where
  you left them on another device (#10).
- A player around the notch of a MacBook's screen. While a track is
  loaded, the black around the camera widens with its cover, and bars
  that move while it plays. Click either side for a panel with the
  player, the progress and the line of the lyrics sung now; Escape, a
  click on either side or moving the pointer away for two seconds closes
  it, and its cover opens MixMusic on the now playing page. It shows over
  every app and on every Space, full screen too, leaves the keyboard in
  the app you were using until it opens, and stays while MixMusic is in
  the tray. Turn it off in Settings › On the desktop, which shows the
  switch only on Macs with a notch, or with `notchPlayer` in
  settings.json. Other Macs, external displays, Windows and Linux show
  nothing new (#46).
- Layouts, in Settings › Appearance, apart from the theme: Blend, now the
  default, has rounded shapes and springy motion on dense rows of big
  covers in cards, under bold headings; Expressive, after Material 3
  Expressive, has tonal fills, pill buttons and chips, large rounded
  cards, a floating player bar and springs; Crate is dense and
  restrained, so that the covers do the work; and Tiles is MixMusic's
  look so far, with flat tiles and giant lowercase words. Every theme
  shows in every layout. MixMusic opens in Blend after the update: to
  keep the look you had, choose Tiles. It's `layout` in settings.json
  too (#68).
- Four themes: Material and Material Dark, whose every colour is made
  of the cover that plays (with Follow the artwork, on by default), in
  Roboto Flex, and Walnut and Walnut Light, walnut and warm paper lit by
  tangerine, with heavy condensed headings. MixMusic grows by about
  0.5 MB with the two new typefaces (#68).
- Colours from a file, in Settings › Appearance: MixMusic wears the
  colours pywal or matugen make of your wallpaper, or those of a file of
  your own, and changes colour as the file does, with no reload. The theme
  keeps its type, corners and effects. MixMusic changes the file's colours
  that wouldn't read, keeping their hue; a file with one set of colours,
  as pywal's, decides whether MixMusic is light or dark, title bar
  included; and while Follow the artwork is on, as it is by default, the
  cover playing still sets the accent (#74).
- Dislike songs you never want to hear again: with the thumbs-down beside
  the heart in the player bar, on the now playing page and in the mini
  player, with Dislike in every track's menu, with Ctrl+D (⌘D on macOS),
  or from any app with a global shortcut of your own. MixMusic then passes
  over them as the queue moves on, forward and back, moves on at once from
  the song you disliked as it plays, and leaves them out of radios; a
  toast's undo takes a dislike back and goes back to the song where it
  was. A song you pick yourself still plays, and the play buttons of
  albums, playlists, mixes and artists start at their first song you don't
  dislike. A song is liked or disliked, not both: disliking a liked song
  unlikes it, on Spotify too. Signed in to YouTube Music, dislikes of its
  songs go there too; Spotify takes none, so those stay in MixMusic. The
  library lists them in a new disliked pivot, the queue shows the songs it
  skips, and Settings › Playback turns the skipping off. The mini player's
  artwork is narrower, to make room for the button. Plugins get
  `api.library.isDisliked` and `setDisliked` (#77).
- Downloads: download tracks, albums, playlists, mixes and your liked
  songs from their menus and pages, and they play without the network.
  They stay until you remove them, apart from the offline cache, and
  gather in Library › Downloads, where you see what downloads and why one
  failed. Spotify tracks download as their recording on YouTube Music.
  Track rows show a mark for each download. Two download at a time, in the
  quality of Settings › Playback, and they wait while you're offline, on a
  metered connection unless you allow it, and when YouTube turns requests
  down, going on where they stopped, after a restart too. Settings ›
  Playback shows them, the folder they're in, which you can change, and
  the switch for metered connections. A live stream doesn't download: a
  playlist holding one downloads the rest, and once MixMusic has found it
  live its menu offers no Download. While you're offline, the queue now
  skips tracks that aren't downloaded or cached, without an error for
  each, and waits for the network only once none is left (#78).
- Global shortcuts: keys of your own that play or pause, skip, turn the
  volume up or down, mute, like what plays or show MixMusic whichever
  window is in front, with its window closed to the tray too. Record them
  in the new "from any app" group at the top of Settings › Shortcuts.
  None are set on a new install. They need Ctrl, Alt or the Windows key
  (Super on Linux, ⌘ or ⌃ on macOS); F1 to F12 may stand alone, but F12
  on Windows. While MixMusic runs, those keys do nothing else, in its own
  window too, so a window shortcut of another command gives them up.
  Keys the system refuses, being in use already, stay yours, with the
  reason on the row and try again; MixMusic tries again at each start.
  Liking from another window shows a notification when notifications are
  on. The media keys stay as they are (#80).
- Sign in to YouTube Music without the sign-in window, in Settings ›
  Accounts: import the session of a browser you signed in with (Firefox,
  Chrome, Chromium, Edge, Brave, Opera, Vivaldi, Helium, Zen, and Safari
  on macOS), which MixMusic reads with its yt-dlp, keeping YouTube's
  cookies alone, or paste the Cookie header of a signed-in tab. A session
  signed in to several Google accounts, or with brand channels, asks
  which to use, and keeps it beside the session. Close the browser before
  you import; recent Chrome on Windows seals its cookies, so use Firefox
  or paste a header there (#96).
- Skip songs made with AI, in Settings › Playback and off by default: as
  the queue moves on by itself, at the end of a song, on Next and
  Previous, into autoplay's radios, and from the play and shuffle buttons
  of a list, MixMusic passes over the songs YouTube labels as made with
  AI. YouTube labels only some of them, so others still play. A song you
  pick by name plays anyway. Skipped songs stay in the queue, dimmed, and
  one notice tells of them. MixMusic asks YouTube about each song once,
  signed out, which can add up to half a second before a song you skip
  to starts; a song YouTube doesn't answer for within 10 seconds plays.
  Spotify songs are checked by their recording on YouTube Music when they
  play from there (#97).
- Play songs faster or slower, from half speed to three times, at their
  own pitch: Settings › Playback › Speed, or Speed up, Slow down and
  Normal speed in the command palette, which have no keys until you give
  them some in Settings › Shortcuts. The player bar shows the speed after
  the artists while it isn't 1×. The clock, the seek bar and synced lyrics
  keep the track's own time, a crossfade lasts the seconds you hear, and
  Last.fm counts how much of a song you heard. The system's media
  controls move on at the speed, and setting the rate over MPRIS sets it.
  Spotify plays its own tracks at their own speed, and live streams play
  as they're broadcast (#99).
- Music videos can show their picture on the now playing page, in place of
  the cover: turn on "Show the video of music videos" in Settings ›
  Playback, or press the video button beside the transport while one
  plays. The picture is muted and follows MixMusic's own sound, pausing
  and seeking with it. It shows only for official music videos and
  uploads, never for songs or live streams, and keeps the cover where the
  system has no web view (WebView2 on Windows, WebKitGTK on Linux). The
  picture is fetched as it plays, about 3 MB a minute at 360p, the lowest
  quality with data saver, and nothing while it's paused or not shown: on
  another page, with the window hidden or minimized, or with the mini
  player open. While the system asks for less motion, a button on the
  cover plays it. A web view then runs beside MixMusic, which takes memory
  until a minute after the last picture (#103).
- Live streams play, such as YouTube's 24/7 radio channels: from about 15
  seconds behind the broadcast, for as long as it goes on. The player bar
  and the mini player say "live" in place of a time and a length, with no
  seeking, and Previous goes to the track before. A live stream ends only
  when its broadcast does, isn't kept in the offline cache, and isn't
  scrobbled to Last.fm, which hears only that it plays; Discord shows how
  long it played. It plays at its own level, as YouTube measures none for
  Even loudness. After a pause of up to about an hour it goes on from
  where it was, after a longer one from near the live edge. Data saver
  plays its 22 kHz stream (#123).
- For developers: an AC-3 decoder in Go (`ac3`), written from the ATSC
  A/52 standard: every channel layout up to 5.1 and every tool of the
  standard, decoded into six channels, with the stereo downmix each
  stream's own mix levels ask for, and the `dac3` box of MP4 files (#124).
- For developers: the `ac3` package decodes E-AC-3 too, of ATSC A/52's
  Annex E: frames of 1 to 6 blocks, spectral extension and the adaptive
  hybrid transform among its tools, and the 5.1 of a stream of more
  channels or of objects; and reads the `dec3` box of MP4 files (#124).
- 5.1 surround: a track YouTube Music has a 5.1 surround mix of, beside
  its stereo one, shows a 5.1 button by the player, which plays the
  surround mix from where the stereo one was. On a 5.1 output device the
  six channels play as they are; on headphones or two speakers they are
  mixed down to stereo, as the track's own downmix levels say. The choice
  stays for later tracks: "Play 5.1 surround when a track has it" in
  Settings › Playback, off by default. While it is on, stereo tracks play
  on the front left and right speakers of a 5.1 system, the surround mix
  of a track goes to the offline cache beside its stereo audio, and a
  surround mix that can't play gives way to the stereo one. The surround
  mix plays at the speed you set, as the stereo one does; downloads keep
  the stereo audio, which plays while you're offline (#124).
- Lyrics come from Unison first, an open, crowdsourced database of
  lyrics, then from LRCLIB, then from YouTube Music, so more tracks have
  synced lyrics. Unison is asked by the track's video ID, or by its
  title, artist, album and length, and the lyrics panel ends its lyrics
  with "lyrics from Unison (https://unison.boidu.dev)", as their licence
  asks. While Unison is out of reach, lyrics come from LRCLIB as before,
  after a wait of 3 seconds at most (#135).
- Lyrics in another language can be translated: turn on "translate"
  beside the lyrics, in the lyrics panel or on the now playing page, and
  each line shows its translation under it, smaller, moving with it in
  synced lyrics. The switch shows for lyrics in another language than
  the one they translate to, the system's by default, which "Translate
  lyrics to" in Settings › Playback changes, and is remembered for the
  next song in the same language. While it's on, the lyrics of the track
  playing and its video ID go to Unison, which has Google translate them;
  nothing is sent while it's off, as it is by default. Each translation
  is kept on disk, so a song is translated once. When the service is
  busy, the lyrics show untranslated with "Translation isn't available
  right now." and try again (#136).
- Japanese lyrics can show in romaji: turn on "romaji" beside them, in
  the lyrics panel or on the now playing page, then choose "romaji
  under" or "romaji only", as "kyō wa ii tenki" for 今日はいい天気. With
  translate on too, a line shows the Japanese, its romaji, then its
  translation. The romaji is made on your computer, and the lyrics
  aren't sent anywhere for it. The first time, MixMusic downloads the
  dictionary of the readings of kanji, 11.7 MB, from GitHub, and keeps
  it in its cache: until it's there, or while it can't be downloaded,
  kana show in romaji and kanji as they are. While Japanese lyrics show
  in romaji, MixMusic uses some 90 MB more memory, which it gives back a
  few minutes after (#137).
- Lyrics in Korean, Chinese, Russian or Arabic can show in Latin letters:
  turn on "romanize" beside the lyrics, then choose "romanized under",
  each line with its romanization under it, or "romanized only", in
  place of the line. With translate on too, a line shows the original,
  its romanization, then its translation. The choice is remembered for
  the next song in the same language. Arabic's romanization comes with
  the translation from Unison: while it's on, Arabic lyrics go to Unison
  with the track's video ID (#137).
- Lyrics timed word by word, as Unison has many, light each word of the
  line sung now as it's sung, in the lyrics panel and on the now playing
  page: the word fills with the accent from its start to its end, words
  sung stay lit, and words to come keep the colour of the lines to come.
  Background vocals light the same way. Lyrics timed line by line show as
  before, and so do lines shown in Latin letters in their place, which
  light whole. While the system asks for less motion, each word lights
  whole as it starts. Plugins reading lyrics get the words and their
  times too (#197).
- A speed button by the player, beside the volume, shows the speed while
  it isn't 1× and opens a menu of steps, from 0.75× to 2×, with a slider
  for the rest. The mini player has the steps in its menu, on a right
  click. Podcasts get a speed of their own, Podcasts in Settings ›
  Playback › Speed, 1× at first: episodes play at it, songs at theirs, and
  the button, the command palette's Speed up, Slow down and Normal speed,
  and the rate the media controls set change the speed of the kind that
  plays. While Spotify plays its own tracks or episodes, or a live stream
  plays, the button is dimmed, and its tip tells why (#198).
- On Linux, Settings › Accounts imports a YouTube Music sign-in from
  Firefox, Chrome, Chromium, Edge and Brave installed from Flathub too:
  each shows in the list of browsers as "Chrome (Flatpak)" and so on,
  apart from the same browser installed otherwise. MixMusic's Flatpak may
  read their profiles, read-only, to do so (#199).
- Scrolled down a playlist, an album, an artist or your liked songs, the
  top bar shows what you're in: its cover, its name, and Play, Shuffle
  and Download, which fade in as the page's head scrolls away, and show
  at once when your system asks for less motion. In a narrow window the
  name shortens, and Download moves into the bar's menu. The search and
  Sort of a playlist's tracks, and of your liked and disliked songs, stay
  pinned under the top bar while the list scrolls, and what you search
  or sort there shows from the list's start. Liked songs get Play and
  Shuffle buttons of their own (#200).
- Casting: a cast button by the player, shown while a Chromecast, a
  Google Home or Nest speaker, or a TV with Chromecast built in is on your
  network, lists them by name and model, and moves what plays there from
  where it was; "This computer" brings it back. The device plays YouTube
  Music tracks, and Spotify tracks in "Match" mode as their YouTube Music
  recording, with their cover, title and artist on its screen; Spotify
  tracks that play on Spotify Connect, or in MixMusic itself, stay where
  they play. Play, pause, seek, next, previous and the volume act on the
  device, and its own buttons and remote show in MixMusic. The queue stays
  in MixMusic, which sends each track as the one before ends, so tracks
  don't crossfade on a device. The device fetches the audio from MixMusic
  while casting, which may ask the first time whether MixMusic may take
  connections from your network: Windows' firewall does, and macOS asks
  for the local network. Live streams don't cast yet (#201).
- An episode shows its details where a song shows its lyrics: the side
  panel says Details, and it and the now playing page show the show, the
  views, the day it came out and its length, then the whole description.
  The show's name opens its page, times in the description, as 12:34 or
  1:02:03, play the episode from there, and links open in the browser.
  The chapter that plays lights up, and the notch player shows it with
  those before and after it. The details come from YouTube Music's
  Details tab, or from Spotify, when an episode plays, and are kept
  (#226).
- A playlist, an album, a mix or your liked songs that you downloaded
  download the songs added to them since on their own, as you open their
  page or refresh it. Over a hundred songs at once, or a gigabyte, wait
  for you to choose Download more, as a notice tells. A song that failed
  isn't tried again, and one whose download you removed isn't downloaded
  again: Download more still offers them. Songs taken out of a playlist
  stay downloaded with it until you remove its download. Download songs
  added later, in Settings › Playback, turns it off (#227).
- Pull a page down past its top to refresh it, with two fingers on a
  touchpad, a finger on a touchscreen, or the mouse wheel turned up there
  three notches or more. A strip above the page grows with the pull and
  says when letting go refreshes; the refresh is Refresh this page's,
  with its spinner and its toast when it fails. Scrolling that carries
  the page up to its top, a fling's momentum among it, never pulls, nor
  does Shift+scroll on a shelf, scrolling sideways, or scrolling the
  lyrics on the now playing page. Pages Refresh is dimmed on don't pull,
  and while your system asks for less motion the strip opens and closes
  at once (#229).
- The mini player has a volume button, after the heart and the
  thumbs-down: a click mutes and unmutes, the wheel over it turns the
  volume up and down a step a notch, as Volume up and Volume down do, and
  a small slider shows above it while the pointer is over either. Tab
  reaches the button, and goes on into the slider, whose arrows set the
  volume; Escape closes it. The wheel turns the player bar's volume too.
  The mini player's artwork is narrower in its narrowest window, to make
  room for the button (#230).
- Signed in to YouTube Music, an episode starts where you left it on
  another device, when that's newer than where you left it in MixMusic,
  and one you finished there starts from its start. While an episode
  plays, MixMusic tells YouTube Music how far you got, as its own player
  does, so that YouTube Music on your phone goes on from there. The switch
  that adds what you play to your history, in Settings › Accounts,
  governs it too (#231).
- Lyrics in every major script show in Latin letters made on your
  computer, nothing of them sent, and the "romanize" switch shows for
  them at once: Korean by the Revised Romanization, with its sound
  changes ("한국어" is "hangugeo"); Chinese in pinyin with tone marks, a
  character of several readings read by its word ("银行" is "yín háng"),
  simplified and traditional; Russian, Ukrainian, Belarusian, Bulgarian,
  Serbian, Macedonian, Kazakh, Kyrgyz and Mongolian, each by its own
  system; Greek, Armenian and Georgian; Hindi, Bengali, Punjabi,
  Gujarati, Odia, Tamil, Telugu, Kannada, Malayalam, Sinhala and the
  other languages of their scripts; Thai, a syllable apart; Tibetan;
  Hebrew and Persian; and Amharic. The English of a line, as in K-pop,
  stays as it is. Lines timed word by word light each word in Latin
  letters too, and the notch player shows the lyrics in Latin letters as
  the panel does. Arabic still comes from Unison, whose romanization has
  the vowels Arabic doesn't write: until it comes, and when it can't,
  Arabic shows in Latin letters made on your computer (#238).
- Lyrics in Lao, Khmer and Burmese show in Latin letters made on your
  computer too, nothing of them sent, and the "romanize" switch shows for
  them at once: Lao by BGN/PCGN ("ຫຼວງພະບາງ" is "louang pha bang"), Khmer
  by the UN's system, its vowels read by their consonants ("ភ្នំពេញ" is
  "phnum pénh"), and Burmese by BGN/PCGN, its sounds voiced as said
  ("မင်္ဂလာပါ" is "min ga la ba"), each a syllable apart. Lines timed word
  by word light each word in Latin letters too (#243).

### Changed

- The system's media controls (the Windows media flyout, macOS's Now
  Playing, and the desktop's controls on Linux) take each cover from the
  covers MixMusic keeps on disk, so a cover already shown in the window
  shows there at once, isn't downloaded again, and shows offline too. On
  Linux, the desktop reads it from a file in MixMusic's cache folder
  (`now-playing/`) rather than downloading it itself (#196).
- On macOS, the yt-dlp MixMusic installs starts in 0.2 seconds instead of
  8: MixMusic installs yt-dlp's unpacked build, which doesn't unpack Python
  again at every start. A track that only yt-dlp can play starts that much
  sooner, and so does Settings › Playback › Tools. The yt-dlp installed
  before is replaced a minute after MixMusic starts, at its update check,
  whatever its version. It takes about 120 MB on disk instead of 37 (#245).

### Fixed

- The notification of a new track shows again, once you hear it, while
  MixMusic is in the background. It never showed: the track changed while
  it loaded, and played only after. Between two tracks, Discord's status no
  longer goes away and comes back, the system's media controls no longer
  show the next track as paused for a moment, and the taskbar's play
  button no longer turns to Play and back (#145).
- While a track loads, it can't be paused or moved: the play button, the
  position and the lines of timed lyrics take no click until you hear it,
  and the keys, the media keys and the system's media controls do nothing
  either. A track counted as playing before its sound came, as YouTube held
  its audio back or Spotify's player still loaded it, and a move started
  its load over from there (#184).
- Spotify songs that play in MixMusic itself follow one another without a
  gap: Spotify goes on to the next by itself, where 1 to 2 seconds of
  silence came between them. MixMusic hears at once that a Spotify song
  ended, so a YouTube Music song after it starts without a wait too. When
  the queue changed meanwhile, as a song was played next, what Spotify
  went on to is silenced and the chosen song plays (#187).
- The sidebar shows MixMusic's name whole beside the logo. It showed cut
  short, as "mixmu…", in the Sakura theme, and in every theme with the
  compact density; it now shrinks a little where the theme's face is too
  wide for the sidebar (#191).
- With Crossfade on, a pause made in the last seconds of a song holds:
  the next song no longer fades in a moment later. Played again, the song
  fades into the next one while there's time, or the next one starts as
  it ends (#193).
- Choosing a YouTube brand channel never quietly uses the account itself.
  A brand channel YouTube lists without the ID MixMusic needs shows in the
  list, but can't be chosen, and says why. If YouTube Music still answers
  as the account itself after a switch to a brand channel, MixMusic goes
  back to the one used before and says so (#194).
- Playlists longer than about 1,000 tracks, and the songs liked on
  YouTube Music, are no longer cut short: up to 20,000 tracks are read.
  The first hundred or so show at once and the rest load beside them;
  Play and Shuffle, pressed meanwhile, wait for all of them, and the
  menu's queue items and the download show once they're read. A page
  that fails is tried again a few times, then the playlist says how many
  of its tracks loaded, with Try again; one past 20,000 tracks says where
  it was cut. Playing a long playlist from its tile, downloading it or
  copying it into a mix reads all of it first, which can take a while
  (#195).
- With the colours of pywal, matugen or a file of your own, the theme no
  longer goes back to the file's colours from before it changed, when
  Reload add-ons and the file's watch read it at the same time (#220).
- Removing the download of a playlist removes the songs taken out of it
  before you downloaded more of it, which used to stay downloaded for
  good (#227).
- On Windows, MixMusic connects through the proxy set in Windows'
  settings, as browsers do: a setup script, automatic detection or a
  proxy server, with its bypass list. It follows the setting as VPN and
  proxy apps turn it on and off. Before, it connected directly, so on a
  network that lets traffic out only through a proxy, songs were found
  but didn't play: "MixMusic could not reach YouTube's audio servers"
  (#228).
- Page titles and pivots are set in Selawik Light at every launch. In
  about half of them they were Selawik Semilight, heavier and wider, as
  the two faces both say they weigh 300 and the first registered won
  (#241).
- On macOS, Settings › Playback › Tools shows a JavaScript runtime you
  choose at once. It loaded for 8 seconds or more at each choice, as yt-dlp
  was asked its version again, and after a few choices in a row yt-dlp
  showed without it (#246).
- On macOS, MixMusic's name beside the logo shows whole: the window's
  buttons keep the top of the sidebar, with the button that folds it, and
  the logo and name go under them. The name was cut to "mix…", and the
  folded sidebar's button to unfold it was out of sight (#247).
- Ctrl and the keypad's + zoom in on Windows and Linux, and ⌘ and the
  keypad's + on macOS, where View › Zoom In now shows ⌘+. They did nothing,
  though the keypad's − zoomed out. As Ctrl+Shift+= does, they follow Zoom
  in while it keeps its own shortcut in Settings › Shortcuts (#251).
- A track YouTube Music has a 5.1 surround mix of shows the 5.1 button by
  the player, and plays the mix while "Play 5.1 surround when a track has
  it" is on. No track did: YouTube didn't list the mix where MixMusic
  asked for the stereo audio. The mix takes yt-dlp and a JavaScript
  runtime, which find it in a few seconds when it first plays (#257).
- In the Expressive layout, the sidebar's card of what plays shows whole
  above the floating player bar. In a window under about 800 px tall, as
  macOS's default one, it ran on under the bar, half of its title hidden;
  the sidebar's pages now sit a little closer there to keep the card's
  room, and in a window too short for it the card goes, in every layout
  (#258).
- A zoom chosen in the mini player is read out once, by the mini player.
  The main window, open behind it, showed it in a toast too, which screen
  readers read out as well. A zoom chosen in settings.json still shows
  its toast in the main window (#259).
- A track's "Add to YouTube Music playlist" lists your playlists the first
  time it opens. It said "Loading your playlists…" for as long as it was
  open, and listed them only once opened again. MixMusic now reads them
  once you're signed in, before any menu opens, and a playlist you make
  shows there at once. When YouTube couldn't answer, it says "Couldn't
  load your playlists", where it said "No playlists of yours", and asks
  again for the next time it opens (#260).
- Songs you play from YouTube Music reach your history there again, with
  "Add what you play to your YouTube Music history" on, so YouTube Music's
  home and radios learn from what you play in MixMusic. YouTube had begun
  answering MixMusic's request as if every video were unavailable, and no
  play reached the history. When YouTube gives no way to report a play,
  the log now says why (#261).
- A track whose audio is on a YouTube server that the network doesn't
  reach plays from another of YouTube's servers that hold it, as
  YouTube's own player does. It failed with "MixMusic could not reach
  YouTube's audio servers" after waiting on that one server for about 10
  seconds, while other tracks played fine; the next server is now asked
  within 3 seconds, and the tracks after it start with the servers that
  answered (#262).
- A song played from a shelf of the home page or Explore, such as Quick
  picks, counts as a play once it played 30 seconds, or half of a shorter
  one, as other songs do. It counted as soon as it started: skipped a
  second later, or failing to load, it still joined the history and
  Recently played, and went to the YouTube Music history (#264).
- A show saved on YouTube Music or Spotify, as when you followed it in
  another profile or on your phone, says Following on its page, and
  Following stops following it there. It said Follow, though Library ›
  Podcasts listed it under the shows saved there, and Follow followed it
  again (#271).
- A cover downloads once, however many windows and the system's media
  controls ask for it. Now and then, one that asked just as its download
  for another ended downloaded it a second time (#272).
- Spotify songs that play in MixMusic itself follow one another without a
  gap also after a seek into a song's last seconds. More than a second of
  silence came before the next song then, or a song of the queue was
  skipped: Spotify stops on the song a moment while it loads the next, and
  MixMusic took that for the end and told it to play the next song itself.
  A Spotify song sought into its last second no longer starts over instead
  of moving on (#273).
- An episode played from a show's page starts where you left it, in
  MixMusic or on another device, whatever played before. It started from
  its start when what played before was at the same place in its queue,
  as the first episode played while the first track of the queue before
  was current (#275).
- Lyrics are offered "synced" only when their lines can be followed in
  time. Lyrics of one timed line, or of lines all at one time, as LRCLIB's
  "probe" for Never Gonna Give You Up, offered it, and never moved; they
  are plain now, and another source's lyrics take their place when it has
  more of them (#277).

## [0.5.8] - 2026-11-02

Delete the playlists you made on YouTube Music, and take saved ones out of
your library (#75). F5 (⇧⌘R on macOS) refreshes a page without restarting
MixMusic (#92). A box on the Settings page finds a setting as you type its
name (#93), and what you save to `settings.json` applies while MixMusic
runs (#81). "Add to YouTube Music playlist" lists your playlists again
(#75), and "use it" on a marketplace theme works while the themes follow
the system (#174). MixMusic 0.3.0 and later offer this version in
Settings › About, except the Linux x64 copies of 0.5.6, which don't start:
install it from the release page.

### Added

- Signed in to YouTube Music, delete a playlist you made there from the
  More menu of its page, or from its tile in Library › youtube music:
  "Delete from YouTube Music…" asks first, since it goes from YouTube too
  and can't be undone. The tile of a playlist you saved has "Remove from
  library" instead. Either leaves the library at once; mixes copied from
  the playlist, and the queue, keep their tracks (#75).
- What you save to `settings.json` applies while MixMusic runs, as if
  changed in Settings, and MixMusic reads the file again before it saves,
  so it doesn't write over an edit it hasn't read. A key you take out goes
  back to its default, as at start: taking out `spotifyClientId` signs you
  out of Spotify, and plugins enabled in the file start without asking. A
  mistake is told with its line and column, and the file stays as you
  saved it until you fix it; if MixMusic quits first, it keeps your text
  as `settings.json.broken`. A file with a byte order mark, or in UTF-16 as
  Windows PowerShell 5.1 writes it, reads now, a `settings.json` that links
  to your dotfiles stays a link, and a file that didn't read at start is
  told of. Settings › Desktop and the command palette open the file (#81).
- Refresh this page asks the services again for what the page shows,
  without restarting MixMusic: press F5 (⇧⌘R on macOS), or the new
  button after the back and forward arrows. The page keeps showing what
  it has until the answers come, with a spinner in the button meanwhile;
  if the refresh fails, as offline, it keeps what it showed and one
  toast says why. Home, searches, albums, artists, playlists, Explore,
  your Spotify and YouTube Music libraries, the lyrics, the marketplace
  (past the ten minutes MixMusic keeps the registry) and Settings' checks
  refresh; plugin pages and widgets render again, running their cleanup
  first. Ctrl+R (⌘R on macOS) stays Repeat. The command is in the
  command palette, Settings › Shortcuts and macOS's View menu; if you
  gave F5 or ⇧⌘R to another command, it keeps it and Refresh starts with
  no key. The button is dimmed on pages that always show what is
  current, as liked songs and stats (#92).
- Search the settings: a box on the Settings page finds a setting by its
  name, its description, its group or its section, regardless of case,
  accents or spelling ("color" finds Accent colour), and keeps its
  control, so it can be changed right there. The search is part of the
  page's location, so Back returns to it, and each group found links to
  its section. Whenever something is typed, the command palette ends its
  list with "Search the settings for …", and a new command, "Search the
  settings", opens the box; Settings › Shortcuts can give it a key. The
  box sits under the sections' names, and doesn't take the focus when
  Settings opens. Settings › Appearance draws faster too: its theme
  cards no longer remake every theme's look in every frame (#93).

### Fixed

- A track's "Add to YouTube Music playlist" lists your playlists again,
  where it said "No playlists of yours" since 0.5.0. When YouTube Music
  won't let a track into a playlist, the toast says so, and you stay
  signed in: a refused edit no longer asks you to sign in again (#75).
- The save button of an album or a playlist you saved on YouTube Music
  says "saved" again, and pressing it takes it out of your library.
  YouTube Music draws that button with new icons now: MixMusic said
  "save", and pressing it saved it again (#75).
- Home's jump back in tile, recently played and your mixes follow what
  you play and the mixes you make or delete, rather than keep what they
  held when Home first showed. When Spotify or YouTube Music can't load
  its shelves, Home now says so under the others, with try again, rather
  than leaving them out unsaid (#92).
- After try again, signing in to another Spotify account or a refresh,
  your liked songs on Spotify start again at the first page, rather than
  keep the pages load more brought before (#92).
- Resetting a shortcut in Settings › Shortcuts gives the command its own
  keys back, from the command you gave them to, rather than leave it
  with none (#92).
- "use it" on a theme's card in the marketplace works while the themes
  follow the computer's light or dark mode: the theme becomes the one of
  the mode the computer is in, as the "use it" of the toast after an
  install already did. It changed a setting that nothing showed, then
  went away as if the theme were in use (#174).

## [0.5.7] - 2026-11-02

MixMusic starts again on Linux x64: 0.5.6's x64 downloads asked for a
system loader only NixOS has, so they didn't start elsewhere (#166). If
yours updated itself to 0.5.6, it can't update on its own: install 0.5.7
from the release page, or run install.sh again. After a music video,
Autoplay and Start radio carry on with songs (#90). Linux on x64 also gets
a Flatpak (#79). MixMusic 0.3.0 and later offer this version in
Settings › About, except the Linux x64 copies of 0.5.6.

### Added

- A Flatpak for Linux on x64, `mixmusic-<version>-linux-amd64.flatpak`,
  which brings GTK, WebKitGTK, the tray's library and Node.js for yt-dlp
  with it: `flatpak install ./mixmusic-<version>-linux-amd64.flatpak`
  installs it, and fetches the GNOME runtime from Flathub the first time.
  Settings › About calls it a Flatpak and offers each new version's
  file. Inside it, MixMusic shows in the desktop's media controls under
  its Flatpak's name, starts at login through the desktop's Background
  portal, and asks that portal to play on with close to the tray on. Its
  settings and library are its own, in
  `~/.var/app/io.github.fzn0x.mixmusic`, apart from a .deb or AppImage
  copy's: the README tells how to bring them over. Report a problem now
  says how MixMusic was installed (#79).

### Fixed

- After a music video, Autoplay and Start radio carry on with songs.
  YouTube Music makes the radio of a video out of videos, so Autoplay
  queued 25 more music videos, Start radio 49, and Autoplay kept to videos
  from then on. A video's radio now starts from its song, and a cover or
  other upload without one from a song its radio suggests, or else the
  song of one of its videos. That takes about half a second longer, and a
  second for an upload. A radio of songs no longer adds the covers,
  mashups and other uploads it suggests beside them, unless it has
  nothing else left (#90).
- MixMusic starts again on Linux x64. 0.5.6's x64 downloads, the .deb, the
  .rpm, the AppImage and the archive, asked for a system loader that only
  NixOS has, so they didn't start on other systems. A copy that updated
  itself to 0.5.6 can't update on its own: install 0.5.7 from the release
  page, or run install.sh again. Your settings and library are kept (#166).

## [0.5.6] - 2026-11-02

Zoom makes the whole interface larger or smaller, from 80% to 200%
(#106). YouTube Music tracks start in a moment (#69), play at an even
loudness (#83), and play when YouTube has them only in AAC (#120). When
the network drops, a song waits for it rather than MixMusic skipping
songs (#91). MixMusic has a new icon, shows what plays on Discord with
nothing to set up, and is under the MIT license or the GPL 3.0.
MixMusic 0.3.0 and later offer this version in Settings › About.

### Added

- The main window's title names the track that plays, for the taskbar,
  the window switcher and screen readers, and starts with "Paused:" while
  it's paused. Turn off "Show what plays in the window title" in
  Settings › Desktop to keep it to MixMusic's name (#105).
- Covers you saw are kept on disk, up to 256 MB, so they show at once at
  the next start, even offline. Settings › Playback says how many are
  kept, under the offline cache, and clears them (#73).
- Even loudness, in Settings › Playback and on by default: loud YouTube
  Music tracks are turned down to one level from their first note, from
  YouTube's own measure of each, so none jumps out. Quiet recordings keep
  their level. Loud masters are common, and play 4 to 7 dB quieter than
  before: turn up your system's volume, or turn Even loudness off, if they
  sound too quiet now. Once measured, tracks keep their level offline too,
  from the cache. Spotify tracks that Spotify plays keep Spotify's own
  loudness (#83).
- Zoom makes everything in MixMusic larger or smaller at once, from 80% to
  200%: text, icons, buttons, rows and artwork, drawn sharp at the new size.
  Ctrl+= (or Ctrl+Shift+=) zooms in, Ctrl+- zooms out and Ctrl+0 goes back
  to 100%, or ⌘ on macOS. The keys are in the command palette and Settings ›
  Shortcuts, where you can change them, and macOS's View menu has Zoom In,
  Zoom Out and Actual Size. Settings › Appearance has a Zoom select above
  Text size. Each change shows a toast with a reset button. The mini player
  zooms too, and its window grows and shrinks with it. A window too small
  for the zoom you chose zooms only as far as it has room for, and says so
  (#106).
- For developers: MixMusic's audio code is in Go packages anyone can
  import, documented on pkg.go.dev: an Opus decoder (`opus`), an AAC-LC
  decoder (`aac`), the audio of WebM and MP4 files (`webm`, `mp4`),
  sample rate conversion (`resample`), synced lyrics from LRCLIB
  (`lyrics`) and MixMusic's logo (`resources`).
- For developers: with Nix, `nix build` builds MixMusic for Linux from
  source, and `nix develop` gives the tools releases are built with, at
  the versions `flake.lock` pins. Releases are built that way and
  published with GoReleaser.

### Changed

- A new icon: two pink rings and a play button on black, in the app, the
  tray, the website and the README.
- MixMusic is dual-licensed: use it under the MIT license, as before, or
  under the GNU General Public License, version 3, whichever you prefer.
  Settings › About says so.
- Your Discord profile shows what plays without setting anything up:
  MixMusic has its own Discord application now, so you no longer create
  one and paste its ID. It is on unless you turn off Show what plays on
  Discord in Settings › Desktop, where you can still paste the ID of an
  application of yours to show its name. If you had set up your own
  application and turned it off, it stays off.
- Discord's member list says "Listening to MixMusic" rather than the
  artists, for Spotify and YouTube Music tracks alike; your profile still
  shows the title, the artists and the artwork. Discord's own Spotify
  status comes from your Discord account's link to Spotify: to show only
  MixMusic, turn off Display Spotify as your status in Discord's
  Connections.
- The Windows installer offers to close a running MixMusic before it
  upgrades it, and stops one that keeps running in the tray.
- When the network drops, a song waits for it and plays on from where it
  stopped once it's back, rather than MixMusic skipping songs until three
  in a row failed. The player bar says that it waits for the network,
  and the play button chooses whether the song plays then. Songs whose
  audio MixMusic holds, in the offline cache or found shortly before,
  still play meanwhile, and at the end of the queue the radio waits too
  (#91).
- MixMusic pauses as the computer goes to sleep, and stays paused when it
  wakes, ready where it was; a Spotify Connect device elsewhere plays on
  (#91).
- The volume slider has even steps of loudness: each tenth of it is 6 dB,
  and one press of up or down is about 3 dB at every level down to -40 dB,
  where it used to jump 6 dB or more near the quiet end. Your volume sounds
  as before; the slider shows it further right. Spotify played in MixMusic
  follows the same curve, and Spotify Connect devices get the same
  percentage as before for the same loudness. A plugin that sets a fixed
  volume with `api.player.setVolume()` gets that position's new loudness
  (#82).
- Data saver plays the original audio of the few videos that also come in
  YouTube's stable-volume version, which is no smaller and squeezes the
  sound (#83).
- Plugins are written in TypeScript: the community's sleep timer and
  spectrum, and the plugin API itself, which the type checker now holds to
  the types of `sdk/index.d.ts`. MixMusic still runs the JavaScript they
  compile to, so every plugin, and every installed version, works as
  before. The plugin docs show how to write and compile one.

### Fixed

- On macOS 26 and 27, macOS no longer runs an AutoFill helper for
  MixMusic's windows, which took CPU and could stall scrolling (#85).
- On macOS, Cmd+Q, the Dock's Quit and the tray menu's Quit end MixMusic
  even when a restart, as after an update, left it running without a
  window (#86). The restart that installs this version still runs the old
  version's code.
- On Windows, a frame the screen never takes no longer holds up MixMusic
  for a second (#87).
- On macOS, closing the window in full screen with "Close to the tray"
  on no longer leaves an empty, black Space: the window leaves full
  screen, then hides in the tray, and comes back in full screen (#84).
- After a restart, a queue of more than 500 tracks comes back on the
  track that was playing, where it was, with what comes up after it.
  MixMusic kept the queue's first 500 tracks, and when the track playing
  was past them it came back on the 500th, at the other track's place;
  it now keeps the 500 tracks around the one playing (#88).
- Changes to the queue made while a song is paused, or before anything
  plays after a start, are kept when MixMusic quits, as are the changes
  made right before quitting: the queue, where the song was, the volume
  and the colours of Settings › Appearance. This holds when Windows
  shuts down, restarts or signs out too (#89).
- Shuffle mixes in the tracks before the one playing that haven't played
  yet, as when you start an album or a playlist from one of its tracks:
  it left them out, and with Repeat all they played in the list's order.
  The track playing then comes first, so Previous starts it again. Your
  own tracks stay ahead of the radio's when you shuffle, and turning
  shuffle off puts the tracks you added with Play next or Add to queue
  back where you added them, after a restart too (#95).
- After a restart, the radio's tracks keep their "radio" mark in up
  next, and Add to queue still puts tracks before them (#95).
- YouTube Music tracks start in a moment, rather than several seconds
  after you press play: MixMusic asks YouTube for a track's audio itself,
  and runs yt-dlp only for the tracks YouTube won't give it that way, such
  as those made for children. The next track is found a second into the
  one playing, so skipping ahead is quick too (#69).
- A song whose audio stops arriving partway through, as when the
  connection drops or is slow, loads again where it was up to three
  times, waiting a little longer each time, rather than skipping to the
  next song the second time it happens. The play button shows that it
  loads meanwhile, a song paused stays paused, and the song has all its
  tries again once it plays on for ten seconds, or when it plays again
  with Repeat: one or Previous (#101).
- When three songs in a row can't play, MixMusic stops again rather than
  stepping through the rest of the queue (#101).
- A song whose audio stops coming, as when the connection drops, connects
  again after 15 seconds without a byte rather than two minutes, and
  keeps trying for two minutes, playing on as soon as the network
  answers. A slow connection no longer cuts off a long song's download
  every two minutes (#100).
- A song whose audio stops coming in its last second ends there, rather
  than pausing to load that moment again, or showing an error for a song
  that had all but finished, and Repeat: one plays it again (#94).
- A cover that fails to load on a flaky connection tries again within
  seconds, rather than staying grey for a minute, and the others of its
  service follow as soon as one loads. A connection that went silent is
  dropped in about half a minute. The log now says which service's
  covers failed and why (#73).
- Dialogs fit a window shorter than they are, as on a small screen at a
  large scale or with a tiling window manager: their title and buttons
  stay in view, and what lies between them scrolls. This covers the
  command palette, the update offer, and the questions MixMusic asks
  before an action, which now look like its other dialogs. The palette's
  list follows Up and Down, and the update offer opens with the focus on
  later, so a Space pressed while it shows puts it off rather than
  skipping that version (#76).
- A volume that isn't a number, from a plugin or MPRIS, is ignored. It
  used to silence MixMusic's own player until a restart, and break every
  plugin's `api.player.state()` (#82).
- Space plays or pauses after you click a tile, a switch, a chip, a link,
  the player's buttons or a track's buttons. It no longer presses what you
  clicked again, and after a search it no longer types a space into the
  search box: a one-key shortcut such as M for mute runs instead. A
  control you reach with Tab still takes Space and Enter (#98).
- On Windows, a sign-in the Credential Manager can't keep is kept
  encrypted for your Windows account, so you stay signed in after a
  restart. Sign-in files in plain text found in MixMusic's data folder,
  such as from a copy made on Linux, move into the system's credential
  store and are deleted, on Linux too once a keyring is there (#104).
- On macOS, ⌘R reaches Repeat. The View menu's Reload and Force Reload,
  which did nothing in MixMusic's windows, held the key; MixMusic's own
  View menu leaves them out (#106).
- Tracks YouTube Music has only in AAC play now. They used to load for a
  few seconds and fail, saying the audio format is not supported; the
  audio engine plays AAC itself now, as it plays every other track. Other
  tracks play as before (#120).
- The lyrics panel says "No lyrics found for this track." when a track has
  none, even while the lyrics service is too busy to look beyond its own
  database, when trying again may still find some. It says lyrics can't be
  loaded right now, with a button to try again, only when the service is
  down or out of reach, rather than an error such as "lyrics: LRCLIB
  answered HTTP 503".
- A video that also comes in 5.1 surround plays its stereo audio: MixMusic
  never picks a surround version, which it can't play yet, before it.

## [0.5.5] - 2026-10-28

Pages scroll smoothly on every system, even with thousands of liked songs
(#59). A song that loads plays rather than pausing, and Exit in the tray
menu quits MixMusic with "Close to the tray" on. MixMusic 0.3.0 and later
offer this version in Settings › About.

### Fixed

- Pages scroll smoothly (#59): liked songs, playlists, the history and
  the libraries of Spotify and YouTube Music, and the shelves of Home,
  Explore and the artists' pages. Every frame built every row and tile of
  the page, so that a list of a thousand songs scrolled at a few frames a
  second; MixMusic now builds only what is near the window, as fast for
  five thousand songs as for a hundred, and sorts and searches a list
  when it changes, not in every frame.
- A song that loads can no longer be paused, from the play button, the
  media keys or the tray: it plays once it loaded. Choosing another song
  still leaves it. The play button shows that the song loads.
- Exit, in the tray icon's menu, quits MixMusic again with "Close to the
  tray" on: it closed the window to the tray instead, and MixMusic kept
  running. Quit (Cmd+Q) on macOS did the same.

## [0.5.4] - 2026-10-24

On Linux, signing in to YouTube Music no longer closes MixMusic (#51).
MixMusic 0.3.0 and later offer this version in Settings › About.

### Fixed

- On Linux, signing in to YouTube Music no longer closes MixMusic as the
  sign-in window opens (#51). MixMusic now asks whether WebKitGTK is
  there on the thread that opens windows, where WebKit has to start.

## [0.5.3] - 2026-10-23

Signed in to Spotify, MixMusic no longer flickers between signed in and
signed out, and when Spotify refuses the sign-in, MixMusic says why
rather than that you signed in. On Linux, MixMusic no longer damages its
own memory as its first window opens. Signing in to YouTube Music can
still close MixMusic on Linux (#51): that fix is planned for 0.5.4.
MixMusic 0.3.0 and later offer this version in Settings › About.

### Fixed

- After signing in to Spotify, the library, Settings › Accounts and
  Settings › Playback no longer flicker between signed in and signed out
  (#50). When Spotify refuses the sign-in, Settings › Accounts now says
  why and offers to sign out, and MixMusic asks Spotify again a minute
  later, not several times a second.
- A Spotify sign-in that Spotify refuses, as it refuses every app whose
  owner has no Premium, no longer says "Signed in to Spotify": it tells
  Spotify's reason instead.
- On Linux, on computers with a GPU, MixMusic no longer damages its own
  memory as its first window opens, which could crash it later: it now
  uses MyGo 0.6.2.

## [0.5.2] - 2026-10-22

The arrow keys seek and change the volume with nothing to set first. On
Windows the music no longer stutters while a game or a browser keeps the
computer busy, and on Linux closing the mini player no longer crashes
MixMusic. MixMusic 0.3.0 and later offer this version in Settings › About.

### Changed

- The arrow keys seek and change the volume with no shortcut to set first:
  left and right go back and forward 10 seconds, up and down turn the
  volume up and down. They took Shift and Ctrl (⌘ on macOS) before, and a
  shortcut you set in Settings › Shortcuts stays yours. The search box
  keeps the arrows for its cursor, and sliders, lists and the stats page's
  chart keep them while they have the focus. Up and down no longer scroll
  a page line by line: the mouse wheel, Page Up and Page Down, and Home
  and End do. The mini player takes no shortcuts (#38).

### Fixed

- On Linux, closing the mini player, with its close button or with Open
  MixMusic, no longer crashes MixMusic (#37).
- The settings icon in the sidebar is a gear again, centred and as large as
  the icons beside it, where it looked like a light bulb (#40).
- On Windows, the music no longer stutters while a game or a browser keeps
  the computer busy: MixMusic's audio now goes ahead of other programs, as
  the audio of browsers and other players does.

## [0.5.1] - 2026-10-22

A track no longer fails now and then with "YouTube refused to send its
audio", only to play when you try it again, and a copy built on a Mac for
its own processor installs new versions again. MixMusic 0.3.0 and later
offer this version in Settings › About.

### Fixed

- A track no longer fails now and then with "YouTube refused to send its
  audio". YouTube sends a track's audio only a few seconds after it gave
  out its address, and MixMusic asked at once, then again at a new
  address, refused as well. It now waits until the moment yt-dlp tells, so
  a track MixMusic hadn't looked up ahead starts a few seconds later
  instead.
- A copy of MixMusic built on a Mac for that Mac's processor, as
  `bun run build` makes it there, installs new versions again: it asked for
  an update file that releases did not publish, and failed with a 404.
  Releases publish it now, and check before they publish that every app
  finds its update (#33, #34).

## [0.5.0] - 2026-10-22

MixMusic draws its own interface, and plays through an audio engine of its
own, without the webview it ran in: it takes far less memory. Spotify plays
in MixMusic itself on Windows, with no Spotify app open, plugins move to a
new API, and a new version asks to install in a pop-up. Spotify Connect no
longer shows a track paused while your device plays it, and goes on to the
next track.

Four new themes join the others, a search says how many times each song
played, and on Windows the media flyout and the taskbar's thumbnail control
what plays. MixMusic 0.3.0 and later offer this version in Settings ›
About. The queue of 0.4.0, which its webview kept, doesn't come along: the
player starts on an empty queue once, and your library, mixes and settings
stay.

### Added

- MixMusic draws its interface itself, on the GPU: every page, the player
  bar, the queue and lyrics, the mini player and the command palette, in
  every theme. The built-in themes' effects come along: Mix's tile
  textures, Aurora's drifting lights and frosted glass, Sakura's petals,
  and Terminal's scanlines, glow and phosphor-green artwork (#4).
- MixMusic's own audio engine: it decodes the audio of YouTube Music itself
  and plays it on your audio device, with the sound the webview gave it:
  the equalizer, crossfades, volume levelling and the visualizer work as
  before. It shows what plays in the system's media controls, which the
  media keys and headphones reach as before: Windows' media flyout, with
  the artwork, the buttons and the seek bar, Now Playing on macOS, and the
  desktop's player controls on Linux.
- On Windows, the thumbnail of MixMusic's taskbar button has previous, play
  or pause, and next buttons, as other music players have (#13).
- Spotify plays in MixMusic itself, on Windows: choose in mixmusic, from
  spotify in Settings › Playback. MixMusic plays at Spotify's own quality,
  with no Spotify app open, and shows among your Spotify devices while it
  plays. It needs Spotify Premium; new installs on Windows play Spotify
  tracks this way. Sign in to Spotify again once, for Spotify to let
  MixMusic play.
- Plugins of a new API, 2: each runs in a JavaScript engine of MixMusic's
  own, and adds pages, widgets, sidebar items, track actions and commands,
  made of elements MixMusic draws in your theme and of canvases the plugin
  draws on. The community's sleep timer and spectrum are updated for it,
  and docs/plugins.md tells how to move a plugin.
- Settings › About shows the licenses of the code and the faces MixMusic
  includes: libopus, Chromium, Selawik and Quicksand.
- A new version asks to install in a pop-up, unless MixMusic installs
  updates without asking. Later asks again at the next start, skip this
  version at the next version. Copies that cannot replace themselves, the
  .deb, the .rpm and the AppImage, offer the new version's download there.
- Copy link in the menus of albums, playlists and artists, on their pages
  and their tiles, as tracks have it.
- Four new themes. Greenroom: charcoal and one jade light, its ground
  washed down from the top in the colour of the cover playing (#8).
  Glasshouse: white glass and a rose-red accent, the cover's colours
  glowing under translucent panels (#7). Anime: summer-sky blues and cel
  colours, speed lines that burst as each track starts, and manga
  screentone (#7). Mochi: kawaii candy pastels in a round face, with
  sparkles drifting up (#7).
- A search's songs say how many times each played on YouTube Music, or
  how many views a video has: "1.2B plays", "905M views", so that the
  original stands out of the covers. Spotify gives apps no such count.

### Changed

- Themes set the colors, fonts and corners MixMusic draws everything with.
  Themes made for the web interface still load, without their stylesheet
  and options, which nothing draws now. The labels of the sidebar take a
  theme's own face too, as Terminal's monospaced one.
- On Linux, MixMusic plays through PulseAudio, which PipeWire serves too,
  or ALSA, and no longer needs GStreamer; WebKitGTK is needed only to sign
  in to YouTube Music.
- MixMusic looks for a new version at start and every few hours, no longer
  once a day. Copies that cannot replace themselves look too, and Settings ›
  About turns that off.
- A Spotify track that plays from YouTube Music instead says why once, not
  at every track.
- On Windows and Linux, the main window has the system's own title bar and
  buttons, as asked in #6. On macOS, the traffic lights stay over the
  window.

### Removed

- The web interface. MixMusic opens a webview only for Google's sign-in to
  YouTube Music and, on Windows, for Spotify's player.
- Your own CSS, in Settings › Appearance.
- Plugins made for the web interface: they no longer run, as Settings ›
  Plugins says, until their authors move them to the new API.
- A track YouTube serves without Opus audio, which is rare, no longer plays:
  the webview played those.

### Fixed

- MixMusic no longer opens on a black window for up to 40 seconds when
  Spotify or YouTube Music is slow to answer: it shows at once, and the
  names of your accounts follow.
- On Spotify Connect, a track no longer shows paused while your device plays
  it: MixMusic took Spotify's late reports, and the copy of a track Spotify
  plays in your country, for someone else playing on the device.
- On Spotify Connect, the next track plays when one ends, where MixMusic
  waited on the track that ended.
- A Spotify device that takes a track but waits, paused, is told once more to
  play it.
- Skipping a track in its last seconds on Spotify Connect no longer skips the
  next one too.
- Moving through a Spotify track before it plays, as after a restart, no
  longer starts it on your device behind MixMusic's back.
- The title bar and its buttons are light or dark as the theme you wear
  is: on a light theme while the computer was dark, the buttons showed
  white on white.
- On a screen smaller than the window, as a laptop's, the window opens
  fitting it, where its title bar and buttons were above the screen, out
  of reach (#6). A window the last run left there comes back within it.
- A link you copy is no longer MixMusic's own address, as
  mygo://localhost/…, which opens nothing: Copy link copies the link on
  YouTube Music or Spotify, which anyone can open (#15).
- Choosing another track stops the one that plays at once. It played on
  under the new track's title, artwork and lyrics while MixMusic found the
  new track's audio, which can take a while on macOS, and for good when
  that failed.
- When Spotify refuses the account after you sign in, Settings › Accounts
  says why and what to do, where it showed only "spotify: HTTP 403": the
  account isn't one of your app's users, to add under Users Management in
  its settings, or the app's owner has no Spotify Premium, which Spotify
  asks of apps in development mode since February 2026. The steps to
  connect Spotify ask for both beforehand, and a refused sign-in can be
  signed out of.
- The visualizer of the now playing page, and the analyser plugins read,
  as the spectrum, move for Spotify tracks too, on Windows: MixMusic hears
  Spotify where it plays on this computer, in MixMusic or in the Spotify
  app on Spotify Connect, through Windows' process loopback, which hears
  one app's audio and never records it. Where MixMusic can't hear the
  track, as on another device, the visualizer shows nothing, not a flat
  line.

## [0.4.0] - 2026-10-20

MixMusic tells you when a yt-dlp you installed yourself is out of date,
how to update it, and offers its own, which stays up to date. MixMusic
0.3.0 and later offer this version in Settings › About.

### Added

- When the yt-dlp MixMusic runs is your own, as from winget, Homebrew or
  pip, and older than the latest release, Home and Settings › Playback say
  so. They give the command that updates it, when its folder tells how you
  installed it, and offer MixMusic's own yt-dlp, which stays up to date.
  A report of a problem says when yt-dlp is out of date.

## [0.3.2] - 2026-10-20

A track whose audio fails to load says why, and stops the queue only when
three in a row cannot play. MixMusic 0.3.0 and 0.3.1 offer this version in
Settings › About.

### Fixed

- A track whose audio fails to load says why, such as YouTube refusing it
  or its servers out of reach, instead of "the audio format is not
  supported", and the log keeps the reason. It counts as one track that
  could not play, not two or three, so the queue no longer stops at the
  first.

## [0.3.1] - 2026-10-20

Discord shows what you play on Windows, which it never did. MixMusic 0.3.0
offers this version in Settings › About.

### Fixed

- On Windows, your Discord profile shows what plays once you turn it on and
  paste your application ID: MixMusic looked for Discord in the wrong place,
  found it not running, and said nothing.

## [0.3.0] - 2026-10-20

Playlists of your own and a brand channel on YouTube Music, an About page
that shows the newest version and installs it, and downloads for more
systems. Signing in to YouTube Music no longer freezes MixMusic, and the
name of every page fits the window, with all of its sections. Copies of
0.2.0 from an installer or install.sh offer to update to this version;
install the .deb by hand.

### Added

- Make playlists on YouTube Music, from the library or from a track's or a
  list's menu, add tracks of either service to them, and take tracks out of
  your own playlists.
- Use YouTube Music as a brand channel of your account, with its own home
  feed, library and likes, chosen in Settings › Accounts.
- What you play joins your YouTube Music history, which its home and its
  radios are made from; a switch in Settings › Accounts turns it off.
- A log of what goes wrong, without your sign-ins, and the report of a
  crash, which the next start offers to send. Settings › About opens the
  logs folder, and reports a problem with the version and setup filled in.
- Going back or forward returns to where the page was scrolled.
- Downloads for more systems: a portable zip for Windows, an RPM package
  for Fedora and openSUSE, and an AppImage for any Linux.
- Settings › About shows the newest version. A copy that updates itself
  installs it there, with its progress, and restarts into it; the `.deb`,
  the `.rpm` and the AppImage download their own file of it.

### Changed

- The name of a page and its sections shrink to fit the window, and a long
  name, such as a search, an album or a playlist, wraps, instead of running
  off the edge: every section of Settings shows, About included. Tiles,
  empty pages and settings follow the room the side panels leave too.
- Tracks play signed out also when you signed in to YouTube Music: with your
  session, yt-dlp found fewer streams, often none it could play. No file
  keeps a copy of the session for it any more, and the audio quality of
  your account no longer applies.
- MixMusic updates the yt-dlp it installed, once a day.
- Discord's member list shows the artists you listen to, instead of
  MixMusic.
- The Debian package installs the libraries of the tray icon and of the
  keyring.

### Fixed

- Signing in to YouTube Music no longer leaves a blank window and freezes
  MixMusic, its search included, once Google sends the window back to
  YouTube Music, which it does at once when it remembers you.
- Home shows all of its shelves, not only those YouTube sends first.
- Your saved playlists, albums and artists of YouTube Music show in full,
  not only their first page.
- Opened from the Finder, the Dock or a desktop menu, MixMusic finds yt-dlp
  and Deno, Node.js or Bun installed with Homebrew, or by Deno's and Bun's
  own installers.
- When YouTube Music ends your session, MixMusic says so and asks you to
  sign in again, instead of showing an empty library.
- A track that cannot play says why in plain words, such as being not
  available in your country, instead of yt-dlp's message.
- The radio skips other uploads, and the music videos, of songs already in
  the queue.
- After three tracks in a row that cannot play, the queue stops instead of
  skipping through all of it, and a last track that cannot play still
  starts the radio.
- On macOS, a click on the Dock icon shows the window closed to the tray.
- Discord shows what plays from its Canary build, and from Vesktop,
  installed with Flatpak.

## [0.2.0] - 2026-10-19

Your own YouTube Music in MixMusic, an explore page, Last.fm and Discord, and
updates that install themselves. MixMusic 0.1.0 cannot update itself, so
install this version by hand once; the versions after it come to the app.

### Added

- Sign in to YouTube Music, in a window showing Google's own page: your home
  feed and library (liked songs, playlists, albums, artists), likes that reach
  YouTube Music, and the audio quality of your account.
- Save albums and playlists, and follow artists, on their service, YouTube
  Music or Spotify, from their pages.
- Explore: new albums and singles, trending songs, the charts, and moods and
  genres, each with its playlists.
- Related songs, playlists and artists at the foot of the now playing page.
- Scrobbling to Last.fm, with your own API account; plays sent later when
  offline, and a switch to pause it.
- What plays on your Discord profile, with the artwork and the time left.
- Updates: MixMusic checks the latest release once a day and installs it when
  you agree, or by itself if you choose.
- Change the keys of every command, plugins' too, in Settings › Shortcuts.
- Follow the computer's light or dark mode with a theme for each.
- The minutes listened day by day on the stats page, also as a table.
- Search inside a playlist, a mix or the liked songs, and sort them by title,
  artist or length.
- Show synced lyrics as plain text.

### Changed

- Home shows YouTube Music's home feed; what was its explore page moved to
  Explore.

## [0.1.0] - 2026-10-19

The first release of **MixMusic**: Spotify and YouTube Music in one desktop
player, for macOS, Windows and Linux.

### Added

- Search YouTube Music and Spotify at once, with search suggestions.
- One queue for tracks of both services: shuffle, repeat, crossfade, and a
  radio carrying on when the queue runs out.
- Spotify tracks on Spotify Connect, or from their recording on YouTube
  Music, without Premium.
- Home shelves of both services and of your own listening; album, artist and
  playlist pages.
- Liked songs and mixes holding tracks of every service; import of a
  playlist into a mix; history and listening stats.
- Synced lyrics, a visualizer, a 10-band equalizer, volume levelling and an
  offline cache of played tracks.
- The tray, media keys, notifications, a mini player, start at login and
  `mixmusic://` links.
- Six themes, theme options, an accent following the artwork, your own CSS,
  and plugins with a typed API.
- A marketplace of community themes and plugins.
- Apps for macOS 12 and later (Apple silicon and Intel), Windows 10 and 11,
  and Linux, on x64 and Arm64.

[Unreleased]: https://github.com/fzn0x/mixmusic/compare/v0.5.10...HEAD
[0.5.10]: https://github.com/fzn0x/mixmusic/compare/v0.5.9...v0.5.10
[0.5.9]: https://github.com/fzn0x/mixmusic/compare/v0.5.8...v0.5.9
[0.5.8]: https://github.com/fzn0x/mixmusic/compare/v0.5.7...v0.5.8
[0.5.7]: https://github.com/fzn0x/mixmusic/compare/v0.5.6...v0.5.7
[0.5.6]: https://github.com/fzn0x/mixmusic/compare/v0.5.5...v0.5.6
[0.5.5]: https://github.com/fzn0x/mixmusic/compare/v0.5.4...v0.5.5
[0.5.4]: https://github.com/fzn0x/mixmusic/compare/v0.5.3...v0.5.4
[0.5.3]: https://github.com/fzn0x/mixmusic/compare/v0.5.2...v0.5.3
[0.5.2]: https://github.com/fzn0x/mixmusic/compare/v0.5.1...v0.5.2
[0.5.1]: https://github.com/fzn0x/mixmusic/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/fzn0x/mixmusic/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/fzn0x/mixmusic/compare/v0.3.2...v0.4.0
[0.3.2]: https://github.com/fzn0x/mixmusic/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/fzn0x/mixmusic/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/fzn0x/mixmusic/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/fzn0x/mixmusic/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/fzn0x/mixmusic/releases/tag/v0.1.0

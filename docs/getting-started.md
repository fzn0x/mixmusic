# Getting started
<!-- docs: group=start order=10 -->

MixMusic is a free desktop player for Spotify and YouTube Music on macOS, Windows and Linux. This page installs it and shows you around.

MixMusic puts both services in one window: one search looks through YouTube Music and Spotify at once, one queue plays tracks of either, and your liked songs and mixes hold tracks of both. YouTube Music plays as soon as you open it, with no account. Spotify, and your own YouTube Music library, are there once you sign in. MixMusic has no account, server, analytics or telemetry of its own, and it is open source, under the MIT license or the GPL 3.0.

## Install

Download the file for your system from the
[latest release](https://github.com/fzn0x/mixmusic/releases/latest):

- **macOS 12 and later**, on Apple silicon or Intel: `mixmusic-<version>-macos-universal.dmg`. Open it and move MixMusic into your Applications folder. Or let [Homebrew](https://brew.sh) install it, and `brew upgrade` update it:

  ```sh
  brew install --cask fzn0x/tap/mixmusic
  ```

- **Windows 10 and 11**, on x64 or Arm: the installer, `mixmusic-<version>-windows-<arch>-setup.exe`, or the portable zip, `mixmusic-<version>-windows-<arch>-portable.zip`, which you unpack anywhere and start with `MixMusic.exe`. When MixMusic runs from the install folder, the installer, and the uninstaller, asks "MixMusic is running. Close it to continue?": **OK** asks its windows to close and, after 10 seconds, stops what still runs, as a copy hidden in the tray; **Cancel** stops the install. If one still won't stop, it says "MixMusic is still running. Quit it, then try again." and quits. A silent install, with `/S`, closes MixMusic without asking. MixMusic's own updates don't run the installer: they replace its files while it runs, and the new version starts when MixMusic restarts.
- **Linux**, on x64 or Arm64: the `.deb` for Debian, Ubuntu and their derivatives, the `.rpm` for Fedora and openSUSE, the AppImage for any Linux (make it executable with `chmod +x`, then open it), the Flatpak for any Linux, or the `.tar.gz` archive with its `install.sh`, which installs MixMusic in your home folder.

`<arch>` is `amd64` for x64 and `arm64` for Arm. Each release lists `SHA256SUMS` to check the downloads with, and files you don't need, which installed copies update from. The [README](../README.md#install) has the full table of downloads.

The installers, the portable zip and the Linux archive update themselves: MixMusic looks for a new version at start and every few hours, and asks before it installs one, unless you let it install updates by itself. The `.deb`, the `.rpm`, the AppImage and the Flatpak can't replace their own files, so MixMusic tells you when a new version is out and offers its file: the button opens the file's address on the release in your browser, and you install it as you did the first. For a Flatpak that is the bundle of its own architecture, so an Arm64 Flatpak is offered the Arm64 one. [Settings › About](settings.md#about) shows your version and the newest one.

The Windows installer, macOS's list of languages for the app (System Settings › General › Language & Region › Applications) and the Linux desktop entry come in 46 of MixMusic's languages, as German, Arabic, Japanese and Brazilian Portuguese; the installer picks the one Windows shows. On macOS, the prompts that ask to find devices on your local network (for [casting](casting.md) and the remote control) and to hear Spotify's audio (for the visualizer) are in them too. Twenty languages, Amharic, Azerbaijani, Bengali, Filipino, Gujarati, Hausa, Hindi, Kazakh, Khmer, Kannada, Malayalam, Marathi, Burmese, Nepali, Punjabi, Sinhala, Swahili, Tamil, Telugu and Urdu, are in the app but not there: the installer, macOS's list and the desktop entry name MixMusic in English.

## The first launch

- **macOS.** The app isn't notarized by Apple yet, so the first time you open it, macOS says it can't check it. Open it anyway from System Settings › Privacy & Security › Open Anyway, or run this once in Terminal:

  ```sh
  xattr -dr com.apple.quarantine /Applications/MixMusic.app
  ```

- **Windows.** The installer isn't signed yet, so SmartScreen may warn about it: choose More info, then Run anyway. Signing in to YouTube Music and playing Spotify in MixMusic itself need the WebView2 Runtime, which Windows 11 includes. On Windows 10, install it from [Microsoft](https://developer.microsoft.com/microsoft-edge/webview2/).
- **Linux.** MixMusic needs GTK 3, and PulseAudio (which PipeWire serves too) or ALSA. Signing in to YouTube Music needs WebKitGTK 4.1, the tray icon needs `libayatana-appindicator3`, and keeping your sign-ins in the system's keyring needs `libsecret`. The `.deb` and the `.rpm` install them and the Flatpak brings them; for the AppImage and the archive:

  ```sh
  # Debian and Ubuntu
  sudo apt install libgtk-3-0 libpulse0 libwebkit2gtk-4.1-0 libayatana-appindicator3-1 libsecret-1-0
  # Fedora
  sudo dnf install gtk3 pulseaudio-libs webkit2gtk4.1 libayatana-appindicator-gtk3 libsecret
  ```

  MixMusic carries no font for Chinese, Japanese and Korean letters, and on Linux a letter that no installed font has shows as blank space. The `.deb` and the `.rpm` ask for one (`fonts-noto-cjk`, or a font that provides `font(notosanscjkjp)`), and the AUR package lists `noto-fonts-cjk` as optional. `dpkg -i` skips what a package only recommends, and pacman only prints what is optional, so for those, the AppImage, the archive and the Flatpak, install a font yourself and restart MixMusic:

  ```sh
  # Debian and Ubuntu
  sudo apt install fonts-noto-cjk
  # Fedora
  sudo dnf install google-noto-sans-cjk-vf-fonts
  # Arch
  sudo pacman -S noto-fonts-cjk
  ```

  The Flatpak reads the fonts of your system too. `fc-list :lang=ja` prints the fonts you have for Japanese; it prints nothing when there is none.

- **yt-dlp, on every system.** MixMusic finds the audio of most YouTube Music tracks itself. For the rest, such as tracks made for children, it runs [yt-dlp](https://github.com/yt-dlp/yt-dlp). Until yt-dlp is there, Home says "Some tracks need yt-dlp", with a button that installs it; Settings › Playback › Tools installs it too, and MixMusic keeps the copy it installed up to date. yt-dlp needs a JavaScript runtime: [Deno](https://deno.com), Node.js, Bun or QuickJS, which MixMusic finds once you install one. The Flatpak has Node.js. [Where the audio streams come from](streams.md#when-yt-dlp-is-needed) tells which tracks need it.

MixMusic opens on Home in the [Blend layout](look-and-feel.md#layouts). Your settings and library stay in a [data folder](data-and-privacy.md#the-data-folder) of your own: `%AppData%\MixMusic` on Windows, `~/Library/Application Support/MixMusic` on macOS and `~/.config/MixMusic` on Linux.

### The Flatpak

The Flatpak runs MixMusic in a sandbox, which sees the network, the sound server, the display and the GPU, and none of your home folder. It keeps its settings and library in `~/.var/app/io.github.fzn0x.mixmusic`, apart from a `.deb`, `.rpm`, AppImage or archive copy, and both can run at once. The [README](../README.md#the-flatpak) tells how to bring another copy's settings and library over.

The files are `mixmusic-<version>-linux-amd64.flatpak` and `mixmusic-<version>-linux-arm64.flatpak`, the second for the Raspberry Pi 5, Asahi Linux and Snapdragon laptops. Open the file with your software center, or run `flatpak install ./mixmusic-<version>-linux-<arch>.flatpak`; the first time, Flatpak fetches the GNOME runtime from Flathub. GTK and WebKitGTK come with that runtime, and the Flatpak brings the tray's library and Node.js for yt-dlp itself.

MixMusic isn't on Flathub for now: install the release's `.flatpak`. MixMusic's source also builds as Flathub builds apps: CI checks that build when a change touches it, and once a week. A copy built so calls itself Flathub in Settings › About, tells you a new version comes through your software center, offers no `.flatpak` to download, and runs the yt-dlp that comes with it, unless you chose another, rather than downloading one.

In the Flatpak, start at login goes through your desktop, which may ask you first, and a yt-dlp of your own must be in `~/.var/app/io.github.fzn0x.mixmusic`, as the sandbox sees no other folder. A Discord app that isn't a Flatpak shows what plays only if it was running when MixMusic started.

## The window

![MixMusic's window on Home, in its default look: the sidebar on the left, with its pages and Your mixes under them, the top bar with the back and forward arrows, Refresh and the search box, Home's Jump back in, Add Spotify, Liked songs and Your mixes tiles over its Recently played shelf, and the player bar along the foot](shots/home.webp)

The window has the system's own title bar and buttons on Windows and Linux. On macOS the traffic lights sit over the window. The title bar is light or dark as the [theme](look-and-feel.md#themes) is. The window opens at 1280 by 820, and no smaller than 940 by 600, and comes back at the size and place the last run left it, kept in `window-state.json` in the data folder. On a screen smaller than that, it opens fitting the screen; a window the last run left out of reach, with its title bar off the screen, is moved back within it, and a place on a screen that is no longer connected isn't used. A maximized or full-screen window stays so.

### The sidebar

The sidebar on the left goes to MixMusic's pages: Home, Search, Explore, Library, Stats, [Marketplace](marketplace.md) and [Settings](settings.md), then the pages plugins add. Under them, Your mixes lists your liked songs, your downloads once you have some, and each [mix](library.md#mixes) you made, with how many tracks each holds; its + button makes a new mix. At its foot, a card shows what plays, and opens the now playing page; with nothing loaded it says "Nothing playing yet". The button beside MixMusic's name folds the sidebar to its icons, as Ctrl+B (⌘B on macOS) does; in a narrow window it folds by itself. On macOS, the window's buttons take the left of that top row, with the fold button, and the logo and name go under them.

While a track plays and another is up next, the card's cover turns over after about 5 seconds to show "Up next" and the next track's title, then turns back after as long. It doesn't turn while paused, while the next track is the same one, in the folded sidebar, or while the system asks for [less motion](look-and-feel.md#reduced-motion). In a window too short to show the card whole above the player bar, the card goes, in every layout; in Blend, Expressive and Crate the sidebar's pages first sit a little closer to keep its room.

- **Home** has your own tiles first: Jump back in plays the track you played last, then Liked songs and Your mixes, and Add Spotify while you aren't signed in to it. Then come your shelves, such as Recently played, [New from artists you follow](library.md#new-releases), and those of YouTube Music and Spotify. The pivots at the top keep Everything, only Yours, only Spotify's or only YouTube Music's, and [Edit home](home.md#arrange-home) moves, hides and adds its shelves.
- **Explore** has YouTube Music's new albums, singles and trending songs, its charts, and its moods and genres, each with playlists of its own.
- **Library** has your liked songs, mixes, podcasts, downloads and history, and your library on each service you signed in to: see [Your library](library.md).
- **Stats** shows what you listened to, day by day: see [Stats](library.md#stats).

### The top bar

The top bar holds the back and forward arrows, which return to a page as it was: scrolled where it was, with what you typed there and the focus where it was. Alt+Left and Alt+Right (⌘[ and ⌘] on macOS) and the back and forward buttons of a mouse do the same. MixMusic keeps the last 100 pages, and the state of the 10 nearest the one shown. Beside them are [Refresh this page](settings.md#refresh-a-page), which asks the services again for what the page shows (F5, or ⇧⌘R on macOS). [Pulling a page down](settings.md#pull-to-refresh) past its top refreshes it too. Then comes the search box, "Search Spotify and YouTube Music" (Ctrl+F, ⌘F on macOS), which suggests searches, songs, albums, artists and playlists as you type: see [Playing music](playing-music.md#search). Once you scroll down a playlist, an album, an artist or your liked songs, the top bar shows its cover and name, with Play, Shuffle and Download. The button at the right of the top bar, which shows its key, opens the [command palette](settings.md#the-command-palette).

### The player bar

The player bar runs along the foot of the window. In the Blend and Expressive layouts it floats over it; Crate and Tiles keep it at the foot.

- **On the left,** what plays: its cover, which opens the now playing page (and its menu, on a right click), its title, which opens its album, and its artists. The line under the title says "via YouTube Music" for a Spotify track that plays from YouTube Music, "on Spotify" for one on Spotify Connect (see [How Spotify tracks play](playing-music.md#how-spotify-tracks-play)), the [speed](playing-music.md#speed) when it isn't 1×, and "waiting for the network" when the [network dropped](playing-music.md#without-the-network). In a wide window, the track that plays next shows under it. The heart [likes](library.md#liked-songs) the track and the thumbs-down [dislikes](playing-music.md#dislikes-and-skipping) it.
- **In the middle,** Shuffle, Previous, Play or Pause, Next and Repeat, and the seek bar with the time and the length.
- **On the right,** the 5.1 button for a track that has a 5.1 surround mix, Lyrics, Queue, the [cast button](casting.md) while a Chromecast, a Google Home or Nest speaker, a TV with Chromecast built in, a DLNA renderer or an FCast receiver is on your network, the Spotify Connect devices when Spotify tracks play on Spotify Connect, Speed, the [output device](playing-music.md#the-output-device) where the window has room for it, the volume, and the [mini player](desktop.md#the-mini-player).

### Up next, lyrics and now playing

- **Up next,** the queue, opens at the right of the window from the Queue button, or with Ctrl+J (⌘J): the track playing, then what plays after it. See [The queue and Up next](playing-music.md#the-queue-and-up-next).
- **The lyrics panel** opens in the same place from the Lyrics button, or with Ctrl+Shift+L (⇧⌘L): see [Lyrics](lyrics.md). For a podcast episode, the same button shows [its details](podcasts.md#episode-details-and-chapters).
- **The now playing page** gives the track the whole window: open it with its cover in the player bar, the card in the sidebar, or Alt+4 (⌥4). It shows the cover with a visualizer over its foot (or the [picture of a music video](playing-music.md#the-picture-of-music-videos), if you turn that on), the artists and the album, the seek bar, play and next, like and dislike, the lyrics, the [widgets of plugins](marketplace.md#what-a-plugin-can-do), and at its foot the songs, playlists and artists YouTube Music relates to the track.

![The now playing page of アイドル by YOASOBI: its cover with the visualizer over its foot, the seek bar, play, next, like and dislike, and its synced lyrics under the Synced, Plain text, Translate and Romaji switches, with Up next open at the right of the window](shots/now-playing.webp)

### The command palette and shortcuts

The command palette, Ctrl+K (⌘K on macOS), finds every command by its name, plugins' commands too, with its key beside it. Type anything and it also offers to search for it, and to search the settings for it. These keys work from the start; [Settings › Shortcuts](settings.md#change-a-shortcut) changes any of them, and gives keys to the commands that have none:

| Keys | What they do |
|---|---|
| Space | Play or pause |
| ← and → | Back or forward 10 seconds |
| ↑ and ↓ | Volume up or down |
| M | Mute |
| Ctrl+← and Ctrl+→ (⌘ on macOS) | Previous and next track |
| Ctrl+S, Ctrl+R | Shuffle, and Repeat |
| Ctrl+L, Ctrl+D | Like or dislike what plays |
| Ctrl+F | Search |
| Ctrl+J, Ctrl+Shift+L | Show or hide the queue, or the lyrics |
| Alt+1 to Alt+5 (⌥ on macOS) | Home, Library, Liked songs, Now playing, Explore |
| Ctrl+M | Mini player |
| Ctrl+B | Fold or unfold the sidebar |
| Ctrl+, | Settings |
| Ctrl+= and Ctrl+-, Ctrl+0 | Zoom in and out, back to 100% |
| F5 (⇧⌘R on macOS) | Refresh this page |

On macOS, ⌘ takes the place of Ctrl in every one of them. The [media keys](desktop.md) of your keyboard and headphones work too, and Settings › Shortcuts can set [keys that control the music from any app](desktop.md#global-shortcuts).

## What works without an account

With no account at all, MixMusic:

- searches YouTube Music and plays it, with its home shelves, Explore, artists, albums, playlists, [podcasts](podcasts.md) and [radios](playing-music.md#autoplay-and-radios);
- keeps your liked songs, dislikes, mixes, history and stats on your computer, in [your library](library.md);
- shows [lyrics](lyrics.md), [downloads tracks](library.md#downloads) to play without the network, and keeps the tracks you played in its [offline cache](library.md#the-offline-cache);
- shows what plays on your [Discord profile](desktop.md#discord), from the Discord app on your computer, with nothing to set up (Settings › Desktop turns it off).

Every account is optional, and you add them in Settings › Accounts: [Accounts](accounts.md) tells each step.

### What each account adds

- **YouTube Music.** Sign in with [Google's own page](accounts.md#the-sign-in-window) in a window MixMusic opens, or [import the session of a browser](accounts.md#import-from-a-browser) you signed in with. You get your home feed, your library (playlists, albums, artists you follow and liked songs), likes and dislikes that reach YouTube Music, saving albums and playlists, making and deleting playlists, and a [brand channel](accounts.md#several-google-accounts-and-brand-channels) of your account if you have one. What you play joins your YouTube Music history, so its home and radios learn from it, and podcast episodes [go on where you left them](podcasts.md#with-youtube-music) on your phone. Tracks play the same signed in or out, unless you turn on [Play audio as your account](playing-music.md#play-audio-as-your-account), which asks as you first.
- **Spotify.** Spotify lets an app serve up to five accounts, and only while the app's owner has Premium, so you sign in with a Spotify app of your own: [Spotify](spotify.md) walks you through it. You can then search Spotify too, see your Spotify library, and like, save and follow there. Spotify tracks play in MixMusic itself (with Premium, on Windows and macOS), on Spotify Connect (with Premium), or from their recording on YouTube Music, which needs no Premium.
- **Last.fm.** Paste the API key and shared secret of a Last.fm API account of yours, and approve MixMusic in your browser: what you listen to joins your Last.fm profile once half of a track, or four minutes of it, played. Scrobbles made offline are sent once you're back online.
- **ListenBrainz.** Paste the user token of your free ListenBrainz account, or one of a server of your own, such as Maloja: what you listen to joins your listens there by the same rules, beside Last.fm or without it.
- **Libre.fm.** Connect, and approve MixMusic on Libre.fm's page in your browser, or on another GNU FM server's: what you listen to joins your profile there by the same rules, with no API account to create.

## Next

- [Playing music](playing-music.md): search, the queue, radios, the sound and speed.
- [Your library](library.md): liked songs, mixes, downloads, history and stats.
- [Lyrics](lyrics.md): synced lyrics, translations and Latin letters.
- [Podcasts](podcasts.md): shows, episodes and where you left them.
- [Troubleshooting](troubleshooting.md), when something doesn't work.

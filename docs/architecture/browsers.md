# Signing in through a browser

Settings › Accounts › YouTube Music signs in through a browser of the
user's (#472), where Google refuses the sign-in window: `internal/browser`
finds the browsers installed, and opens one with a profile of its own, made
for the sign-in alone, at YouTube Music's sign-in. Once the profile holds
YouTube's signing cookies, it takes them, closes the browser and deletes
the profile, and `internal/app` (`youtube_browser.go`) signs in with them
as every way does (`youtubeSignIn`), keeping those a request to
music.youtube.com sends. The settings' row is `accounts_browser.go`.

## Finding the browsers

`browser.Find` looks for Chrome, Edge, Brave, Chromium, Vivaldi, Helium,
Firefox and Zen, in that order, through a `system` that tests fake, so
that each system's search runs on every system:

- **Windows**: where each installer puts the program, under Program Files,
  Program Files (x86) and the user's LOCALAPPDATA, then the programs the
  registry names, under `Clients\StartMenuInternet` (for the default
  browser's choice) and `App Paths`, of the machine and of the user.
  Chrome, Chromium and Helium all name theirs `chrome.exe`: its folder
  tells them apart.
- **macOS**: the apps in /Applications and ~/Applications.
- **Linux**: the browsers' commands on PATH, then the folders their
  packages install to. A command of /snap/bin, or a script that starts
  one, as Ubuntu's `firefox` is, is a snap, whose sandbox doesn't see the
  temporary folder: its profile goes in `~/snap/<name>/common`. MixMusic's
  own Flatpak sees no program of the system, and finds none.

## The profile

Each sign-in makes a folder `mixmusic-signin-*` in the temporary folder,
readable by the user alone, and starts the browser with it, apart from the
user's own browser if it runs. The browser closes and the folder goes once
the user signed in, closed the browser, cancelled, or after 10 minutes; on
Windows, a process of the browser may hold a file a moment longer, so the
deletion tries again for a few seconds. If MixMusic quits meanwhile,
`browser.KillAll` ends the browser, and the next start deletes what was
left (`RemoveLeftovers`, from `openYouTube`). On Windows, the browser's
processes are followed by their parents and the times they started, as
Windows doesn't end them with it; elsewhere it runs in a process group of
its own.

## Reading the cookies

- **Chromium.** The browser starts with `--remote-debugging-pipe`: the
  DevTools protocol on two pipes MixMusic gives it, as its file
  descriptors 3 and 4, or on Windows as handles it inherits, named by
  `--remote-debugging-io-pipes`. Nothing listens on the network. MixMusic
  talks to the browser itself, never to a page: `Target.getTargets` tells
  whether a window is open, as on macOS the browser runs on without one,
  and `Storage.getCookies` reads the cookies; `Browser.close` closes it.
  Chrome 136 and later allow this only for a profile other than the
  user's own, which this is. A MixMusic run as administrator has the
  browser keep its rights (`--do-not-de-elevate`) and its environment
  without `__COMPAT_LAYER`, as Chrome and Edge would otherwise start again
  through another process, which leaves the pipes behind.
- **Firefox.** The profile's `user.js` skips the first run's pages, and
  the offer to keep the password; `-wait-for-browser` keeps Firefox's first
  process on Windows until the browser it starts ends, and
  `MOZ_DISABLE_UPDATE_PROCESSING` keeps it from applying an update the
  user's Firefox downloaded, which would start it again apart from
  MixMusic. Firefox keeps the cookies that outlast it in the profile's
  `cookies.sqlite`, whose last changes may be in its write-ahead log:
  `sqlite.go` reads it, both files, as SQLite would, in Go, and only what
  it needs: the rows of a table, in UTF-8. The cookies of containers and
  those kept for one site embedded in another are left out. WebDriver BiDi
  would read them too, but a Firefox under remote control tells the pages
  so (`navigator.webdriver`), which a sign-in page may refuse.

The sign-in is done once the cookies hold YouTube's signing cookie, and
stopped changing from one read to the next, a second apart, as the site
sets more once it comes back from Google.

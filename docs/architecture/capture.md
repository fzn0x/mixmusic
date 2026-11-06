# Capture mode

`MIXMUSIC_CAPTURE=<dir>` screenshots pages of the main window, and
`MIXMUSIC_CAPTURE_QUIT=1` quits once they are saved:

```sh
MIXMUSIC_CAPTURE=shots MIXMUSIC_CAPTURE_QUIT=1 MIXMUSIC_CAPTURE_ROUTES="/|/search?q=jazz|/settings" MIXMUSIC_CAPTURE_SIZE=1440x900 ./MixMusic
```

`MIXMUSIC_CAPTURE_WAIT` sets how long each step gets (`5s`). Steps that
aren't pages act: `act:shot:<name>` saves the window as it is,
`act:mute` mutes the player, `act:play:<search>` plays what a search
finds, `act:resume` plays the queue kept in the data folder,
`act:wait:<duration>` waits, `act:palette` opens the command palette,
`act:refresh` refreshes the page, as F5 does,
`act:panel:<queue|lyrics|none>` shows a side panel, `act:scroll:<y|end>`
scrolls the page, `act:theme:<id>` wears a theme,
`act:colors:<off|pywal|matugen|path>` takes the colours from a file, as
Settings › Appearance does, `act:mini` opens the mini player, which saves
`mini.png`, `act:spotify` logs whether MixMusic's own Spotify device
could play, `act:title` logs the window's title, which the screenshots
leave out, and `act:memory` and `act:heap` log the Go heap and save a
profile of it. Nothing plays unless the player is muted,
or `MIXMUSIC_SILENT=1` has the audio engine play into no device, which
keeps the visualizer and the lyrics moving without a sound.
`MIXMUSIC_CAPTURE_COMMUNITY=<dir>` has the marketplace read the community
registry from a copy of `community/`, for add-ons not on GitHub yet. The
site's screenshots are made this way, from a throwaway profile, at
1536x900 (`MIXMUSIC_CAPTURE_SIZE=1552x909`: the size includes the
window's invisible borders). `go test . -run Render` with
`MIXMUSIC_RENDER=<dir>` draws the pages and themes without a window.

On Linux, [linux-app.py](../../.github/scripts/linux-app.py) starts a build
in capture mode, then again as a desktop does, and checks it from outside:
its window, its tray icon, the media keys, MPRIS, the zoom keys, and
settings.json and colour files edited while it runs. The Linux app
workflow runs it on the Nix build and on the release's archive, and keeps
the frames, the logs and the D-Bus answers. On a Linux machine with Xvfb,
xdotool, ffmpeg, PulseAudio and PyGObject:

```sh
xvfb-run -a -s "-screen 0 1600x1000x24" dbus-run-session -- python3 .github/scripts/linux-app.py ./mixmusic out
```

# Control from another program
<!-- docs: group=using order=66 -->

A Stream Deck plugin, a macro, a script of your own or an overlay can play,
pause, skip, seek, set the volume, shuffle, repeat, and like what plays in
MixMusic, over HTTP on this computer. It is off at first.

Keys and scripts that only play, pause and skip are simpler with
[MixMusic's flags](desktop.md#control-from-a-key-or-a-script); this page is
for the programs that need more, and for the ones that keep the state.

## Turn it on

Open Settings › Desktop. Under **Now playing for overlays**, turn on
**Let programs on this computer control MixMusic**. It shares that
server's **Overlay port**, 8284 at first, which the rows above choose. On,
the settings show the address a program sends its token to.

MixMusic listens on `127.0.0.1`, this computer's own address: nothing
leaves your computer, and no other computer reaches it.

## The token

Every request of `/control` carries a token, 256 random bits, in the
`Authorization` header:

```
Authorization: Bearer 4f3c…c9a1
```

The token and the port are in `control.json` in
[MixMusic's data folder](data-and-privacy.md#where-mixmusic-keeps-its-data),
which only you may read, so that a script finds both:

```json
{
  "port": 8284,
  "token": "4f3c…c9a1"
}
```

**new token**, beside **Token** in the settings, makes another one, which
cuts off every program that had the old one at once: the server stays on,
and `control.json` changes.

## Commands

Send a `POST` to one of these, with the token. A body is JSON, of 1 KiB at
most, and most commands need none:

| Command | What it does |
|---|---|
| `/control/toggle` | plays a paused track, pauses a playing one |
| `/control/play`, `/control/pause` | plays, pauses |
| `/control/next`, `/control/previous` | skips to the next track, goes back |
| `/control/mute` | mutes or unmutes |
| `/control/shuffle` | turns shuffle on or off |
| `/control/repeat` | goes from off to Repeat: all, to Repeat: one, and back to off; `{"repeat": "off"}`, `"all"` or `"one"` sets it instead |
| `/control/like`, `/control/dislike` | likes or dislikes the track that plays, or takes it back, as the media keys do |
| `/control/track` | plays the track of the queue its `id` names: `{"id": "yt:abc123"}` |
| `/control/seek` | goes to `{"position_seconds": 72.5}`, or moves by `{"by_seconds": -15}` |
| `/control/volume` | sets the volume to `{"percent": 40}`, from 0 to 100, or moves it by `{"by_percent": -5}` |

That is fifteen commands over thirteen addresses: seek and volume each take
a place, or a step from where the track is.

Each answers with the state below, after the command: `{"volume": 40, …}`.
A command that can't run answers with its reason, a `409` when nothing is
loaded or the track isn't in the queue, and a `503` while the player isn't
there:

```json
{"error": "nothing is loaded"}
```

Only `POST` reaches a command: a `GET` of one answers `405`.

## From a shell, or a Stream Deck

With the token `control.json` keeps, `curl` sends a command:

```sh
curl -X POST -H "Authorization: Bearer $(jq -r .token control.json)" http://127.0.0.1:8284/control/next
```

PowerShell sends the same, as **Copy a PowerShell example** in the settings
does:

```powershell
Invoke-RestMethod -Method Post -Headers @{Authorization="Bearer $(Get-Content control.json | ConvertFrom-Json).token"} http://127.0.0.1:8284/control/next
```

A Stream Deck key's built-in **Website** action sends only a `GET`, with no
header of your own, which this refuses: use a plugin that sends an HTTP
request, as the HTTP Request plugin of Elgato's Marketplace does, with the
`Authorization` header of the token.

## The state

`GET /control/state` answers with what [the overlay data](stream-overlay.md#make-your-own-overlay)
tells, and the player's own state beside it: the volume from 0 to 100, the
mute, the shuffle, the repeat, whether the track is liked, whether the
player can go next or back, and the next 40 tracks of the queue.

```json
{
  "active": true,
  "playing": true,
  "id": "yt:abc123",
  "title": "Song",
  "artists": ["Artist", "Another"],
  "artist": "Artist, Another",
  "album": "Album",
  "source": "youtube",
  "positionMs": 72000,
  "durationMs": 214000,
  "live": false,
  "speed": 1,
  "cover": "http://127.0.0.1:8284/cover?v=3",
  "volume": 40,
  "muted": false,
  "shuffle": false,
  "repeat": "off",
  "liked": false,
  "canNext": true,
  "canPrevious": false,
  "queue": [
    {"id": "yt:def456", "title": "Next Song", "artists": ["Artist"]}
  ]
}
```

`repeat` is `"off"`, `"all"` or `"one"`. `queue` is the next tracks, 40 at
most, each with its ID, title and artists.

## Events

`GET /control/events` streams the same state as [Server-Sent
Events](https://html.spec.whatwg.org/multipage/server-sent-events.html): at
once, then at each change of what plays, of the volume, of the mute, of the
shuffle, of the repeat, of the like, of the queue, and of the cover. Nothing
polls.

```js
const events = new EventSource("http://127.0.0.1:8284/control/events");
events.addEventListener("state", (e) => console.log(JSON.parse(e.data)));
```

A browser's `EventSource` sends no headers, so it can't carry the token:
stream with `curl -N`, or with a program of your own.

## Limits and guards

- **Only this computer:** a request that names a host other than
  `127.0.0.1`, `::1` or `localhost` is refused, as for the overlay's data.
- **No web pages:** a request that carries an `Origin` or a `Sec-Fetch-Site`
  header is refused with `403`, so a page on the web can't reach the server
  through your browser. The answers carry no CORS header.
- **The token:** a missing or another token answers `401`, and the token's
  check doesn't tell how far it matched.
- **The pace:** 20 commands at once, then about 20 a second, so that a
  volume dial keeps up; past that, a `429` with `Retry-After`.
- **The size:** a body of 1 KiB at most, and headers within 2 seconds.
- **Turning it off** closes the server, and ends every program's stream at
  once. The token is kept, so turning it back on needs no new one.

## What it promises

The page and every request are plain HTTP on this computer: any program
that runs as you can read `control.json` and control MixMusic until you
make a new token. Nothing goes over the network, and no other computer
reaches the port, as it listens on `127.0.0.1` alone. `control.json` keeps
a secret, which [a backup](data-and-privacy.md#back-up-and-restore) leaves out.

[Settings](settings.md#desktop) lists the other rows of Settings ›
Desktop.

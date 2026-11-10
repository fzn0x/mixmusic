# Overlays for streams
<!-- docs: group=using order=65 -->

MixMusic can tell a streaming program what plays, so that an overlay shows
the cover, the title and a progress bar without reading a window's title.
It is off at first.

## Turn it on

Open Settings › Desktop, and turn on **Serve what plays to overlays**, under
**Now playing for overlays**. Under it MixMusic shows:

- **Overlay port**, 8284 at first. Any port from 1024 to 65535 works, and
  a new one applies at once, while it is on. If another program already
  uses the port, the page says so in red: choose another.
- **Browser source address**, which is `http://127.0.0.1:8284/overlay` at
  first, and **Data address**, with **copy** beside each.

MixMusic listens on `127.0.0.1`, this computer's own address, so that no
other computer can reach it, and nothing leaves your computer. It takes no
commands: it only tells what plays.

## Show it in OBS

1. In OBS, add a source under Sources, **Browser**.
2. Paste the **Browser source address** from MixMusic's settings as its
   URL.
3. Set the width to 480 and the height to 112, or larger, and keep the
   source's background transparent: the page has none of its own.
4. Play something in MixMusic. The card fades in with the cover, the
   title, the artists and a bar that moves with the track. It fades out
   when nothing plays.

Other streaming programs that have a browser source work the same way.

## Make your own overlay

A page of your own reads the same addresses, which all answer to `GET`
and nothing else:

| Address | What it answers |
|---|---|
| `/now-playing` | JSON of what plays now. |
| `/cover` | The cover, as an image; it is empty while the track has none. |
| `/events` | Server-Sent Events: its state at once, then again at each change of track, at each play or pause, at each seek, and when the cover changes. |
| `/overlay` | The page for a browser source. |

`/now-playing` looks like this:

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
  "cover": "http://127.0.0.1:8284/cover?v=3"
}
```

`source` is `youtube`, `spotify` or `file`. `positionMs` is where the track
is at the moment of the answer; between events, add the time that passed,
times `speed`, while `playing` is true. `cover` is empty while the track
has none, and changes with each track. `active` is false while nothing is
loaded. The events are named `now-playing` and carry the same JSON:

```js
const events = new EventSource("http://127.0.0.1:8284/events");
events.addEventListener("now-playing", (e) => console.log(JSON.parse(e.data)));
```

The addresses answer to pages of any origin, so a page on the web can read
what plays, while the server is on. They refuse a request that names a
host other than this computer's.

[Settings](settings.md#desktop) lists the other rows of Settings ›
Desktop.

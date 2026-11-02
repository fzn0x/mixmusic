# Casting

MixMusic plays on cast devices, Chromecasts, Google Home and Nest speakers
and TVs with Chromecast built in, through their default media receiver,
which plays the media at a URL it's given and shows its title, artist and
cover on a screen.

- **`internal/cast`** finds the devices and speaks to them, in Go, with no
  library: the Cast v2 protocol, from its public descriptions.
  - `Browse` asks for `_googlecast._tcp.local.` by mDNS, as a one-shot
    querier (RFC 6762, 6.7): from a port of its own, to the mDNS group on
    each of the computer's networks, so the devices answer it directly, and
    MixMusic listens on no port of mDNS's. The few DNS records it reads,
    PTR, SRV, TXT and A, it reads by hand (`dns.go`); a device that
    answers the first question alone is asked for the rest. The TXT record
    tells the device's ID, its name (`fn`) and its model (`md`); groups of
    speakers are left out.
  - A `Session` connects over TLS to the device's port, 8009, with the
    protocol's frames: a 4-byte length, then a `CastMessage` protocol
    buffer, whose seven fields `message.go` writes and reads by hand. It
    pings the device every 5 s, answers its pings, and takes it for lost
    after 16 s of silence. It launches the default media receiver,
    `CC1AD845`, or joins it where it runs, connects to its transport, and
    loads, plays, pauses, seeks and stops the media, sets the volume, and
    hears the device's statuses: of the media, which come as it changes,
    statuses of media a later load took over from left out, and of the
    receiver, whose volume shows in MixMusic and whose end, as another app
    takes the device over, ends the session. Devices have certificates of
    their own, which aren't checked: the connection is encrypted, and
    MixMusic doesn't ask the device to prove what it is.
  - `casttest` is a fake device for tests, a TLS server on the loopback
    address that speaks the protocol and keeps the messages it gets, and an
    mDNS responder for it, with DNS and frames of their own, apart from
    the package's.
- **The player** (`internal/player/output.go`) plays the queue as it does,
  through a `Stream`; `SetOutput` gives it the cast device's in the
  engine's place, on its goroutine, and loads the current stream again
  there, where it was, playing or paused as it was. On a cast device the
  volume is the device's, which the settings don't keep, and tracks don't
  crossfade: it plays one stream at a time, and the player sends the next
  as one ends. Spotify tracks that play on a Spotify device stay there.
- **The app** (`internal/app/cast*.go`) looks for devices a few seconds
  after start, then every 45 s and as the cast menu opens, and lists those
  seen in the last 3 minutes. `castOut` turns the player's stream commands
  into the session's, one after another beside the player's goroutine, and
  the device's statuses into the engine's events: the position moves on
  between them, and the device is asked where it is every 5 s. The engine's
  events go nowhere while casting.
- **The audio** comes from MixMusic: the addresses the player plays are
  MixMusic's own, which no device can open, and YouTube's streams need the
  proxy, which fetches them in chunks, waits for YouTube to serve them, and
  serves downloads and the cache first. `castServer` serves the proxy's
  `/yt/` paths, only while casting, at a port of its own on the address
  that reaches the device, to that device's address alone, under a random
  128-bit token. A load the device fails without fetching anything is told
  as a firewall's doing.

Not yet: live streams, gapless playback through the receiver's own queue,
lyrics on screens, groups of speakers, a receiver of MixMusic's own, and
video.

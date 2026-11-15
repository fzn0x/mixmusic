# Rooms

Listen with friends (#561): one MixMusic hosts a room on the network,
the others join it by its code or its invite link, and each guest plays
what the host plays, in step, from where it can. The host's audio is
never sent: each guest plays the same track of YouTube Music or Spotify
as it plays its own, and a file of the host's as the same song of its own
files, or not at all. There is no server and no account.

## Who it keeps out

The threat is the one of [remote control](remote.md#who-it-keeps-out):
someone else on the same network, who can see, send, change and repeat
any message on it. So:

- only a computer that proves the room's code, or its invite link's
  secret, joins it, and only until the host removes it;
- a guest that came by a link speaks to the host the link names, and not
  to someone who takes its name or its address;
- no one else reads what plays in the room, or changes it;
- each try of a code checks one code, online: an address that tried 5
  wrong ones waits a minute.

Nothing listens unless a room is open: the host's server starts with the
room and ends with it, and a guest listens nowhere.

## The parts

- **`internal/room`** is the protocol, apart from the app: the codes and
  the invite links, SPAKE2, the host's server and the guest's session,
  keeping in step (`sync.go`), and finding the rooms (`Discovery`: `MDNS`
  on the network, `roomtest`'s fake in memory).
- **`internal/remote`** lends its transport (`transport.go`): each
  computer's key and its certificate, TLS 1.3 with a certificate on each
  side and no chain checked, the key that counts, and the framing of JSON
  messages, each after its length in 4 bytes, of at most 1 MiB.
- **The app** (`internal/app/room*.go`) opens and ends the room this
  computer hosts, tells its server what its player plays, and holds each
  start of its player for the guests (`roomhost.go`); finds the rooms,
  knocks, joins, comes back and leaves (`roomguest.go`); and plays what
  the host plays (`roomfollow.go`). A room keeps nothing on the disk.
- **The windows**: the cast button's devices menu starts a room, opens
  the one this computer hosts or is in, and lists those found; `/room`
  shows the room; `/room/{id}` takes the code of one found; and Settings ›
  Desktop › Listen with friends starts one, and keeps the name in rooms
  and Follow the host's volume.

## The key

A room speaks with remote control's key ([The key](remote.md#the-key)),
which the credential store keeps: the host's ID, which discovery tells,
is that key's, and a guest is its key to the host, which removes and
blocks it by it.

## Finding the rooms

A computer that hosts a room announces `_mixmusic-room._tcp.local.`,
apart from remote control's service: an instance named after its ID, an
SRV record with the port of its server, a TXT record with its ID (`id`),
its name in rooms (`name`) and the protocol's version (`v=1`), and the A
record of its address on the asker's network. It tells neither the code
nor what plays. The others look for rooms as they look for cast devices:
a few seconds after start, every 45 s, and as the devices menu opens.

## Joining

A TCP connection to the server's port, then TLS 1.3, ALPN
`mixmusic-room/1`. The guest says `hello`, wanting to `join` by the
code or by the link, with its name in rooms and its SPAKE2 message.

The code is 6 characters of 33, the digits and the letters but I, L and
O, about 1.3 billion codes: read in either case, O as 0 and I and L as 1.
The invite link is `mixmusic://join/<code>#<key>.<secret>`: the host's
key in hex, which the guest pins, as TLS has it, and a secret of 16 bytes
from `crypto/rand`, in base64url, which the guest proves in place of the
code. What follows `#` stays in the app that opens it.

A short code that stays the room's can't be proved as remote control
proves its own, bit by bit: each failure would tell a bit, and the code
would fall in a few dozen tries. So the two run SPAKE2 (RFC 9382), of the
ciphersuite SPAKE2-P256-SHA256-HKDF-HMAC, whose test vectors the tests
check: the guest is A, the host B, each named by its key's fingerprint.
The password scalar is SHA-512 of the code, or the secret, modulo the
group's order, and the confirmation keys are bound to 32 bytes the TLS
connection exports (`EXPORTER-MixMusic-room`), as AAD. Each message
commits to one guess of the code, so each try checks one code; someone
between the two has a TLS connection with each, whose exported bytes
differ, so the confirmations of one don't check on the other.

1. The guest sends its message, pA, in `hello`.
2. The host, unless it refuses the guest (blocked, a full room, an
   address that waits), sends its message, pB, in `spake`.
3. The guest sends its confirmation, cA, in `confirm`. A wrong one counts
   against the guest's address: 5 wrong ones, and it waits a minute.
4. The host sends its own, cB, in `welcome`, with a preview: who hosts,
   what plays and how many listen. The guest checks cB.
5. The guest's user sees the preview, and joins (`join`), or not: the
   guest pings meanwhile. The host lets it in, with `joined`, the
   members, its queue and its state.

A room takes 200 guests.

## Coming back, removing, blocking

A guest that drops off, as a computer that sleeps or a network that went,
keeps its place 15 minutes: it comes back by its key alone, with `hello`
wanting to `rejoin`, to the host's key it pinned. It tries after 1 s,
then twice as long each time up to 32 s, 15 times, and at once as its
computer wakes; the room ended, for it, once it ran out of tries.

The host removes a guest, which is told so (`ended`, `removed`), and joins
again only by the code or the link; or blocks it, by its key, which can't
join, even with the code, until the host unblocks it. Ending the room
tells each guest (`ended`, `ended`), and stops the server and the
announcement. A guest that leaves says so (`leave`), and the host no
longer keeps its place.

## Keeping in step

Each computer's clock for rooms is monotonic, from when MixMusic started
(`room.SystemClock`): a clock set, or a time zone changed, moves neither.

- **Where the host is.** The host's state tells its entry and its track,
  whether it plays, and where its track is heard, as the engine's
  `Position` tells it, less what waits in the output, at a time of its
  clock: a `Playhead`. It is sent at each change, a track, a play, a
  pause, a seek, which a position far from where the track would be
  tells, or the queue, at most every 100 ms, and every 2 s besides. The
  queue goes when it changes: up to 1,000 tracks, fewer when its message
  would be longer than 1 MiB.
- **The host's clock.** A guest pings the host 5 times as it joins, then
  every 2 s; each answer tells when the host answered, by its clock. The
  offset of the host's clock is the guest's middle of the ping's round
  trip; of the last 8, the one that came back soonest, which the network
  held least (`Offset`).
- **Comparing.** Every 250 ms a guest compares where its track is heard,
  by its engine, with where the host's is at that moment
  (`Follower.Step`): under 50 ms it does nothing; up to 750 ms it plays
  up to 2 percent faster or slower, by its engine's speed at the same
  pitch, 0.2 percent for each 10 ms, until it is within 15 ms; past that
  it seeks, a moment ahead by how far the last seek landed behind, and
  rests 6 s. A Spotify device has no speed but the track's own: those
  tracks only seek, past 750 ms. A live stream plays and pauses as the
  host's does, and can't seek.
- **A track change.** As the host's player starts a track, the engine
  loads it paused (`roomStream`), and the host tells its guests the
  track (`prepare`). Each loads it, paused at its start, and says so
  (`ready`), or at once when it can't play it. Once all did, or 5 s
  passed, the host starts them all at one moment 350 ms ahead, by its
  clock (a `state` that `Starts`), and its own engine plays then. A guest
  wakes at that moment, not at its next comparison.

The tests run the follower against a fake engine, by a fake clock, whose
sound card's clock drifts by up to 200 ppm, a play sounds 120 ms after it
is asked and a seek lands 80 ms after, over a network that holds each
message 1 to 20 ms each way; and the protocol with 100 guests, each of a
clock of its own, under the race detector and at GOMAXPROCS=1 beside CPU
hogs.

## What a guest plays

The guest's player holds the host's track alone, as one the user chose:
so it moves on as the host does, not by itself, and passes over nothing
the settings would skip. A track of YouTube Music or Spotify plays by its
ID, as the guest's settings play those: a Spotify track from YouTube
Music on a guest without Spotify. A file of the host's plays as the same
song of the guest's own files, as a hand-over finds it
([Handing over](remote.md#handing-over)), or the page shows **Can't play
here**, and the guest tells the host it's ready at once. While the guest's
settings hide explicit content, an explicit track stays silent, with a
note.

While it follows a host, the guest's crossfade, autoplay, speeds and skips
of disliked songs and songs made with AI rest (`roomSettings`), and its
player keeps no queue: `queue.json` keeps the user's own, as it was as
they joined, which comes back, paused, as they leave, or the room ends,
and at the next start if MixMusic quit in the room. Its volume, output
device and equalizer stay its own, unless **Follow the host's volume**.

## Listening on the network

While a room is open, MixMusic listens on a TCP port the system picks, on
every network, and on mDNS's UDP port, as remote control does: the
firewall notes of [Remote control](remote.md#listening-on-the-network)
hold for rooms too.

## Two copies against each other

Two copies of MixMusic on one computer, as a running-app check of rooms
needs, are two builds of other names and identifiers: MyGo's
single-instance lock hands a second copy of the same ones over to the
first. Build one as it is, and one whose `mygo.config.ts` names it
otherwise, as `name: "MixMusic Test"` and
`identifier: "app.mixmusic.test"`, and start each with
`MIXMUSIC_DATA_DIR` naming a folder of its own. Each then has a key of
its own, which the credential store keeps under its identifier. Start a
room on one; the other finds it on mDNS, on the same computer, in its
devices menu: join it with the code, or paste the invite link on its
Listen with friends page, as a link opens the copy that registered
`mixmusic://`. Both play
aloud: one on headphones and one on speakers, as Settings › Playback ›
Output device chooses, tells them apart.

## Not yet

- **Songs from guests** (Add them, Ask me first, Off), **Guests can
  control playback**, **Ask me before someone joins**, the requests on
  the room's page with a notification, and **Make host**.
- **Phones and browsers**: the page the host would serve to browsers,
  which waits for the decision on the phone page's plain HTTP (#560).
- The announcement keeps one socket, as remote control's did before
  #725: once `dnssd`'s `Keep` is on main, a room announces through it.
- A guest that drops off plays on alone, until it comes back.
- A crossfade on the host starts the next track at once; its guests
  join it in step, without the wait for them.

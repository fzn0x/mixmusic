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
- each try of a code checks one code, online, and the room counts the
  tries ([Admission](#admission)): guessing the code takes decades.

Nothing listens unless a room is open: the host's server starts with the
room and ends with it, and a guest listens nowhere.

A room that friends outside the network join faces anyone on the internet
who knocks ([Friends outside the network](#friends-outside-the-network)):
the same admission holds, with limits of what comes in on top
([Admission](#admission)).

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
nor what plays. It answers on a socket `dnssd`'s `Keep` keeps working, as
remote control's announcement does: a room opened before macOS let
MixMusic use the local network is found once the user lets it (#725). The others look for rooms as they look for cast devices:
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
code; then, while friends outside may join, `@` and where they reach the
host, 4 at most, between commas, IPv6 in brackets escaped
(`weblink.go`). What follows `#` stays in the app that opens it. The
link's https form, `https://mixmusic.functionx.dev/join/#<code>.<key>.<secret>`
and the rest, opens a page of the website, for chat apps that don't open
`mixmusic://` links, which opens the app's link or offers the download:
the code and the rest stay after the `#`, which the browser never sends
to the website.

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
2. The host, unless it refuses the guest (blocked, a full room, a
   source or a room that waits, or a room busy with other codes), counts
   the try ([Admission](#admission)), and sends its message, pB, in
   `spake`.
3. The guest sends its confirmation, cA, in `confirm`, which the host
   checks: a wrong one counts as wrong, a right one not at all.
4. The host sends its own, cB, in `welcome`, with a preview: who hosts,
   what plays and how many listen. The guest checks cB.
5. The guest's user sees the preview, and joins (`join`), or not: the
   guest pings meanwhile. The host lets it in, with `joined`, the
   members, its queue, its state and its rules (`rules`); or first asks
   its user to, when they ask it to ([Asking before a join](#asking-before-a-join)).

A room takes 200 guests.

## Admission

The code is all that keeps someone on the network out, and SPAKE2 has
each try check one code, online: so the room counts the tries, and holds
few computers that proved nothing yet (`admit.go`).

- **By source.** A source is an IPv4 address, or the /56 of an IPv6 one,
  as a home's router has from its provider, within which one home changes
  its addresses as it likes. A source tries 5 codes, then waits a minute.
- **Room-wide.** The room takes 30 wrong codes a minute in all, from
  whatever sources of the internet, and 30 more of its own network,
  private, loopback or link-local, apart, so that wrong codes from the
  internet lock no code out on the network: past them, it checks no code
  of those until the minute is over.
  At 30 a minute, 43,200 a day, the 33^6, about 1.29 billion, codes take
  82 years, half of them 41; a room lasts an evening. A link's secret,
  of 128 bits, isn't guessed, and isn't counted: link holders, and guests
  that come back by their keys, get in while codes wait. The minute rolls
  whatever tries are under way, which go on into the next. Someone who
  keeps guessing keeps codes out as long as they guess: so the guests are
  told the room takes no code for a minute (`locked`), and to ask for the
  invite link, not that they tried too many; and the host is told, once
  each time (`Guessing`), and the activity says so, that someone guesses
  the code: the invite link, or a new code, gets friends in.
- **Before it runs.** A try counts as it starts, under the server's lock,
  before SPAKE2 runs: tries at once count too. Those under way count
  against both limits without making anyone wait, as they may be right:
  one more than the limits take is told the room is busy (`busy`), and
  tries again in a moment. A right code doesn't count; a try that ends
  before its code was checked, as the guest went, checked none.
- **Bounded.** The room counts 4096 sources at most, and forgets those that
  tried a minute ago and neither wait nor try.
- **What comes in** (`limits.go`), as anyone on the internet may knock
  once friends outside join: a source holds 32 connections at once at
  most, proven or not, and the room 512, past which a connection is
  closed as it comes, each more than the places of the unproven, so that
  a new connection takes the place of the oldest rather than be closed.
  A source opens 3 connections a second, after a burst of 8, past which
  one is closed before TLS, so that no source has the host run TLS as
  often as it likes. A connection's messages past 60 a second, after a
  burst of as many, wait to be read, so that its sender's sends wait in
  turn. A guest takes the host's messages of 4 KiB at most, too, until
  the host's welcome proved the code: at an address typed in, anyone may
  answer. While friends outside may not join, the room takes connections
  of this computer's networks alone, before TLS.
- **The previews** (`previews.go`): those that proved the code or the
  link and wait to join, as a link forwarded beyond its friends brings.
  The room holds 4 of a source and 32 in all: a new one takes the place
  of the oldest of its source, or of the source that holds the most, and
  each waits 5 minutes at most (`Preview`), then is told the preview
  ended. One that waits for the host's user to let it in
  ([Asking before a join](#asking-before-a-join)) is a preview too.
  Guests, and those that come back by their keys, take no preview's
  place.
- **The unproven.** A connection holds one of 32 places in the room, 4 of a
  source, from its accepting until it proved the code, the link or its
  key, or ended, 5 s at most in all (`Prove`), not a Silence for each
  step. A new one takes the place of the oldest of its source, when the
  source holds 4, and of the oldest of the source that holds the most,
  when the room holds 32: held open, connections keep no one out, and a
  flood of many sources takes its own places, not a far guest's; it must
  open them faster than a proof takes. A guest the room keeps, which comes back by
  its key, gives its place back as soon as TLS tells its key. Until it
  proved something, a connection's messages are of 4 KiB at most, refused
  from their length before any of them is read (`Frames.Limit`); a hello
  is a few hundred bytes. Once proven, a guest's messages are of 64 KiB at
  most (`guestMessage`): a song it adds is the largest, of a few KiB.
  From then on its messages are read on a goroutine of their own
  (`messages`), so that the host waits on them and on its user at once,
  into the fields of a guest's messages alone (`sent`): a guest's message
  of the host's fields builds nothing of them.

## Coming back, removing, blocking

A guest that drops off, as a computer that sleeps or a network that went,
keeps its place 15 minutes: it comes back by its key alone, with `hello`
wanting to `rejoin`, to the host's key it pinned. It tries after 1 s,
then twice as long each time up to 32 s, 15 times, and at once as its
computer wakes, 32 times: about 15 minutes, as long as the host keeps
its place, after a sleep or a change of network on either side. The room
ended, for it, once it ran out of tries.

The host removes a guest, which is told so (`ended`, `removed`), and joins
again only by the code or the link; or blocks it, by its key, which can't
join, even with the code, until the host unblocks it. Both go by the key:
a computer that makes a new key knocks again with the code, or the link,
it has. **New code** gives the room a new code and a new link's secret
(`SetCode`), which shuts it out, and the guests in the room stay. Each
proof is of the code of its time: a new code refuses the proofs of the
old one under way, and ends the connections that proved it and wait to
join, which are told the room has a new code (`new code`). Ending
the room
tells each guest (`ended`, `ended`), and stops the server and the
announcement. A guest that leaves says so (`leave`), and the host no
longer keeps its place.

## The host's rules

What guests may do is the host's to say, as its settings choose
(`Rules`, `rules.go`), and the host checks each ask of a guest as it
comes, by the rules of that moment: a guest's MixMusic, which shows only
what the rules let it do, decides nothing. Each ask is of a guest the
room has now, on its connection; what isn't a guest's ask is ignored.
The guests are told the rules as they join, and as they change
(`RulesChanged`, and every 2 s besides). The app adds the songs, has its
player do the controls, and tells its user what waits for them
(`Guests`): in the windows, and in a notification that opens the room's
page, which nothing waits on.

- **Songs from guests** (`songs.go`). A guest adds a song (`add`, its
  number and the track, cleaned as a hand-over's), which the host adds to
  the end of its queue at once (`SongsAdd`), or keeps until its user
  accepts or declines it (`SongsAsk`, as at first), or refuses
  (`SongsOff`); the guest is told which (`added`). A guest adds
  `songsPerMinute` songs, 10, at most, ten at once then one each 6 s, and
  has `MaxWaiting`, 10, wait at most. The host takes a guest's word on a
  song for nothing but its ID and its kind: it finds each by its ID
  (`Guests.Look`), on YouTube Music, signed out, on Spotify while signed
  in to it, or among its own files, and shows, plays and tells Discord
  and the scrobblers of the song as that tells it, its covers and whether
  it's explicit included. A Spotify song is refused while the host isn't
  signed in to Spotify, as nothing else tells what it is. It finds them
  beside the guest's messages, 16 at once in the room (`maxLooks`), 4 of
  a guest (`maxLooksEach`), 10 at once then one a second (`lookBurst`,
  `lookEvery`), 10 s each at most; a song of no title it refuses first.
  In Ask me first, a file of the
  guest's waits for the host's user, who finds it among their files as
  they accept it, so that the guest learns nothing of the host's files
  meanwhile; and a song the host's settings hide is refused as its user
  accepts it. The songs that wait go with their guest, as it leaves or is
  removed or blocked.
- **Controls** (`control.go`). While the rules let them, a guest has the
  host's player play, pause, go to the next track or the last (`control`):
  5 at once at most, then one a second; one the rules don't let through,
  or past that, is refused (`control`, with why). The activity tells who
  did what.
- **Each source too.** Songs and controls are counted by the guest's key,
  and by its source, an IPv4 address or IPv6 /64, which takes twice as
  many: a computer that proved the code makes new keys as it likes.

### Asking before a join

While its user asks to (`Rules.Ask`, `letin.go`), the host lets in no
computer that proved the code or the link and asks to join until its
user says so: it tells the guest it asks (`asking`), answers its pings
meanwhile, and lets it in, or turns it away (`refused`, `declined`), as
its user answers, or after 2 minutes (`AskFor`). 20 wait at most
(`maxKnocks`), 2 of a source (`maxKnocksFrom`): one more is refused
(`many asking`). The host's user is told of a source's first knock, and of
none of it again within a minute; each shows the start of its key's ID,
which tells two of one name apart, and whether it comes from the host's
own networks or the internet. Guests that come back by their keys aren't
asked about, but for the seats of a room that moved here, the first time
(`letInSeat`): turned away, a seat goes. A new code ends those that wait.

## Make host

The host moves the room to a guest, if the others can reach it
(`move.go`), and only to one whose MixMusic says it can (`move`, among
the hello's features):

1. The host asks the guest to host (`host`), with the seats to keep: the
   keys and names of the other guests and of itself, and the keys it
   blocked, 2,048 at most (`maxBlocked`).
2. The guest's user is asked whether this computer hosts the room, 50 s
   at most: declined, or unanswered, or asked again within a minute, or
   by a host declined twice in the stay, it says so (`hosting`,
   `declined`). The host waits a minute for that (`AnswerFor`). Once its
   user agreed, the guest opens a room of its own, of a new code and link,
   which keeps those places as of guests that dropped off and blocks
   those keys (`Seats`), and says at which port (`hosting`). That room
   lets no one in until the room moved there (`Open`).
3. The host tells each other guest where (`reach`): the new host's key,
   and its address, as the host's connection with it tells, with that
   port. Each tries a TLS connection to that key, where discovery finds it
   or at that address, and says whether it got one (`reach`); the host
   tries it too. A guest tries once each 10 s at most, at that address
   and where discovery finds that key alone, its tries all ending by 3 s
   after it was asked, as it says it didn't then, however soon it knew,
   so that a host can't use its guests to knock at address after address.
4. Once all did, the host tells them all the room moved (`moved`), before
   it ends the room: each comes back to the new host by its key, as after
   a drop, the host too. The new host's player takes the room's queue,
   where the host played it.

One that can't reach it, or no answer in 10 s once the guest opened its
room (`MoveFor`), keeps the room where it is: the guest is told to close
the room it opened (`unhost`), which it also closes by itself after twice
that. So does a Remove or a Block meanwhile, as the room it would move to
keeps the seats and blocks of its asking, a guest that leaves, and the
guest it moves to leaving. As it moves, and once it moved, the room lets no one join, or
come back (`moving`), as their places wouldn't move with it. A guest takes
a seat only of a key, 201 at most.

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
- **A crossfade.** As the host's player crossfades into the next track,
  the engine loads it on its other deck (`Preload`), the track that ends
  plays on, and the guests are told the next is loading, at its start,
  as a track change does (`roomfade.go`): they ready it, and the fade
  starts at the moment they all start it. The host waits half the fade at
  most, less the lead (`PrepareFade`), so that the track that ends still
  plays then.

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
of disliked songs, songs made with AI and the parts SponsorBlock marks
rest (`roomSettings`): the host's own skips, which it hears as seeks,
are the room's, and its
player keeps no queue: `queue.json` keeps the user's own, as it was as
they joined, which comes back, paused, as they leave, or the room ends,
and at the next start if MixMusic quit in the room. Its volume, output
device and equalizer stay its own, unless **Follow the host's volume**.

## Listening on the network

While a room is open, MixMusic listens on a TCP port, on every network,
and on mDNS's UDP port, as remote control does: the firewall notes of
[Remote control](remote.md#listening-on-the-network) hold for rooms too.
The port is kept from room to room (`RoomPort`), picked at random from
1024 to 65535 as the first opens, so that a port forwarded by hand keeps
working; when another program has it, the room listens at one the system
picks, and its page says so.

## Friends outside the network

The switch of the room's page, **Let friends outside this network join**,
off at first (`RoomAnywhere`), has the room reached from the internet
(`internal/app`'s `roomreach.go` and `roomfar.go`). There is no server and
no relay: guests reach the host's computer directly.

- **The host.** Its listener asks the router, while the switch is on, to
  forward the room's port ([Port mapping](portmap.md)), and opens it at
  the router of IPv6 by PCP. The invite link carries the router's
  internet address, the port it forwards, and this computer's IPv6
  address: or, in place of both, the address the user typed for invites
  (`RoomAddress`), as a VPN's both sides run, or a router's with a port
  forwarded by hand, which nothing asks of the router. The forward goes
  as the switch goes off or the room ends. The page tells what was set
  up, or why nothing reaches the room, as a router behind another, and
  that the port can't be tested from outside; and once the addresses
  changed since the link was copied, as after the router's internet
  address changed, it tells the host to send the new link.
- **The guest.** Knowing the host's key, by the link or from its stay,
  it connects to the address discovery found and to the link's, each
  250 ms after the one before, TLS and the key's pin too, and keeps the
  first that is the host (`connectFirst`), as it knocks and as it comes
  back: a guest on the host's network gets there, with no hairpin
  through the router, and an address gone stale, or another's, that
  answers first wins nothing. By a code alone, it connects where it was
  told. The addresses are those of the knock that started, which proves
  the link: a link of the host's of the same secret, which a guest that
  dropped off opens, brings it back at once to the new address, and one
  of another secret, or of none, leaves them. Join a room takes an
  address and a code, which the guest proves as on the network. One that
  can't reach a room outside is told the likely causes: a router that
  doesn't forward the port, a firewall, or a room that ended.
- **Firewalls.** The rule of Settings › Desktop › Windows Firewall lets
  in any address on private and domain networks, which a forwarded port
  is; macOS's firewall must allow MixMusic's incoming connections, and on
  Linux ufw or firewalld must let the port in.
- **Not yet.** Turning the switch on is to suggest **Ask me before
  someone joins**, which waits for that setting (#734). Checking the
  forward on a real router waits for a running app.

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

- **Phones and browsers**: the page the host would serve to browsers,
  which waits for the decision on the phone page's plain HTTP (#560).
- A guest's **Sound mood** (#547) and **Skip silence** (#552), once they
  are on main, rest through `roomSettings` too.
- A guest that drops off plays on alone, until it comes back.
- A guest away as the room moves comes back to the old host, which is
  gone: its place in the new room waits for it 15 minutes, but it finds
  the room only by the code or the link.

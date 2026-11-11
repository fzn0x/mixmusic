# Remote control

MixMusic on one computer controls MixMusic on another in the same network
(#353): it finds it, pairs with it once by a short code the other shows,
and from then on shows what plays there, with its controls and its queue,
and can move what plays here there. There is no server and no account:
the two computers speak to each other alone.

## Who it keeps out

The threat is someone else on the same network, as a guest on the Wi-Fi,
who can see, send, change and repeat any message on it: answer in place of
another computer, connect to MixMusic, or sit between two computers and
pass on what each sends. So:

- only a computer the user paired controls this one, and only until the
  user removes it;
- a computer that controls another speaks to the one it paired with, and
  not to someone who takes its name or its address;
- no one else reads what plays or the commands, or changes or repeats
  them;
- a guess at the code gets in once in a million tries, and each code is
  good for one try.

What it doesn't keep out: someone with the user's account on either
computer, who can read its files; someone who keeps the network from
working, as no one can stop that; and the sight of two computers talking,
and how much.

## The parts

- **`internal/dnssd`** finds services, and announces them, by DNS-SD
  (RFC 6763) over mDNS (RFC 6762), in Go: it writes and reads the few
  records it needs (PTR, SRV, TXT and A) itself. `Browse` asks as a
  one-shot querier (RFC 6762, 6.7), from a port of its own, as casting
  does, so that the answers come to it directly. A `Responder` answers
  the questions about one instance of a service: to a one-shot querier
  directly, and to mDNS's own port in mDNS's group, with the address the
  asker can reach.
- **`internal/remote`** is the protocol, apart from the app: each
  computer's key, pairing, the encrypted connection, and the messages of a
  session. Its `Discovery` interface finds and announces the computers:
  `MDNS` on the network, and `remotetest`'s fake, which tests use, in
  memory.
- **The app** (`internal/app/remote*.go`) keeps the settings and the
  paired computers in `remote.json`, and the key in the system's
  credential store; runs the server while remote control is on; answers
  with the player's state and runs the commands it gets; and, as the
  other side, looks for the computers on the network, pairs with them, and
  keeps a session with the one whose page shows.
- **The windows**: Settings › On the desktop › Remote control turns it on
  and lists the paired computers with Remove; the player bar's devices
  menu, the cast button's, lists the computers found; `/remote/{id}` pairs
  with one, then shows what plays there with its controls and its queue
  (`remotequeue.go`), and what plays here, to move there
  (`remotehandover.go`); and a dialog shows the code while another
  computer pairs with this one.

## The key

Each MixMusic has an Ed25519 key pair, made the first time it pairs or
turns remote control on, and kept in the system's credential store as the
sign-ins are (the Keychain, the Credential Manager or a file sealed with
DPAPI, the Secret Service or a file only the user reads). Each start makes
a self-signed certificate of it for TLS; only the key counts. A computer
is its key's fingerprint, the SHA-256 of the certificate's public key
(SubjectPublicKeyInfo), and its ID, which discovery tells, is the first 80
bits of it, as 16 letters and digits.

The app reads and makes the key (`identity`) with no lock held that a
frame takes. On the main thread, MyGo runs the window's events while the
credential store answers, which may take a while as the system asks the
user. A frame then shows the code and the page, and takes the lock of
remote control: held around the key, the main thread would wait for
itself (#464). A second call while the key is read, a click meanwhile,
is told to try again, rather than wait for the first.

## Finding the computers

A computer with remote control on announces `_mixmusic._tcp.local.`: an
instance named after it, an SRV record with the port of its server, a TXT
record with its ID (`id`), its name (`name`) and the protocol's version
(`v=1`), and the A record of its address on the asker's network. It
answers from mDNS's port, 5353, which it shares with the system's own
responder. The others look for it as they look for cast devices: a few
seconds after start, every 45 s, and as the devices menu opens.

What discovery tells is only where to connect: anyone can answer. The key
decides who answers there.

## The connection

A TCP connection to the server's port, then TLS 1.3, and no older
version, with a certificate on each side. Neither side checks a chain of
certificates: a computer that controls another checks that the other's key
is the one it paired with, and the one controlled checks that the key of
the computer connecting is among those it paired with, unless it asks to
pair. TLS encrypts what they send, and keeps it from being changed or
repeated: each connection has keys of its own, and each record a sequence
number, so a record sent again, in the same connection or another, is
refused. Go's TLS sends no early data (0-RTT), which could be repeated.

The messages are JSON objects, each after its length in 4 bytes, of at
most 1 MiB. The first is the connecting computer's `hello`, which asks to
pair or to control.

## Pairing

The user chooses the computer in the devices menu, and Pair. The other
computer, while its remote control is on and no other pairing runs, shows
a code of 6 digits from `crypto/rand`, with the name of the computer that
asks, and the user types it on the first. The code is good for 2 minutes
and one try.

A code that short can't be sent as a proof: whoever got an HMAC of it
could try the million codes in a moment, offline. So the two prove it bit
by bit, as Bluetooth's passkey entry does: in each of 20 rounds, one for
each bit of the code (999,999 fits in 20 bits), each commits to the bit
before either shows it.

1. The computer that pairs sends its commitment: an HMAC-SHA256 keyed by a
   new random 128-bit nonce, of its role, the round, the bit, both keys'
   fingerprints, and 32 bytes the TLS connection exports (RFC 8446, 7.5),
   which are the same on both its ends and nowhere else.
2. The computer paired with sends its own, of its role.
3. The first sends its nonce, and the second checks the commitment with
   its own bit of the code.
4. The second sends its nonce, and the first checks it the same way.

A computer that doesn't know the code commits to each bit before it
learns it, and gets past each round once in two: past all 20, once in a
million. What it learns of the code as it fails is no use, as the code
was for that try alone. Someone between the two has a TLS connection with
each, whose exported bytes and keys differ, so the commitments of one
don't check on the other. A mismatch ends the pairing and its code, on
both sides.

Then the computer paired with keeps the other's fingerprint, ID and name
in its paired computers, as one that may control it, and the other keeps
its fingerprint, ID and name, as one it controls.

## Removing

Remove, in Settings › Remote control, forgets a paired computer: its
session closes at once, and the next one is refused as not paired, so it
has to pair again. Turning remote control off closes the server, its
sessions, which it tells that it is off, and the discovery answers.

## A session

After `hello`, the computer controlled answers `welcome`, and sends what
its player does (`state`: the track, playing or paused, where it is, how
long it is, the volume, and up to 20 tracks of the queue after it) as it
changes, at most 5 times a second, and every 2 s besides; the other sends
commands (`command`: play, pause, next, previous, seek, volume, and those
on the queue below), a hand-over (`handover`, below), and a `ping` every
2 s. The computer controlled checks, before each command and each
hand-over, that the one that sends it is still paired. Either side
drops a connection after 6 s with no message, as from a computer that
went to sleep or left the network, and the controlling one connects again,
after 1 s, then 2, then up to 10, while its page shows, and looks for it
anew, as a computer that started again listens at another port. The page
moves the position on between two states as the track plays, and the
session ends 30 s after the page closes.

## What each version can do

The version, 1, which `hello`, discovery's TXT record and TLS's ALPN tell,
changes only when two MixMusic apps can no longer understand each other:
then they refuse each other, and say to update both. What is added since
goes by name, in the `features` of `hello` and `welcome`: `queue` for the
commands on the queue, `handover` for taking what the other hands over.
An older MixMusic doesn't know the field, so it tells none, and ignores
the other's; it skips a message of a type it doesn't know, as `handover`,
and runs no command it doesn't know. So:

- a newer computer that controls an older one shows what plays there and
  controls it as before; it sends neither the commands on the queue nor a
  hand-over, as the older one's `welcome` tells no features, and its page
  says to update MixMusic there;
- an older computer that controls a newer one gets states whose tracks
  name their entries (`entry`), a field it ignores, and the `features` of
  `welcome`, which it ignores too.

## The queue

A state names the entry of each track in the queue by its UID there
(`entry`), an ID of the entry, not of the track, which tells apart two
entries of the same track and stays the same as the queue moves. The
commands on the queue name entries so, as an index would name another
entry once the queue moved: `playEntry` plays one, `remove` takes one out,
`move` moves one to the place of another, as dragging one onto the other
does in this computer's queue, and `clear` empties the queue after the
track that plays. Only the entries after the track that plays move or go,
as only those show; an entry the queue no longer holds changes nothing.

## Handing over

The page of a computer controlled shows what plays here, and Move to
moves it there: this computer sends `handover`, with its queue, in the
order it plays, the index of the track that plays, where it is, whether
it plays, shuffle and repeat, and an ID. The other plays it in place of
its own, and answers `taken`, with the ID, how many tracks it plays and
how many it left out, and whether it left out the one that played. This
computer then pauses, unless the other took nothing. One hand-over runs
at a time, and this computer waits 30 s at most for the answer.

Each track goes with its service (`youtube`, `spotify` or `file`), its ID
there, and what shows of it: its title, its artists and album, with their
IDs, its length, its covers from `https://`, and the podcast of an
episode. The computer that takes it plays a track of YouTube Music or
Spotify by its ID, when the ID is one of that service's (a YouTube video
ID, a Spotify ID of letters and digits), as it plays its own. A file
goes without its path, its IDs or its cover, which name what is on the
computer that hands it over: the other plays its own file of the same
song, of the same title and an artist of the same name, as their tags
tell, and of the same length give or take 3 s, of the same album first,
and leaves it out when it has none. It never plays a path the other
names, and never looks for a file on a service. When the track that
played was left out, the next track kept plays from its start.

A hand-over holds up to 100 tracks before the one that plays and the rest
after it, 1,000 tracks at most, and fewer when its message would be
longer than 1 MiB. The computer that takes it cleans it as it cleans a
state: control characters out, lengths bounded, IDs of printable ASCII,
covers from `https://` only, the index in the queue.

## Listening on the network

While remote control is on, MixMusic listens on a TCP port the system
picks, on every network, and on mDNS's UDP port. So the first time, the
system may ask: Windows Defender Firewall whether MixMusic may take
connections from the network (allow it on private networks), macOS
whether it may find and connect to devices on the local network.

On Windows, `internal/firewall` reads what the firewall does to
MixMusic's executable through its COM interface, `INetFwPolicy2`, which
needs no administrator: whether it's on for the networks the computer is
on, whether those are public, whether an organization's policy decides,
whether it blocks every incoming connection, and the inbound rules of the
executable. Settings › Desktop › Windows Firewall shows it, read again
after remote control turns on, after Allow, and at most every 5 seconds
while the settings show. **Allow** starts MixMusic again with
`--allow-firewall`, through `ShellExecuteEx`'s `runas`, so Windows asks
for an administrator's approval; that copy, which `main` runs before the
single-instance lock, takes private and domain networks off the inbound
rules that block the executable, adds the rule "MixMusic" that lets it in
on them, or turns that rule back on, and ends. Its exit status tells the
settings how it went. Public networks stay closed. Tests
listen on 127.0.0.1 alone, and find the computers through
`remotetest`'s fake.

Not yet: seeing more than the other's 20 next tracks, and moving what
plays there to this computer.

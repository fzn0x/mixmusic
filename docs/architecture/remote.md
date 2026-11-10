# Remote control

MixMusic on one computer controls MixMusic on another in the same network
(#353): it finds it, pairs with it once by a short code the other shows,
and from then on shows what plays there, with its controls. There is no
server and no account: the two computers speak to each other alone.

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
  with one, then shows what plays there with its controls; and a dialog
  shows the code while another computer pairs with this one.

## The key

Each MixMusic has an Ed25519 key pair, made the first time it pairs or
turns remote control on, and kept in the system's credential store as the
sign-ins are (the Keychain, the Credential Manager or a file sealed with
DPAPI, the Secret Service or a file only the user reads). Each start makes
a self-signed certificate of it for TLS; only the key counts. A computer
is its key's fingerprint, the SHA-256 of the certificate's public key
(SubjectPublicKeyInfo), and its ID, which discovery tells, is the first 80
bits of it, as 16 letters and digits.

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
sessions and the discovery answers.

## A session

After `hello`, the computer controlled sends what its player does (`state`:
the track, playing or paused, where it is, how long it is, the volume, and
up to 20 tracks of the queue after it) as it changes, at most 5 times a
second, and every 2 s besides; the other sends commands (`command`: play,
pause, next, previous, seek, volume) and a `ping` every 2 s. Either side
drops a connection after 6 s with no message, as from a computer that
went to sleep or left the network, and the controlling one connects again,
after 1 s, then 2, then up to 10, while its page shows. The page
moves the position on between two states as the track plays.

## Listening on the network

While remote control is on, MixMusic listens on a TCP port the system
picks, on every network, and on mDNS's UDP port. So the first time, the
system may ask: Windows Defender Firewall whether MixMusic may take
connections from the network (allow it on private networks), macOS
whether it may find and connect to devices on the local network. Tests
listen on 127.0.0.1 alone, and find the computers through
`remotetest`'s fake.

Not yet: handing what plays here over to the other computer, playing a
track of the other's queue from here, and seeing more than its 20 next
tracks.

# Remote control
<!-- docs: group=using order=75 -->

With MixMusic on two computers of the same network, as a desktop and a
laptop, one can show what the other plays and control it: play, pause,
next, previous, seeking, the volume and the queue. It can also move what
plays on it to the other. There is no server and no account: the two
find each other on the network and talk directly.

## Turn it on

On the computer you want to control, open Settings › Desktop › Remote
control and turn on **Allow remote control**. It is off at first. The
other computers with MixMusic on the same network then find this one,
by its name, which the setting tells.

Turned off, no other computer finds it or controls it, those paired
included, until you turn it on again.

## Pair once

On the computer you control from:

1. Click the [cast button](casting.md) in the player bar. Below the cast
   devices, its menu lists the computers with remote control on, as
   "Desktop · MixMusic".
2. Choose one. Its page opens and asks to pair: click **Pair**.
3. The other computer shows a code of 6 digits, with this computer's name.
   Type the code on this one, and **Pair**.

The code works once, for 2 minutes. If you didn't ask for it, **Cancel**
on the computer that shows it refuses the pairing. A code typed wrong
fails the pairing on both computers: click **Pair** again for a new one.

The two computers remember each other from then on, and pair no more.

## Control it

The page of a computer paired shows what plays there: the cover, the
title, the artists and the album, where the track is, which moves on as
it plays, and the tracks after it. Under them:

- **Previous**, **Play** or **Pause**, and **Next**;
- the seek bar, which seeks once you let go of it;
- the volume of that computer.

Its queue, under **Next on …**, works as this computer's does: click a
track's cover, or double-click the track, to play it there; drag a track
onto another to move it to that one's place; the cross beside a track, or
**Remove from the queue** in its menu, takes it out; and **Clear** empties
the queue there after the track that plays.

What you do there, on its own window, its keys or its media controls,
shows here too. The page connects again by itself when the other computer
was asleep or left the network a moment.

## Move what plays here there

While something plays on this computer, the page of the other shows it,
under **On this computer**, with **Move to …**. That moves it there: the
queue, the track that plays and where it is, whether it plays, shuffle
and repeat. The other computer plays it in place of what it played, and
this one pauses. The queue moves with up to 100 tracks before the one that
plays and the rest after it, 1,000 tracks at most.

Each track plays there as if it had been chosen there:

- **YouTube Music and Spotify** tracks play as that computer plays
  theirs: a Spotify track as its Spotify settings say, or from YouTube
  Music when it isn't signed in to Spotify. A track that needs a sign-in
  that computer hasn't fails there, as it would if chosen there.
- **Music files of this computer** don't move: their paths name nothing
  on the other computer. It plays its own file of the same song instead:
  one whose tags have the same title and an artist of the same name, of
  the same length give or take 3 seconds, of the same album first. A file
  it doesn't have is left out of the queue there, and this computer says
  how many were. When the track that plays here is one, the next track
  plays there, from its start. When it has none of the tracks, nothing
  moves, and the music plays on here. A file is never looked for on
  YouTube Music or Spotify.

## Paired computers

Settings › Desktop › Remote control lists the computers paired, with what
each does: one that **can control this computer**, or one **you control
from here**. **Remove** forgets it. A computer removed while it controls
this one stops at once, and its page says so: it has to pair again.

## Safe on a shared network

What the two send is encrypted, and only they can read it. Each MixMusic
has a key of its own, which your system's credential store keeps, and the
code proves, as you pair, that each talks to the other and not to someone
else on the network: from then on, each knows the other by its key. A
computer that only takes the other's name, or its address, is refused.
Someone who tries codes has one chance in a million each time, and each
code is good for one try. [How it works inside](architecture/remote.md)
tells more.

## Firewalls and the local network

While remote control is on, MixMusic listens for the other computers on
your local network. So the first time you turn it on, your system may ask
about it:

- **Windows**: Windows Defender Firewall asks whether MixMusic may take
  connections from your network. Allow it on private networks, such as the
  one at home. If your network is set as public, the other computers can't
  reach MixMusic: in Settings › Network & internet, make it a private
  network, or allow MixMusic on public networks in the firewall's allowed
  apps.
- **macOS**: macOS asks whether MixMusic may find and connect to devices
  on your local network. Allow it on both computers; System Settings ›
  Privacy & Security › Local Network changes it later. With the macOS
  firewall on, allow MixMusic's incoming connections too.
- **Linux**: a firewall such as ufw or firewalld must let in the port
  MixMusic picks, which changes each start, and mDNS's, UDP 5353: allow
  incoming connections from your local network, as
  `sudo ufw allow from 192.168.1.0/24` does for the addresses 192.168.1.x.

## When it doesn't work

- **The computer isn't in the menu.** Remote control must be on there,
  MixMusic running, and both computers on the same network: a guest
  network, or a router that keeps its devices apart, hides them from each
  other, and some networks don't pass the messages computers are found by
  (mDNS) between Wi-Fi and wired ones.
- **"Couldn't reach …"**: the computer may be asleep, or its firewall may
  turn MixMusic away (see above).
- **"That isn't the code … showed"**: click **Pair** again, for a new code.
- **"… removed this computer"**: it was removed there. Pair again.
- **"Another computer answered in place of …"**: something else on the
  network answers at the address of the computer paired. MixMusic doesn't
  talk to it.
- **"MixMusic on … is older"**: update MixMusic there. Until then its page
  here shows what plays there and controls it, but doesn't change its
  queue or move what plays here to it.

## Not yet

Seeing more than the other computer's next 20 tracks, and moving what
plays there to this computer.

# Listen with friends
<!-- docs: group=using order=76 -->

With MixMusic on several computers of the same network, friends at home
can all hear what one of them plays, at the same moment: one starts a
room, the others join it, and each computer plays the host's music, in
step, on its own speakers. Your audio is never sent: each computer plays
the same track from where it can. There is no server and no account.

## Start a room

Open the [cast button](casting.md)'s menu in the player bar and choose
**Start a room**, or click **start a room** in Settings › Desktop ›
Listen with friends. The room's page shows its code, of 6 letters and
digits, **copy invite link**, and **end room**.

Friends join with the code, or with the invite link, which you send them
as you like. The room lasts until you end it, or quit MixMusic.

## Join a room

1. Open the cast button's menu. Below the devices, it lists the rooms on
   your network, as "Desk's room".
2. Choose one, and type the code its host's page shows. Its letters may be
   in either case; O counts as 0, and I and L as 1.
3. The room shows who hosts it, what plays and how many listen. Click
   **join**.

An invite link, `mixmusic://join/…`, opens the room in MixMusic, which
then shows the same, with **join**. Pasted on the Listen with friends
page, as **Invite link**, it does too.

Five wrong codes from a computer, and it waits a minute before it may try
again; 30 wrong codes in a minute from anyone, and the room takes no code
for a minute, though invite links and guests who come back still get in.
Then the room says it isn't taking codes for a minute, and the host is
told someone is guessing the code: the invite link, or a new code, gets
friends in.

## In a room

Your computer plays what the host plays, from where the host is: each
track, as it pauses, plays and seeks. When the host changes track, it
waits for everyone to have the next one ready, 5 seconds at most, then
all start at once. With crossfade on, the host's track plays on while the
next waits, half the fade at most, and the fade starts as all start. Your computer keeps itself within about 50
milliseconds of the host's: a little faster or slower, at the same pitch,
when it drifts, and a jump when it's far off.

- **Spotify tracks** play on your Spotify as your settings play them, or
  from YouTube Music when you have no Spotify. Those Spotify plays itself
  only jump, as it plays at its own speed.
- **The host's music files** play from your own file of the same song, as
  its title, artist and length tell, or the page says **Can't play here**.
- **Explicit songs**, while your settings hide them, stay silent, with a
  note.
- Your **volume**, output device and equalizer stay yours, unless you turn
  on **Follow the host's volume** in Settings › Desktop › Listen with
  friends.
- Your **crossfade**, **autoplay** and **speeds**, and the skipping of
  songs you dislike, songs made with AI and the parts SponsorBlock marks,
  rest while you're in a room. The host's own skips are heard by all.
- **Songs of your own**: while the host takes songs from guests, a song's
  menu has **Add to the room**, or **Ask to add to the room** when the
  host accepts each first. A note tells you what became of it.
- **Controls**: while the host lets guests control playback, the room's
  page has **previous**, **play** or **pause**, and **next**, which the
  host's player does.

The room's page shows what plays, who listens, and what comes next. Click
**leave room** to leave: your own queue comes back, paused where it was.
If MixMusic quits while you're in a room, your queue is back at the next
start.

The others see you by your name in rooms, which Settings › Desktop ›
Listen with friends sets: your computer's name when you set none.

## Running a room

The host's page lists the guests: **remove** takes one out of the room,
and it can join again only with the code or the link; **block** keeps it
out, even with the code, until you **unblock** it. **Activity** tells who
joined, left, or was removed or blocked.

Remove and block go by the computer's key, which someone can make anew:
then they can knock again with the code, or the link, they still have.
**new code** gives the room a new code and a new invite link: the old
ones let no one in, those who proved the old code and haven't joined yet
included, and your guests stay.

What guests can do, on the room's page and in Settings › Desktop ›
Listen with friends:

- **Songs from guests**: **add them** to the end of your queue at once,
  **ask me first**, as at first, or **off**. Asked first, each song waits
  on the room's page, with **accept** and **decline**, 10 of a guest at
  most, and a notification tells you while MixMusic is in the
  background: clicking it opens the page. A guest adds 10 songs a minute
  at most, and none your settings would hide or your computer can't play.
  Your computer finds each song by its ID, on YouTube Music, or Spotify
  while you're signed in to it, and shows it, plays it and tells Discord
  and your scrobbles of it as the service tells it, not as the guest
  does: a Spotify song needs you signed in to Spotify. A file of a
  guest's plays your own file of the same song, which your computer looks
  for as you accept it.
- **Guests can control playback**, off at first: they play, pause and
  skip, 5 times at once at most, then once a second.
- **Ask me before someone joins**, off at first: each friend who joins
  with the code or the link waits on the room's page, 2 minutes at most,
  until you **let in** or **turn away**. Each shows its name, the start
  of its computer's key, which tells two of the same name apart, and
  whether it joins from your network or from the internet. Guests who
  come back after a drop aren't asked about.

Your computer checks each of these as a guest asks, whatever the guest's
MixMusic says.

**make host**, beside a guest, moves the room to that guest's computer, if
its user agrees and the others, and yours, can reach it. The guest's page
asks **host the room** or **decline**, for 50 seconds, once a minute at
most, and no more from a host they declined twice; once they agree,
their computer opens a room of its own, which keeps everyone's place, and
each computer tries it. Once all reach it, that guest's player takes the
room's queue, where yours played it, in place of their own, and you and
the others are guests of it, as after a drop. When they decline, or one
can't reach it, or it takes longer than 10 seconds, or you remove or
block someone meanwhile, the room stays with you, and you're told why.
No one joins while the room moves. The new host's room has a new code and
invite link, keeps the computers you blocked out, and, when it asks
before someone joins, asks about each guest too as they come back.

## Dropping off

A guest whose computer sleeps, or loses the network, comes back by
itself, without the code: after a second, then a little longer each
time, up to half a minute, for about 15 minutes, and as soon as its
computer wakes. The host keeps its place 15 minutes. When the host ends the room, or
can't be reached, the guest's page says **The room ended**, and its own
queue comes back.

## Safe on a shared network

What the computers of a room send is encrypted, and only they can read
it. The code is never sent: each guest and the host prove to each other
that they know it, and someone who tries codes has one chance in more than
a billion each time. As the room takes 30 wrong codes a minute at most,
trying them all would take decades. An invite link names the host's key,
so it only opens that computer's room. [How it works inside](architecture/rooms.md)
tells more.

## Firewalls and the local network

While a room is open, MixMusic listens for the guests on your local
network, as for remote control: the first time, your system may ask
about it. [Remote control](remote-control.md#firewalls-and-the-local-network)
tells what to answer, on Windows, macOS and Linux. Guests listen for
nothing.

## Friends outside your network

Friends who live apart join a room too: MixMusic has no server, so they
reach your computer directly.

1. On your room's page, turn on **Let friends outside this network
   join**. MixMusic asks your router to forward the room's port, and the
   page says what it set up: "Your router forwards port 41234 of its
   internet address, …, to this computer", or why it couldn't.
2. Send friends the invite link: **copy invite link**, or **copy web
   link**, an `https://mixmusic.functionx.dev/join/#…` address, for chat
   apps that don't open `mixmusic://` links. Its page opens the room in
   MixMusic, or offers the download. The room's code and address stay
   after the `#`, which browsers never send to the website.
3. Friends open the link, or paste it as **Invite link** on their Listen
   with friends page.

MixMusic can't test the port from outside: ask a friend to try the link.
A friend who can't connect is told the likely causes. Anyone with the
link, or the code, may knock: **remove**, **block** and **new code** keep
them out, as on your network.

- **Routers.** MixMusic asks by UPnP, NAT-PMP or PCP, for that one TCP
  port alone, renews the forward while the room lasts and removes it as
  it ends: each forward goes by itself within 30 minutes, even if
  MixMusic stops at once. A router with all three off answers nothing,
  and one that takes only forwards that last for good is refused: turn
  UPnP on in its settings, or forward the port to this computer by hand.
- **Who knocks.** Anyone the link reaches may knock, as links shared in
  chats go further than the friends they're sent to. The room holds a
  few knocks of an address at once, and each waits 5 minutes at most to
  join, so that no one crowds friends out; wrong codes from the internet
  don't stop codes on your network. While the switch is off, the room
  takes no one from outside your network.
- **Behind another router.** A router whose own internet address starts
  with 100.64 to 100.127, or is private, sits behind another, most often
  your provider's, out of reach from outside. A VPN you both run, such as
  Tailscale, is the way then.
- **IPv6.** A computer with a public IPv6 address is reached there by
  friends with IPv6 too, where your router lets the port in: MixMusic
  asks it to by PCP.
- **Address for invites**, in Settings › Desktop › Listen with friends,
  takes an address in place of what your router tells: this computer's
  address on a VPN you both run, or your router's, with a port you
  forwarded by hand. **Port for rooms** keeps the port rooms listen on,
  picked at random the first time and kept, so that a forward made by
  hand keeps working.
- **A new address.** If your router's internet address changes, the
  room's page says so and offers the new link to send. Friends who
  dropped off come back as they open it.
- **Joining at an address.** **Join a room…**, in the cast button's menu
  and in Settings › Desktop › Listen with friends, opens the Listen with
  friends page, which takes an invite link, or a room's address, with its
  port, and its code.
- **Firewalls.** On Windows, the rule that Settings › Desktop › Windows
  Firewall adds lets in any address on private and domain networks, so a
  forwarded port works. On macOS, the firewall must allow MixMusic's
  incoming connections, and on Linux, ufw or firewalld must let the port
  in, as `sudo ufw allow 41234/tcp` does, with your room's port.

## Not yet

- Phones and browsers joining a room, which waits for the phone page
  (#560).
- A guest's sound mood (#547) and Skip silence (#552) resting in a
  room, once they're in MixMusic.

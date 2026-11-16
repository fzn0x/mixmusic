# Port mapping

A room of [Listen with friends](rooms.md) that friends outside the
network join (#562) must be reached from the internet, and MixMusic has
no server: guests reach the host's computer directly. Most homes are
behind a router that translates one internet address of IPv4 for all
their computers, and lets nothing in unasked. `internal/portmap` asks the
router to forward one TCP port, the room's, to this computer, and learns
the router's internet address from its answer.

## Asking the router

`Client.Map` asks, in turn, until one answers:

1. **PCP** (RFC 6887): a MAP request over UDP to the router's port 5351,
   which names this computer's address, a nonce, and the TCP port,
   answered with the external address and port the router gives, and a
   lifetime.
2. **NAT-PMP** (RFC 6886), which older routers speak: the router's
   internet address, then the forward of a TCP port, each answered with a
   result code. A router of NAT-PMP alone answers PCP with its version, 0,
   which has the client ask by NAT-PMP at once.
3. **UPnP IGD** (Internet Gateway Device, v1 and v2): a search by SSDP,
   over UDP to 239.255.255.250:1900, which the router answers with where
   its description is; the description, over HTTP, which names the
   control of its WANIPConnection or WANPPPConnection service; and SOAP
   actions of that service: AddPortMapping, GetExternalIPAddress and
   DeletePortMapping. Whatever answers the search can name any address,
   so only a description and a control at the address of the one that
   answered are asked, each read up to 256 KiB, and only of a default
   gateway: any other device may answer the search, and the address it
   told would go in the links. A router that takes forwards for good
   alone (error 725) is refused, as a forward left after a crash would
   stay, to whatever computer later has this one's address: the page asks
   for a forward made by hand. A port another computer has (718) has
   another tried.

PCP and NAT-PMP go to the default gateways, which Go has no call for:
`/proc/net/route` and `/proc/net/ipv6_route` on Linux,
`GetIpForwardTable2` on Windows, and a dump of the routing table by
`sysctl` on macOS (`gateway_*.go`). Each is read apart from the system, so
that the tests read each everywhere. UPnP needs no gateway. A question by
UDP is sent again after 250 ms, then twice as long, 4 times in all; a
refused connection, as from a router that doesn't speak the protocol,
ends it at once.

The router is asked for one TCP port, for 30 minutes (`Keeper`), renewed
at half the lifetime it gave, and removed as the room ends: what a
MixMusic that ended at once left goes soon. MixMusic waits a few seconds
at most for the removal as it quits.

## IPv6

IPv6 isn't translated: a computer with a global address of IPv6 is
reached there by guests with IPv6 too, unless the router turns away what
comes in unasked, as most do. `Client.Open` asks the router of IPv6 by
PCP, over IPv6 to its link-local address, to let the port of that
address in. `PublicIPv6` takes the global unicast addresses of the
networks that are up, but unique local ones and those of the
documentation, Teredo and 6to4, which guests seldom reach, and those
made of the network card's hardware address (EUI-64), which a link
forwarded beyond friends would tell: temporary and privacy addresses
alone.

## Behind another router

A router whose own internet address is in 100.64.0.0/10, which providers
share among their customers (RFC 6598), or in a private range, sits
behind another router, most often the provider's, that no one asked:
the internet doesn't reach the forward (`Shared`). `Keeper` removes it,
and tells why (`SharedError`). A VPN both sides run is the way then, with
its address typed for invites. An internet address that is multicast,
broadcast or reserved isn't one (`Unicast`): the forward goes too, and
the router counts as having refused.

## Keeping it

`Keeper` keeps the port reachable while a room is open: it asks for the
forward of IPv4 and the opening of IPv6, renews both at half the
lifetime they were given, 15 minutes at most, and at once when `Poke`, as
after the computer woke; after a failure, it asks again in 15 minutes. It
tells what it set up (`Reach`): where the internet reaches the port and
how it was asked, or why nothing does (`ErrNoRouter`, a `RefusedError`, a
`SharedError`), and the IPv6 address, opened or not. A forward the router
forgot is asked again; a router whose internet address changed tells the
new one, which the room's link then carries.

MixMusic can't test the port from outside: it tells what the router
answered, and the guest that can't connect is told the likely causes.

## Tests

The tests ask the fake routers of `portmaptest`, on the loopback: PCP and
NAT-PMP on one UDP port, of 127.0.0.1 and ::1, and UPnP's search, its
description and its control over HTTP, as their specifications have them,
which keep the forwards asked of them. A router of each protocol alone,
one that refuses, one that takes forwards for good alone, a port taken,
an address behind another router and an address that changes are each
tested. Nothing in the tests, nor in the app's, asks a real router: the
app asks one only without `RoomOptions`, as MixMusic itself runs.

// Package room is Listen with friends (#561): a room that one MixMusic
// hosts on the network, which the others join by its code or its invite
// link, and whose guests play what the host plays, in step with it. The
// host's audio is never sent: each guest plays the same track from where
// it can. It speaks over remote control's transport (internal/remote): each
// computer's key, TLS 1.3 and its framing, and announces the rooms by
// DNS-SD over mDNS. docs/architecture/rooms.md tells why it is so.
package room

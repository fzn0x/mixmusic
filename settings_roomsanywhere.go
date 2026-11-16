package main

// Settings › Desktop › Listen with friends, for friends outside this
// network (#562): Port for rooms, Address for invites, and Join a room….
// The rows of the group register from their features' files.

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/room"
)

// roomRows are the rows of Settings › Desktop › Listen with friends that
// register from their features' files.
const roomRows = "desktop › Listen with friends"

func init() { addSettings(roomRows, 10, (*shell).joinRoomRow) }

// joinRoomRow opens the rooms' page, which takes an invite link, or an
// address and a code.
func (s *shell) joinRoomRow(c *ui.Context) {
	rs := s.a.Rooms()
	if _, hosts := rs.Hosted(); hosts {
		return
	}
	s.row(c, "Join a room…", "Paste an invite link, or type a room's address and its code.", func() {
		if s.button(c, "room-settings-join", "join a room", false).Clicked() {
			s.navigate(roomsPath)
		}
	}, "listen with friends room join link address code")
}

func init() { addSettings(roomRows, 20, (*shell).roomPortRow) }

// roomPortRow keeps the port rooms listen on, which a port forwarded by
// hand to this computer needs.
func (s *shell) roomPortRow(c *ui.Context) {
	saved := ""
	if s.set.RoomPort != 0 {
		saved = strconv.Itoa(s.set.RoomPort)
	}
	s.row(c, "Port for rooms", "The TCP port your rooms listen on, from 1024 to 65535, picked at random as your first room opens and kept, so that a port forwarded by hand keeps working. A change applies to the next room.", func() {
		in, _ := s.settingInput(c, "room-port", saved, func(v string) {
			p, err := strconv.Atoi(v)
			if v != "" && (err != nil || p < 1024 || p > 65535) {
				s.notes = append(s.notes, notice("A port for rooms is a number from 1024 to 65535"))
				return
			}
			s.set.RoomPort = p
			s.save()
		})
		in.Placeholder("picked as a room opens").Width(160).Label("The port rooms listen on")
	}, "port forward router listen with friends room internet")
}

func init() { addSettings(roomRows, 30, (*shell).roomAddressRow) }

// roomAddressRow keeps the address invites carry in place of those
// MixMusic finds.
func (s *shell) roomAddressRow(c *ui.Context) {
	s.row(c, "Address for invites", "Where friends outside this network reach your rooms, in place of what your router tells: this computer's address on a VPN you both run, such as Tailscale, or your router's, with a port forwarded by hand. Empty, MixMusic asks your router.", func() {
		in, _ := s.settingInput(c, "room-address", s.set.RoomAddress, func(v string) {
			if _, ok := room.InviteAddr(v, 1024); v != "" && !ok {
				s.notes = append(s.notes, notice("That isn't an address: type a name or an IP address, with a port or not"))
				return
			}
			s.set.RoomAddress = v
			s.save()
			s.a.Rooms().ReachAgain()
		})
		in.Placeholder("found by MixMusic").Width(220).Label("The address invites carry")
	}, "address vpn tailscale invite internet router listen with friends room")
}

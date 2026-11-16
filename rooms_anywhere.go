package main

// Friends anywhere (#562), in the main window: the switch of the room page
// that lets friends outside this network join, with what MixMusic set up
// for them, or why they can't reach the room, and the suggestion that the
// host let in each friend who joins; the room's https link, for chat apps
// that don't open mixmusic:// links; joining a room at an address, with
// its code; and Join a room… in the devices menu.
// internal/app's roomreach.go and roomfar.go run them.

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/room"
	"github.com/fzn0x/mixmusic/locales"
)

// roomReach shows, on the page of the room this computer hosts, whether
// friends outside this network may join, and how they reach it.
func (s *shell) roomReach(c *ui.Context, h app.HostedRoom) {
	k := &s.lk
	rs := s.a.Rooms()
	reach, ok := rs.Reach()
	if !ok {
		return
	}
	ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Text(c, locales.RoomReachTitle()).FontSize(k.bodySize())
			ui.Text(c, locales.RoomReachDesc()).
				FontSize(k.smallSize()).TextColor(k.muted)
		})
		if s.switchOf(c.Key("room-anywhere"), &s.set.RoomAnywhere).Label(locales.RoomReachTitle()).Changed() {
			s.save()
			rs.SetAnywhere(s.set.RoomAnywhere)
			// Turned on, it suggests Ask me before someone joins (suggestAskJoin).
			s.rooms.suggestAsk = s.set.RoomAnywhere && !s.set.RoomAskJoin
		}
	})
	if !reach.On {
		return
	}
	s.suggestAskJoin(c)
	note := func(text string) { ui.Text(c, text).FontSize(k.smallSize()).TextColor(k.muted).MaxWidth(640) }
	if reach.Asking {
		note(locales.RoomReachAsking())
	}
	for _, d := range reach.Done {
		ui.Text(c, d).FontSize(k.bodySize()).MaxWidth(640)
	}
	if reach.Problem != "" {
		ui.Text(c, reach.Problem).FontSize(k.bodySize()).TextColor(k.danger).MaxWidth(640)
	}
	if reach.Moved {
		ui.Text(c, locales.RoomReachMoved()).
			FontSize(k.bodySize()).TextColor(k.danger).MaxWidth(640)
	}
	ui.Row(c).Gap(8).Children(func() {
		label := locales.RoomReachCopyWebLink()
		if reach.Moved {
			label = locales.RoomReachCopyNewWebLink()
		}
		if s.button(c, "room-copy-web-link", label, reach.Moved).Clicked() {
			if in, err := room.ParseLink(h.Link); err == nil {
				copyText(in.WebLink())
				rs.LinkCopied()
				s.notes = append(s.notes, notice(locales.RoomReachCopied()))
			}
		}
	})
	note(locales.RoomReachTestNote())
}

// suggestAskJoin suggests, once the user let friends outside this network
// join, that they turn on Ask me before someone joins, as anyone the link
// reaches may knock: they turn it on, or not, and the suggestion goes. It
// goes too once the rule is on, from the foot of the page or Settings.
func (s *shell) suggestAskJoin(c *ui.Context) {
	v := &s.rooms
	if s.set.RoomAskJoin {
		v.suggestAsk = false
	}
	if !v.suggestAsk {
		return
	}
	k := &s.lk
	ui.Text(c, "Turn on Ask me before someone joins, and whoever comes with the link or the code waits until you let them in.").
		FontSize(k.bodySize()).MaxWidth(640)
	ui.Row(c).Gap(8).Children(func() {
		if s.button(c, "room-suggest-ask", "turn it on", false).Label("Turn on Ask me before someone joins").Clicked() {
			s.set.RoomAskJoin = true
			s.saveRoomRules()
			v.suggestAsk = false
		}
		if s.button(c, "room-suggest-ask-no", "not now", false).Clicked() {
			v.suggestAsk = false
		}
	})
}

// joinAtAddress joins a room at an address, as a VPN's or a router's, with
// its code, on the rooms' page.
func (s *shell) joinAtAddress(c *ui.Context) {
	k := &s.lk
	v := &s.rooms
	ui.Text(c, locales.RoomReachAddressIntro()).
		FontSize(k.smallSize()).TextColor(k.muted).MaxWidth(640)
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.TextInput(c.Key("room-at-addr"), &v.addr).Width(220).Label(locales.RoomReachAddressLabel()).Placeholder("203.0.113.7:41234")
		in := ui.TextInput(c.Key("room-at-code"), &v.addrCode).Width(120).Label(locales.RoomReachCodeLabel()).Placeholder("ABC 123")
		_, codeOK := room.ParseCode(v.addrCode)
		_, addrOK := room.InviteAddr(v.addr, 0)
		knock := in.Submitted()
		if s.button(c, "room-knock-at", locales.RoomReachJoinButton(), false).Label(locales.RoomReachJoinAtAddress()).Disabled(!codeOK || !addrOK).Clicked() {
			knock = true
		}
		if knock && codeOK && addrOK {
			s.a.Rooms().KnockAt(v.addr, v.addrCode)
			v.addrCode = ""
		}
	})
}

// joinRoomItem offers Join a room… in the devices menu, which opens the
// rooms' page, unless this computer hosts a room or is in one.
func (s *shell) joinRoomItem(m *ui.Menu) {
	rs := s.a.Rooms()
	if _, hosts := rs.Hosted(); hosts {
		return
	}
	if g, ok := rs.Guest(); ok && g.Step != app.RoomEnded {
		return
	}
	if !s.roomsShown() {
		m.Separator()
	}
	if m.Item(locales.RoomReachJoinMenu()).Chosen() {
		s.navigate(roomsPath)
	}
}

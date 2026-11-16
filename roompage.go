package main

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/remote"
	"github.com/fzn0x/mixmusic/internal/room"
	"github.com/fzn0x/mixmusic/locales"
)

// roomsTitle is the title of the rooms' page and of their settings.
func roomsTitle() string { return locales.RoomPageTitle() }

// roomPage is the page of rooms: the room this computer hosts, with its
// code, its invite link and its guests; or the one it is a guest of, with
// what plays there; or else Start a room, and the rooms on the network.
func (s *shell) roomPage(c *ui.Context) {
	rs := s.a.Rooms()
	s.pageScroll(c, func() {
		s.pageHeader(c, roomsTitle(), nil, nil)
		ui.Column(c).MaxWidth(720).Gap(16).Children(func() {
			if h, ok := rs.Hosted(); ok {
				s.hostedRoom(c, h)
				return
			}
			if g, ok := rs.Guest(); ok {
				s.guestRoom(c, g)
				return
			}
			s.roomsLanding(c)
		})
	})
}

// roomFoundPage is the page of the room of the host id, found on the
// network: the code to type, or what the room shows once it was.
func (s *shell) roomFoundPage(c *ui.Context, id string) {
	rs := s.a.Rooms()
	if g, ok := rs.Guest(); ok && g.ID == id {
		s.roomPage(c)
		return
	}
	name := locales.RoomPageThatComputer()
	for _, f := range rs.Found() {
		if f.ID == id {
			name = f.Name
		}
	}
	k := &s.lk
	s.pageScroll(c, func() {
		s.pageHeader(c, locales.RoomPageNameRoom(name), nil, nil)
		ui.Column(c).MaxWidth(720).Gap(16).Children(func() {
			ui.Text(c, locales.RoomPageTypeCode(name)).FontSize(k.bodySize()).MaxWidth(640)
			v := &s.rooms
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				in := ui.TextInput(c.Key("room-code"), &v.code).Width(160).Label(locales.RoomPageRoomCode()).Placeholder("ABC 123").AutoFocus()
				_, ok := room.ParseCode(v.code)
				knock := in.Submitted()
				if s.button(c, "room-knock", locales.RoomPageJoin(), true).Disabled(!ok).Clicked() {
					knock = true
				}
				if knock && ok {
					rs.Knock(id, v.code)
					v.code = ""
				}
			})
		})
	})
}

// roomsLanding tells what rooms are, opens one, and lists those on the
// network.
func (s *shell) roomsLanding(c *ui.Context) {
	k := &s.lk
	rs := s.a.Rooms()
	ui.Text(c, locales.RoomPageIntro()).
		FontSize(k.bodySize()).MaxWidth(640)
	if s.button(c, "room-start", locales.RoomPageStartRoom(), true).Disabled(s.rooms.starting).Clicked() {
		s.startRoom()
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		v := &s.rooms
		in := ui.TextInput(c.Key("room-link"), &v.link).Width(320).Label(locales.RoomPageInviteLink()).Placeholder("mixmusic://join/…")
		_, err := room.ParseLink(v.link)
		open := in.Submitted()
		if s.button(c, "room-open-link", locales.RoomPageOpenLink(), false).Disabled(err != nil).Clicked() {
			open = true
		}
		if open && err == nil {
			if rs.OpenLink(v.link) == nil {
				v.link = ""
			}
		}
	})
	s.joinAtAddress(c)
	found := rs.Found()
	if len(found) == 0 {
		ui.Text(c, locales.RoomPageNoRooms()).
			FontSize(k.smallSize()).TextColor(k.muted).MaxWidth(640)
		return
	}
	ui.Text(c, locales.RoomPageRoomsOnNetwork()).FontSize(k.subtitleSize()).FontWeight(k.leadWeight())
	for _, f := range found {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.RoomPageNameRoom(f.Name)).FontSize(k.bodySize()).Grow(1)
			if s.button(c, "room-open-"+f.ID, locales.RoomPageJoin(), false).Label(locales.RoomPageJoinRoom(f.Name)).Clicked() {
				s.navigate(roomPath(f.ID))
			}
		})
	}
}

// hostedRoom shows the room this computer hosts: its code, its invite
// link, its guests with Remove and Block, those blocked with Unblock, and
// what happened in it.
func (s *shell) hostedRoom(c *ui.Context, h app.HostedRoom) {
	k := &s.lk
	rs := s.a.Rooms()
	ui.Text(c, locales.RoomPageJoinHint()).
		FontSize(k.bodySize()).MaxWidth(640)
	shown := room.ShowCode(h.Code)
	ui.Text(c, shown).FontSize(k.size(2.5)).FontWeight(k.titleWeight()).FontFeatures("tnum").Selectable().Label(locales.RoomPageCodeLabel(shown))
	ui.Row(c).Gap(8).Children(func() {
		if s.button(c, "room-copy-link", locales.RoomPageCopyLink(), true).Clicked() {
			copyText(h.Link)
			rs.LinkCopied()
			s.notes = append(s.notes, notice(locales.RoomPageCopiedLink()))
		}
		if s.button(c, "room-new-code", locales.RoomPageNewCode(), false).Clicked() {
			s.newRoomCode()
		}
		if s.button(c, "room-end", locales.RoomPageEndRoom(), false).Clicked() {
			rs.End()
		}
	})
	// Friends outside this network (rooms_anywhere.go).
	s.roomReach(c, h)
	ui.Text(c, locales.RoomPageRemovedNote()).
		FontSize(k.smallSize()).TextColor(k.muted).MaxWidth(640)
	if h.Waiting {
		ui.Text(c, locales.RoomPageWaitingGuests()).FontSize(k.smallSize()).TextColor(k.muted)
	}
	s.roomWaiting(c)
	ui.Text(c, locales.RoomPageGuests()).FontSize(k.subtitleSize()).FontWeight(k.leadWeight())
	if len(h.Guests) == 0 {
		ui.Text(c, locales.RoomPageNoGuests()).FontSize(k.smallSize()).TextColor(k.muted)
	}
	for _, g := range h.Guests {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			name := g.Name
			if !g.Here {
				name = locales.RoomPageAway(g.Name)
			}
			ui.Text(c, name).FontSize(k.bodySize()).Grow(1).MinWidth(0)
			if s.button(c, "room-remove-"+g.Key, locales.RoomPageRemove(), false).Label(locales.RoomPageRemoveGuest(g.Name)).Clicked() {
				rs.Remove(g.Key)
			}
			if s.button(c, "room-block-"+g.Key, locales.RoomPageBlock(), false).Label(locales.RoomPageBlockGuest(g.Name)).Clicked() {
				rs.Block(g.Key)
			}
			s.makeHostButton(c, g)
		})
	}
	if len(h.Blocked) > 0 {
		ui.Text(c, locales.RoomPageBlocked()).FontSize(k.subtitleSize()).FontWeight(k.leadWeight())
		for _, g := range h.Blocked {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, g.Name).FontSize(k.bodySize()).Grow(1).MinWidth(0)
				if s.button(c, "room-unblock-"+g.Key, locales.RoomPageUnblock(), false).Label(locales.RoomPageUnblockGuest(g.Name)).Clicked() {
					rs.Unblock(g.Key)
				}
			})
		}
	}
	if len(h.Activity) > 0 {
		ui.Text(c, locales.RoomPageActivity()).FontSize(k.subtitleSize()).FontWeight(k.leadWeight())
		for i := len(h.Activity) - 1; i >= 0 && i >= len(h.Activity)-20; i-- {
			act := h.Activity[i]
			ui.Text(c, locales.RoomPageActivityLine(act.Name, roomActivityWhat(act.What), activitySong(act), act.At.Local().Format("15:04"))).FontSize(k.smallSize()).TextColor(k.muted)
		}
	}
	s.hostRules(c)
}

// guestRoom shows the room this computer is a guest of, or knocks at: who
// hosts and what plays before it joins, then what plays, who listens, and
// Leave room.
func (s *shell) guestRoom(c *ui.Context, g app.GuestView) {
	k := &s.lk
	rs := s.a.Rooms()
	say := func(text string) { ui.Text(c, text).FontSize(k.bodySize()).MaxWidth(640) }
	if g.Problem != "" {
		ui.Text(c, g.Problem).FontSize(k.bodySize()).TextColor(k.danger).MaxWidth(640)
	}
	switch g.Step {
	case app.RoomKnocking:
		say(locales.RoomPageChecking(g.Host))
		if s.button(c, "room-cancel", locales.LibraryCancel(), false).Clicked() {
			rs.Leave()
		}
	case app.RoomPreview:
		p := g.Preview
		say(locales.RoomPageHosts(p.Host))
		if p.Title != "" {
			line := locales.RoomPagePlays(p.Title)
			if p.Artists != "" {
				line = locales.RoomPagePlaysArtists(p.Title, p.Artists)
			}
			say(line)
		}
		say(locales.RoomPageJoinPlays(listening(p.Listeners), p.Host))
		ui.Row(c).Gap(8).Children(func() {
			if s.button(c, "room-join", locales.RoomPageJoin(), true).Clicked() {
				rs.Join()
			}
			if s.button(c, "room-cancel", locales.LibraryCancel(), false).Clicked() {
				rs.Leave()
			}
		})
	case app.RoomJoining:
		say(s.joiningLine(g.Host))
	case app.RoomIn, app.RoomAway:
		s.inRoom(c, g)
	case app.RoomEnded:
		if s.button(c, "room-close", locales.CopiesClose(), false).Clicked() {
			rs.Dismiss()
		}
	}
}

// inRoom shows what plays in the room this computer is in, who listens,
// and what comes next there.
func (s *shell) inRoom(c *ui.Context, g app.GuestView) {
	k := &s.lk
	rs := s.a.Rooms()
	title := locales.RoomPageInRoom(g.Host)
	if g.Step == app.RoomAway {
		title = locales.RoomPageStopped(g.Host)
	}
	ui.Text(c, title).FontSize(k.subtitleSize()).FontWeight(k.leadWeight())
	st := g.State
	if !g.HasState || st.Track == nil {
		ui.Text(c, locales.RoomPageNothingPlays()).FontSize(k.bodySize())
	} else {
		t := *st.Track
		ui.Row(c).Gap(20).AlignItems(ui.Center).Children(func() {
			var art []media.Image
			for _, img := range t.Artwork {
				art = append(art, media.Image{URL: img.URL, Width: img.Width, Height: img.Height})
			}
			s.cover(c, "room-cover", art, 120, false)
			ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				ui.Text(c, t.Title).FontSize(k.subtitleSize()).FontWeight(k.leadWeight())
				ui.Text(c, roomArtists(t)).FontSize(k.smallSize()).TextColor(k.muted)
				if !st.Playing {
					ui.Text(c, locales.NotchPanelPaused()).FontSize(k.smallSize()).TextColor(k.muted)
				}
			})
		})
	}
	if g.Note != "" {
		ui.Text(c, g.Note).FontSize(k.bodySize()).TextColor(k.muted).MaxWidth(640)
	}
	if len(g.Members) > 0 {
		ui.Text(c, locales.RoomPageListening(strings.Join(g.Members, ", "))).FontSize(k.smallSize()).TextColor(k.muted).MaxWidth(640)
	}
	s.roomGuestRows(c, g)
	if s.button(c, "room-leave", locales.RoomPageLeaveRoom(), false).Clicked() {
		rs.Leave()
	}
	q := g.Queue
	if len(q.Tracks) > q.Index+1 {
		ui.Text(c, locales.RoomPageNextInRoom()).FontSize(k.subtitleSize()).FontWeight(k.leadWeight())
		for _, t := range q.Tracks[q.Index+1 : min(len(q.Tracks), q.Index+11)] {
			ui.Text(c, locales.RoomPageTitleArtists(t.Title, roomArtists(t))).FontSize(k.smallSize()).MaxWidth(640)
		}
	}
}

// roomArtists names the artists of a track of the room.
func roomArtists(t remote.QueueTrack) string {
	names := make([]string, 0, len(t.Artists))
	for _, a := range t.Artists {
		names = append(names, a.Name)
	}
	return strings.Join(names, ", ")
}

// roomActivityWhat names what a guest did in the room, as the room's activity tells it.
func roomActivityWhat(what string) string {
	switch what {
	case room.Joined:
		return locales.RoomPageJoinedActivity()
	case room.Left:
		return locales.RoomPageLeftActivity()
	case room.Removed:
		return locales.RoomPageRemovedActivity()
	case room.Blocked:
		return locales.RoomPageBlockedActivity()
	case room.Guessed:
		return locales.RoomPageGuessedActivity()
	}
	return what
}

package main

// The host's rules for the guests of its room (#734), as rows of Settings ›
// Desktop › Listen with friends and of the room's page; what waits for the
// host there, songs to accept and computers to let in; Make host; and, for
// a guest, Add to the room in a song's menu and the host's player's
// controls, as the host's rules let them.

import (
	"slices"
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/room"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// roomSongChoices are what becomes of guests' songs, as the settings name
// it, in the order the row shows them.
var roomSongChoices = []string{settings.RoomSongsAdd, settings.RoomSongsAsk, settings.RoomSongsOff}

// roomRuleRows are the rows of the host's rules: Songs from guests, Guests
// can control playback, and Ask me before someone joins.
func (s *shell) roomRuleRows(c *ui.Context) {
	s.row(c, "Songs from guests", "What becomes of the songs your guests add from a song's menu: added to your queue, each waiting for you to accept it, or none.", func() {
		chosen := slices.Index(roomSongChoices, s.set.RoomSongs)
		if s.segmented(c, "room-songs", "Songs from guests", &chosen, "add them", "ask me first", "off") && chosen >= 0 {
			s.set.RoomSongs = roomSongChoices[chosen]
			s.saveRoomRules()
		}
	}, "guests songs requests queue add room")
	s.row(c, "Guests can control playback", "Your guests can play, pause and skip the tracks of your room.", func() {
		if s.switchOf(c.Key("room-guest-control"), &s.set.RoomGuestControl).Label("Guests can control playback").Changed() {
			s.saveRoomRules()
		}
	}, "guests control play pause skip room")
	s.row(c, "Ask me before someone joins", "Each friend who joins with the code or the invite link waits for you to let them in.", func() {
		if s.switchOf(c.Key("room-ask-join"), &s.set.RoomAskJoin).Label("Ask me before someone joins").Changed() {
			s.saveRoomRules()
		}
	}, "join ask let in knock guests room")
}

// saveRoomRules keeps the host's rules, and tells the guests.
func (s *shell) saveRoomRules() {
	s.save()
	s.a.Rooms().RulesChanged()
}

// roomWaiting shows the host what waits for them: computers to let in,
// and songs to accept; and the room moving to a guest, while it does.
func (s *shell) roomWaiting(c *ui.Context) {
	k := &s.lk
	rs := s.a.Rooms()
	if name, ok := rs.Moving(); ok {
		ui.Text(c, "Waiting for "+name+" to host the room…").FontSize(k.bodySize())
	}
	if knocks := rs.Knocks(); len(knocks) > 0 {
		ui.Text(c, "Asking to join").FontSize(k.subtitleSize()).FontWeight(k.leadWeight())
		for _, kn := range knocks {
			id := strconv.FormatInt(kn.ID, 10)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, kn.Name).FontSize(k.bodySize()).Grow(1).MinWidth(0)
				if s.button(c, "room-let-in-"+id, "let in", true).Label("Let " + kn.Name + " in").Clicked() {
					rs.LetIn(kn.ID, true)
				}
				if s.button(c, "room-turn-away-"+id, "turn away", false).Label("Turn " + kn.Name + " away").Clicked() {
					rs.LetIn(kn.ID, false)
				}
			})
		}
	}
	if reqs := rs.Requests(); len(reqs) > 0 {
		ui.Text(c, "Songs from guests").FontSize(k.subtitleSize()).FontWeight(k.leadWeight())
		for _, r := range reqs {
			id := strconv.FormatInt(r.ID, 10)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				line := r.Title
				if r.Artist != "" {
					line += " · " + r.Artist
				}
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, line).FontSize(k.bodySize())
					ui.Text(c, "From "+r.From).FontSize(k.smallSize()).TextColor(k.muted)
				})
				if s.button(c, "room-accept-"+id, "accept", true).Label("Accept " + r.Title).Clicked() {
					s.acceptSong(r.ID)
				}
				if s.button(c, "room-decline-"+id, "decline", false).Label("Decline " + r.Title).Clicked() {
					rs.Decline(r.ID)
				}
			})
		}
	}
}

// hostRules show the host what its guests can do, at the foot of its
// room's page.
func (s *shell) hostRules(c *ui.Context) {
	k := &s.lk
	ui.Text(c, "What guests can do").FontSize(k.subtitleSize()).FontWeight(k.leadWeight())
	s.roomRuleRows(c)
}

// acceptSong adds the song of the request id to the queue beside the main
// thread, as it may find it first, and tells why it can't.
func (s *shell) acceptSong(id int64) {
	go func() {
		err := s.a.Rooms().Accept(id)
		s.update(func() {
			if err != nil {
				s.notes = append(s.notes, noticeOf(err))
			}
		})
	}()
}

// makeHostButton moves the room to the guest g, once its MixMusic can
// host it.
func (s *shell) makeHostButton(c *ui.Context, g app.RoomMember) {
	rs := s.a.Rooms()
	if !g.Here || !rs.CanHost(g.Key) {
		return
	}
	if s.button(c, "room-make-host-"+g.Key, "make host", false).Label("Make " + g.Name + " the host").Clicked() {
		key := g.Key
		go func() {
			err := s.a.Rooms().MakeHost(key)
			s.update(func() {
				if err != nil {
					s.notes = append(s.notes, noticeOf(err))
				}
			})
		}()
	}
}

// roomGuestRows show a guest the host's ask that it host the room, while
// it waits, and what the host lets it do: add songs from a song's menu,
// and control the host's player.
func (s *shell) roomGuestRows(c *ui.Context, g app.GuestView) {
	k := &s.lk
	rs := s.a.Rooms()
	if host, ok := rs.HostAsk(); ok {
		ui.Text(c, host+" asks you to host the room: this computer would play the room's queue, in place of yours, and the others would join it here.").
			FontSize(k.bodySize()).MaxWidth(640)
		ui.Row(c).Gap(8).Children(func() {
			if s.button(c, "room-host-yes", "host the room", true).Clicked() {
				rs.AnswerHost(true)
			}
			if s.button(c, "room-host-no", "decline", false).Label("Don't host the room").Clicked() {
				rs.AnswerHost(false)
			}
		})
	}
	r, ok := rs.RoomRules()
	if !ok || g.Step != app.RoomIn {
		return
	}
	switch r.Songs {
	case room.SongsAdd:
		ui.Text(c, "Add songs to the room from a song's menu.").FontSize(k.smallSize()).TextColor(k.muted).MaxWidth(640)
	case room.SongsAsk:
		ui.Text(c, "Add songs to the room from a song's menu: "+g.Host+" accepts each first.").FontSize(k.smallSize()).TextColor(k.muted).MaxWidth(640)
	}
	if !r.Control {
		return
	}
	control := func(key, label, do string) {
		if s.button(c, key, label, false).Clicked() {
			if err := rs.Control(do); err != nil {
				s.notes = append(s.notes, noticeOf(err))
			}
		}
	}
	ui.Row(c).Gap(8).Children(func() {
		control("room-previous", "previous", room.Previous)
		if g.HasState && g.State.Playing {
			control("room-pause", "pause", room.Pause)
		} else {
			control("room-play", "play", room.Play)
		}
		control("room-next", "next", room.Next)
	})
}

// roomSongItem adds t to the room this computer is in, or asks its host
// to, while the host takes songs.
func (s *shell) roomSongItem(m *ui.Menu, t media.Track) {
	r, ok := s.a.Rooms().RoomRules()
	if !ok || r.Songs == room.SongsOff {
		return
	}
	label := "Add to the room"
	if r.Songs == room.SongsAsk {
		label = "Ask to add to the room"
	}
	if m.Item(label).Chosen() {
		if err := s.a.Rooms().AddSong(t); err != nil {
			s.notes = append(s.notes, noticeOf(err))
		}
	}
}

// joiningLine is what the guest's page says as it joins: that the host's
// user lets it in first, when they do.
func (s *shell) joiningLine(host string) string {
	if s.a.Rooms().Asking() {
		return "Waiting for " + host + " to let you in…"
	}
	return "Joining " + host + "'s room…"
}

// activitySong names the song an activity tells of, if any.
func activitySong(act room.Activity) string {
	if act.Song == "" {
		return ""
	}
	return " " + act.Song
}

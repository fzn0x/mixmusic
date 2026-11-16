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
	"github.com/fzn0x/mixmusic/locales"
)

// roomSongChoices are what becomes of guests' songs, as the settings name
// it, in the order the row shows them.
var roomSongChoices = []string{settings.RoomSongsAdd, settings.RoomSongsAsk, settings.RoomSongsOff}

// roomRuleRows are the rows of the host's rules: Songs from guests, Guests
// can control playback, and Ask me before someone joins.
func (s *shell) roomRuleRows(c *ui.Context) {
	s.row(c, locales.RoomRulesSongsTitle(), locales.RoomRulesSongsAbout(), func() {
		chosen := slices.Index(roomSongChoices, s.set.RoomSongs)
		if s.segmented(c, "room-songs", locales.RoomRulesSongsTitle(), &chosen, locales.RoomRulesSongsAdd(), locales.RoomRulesSongsAsk(), locales.RoomRulesSongsOff()) && chosen >= 0 {
			s.set.RoomSongs = roomSongChoices[chosen]
			s.saveRoomRules()
		}
	}, "guests songs requests queue add room")
	s.row(c, locales.RoomRulesControlTitle(), locales.RoomRulesControlAbout(), func() {
		if s.switchOf(c.Key("room-guest-control"), &s.set.RoomGuestControl).Label(locales.RoomRulesControlTitle()).Changed() {
			s.saveRoomRules()
		}
	}, "guests control play pause skip room")
	s.row(c, locales.RoomRulesAskTitle(), locales.RoomRulesAskAbout(), func() {
		if s.switchOf(c.Key("room-ask-join"), &s.set.RoomAskJoin).Label(locales.RoomRulesAskTitle()).Changed() {
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
		ui.Text(c, locales.RoomRulesMoving(name)).FontSize(k.bodySize())
	}
	if knocks := rs.Knocks(); len(knocks) > 0 {
		ui.Text(c, locales.RoomRulesAskingJoin()).FontSize(k.subtitleSize()).FontWeight(k.leadWeight())
		for _, kn := range knocks {
			id := strconv.FormatInt(kn.ID, 10)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				where := locales.RoomRulesKnockInternet(kn.Key)
				if kn.Local {
					where = locales.RoomRulesKnockNetwork(kn.Key)
				}
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, kn.Name).FontSize(k.bodySize())
					ui.Text(c, where).FontSize(k.smallSize()).TextColor(k.muted)
				})
				if s.button(c, "room-let-in-"+id, locales.RoomRulesLetIn(), true).Label(locales.RoomRulesLetInName(kn.Name)).Clicked() {
					rs.LetIn(kn.ID, true)
				}
				if s.button(c, "room-turn-away-"+id, locales.RoomRulesTurnAway(), false).Label(locales.RoomRulesTurnAwayName(kn.Name)).Clicked() {
					rs.LetIn(kn.ID, false)
				}
			})
		}
	}
	if reqs := rs.Requests(); len(reqs) > 0 {
		ui.Text(c, locales.RoomRulesSongsTitle()).FontSize(k.subtitleSize()).FontWeight(k.leadWeight())
		for _, r := range reqs {
			id := strconv.FormatInt(r.ID, 10)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				line := r.Title
				if r.Artist != "" {
					line += " · " + r.Artist
				}
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, line).FontSize(k.bodySize())
					ui.Text(c, locales.RoomRulesFrom(r.From)).FontSize(k.smallSize()).TextColor(k.muted)
				})
				if s.button(c, "room-accept-"+id, locales.RoomRulesAccept(), true).Label(locales.RoomRulesAcceptName(r.Title)).Clicked() {
					s.acceptSong(r.ID)
				}
				if s.button(c, "room-decline-"+id, locales.RoomRulesDecline(), false).Label(locales.RoomRulesDeclineName(r.Title)).Clicked() {
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
	ui.Text(c, locales.RoomRulesWhatGuests()).FontSize(k.subtitleSize()).FontWeight(k.leadWeight())
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
	if s.button(c, "room-make-host-"+g.Key, locales.RoomRulesMakeHost(), false).Label(locales.RoomRulesMakeHostName(g.Name)).Clicked() {
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
		ui.Text(c, locales.RoomRulesHostAsk(host)).
			FontSize(k.bodySize()).MaxWidth(640)
		ui.Row(c).Gap(8).Children(func() {
			if s.button(c, "room-host-yes", locales.RoomRulesHostYes(), true).Clicked() {
				rs.AnswerHost(true)
			}
			if s.button(c, "room-host-no", locales.RoomRulesDecline(), false).Label(locales.RoomRulesDontHost()).Clicked() {
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
		ui.Text(c, locales.RoomRulesAddSongs()).FontSize(k.smallSize()).TextColor(k.muted).MaxWidth(640)
	case room.SongsAsk:
		ui.Text(c, locales.RoomRulesAskSongs(g.Host)).FontSize(k.smallSize()).TextColor(k.muted).MaxWidth(640)
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
		control("room-previous", locales.RoomRulesPrevious(), room.Previous)
		if g.HasState && g.State.Playing {
			control("room-pause", locales.RoomRulesPause(), room.Pause)
		} else {
			control("room-play", locales.RoomRulesPlay(), room.Play)
		}
		control("room-next", locales.RoomRulesNext(), room.Next)
	})
}

// roomSongItem adds t to the room this computer is in, or asks its host
// to, while the host takes songs.
func (s *shell) roomSongItem(m *ui.Menu, t media.Track) {
	r, ok := s.a.Rooms().RoomRules()
	if !ok || r.Songs == room.SongsOff {
		return
	}
	label := locales.RoomRulesAddToRoom()
	if r.Songs == room.SongsAsk {
		label = locales.RoomRulesAskToAddToRoom()
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
		return locales.RoomRulesWaitingToJoin(host)
	}
	return locales.RoomRulesJoining(host)
}

// activitySong names the song an activity tells of, if any.
func activitySong(act room.Activity) string {
	if act.Song == "" {
		return ""
	}
	return " " + act.Song
}

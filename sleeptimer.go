package main

// The sleep timer (#594) in the windows: the moon button by the player and
// its menu, a submenu of the mini player's menu, its commands, and its
// group of Settings › Playback. The player runs it
// (internal/player/sleep.go); the tray's submenu and its global shortcuts
// are the app's (internal/app/sleeptimer.go).

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// sleepMenuState is what the sleep timer's menu keeps while it's open: the
// tracks of After N tracks, and the minutes typed, with whether they were
// turned down.
type sleepMenuState struct {
	tracks  int
	minutes string
	bad     bool
}

// startSleep starts the sleep timer t, and closes the menu.
func (s *shell) startSleep(t player.SleepTimer, open *bool) {
	s.p.StartSleep(t)
	*open = false
}

// sleepButton is the moon button by the player, beside the speed button:
// while the timer runs, it shows how long it has left, to the second, in
// the accent. It opens sleepMenu.
func (s *shell) sleepButton(c *ui.Context, room *outputSlot) {
	k := &s.lk
	now := time.Now()
	left, on := s.st.SleepLeft(now)
	// With no timer, the moon gives way to the device button, which a
	// narrow window has room for alone: it shows once there is room for it
	// and a little more, and while it showed, until none is left; never while
	// the device button waits for room. A timer that runs always shows, as
	// its time left. Its menu is also in the palette, the mini player and
	// the tray.
	if !on {
		waits := s.outputChoice() && !room.shown
		if room.moon {
			room.moon = !waits && room.free >= 1
		} else {
			room.moon = !waits && room.free >= roomSleep+4
		}
		if !room.moon {
			// Without the gap before it.
			ui.Row(c.Key("sleep-slot")).Shrink(0).MarginEnd(-4)
			return
		}
	}
	label := locales.SleepTimerTitle()
	if on {
		label = locales.SleepTimerLeft(player.SleepClock(left))
	}
	if on && !s.st.Sleep.Ending() {
		// Again as the time shown changes, which the button rounds up: the
		// time of a timer that ends with a track moves with the track.
		c.After(max(left%time.Second, 10*time.Millisecond))
	}
	tip := ui.Box(c.Key("sleep-tip")).Shrink(0)
	open := ui.Local(tip, "open", func() bool { return false })
	menu := ui.Local(tip, "menu", func() sleepMenuState { return sleepMenuState{tracks: 2} })
	// The tip would cover the menu.
	if !*open {
		tip.Tooltip(label)
	}
	tip.Children(func() {
		b := s.buttonBase(c.Key("sleep")).Height(32).MinWidth(32).PaddingX(6).Radius(k.round(32)).Label(label).
			Cursor(ui.CursorPointer).Expanded(*open)
		ink := k.muted
		switch {
		case on:
			ink = k.accent
		case *open || b.Hovered():
			ink = k.text
		}
		if b.Pressed() {
			b.Background(k.press)
		} else if *open || b.Hovered() {
			b.Background(k.hover)
		}
		s.pressMotion(b, 0.9)
		if b.Clicked() {
			*open = !*open
		}
		b.Children(func() {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icon("moon")).FontSize(18).TextColor(ink)
				if on {
					ui.Text(c, player.SleepClock(left)).FontSize(k.smallSize()).FontWeight(600).FontFeatures("tnum").TextColor(ink).
						SingleLine().Shrink(0)
				}
			})
		})
		s.sleepMenu(c, b, open, menu)
	})
}

// sleepMenu is the moon button's menu, above it while *open: while the
// timer runs, what it does, more minutes and Cancel; the lengths, and a
// field for any other; then, but for a live stream, which never ends, the
// end of this track, of the queue or of a few tracks, and Let the track
// finish.
func (s *shell) sleepMenu(c *ui.Context, anchor ui.Element, open *bool, m *sleepMenuState) {
	k := &s.lk
	ui.PopoverBase(c, anchor, open, func(panel ui.Element) {
		panel.AttachTo(anchor, ui.AnchorTopEnd, ui.AnchorBottomEnd).MarginY(8).Width(260).Padding(6).Gap(2).
			Radius(k.radiusSm).Background(k.raised).Border(1, k.border).Shadow(0, 12, 32, -8, ui.RGBA(0, 0, 0, 0.55)).
			TextColor(k.text).Font(k.body)
		panel.Children(func() {
			ui.Text(c, k.words(locales.SleepTimerTitle())).FontSize(k.captionSize()).TextColor(k.muted).Padding(4, 10, 6).SingleLine()
			// MyGo keeps the menu inside the window: in one too short for
			// it, its rows scroll under the title, rather than run over the
			// player bar.
			ui.Scroll(c.Key("sleep-rows")).Shrink(1).MinHeight(0).Gap(2).Children(func() {
				sl := s.st.Sleep
				if line := s.st.SleepLine(time.Now()); line != "" {
					ui.Text(c, line).FontSize(k.bodySize()).FontFeatures("tnum").TextColor(k.accent).Padding(2, 10, 6)
					ui.Row(c).Gap(6).Padding(0, 6, 4).Wrap().Children(func() {
						for _, add := range player.SleepAdds {
							if s.sleepChip(c, "sleep-add-"+strconv.Itoa(add), locales.SleepAdd(add)) {
								s.p.AddSleep(add, false)
							}
						}
						if s.sleepChip(c, "sleep-cancel", locales.SleepCancel()) {
							s.p.CancelSleep(false)
							*open = false
						}
					})
					s.sleepDivider(c)
				}
				for _, n := range player.SleepLengths {
					if s.sleepItem(c, "sleep-"+strconv.Itoa(n), locales.SleepMinutes(n), false) {
						s.startSleep(player.SleepTimer{Mode: player.SleepMinutes, N: n}, open)
					}
				}
				s.sleepField(c, open, m)
				if s.st.Live {
					return
				}
				s.sleepDivider(c)
				if s.sleepItem(c, "sleep-track", locales.SleepEndOfTrack(), sl != nil && sl.Mode == player.SleepTracks && sl.Tracks == 1) {
					s.startSleep(player.SleepTimer{Mode: player.SleepTracks, N: 1}, open)
				}
				if s.sleepItem(c, "sleep-queue", locales.SleepEndOfQueue(), sl != nil && sl.Mode == player.SleepQueue) {
					s.startSleep(player.SleepTimer{Mode: player.SleepQueue}, open)
				}
				s.sleepTracks(c, open, m, sl != nil && sl.Mode == player.SleepTracks && sl.Tracks > 1)
				s.sleepDivider(c)
				ui.Row(c).Gap(10).AlignItems(ui.Center).Padding(4, 10).Children(func() {
					ui.Text(c, locales.SleepFinish()).FontSize(k.bodySize()).Grow(1).MinWidth(0)
					if s.switchOf(c.Key("sleep-finish"), &s.set.SleepTimer.Finish).Label(locales.SleepFinish()).Changed() {
						s.save()
					}
				})
			})
		})
	})
}

// sleepItem is an item of the sleep timer's menu, checked while it is
// what the timer does. It tells whether it was clicked.
func (s *shell) sleepItem(c *ui.Context, key, label string, on bool) bool {
	k := &s.lk
	b := s.buttonBase(c.Key(key)).Height(32).PaddingX(10).Gap(10).Radius(k.radiusSm).Justify(ui.Start).AlignItems(ui.Center).
		Label(label).Role(ui.RoleMenuItemRadio).Checked(on).Cursor(ui.CursorPointer)
	if b.Pressed() {
		b.Background(k.press)
	} else if b.Hovered() {
		b.Background(k.hover)
	}
	b.Children(func() {
		ui.Box(c).Size(16, 16).Shrink(0).Children(func() {
			if on {
				ui.Icon(c, icon("check")).FontSize(16).TextColor(k.accent)
			}
		})
		ui.Text(c, label).FontSize(k.bodySize()).FontFeatures("tnum").SingleLine().MinWidth(0)
	})
	return b.Clicked()
}

// sleepChip is a small outlined button of the sleep timer's menu, as +5
// min. It tells whether it was clicked.
func (s *shell) sleepChip(c *ui.Context, key, label string) bool {
	k := &s.lk
	b := s.buttonBase(c.Key(key)).Height(28).PaddingX(10).Radius(k.round(28)).Border(1, k.border).Label(label).Cursor(ui.CursorPointer)
	if b.Pressed() {
		b.Background(k.press)
	} else if b.Hovered() {
		b.Background(k.hover)
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(k.smallSize()).FontFeatures("tnum").SingleLine().Shrink(0)
	})
	return b.Clicked()
}

// sleepDivider parts the sleep timer's menu.
func (s *shell) sleepDivider(c *ui.Context) { ui.Box(c).Height(1).Margin(4, 6).Background(s.lk.border) }

// sleepField takes a length of any whole number of minutes, from 1 to
// settings.MaxSleepMinutes, which Enter or Start starts; another is
// turned down, with why.
func (s *shell) sleepField(c *ui.Context, open *bool, m *sleepMenuState) {
	k := &s.lk
	ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(4, 10).Children(func() {
		in := ui.TextInput(c.Key("sleep-minutes"), &m.minutes).Placeholder(k.sentence(locales.SleepMinutesField())).
			Label(locales.SleepMinutesFieldLabel()).Grow(1).MinWidth(0)
		if in.Changed() {
			m.bad = false
		}
		start := s.sleepChip(c, "sleep-start", locales.SleepStart())
		if !in.Submitted() && !start {
			return
		}
		n, err := strconv.Atoi(strings.TrimSpace(m.minutes))
		if err != nil || n < 1 || n > settings.MaxSleepMinutes {
			m.bad = true
			return
		}
		m.minutes, m.bad = "", false
		s.startSleep(player.SleepTimer{Mode: player.SleepMinutes, N: n}, open)
	})
	if m.bad {
		ui.Text(c, locales.SleepMinutesInvalid(settings.MaxSleepMinutes)).FontSize(k.smallSize()).TextColor(k.danger).Padding(0, 10, 4)
	}
}

// sleepTracks is After N tracks, from 2 to player.MaxSleepTracks, with
// buttons for fewer and more.
func (s *shell) sleepTracks(c *ui.Context, open *bool, m *sleepMenuState, on bool) {
	m.tracks = min(max(m.tracks, 2), player.MaxSleepTracks)
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Grow(1).MinWidth(0).Children(func() {
			if s.sleepItem(c, "sleep-tracks", locales.SleepAfterTracksItem(m.tracks), on) {
				s.startSleep(player.SleepTimer{Mode: player.SleepTracks, N: m.tracks}, open)
			}
		})
		for _, step := range []struct {
			key, icon, label string
			by               int
		}{{"sleep-fewer", "minus", locales.SleepFewerTracks(), -1}, {"sleep-more", "plus", locales.SleepMoreTracks(), 1}} {
			next := m.tracks + step.by
			b := s.iconButton(c, step.key, step.icon, step.label, false, 16).Disabled(next < 2 || next > player.MaxSleepTracks)
			if b.Clicked() {
				m.tracks = next
			}
		}
		ui.Box(c).Width(2)
	})
}

// sleepItems are the sleep timer in a menu, as the mini player's: a
// submenu of what the moon button's menu offers, but the field of minutes
// and After N tracks.
func (s *shell) sleepItems(m *ui.Menu) {
	m.Submenu(locales.SleepTimerTitle(), func(m *ui.Menu) {
		sl := s.st.Sleep
		if line := s.st.SleepLine(time.Now()); line != "" {
			m.Item(line).Disabled(true)
			for _, add := range player.SleepAdds {
				if m.Item(locales.SleepAdd(add)).Chosen() {
					s.p.AddSleep(add, false)
				}
			}
			if m.Item(locales.SleepCancel()).Chosen() {
				s.p.CancelSleep(false)
			}
			m.Separator()
		}
		for _, n := range player.SleepLengths {
			if m.Item(locales.SleepMinutes(n)).Chosen() {
				s.p.StartSleep(player.SleepTimer{Mode: player.SleepMinutes, N: n})
			}
		}
		if s.st.Live {
			return
		}
		if m.Item(locales.SleepEndOfTrack()).Radio(sl != nil && sl.Mode == player.SleepTracks && sl.Tracks == 1).Chosen() {
			s.p.StartSleep(player.SleepTimer{Mode: player.SleepTracks, N: 1})
		}
		if m.Item(locales.SleepEndOfQueue()).Radio(sl != nil && sl.Mode == player.SleepQueue).Chosen() {
			s.p.StartSleep(player.SleepTimer{Mode: player.SleepQueue})
		}
	})
}

// sleepCommands are the sleep timer's commands: a length of the menu, the
// end of this track or of the queue, the length chosen last, and Cancel,
// whose IDs the last two's global shortcuts share.
func (s *shell) sleepCommands() []command {
	group := locales.CmdGroupPlayback()
	start := func(t player.SleepTimer) func() { return func() { s.p.StartSleep(t) } }
	cmds := make([]command, 0, len(player.SleepLengths)+4)
	for _, n := range player.SleepLengths {
		cmds = append(cmds, command{"player.sleep" + strconv.Itoa(n), locales.CmdSleepMinutes(n), "", group,
			start(player.SleepTimer{Mode: player.SleepMinutes, N: n})})
	}
	return append(cmds,
		command{"player.sleepTrack", locales.CmdSleepTrack(), "", group, start(player.SleepTimer{Mode: player.SleepTracks, N: 1})},
		command{"player.sleepQueue", locales.CmdSleepQueue(), "", group, start(player.SleepTimer{Mode: player.SleepQueue})},
		command{"player.sleepLast", locales.CmdSleepLast(), "", group, func() {
			s.p.StartSleep(player.SleepTimer{Mode: player.SleepMinutes, N: s.set.SleepTimer.Minutes})
		}},
		command{"player.sleepCancel", locales.CmdSleepCancel(), "", group, func() { s.p.CancelSleep(false) }})
}

func init() { addSettings("playback", 35, (*shell).sleepTimerSettings) }

// sleepTimerSettings are the group Sleep timer of Settings › Playback: the
// fade, Let the track finish, and the bedtime it starts at by itself, in
// one window for every day or one for each day of the week.
func (s *shell) sleepTimerSettings(c *ui.Context) {
	st := &s.set.SleepTimer
	s.group(c, locales.SleepTimerTitle(), func() {
		s.row(c, locales.SleepFadeTitle(), locales.SleepFadeDesc(), func() {
			labels := make([]string, len(settings.SleepFades))
			for i, f := range settings.SleepFades {
				labels[i] = locales.SleepFadeSeconds(f)
				if f == 0 {
					labels[i] = locales.SleepFadeOff()
				}
			}
			i := slices.Index(settings.SleepFades, st.Fade)
			if s.segmented(c, "sleep-fade", locales.SleepFadeTitle(), &i, labels...) && i >= 0 {
				st.Fade = settings.SleepFades[i]
				s.save()
			}
		}, "sleep timer fade out volume")
		s.row(c, locales.SleepFinish(), locales.SleepFinishDesc(), func() {
			if s.switchOf(c.Key("sleep-finish-setting"), &st.Finish).Label(locales.SleepFinish()).Changed() {
				s.save()
			}
		}, "sleep timer end track")
		s.row(c, locales.SleepBedtimeTitle(), locales.SleepBedtimeDesc(st.Minutes), func() {
			if s.switchOf(c.Key("sleep-bedtime"), &st.Bedtime).Label(locales.SleepBedtimeTitle()).Changed() {
				s.save()
			}
		}, "sleep timer bedtime night automatic")
		if !st.Bedtime {
			return
		}
		s.row(c, locales.SleepBedtimeWhen(), "", func() {
			i := 0
			if st.ByDay {
				i = 1
			}
			if s.segmented(c, "sleep-by-day", locales.SleepBedtimeWhen(), &i, locales.SleepBedtimeEveryDay(), locales.SleepBedtimeByDay()) {
				st.ByDay = i == 1
				s.save()
			}
		}, "sleep timer bedtime")
		if !st.ByDay {
			every := locales.SleepBedtimeEveryDay()
			s.row(c, every, "", func() { s.bedtimeTimes(c, "sleep-every", every, &st.Every) }, "sleep timer bedtime")
		} else {
			// From Monday; the days of the settings are from Sunday.
			monday := time.Date(2026, time.November, 16, 12, 0, 0, 0, time.UTC)
			for i := range 7 {
				d := monday.AddDate(0, 0, i)
				if int(d.Weekday()) >= len(st.Days) {
					continue
				}
				day := dateIn(d, "EEEE")
				w := &st.Days[d.Weekday()]
				s.row(c, day, "", func() {
					if s.switchOf(c.Key("sleep-day-"+strconv.Itoa(int(d.Weekday()))), &w.On).Label(locales.SleepBedtimeDayLabel(day)).Changed() {
						s.save()
					}
					s.bedtimeTimes(c, "sleep-day-"+strconv.Itoa(int(d.Weekday())), day, w)
				}, "sleep timer bedtime")
			}
		}
		s.hint(c, locales.SleepBedtimeHint())
	})
}

// bedtimeTimes are the menus of the times a window of day starts and
// ends, by half hours, and the times it has if it has others.
func (s *shell) bedtimeTimes(c *ui.Context, key, day string, w *settings.BedtimeWindow) {
	k := &s.lk
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		from, to := dayTime(w.From), dayTime(w.To)
		if s.selectOf(c.Key(key+"-from"), &from, dayTimes(from)).Width(96).Label(locales.SleepBedtimeStartLabel(day)).Changed() {
			w.From = from
			s.save()
		}
		ui.Text(c, locales.SleepBedtimeTo()).FontSize(k.smallSize()).TextColor(k.muted).SingleLine()
		if s.selectOf(c.Key(key+"-to"), &to, dayTimes(to)).Width(96).Label(locales.SleepBedtimeEndLabel(day)).Changed() {
			w.To = to
			s.save()
		}
	})
}

// dayTime writes a time of the day of the settings as the menus of times
// do, with two digits of hours, as "06:00".
func dayTime(t string) string {
	m, ok := settings.DayMinutes(t)
	if !ok {
		return t
	}
	return settings.DayTime(m)
}

// dayTimes are the times of the day by half hours, and t among them, in
// order, when it's none.
func dayTimes(t string) []string {
	out := make([]string, 0, 49)
	for m := 0; m < 24*60; m += 30 {
		out = append(out, settings.DayTime(m))
	}
	if i, found := slices.BinarySearch(out, t); !found {
		out = slices.Insert(out, i, t)
	}
	return out
}

// roomSleep is the width the sleep timer's button needs beside the others
// in the player bar, with no timer, with its gap.
const roomSleep = 36

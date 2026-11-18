package settings

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// SleepTimer is how the sleep timer stops the music (#594), as the group
// Sleep timer of Settings › Playback and the timer's menu set it: its
// fade, Let the track finish, the length chosen last, and the bedtime it
// starts at by itself.
type SleepTimer struct {
	// Fade fades the music out over this many seconds before the timer
	// stops it: one of SleepFades, 30 at first, 0 for none.
	Fade int `json:"fade"`
	// Finish lets the track that plays as the time runs out play to its
	// end, where the music stops.
	Finish bool `json:"finish"`
	// Minutes is the length of the timer of minutes started last, from 1
	// to MaxSleepMinutes: the length Sleep timer: last length and bedtime
	// start.
	Minutes int `json:"minutes"`
	// Bedtime starts the timer by itself, at Minutes, as playback starts
	// inside a window of the night. It is off at first.
	Bedtime bool `json:"bedtime"`
	// ByDay takes the window of each day of the week from Days, in place
	// of Every's for every day.
	ByDay bool `json:"byDay"`
	// Every is the window of every day, 22:00 to 06:00 at first.
	Every BedtimeWindow `json:"every"`
	// Days are the windows of the days of the week, from Sunday, as
	// time.Weekday counts them: seven of them.
	Days []BedtimeWindow `json:"days"`
}

// BedtimeWindow is a window of the night, in 24-hour time, as "22:00", of
// the day it starts on: one whose end comes before its start ends the
// next morning, across midnight.
type BedtimeWindow struct {
	// On has the window start the timer: each day of the week has its own
	// switch.
	On   bool   `json:"on"`
	From string `json:"from"`
	To   string `json:"to"`
}

// The lengths and fades the sleep timer takes.
const (
	// MaxSleepMinutes is the longest a timer of minutes runs.
	MaxSleepMinutes = 9999
	// DefaultSleepFade is the fade of a new install, in seconds.
	DefaultSleepFade = 30
	// defaultSleepMinutes is the length of a new install.
	defaultSleepMinutes = 30
)

// SleepFades are the fades the settings offer, in seconds: none, 10, 30,
// 60 or 120.
var SleepFades = []int{0, 10, 30, 60, 120}

// defaultBedtime is the window of a new install: 22:00 to 06:00.
var defaultBedtime = BedtimeWindow{On: true, From: "22:00", To: "06:00"}

// defaultSleepTimer is the sleep timer of a new install: a fade of 30
// seconds, a length of 30 minutes, and no bedtime.
func defaultSleepTimer() SleepTimer {
	t := SleepTimer{Fade: DefaultSleepFade, Minutes: defaultSleepMinutes, Every: defaultBedtime, Days: make([]BedtimeWindow, 7)}
	for i := range t.Days {
		t.Days[i] = defaultBedtime
	}
	return t
}

// clean is t with a fade of SleepFades, the nearest, a length in range, and
// a window of each day of the week whose times are times of the day.
func (t SleepTimer) clean() SleepTimer {
	d := defaultSleepTimer()
	t.Fade = nearestFade(t.Fade)
	if t.Minutes < 1 {
		t.Minutes = d.Minutes
	}
	t.Minutes = min(t.Minutes, MaxSleepMinutes)
	t.Every = t.Every.clean()
	days := make([]BedtimeWindow, 7)
	for i := range days {
		days[i] = d.Days[i]
		if i < len(t.Days) {
			days[i] = t.Days[i].clean()
		}
	}
	t.Days = days
	return t
}

// nearestFade is the fade of SleepFades nearest s seconds.
func nearestFade(s int) int {
	best := SleepFades[0]
	for _, f := range SleepFades {
		if abs(f-s) < abs(best-s) {
			best = f
		}
	}
	return best
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// clean is w with times of the day: a time that isn't one takes the
// default window's.
func (w BedtimeWindow) clean() BedtimeWindow {
	if _, ok := DayMinutes(w.From); !ok {
		w.From = defaultBedtime.From
	}
	if _, ok := DayMinutes(w.To); !ok {
		w.To = defaultBedtime.To
	}
	return w
}

// DayMinutes reads a time of the day in 24-hour time, as "22:00" or
// "6:30", as the minutes since midnight.
func DayMinutes(s string) (int, bool) {
	hh, mm, ok := strings.Cut(strings.TrimSpace(s), ":")
	h, errH := strconv.Atoi(hh)
	m, errM := strconv.Atoi(mm)
	if !ok || errH != nil || errM != nil || len(hh) > 2 || len(mm) != 2 || hh[0] < '0' || mm[0] < '0' || h > 23 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

// DayTime writes minutes since midnight as a time of the day in 24-hour
// time, as "06:00".
func DayTime(minutes int) string {
	minutes = (minutes%(24*60) + 24*60) % (24 * 60)
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}

// Window returns the window of the day, today's or the one that started
// the day before, that now is inside, and when it ends: false when now is
// in none of them, as when Bedtime is off. A window whose end is its start
// lasts the whole day.
func (t SleepTimer) Window(now time.Time) (end time.Time, in bool) {
	if !t.Bedtime {
		return time.Time{}, false
	}
	y, m, d := now.Date()
	for back := range 2 {
		day := time.Date(y, m, d-back, 0, 0, 0, 0, now.Location())
		w := t.Every
		if t.ByDay && int(day.Weekday()) < len(t.Days) {
			w = t.Days[day.Weekday()]
		}
		from, okFrom := DayMinutes(w.From)
		to, okTo := DayMinutes(w.To)
		if !w.On || !okFrom || !okTo {
			continue
		}
		start := time.Date(y, m, d-back, from/60, from%60, 0, 0, now.Location())
		end := time.Date(y, m, d-back, to/60, to%60, 0, 0, now.Location())
		if to <= from {
			end = time.Date(y, m, d-back+1, to/60, to%60, 0, 0, now.Location())
		}
		if !now.Before(start) && now.Before(end) {
			return end, true
		}
	}
	return time.Time{}, false
}

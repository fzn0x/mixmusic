package app

// The sleep timer (#594) outside the windows: the engine's fade, the
// length kept, its global shortcuts' commands and its submenu of the tray
// icon's menu. The player runs it (internal/player/sleep.go).

import (
	"log"
	"runtime"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// The engine fades for the sleep timer, through the stream of a room this
// computer hosts too.
var _ player.Fader = roomStream{}

func (s engineStream) FadeAway(d time.Duration) { s.e.FadeAway(d) }
func (s engineStream) FadeBack(d time.Duration) { s.e.FadeBack(d) }

func (s roomStream) FadeAway(d time.Duration) {
	if f, ok := s.Stream.(player.Fader); ok {
		f.FadeAway(d)
	}
}

func (s roomStream) FadeBack(d time.Duration) {
	if f, ok := s.Stream.(player.Fader); ok {
		f.FadeBack(d)
	}
}

// KeepSleepMinutes keeps the length of the timer of minutes the user
// started last, which Sleep timer: last length and bedtime start.
func (s playerServices) KeepSleepMinutes(minutes int) {
	if _, err := (&Preferences{s.a}).Update(func(st *settings.Settings) { st.SleepTimer.Minutes = minutes }); err != nil {
		log.Printf("mixmusic: keeping the sleep timer's length: %v", err)
	}
}

// sleepLast starts the sleep timer at the length chosen last.
func (a *App) sleepLast(p *player.Player) {
	p.StartSleep(player.SleepTimer{Mode: player.SleepMinutes, N: a.settings.Get().Clean().SleepTimer.Minutes})
}

// sleepTrayItem is the sleep timer's submenu of the tray icon's menu: the
// lengths and the ends it starts at, and while it runs, what it does, more
// minutes and Cancel. On Linux, whose menu is made once, the items of a
// timer that runs show always, and do nothing while none runs, and a live
// stream's ends show too.
func (a *App) sleepTrayItem() *mygo.MenuItem {
	var st player.State
	if p := a.goPlayer.Load(); p != nil {
		st = p.State()
	}
	made := runtime.GOOS == "linux"
	run := func(fn func(p *player.Player)) func(*mygo.MenuItem, *mygo.Window) {
		return func(*mygo.MenuItem, *mygo.Window) {
			if p := a.goPlayer.Load(); p != nil {
				fn(p)
			}
		}
	}
	var items []*mygo.MenuItem
	if line := st.SleepLine(time.Now()); line != "" && !made {
		items = append(items, &mygo.MenuItem{Label: line, Disabled: true})
	}
	if st.Sleep != nil || made {
		for _, m := range player.SleepAdds {
			items = append(items, &mygo.MenuItem{Label: locales.SleepAdd(m), Click: run(func(p *player.Player) { p.AddSleep(m, false) })})
		}
		items = append(items, &mygo.MenuItem{Label: locales.SleepCancel(), Click: run(func(p *player.Player) { p.CancelSleep(false) })},
			mygo.Separator())
	}
	for _, m := range player.SleepLengths {
		items = append(items, &mygo.MenuItem{Label: locales.SleepMinutes(m), Click: run(func(p *player.Player) {
			p.StartSleep(player.SleepTimer{Mode: player.SleepMinutes, N: m})
		})})
	}
	if !st.Live || made {
		items = append(items,
			&mygo.MenuItem{Label: locales.SleepEndOfTrack(), Click: run(func(p *player.Player) {
				p.StartSleep(player.SleepTimer{Mode: player.SleepTracks, N: 1})
			})},
			&mygo.MenuItem{Label: locales.SleepEndOfQueue(), Click: run(func(p *player.Player) {
				p.StartSleep(player.SleepTimer{Mode: player.SleepQueue})
			})})
	}
	return &mygo.MenuItem{Label: locales.SleepTimerTitle(), Submenu: items}
}

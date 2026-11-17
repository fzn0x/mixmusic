package app

// What MixMusic says outside its windows' views, which it set once, it
// says again as the language changes (#775): the tray's menu and tooltip,
// the taskbar's buttons and the main window's title. The views follow by
// themselves, as they are built for every frame.

import (
	"runtime"

	"github.com/egoist/mygo"
)

// followLanguage says them again whenever the language changes.
func (a *App) followLanguage() {
	mygo.App.OnLocaleChanged(func(string) { a.languageChanged() })
}

// languageChanged says them again in the app's language. It runs on the
// main thread.
func (a *App) languageChanged() {
	a.mu.Lock()
	tray, bar, np := a.tray, a.taskbar, a.nowPlaying
	a.mu.Unlock()
	if tray != nil {
		tray.SetToolTip(trayTip(np))
		if runtime.GOOS == "linux" {
			tray.SetMenu(mygo.NewMenu(a.trayMenu()))
		}
	}
	if bar != nil {
		bar.SetTips(taskbarTips())
	}
	a.retitle()
}

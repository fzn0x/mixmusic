package app

// The thumbnail of the main window's taskbar button has the player's
// buttons on Windows, as music players have them there.

import (
	"errors"
	"log"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/taskbar"
)

// taskbarCommands are the player's commands for the thumbnail's buttons.
var taskbarCommands = map[taskbar.Button]Command{
	taskbar.Previous:  CommandPrevious,
	taskbar.PlayPause: CommandToggle,
	taskbar.Next:      CommandNext,
}

// attachTaskbar puts the player's buttons on the thumbnail of win's
// taskbar button, which show what plays from then on.
func (a *App) attachTaskbar(win *mygo.Window) {
	var bar *taskbar.Bar
	var err error
	mygo.RunOnMain(func() {
		bar, err = taskbar.Attach(win.NativeHandle(), func(b taskbar.Button) {
			a.command(PlayerCommand{Command: taskbarCommands[b]})
		})
	})
	if err != nil {
		if !errors.Is(err, taskbar.ErrUnsupported) {
			log.Printf("mixmusic: the taskbar's buttons: %v", err)
		}
		return
	}
	a.mu.Lock()
	a.taskbar = bar
	np := a.nowPlaying
	a.mu.Unlock()
	bar.Show(np.Playing, np.Track != nil)
}

package main

// Remote control (#353), in the main window: the computers with MixMusic's
// remote control on, in the devices menu of the cast button; the page of
// each (remotepage.go); the dialog of the code this computer shows while
// another pairs with it (remotecode.go); and Settings › Desktop › Remote
// control (settings_remote.go). internal/app's remote*.go runs it.

import (
	"net/url"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// remoteView is what the window keeps of remote control from frame to
// frame.
type remoteView struct {
	// code is what the user typed of the code another computer shows.
	code string
	// seeking and seekTo are the page's seek bar while the user drags it,
	// and volume its volume; seekAt and volumeAt are when the user last
	// seeked and set the volume.
	seeking          bool
	seekTo           float64
	volume           float64
	seekAt, volumeAt time.Time
	// codeOpen is whether the dialog of the code this computer shows is
	// open, for the code codeOf: closing it refuses the pairing.
	codeOpen bool
	codeOf   string
	// turning is the switch of Settings › Remote control while it turns
	// remote control on or off.
	turning *bool
	// handing is set while what plays here moves to another computer.
	handing bool
}

// remotePath is the page of the computer id.
func remotePath(id string) string { return "/remote/" + url.PathEscape(id) }

// remoteItems lists the computers with MixMusic's remote control on in the
// devices menu, after the cast devices, by their names: choosing one opens
// its page, which pairs with it, then controls it.
func (s *shell) remoteItems(m *ui.Menu) {
	computers := s.a.Remote().Computers()
	if len(computers) == 0 {
		return
	}
	m.Separator()
	for _, rc := range computers {
		if m.Item(rc.Name + " · MixMusic").Chosen() {
			s.navigate(remotePath(rc.ID))
		}
	}
}

// remoteDo sends a command to the computer id beside the main thread, and
// tells why it couldn't.
func (s *shell) remoteDo(id string, cmd remote.Command) {
	go func() {
		if err := s.a.Remote().Do(id, cmd); err != nil {
			s.update(func() { s.notes = append(s.notes, noticeOf(err)) })
		}
	}()
}

// followRemote redraws the window as remote control changes, and tells
// what it says, as a pairing that ended.
func (s *shell) followRemote() func() {
	return s.a.OnRemote(func(note string) {
		s.update(func() {
			if note != "" {
				s.notes = append(s.notes, notice(note))
			}
		})
	})
}

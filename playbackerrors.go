package main

import (
	"os"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
)

// The details of the tracks that couldn't play (#351): the toast that
// tells of one has Details, which shows them in a dialog, with Copy. The
// text of the details is failurereport.go's.

// failureView is the dialog of a failure's details: whether it shows, and
// the failure's ID, as the player keeps it.
type failureView struct {
	open bool
	id   int
}

// failureToastTime is how long the toast of a failure shows, for its
// Details to be reached: as long as MyGo shows a toast with an action.
const failureToastTime = 8 * time.Second

// failureToast tells of a track that couldn't play, as the player's
// notice n does, with Details, which Tab reaches.
func (s *shell) failureToast(c *ui.Context, n player.Notice) {
	id := n.Failure
	c.AddToast(ui.Toast{ID: n.Message, Title: n.Message, Type: "error", Timeout: failureToastTime,
		Action: "details", OnAction: func() { s.showFailure(id) }})
}

// showFailure shows the details of the failure id.
func (s *shell) showFailure(id int) { s.failure = failureView{open: true, id: id} }

// failureDialog shows the details of the failure the window shows, over
// it, with Copy, which has the focus as it opens.
func (s *shell) failureDialog(c *ui.Context) {
	if !s.failure.open {
		return
	}
	f, ok := s.p.FailureOf(s.failure.id)
	if !ok {
		s.failure.open = false
		return
	}
	s.dialog(c, &s.failure.open, dialogOptions{key: "failure", title: "Why it couldn't play", width: 560}, func() {
		s.failureDetails(c, f).AutoFocus()
	}, func() {
		if s.dialogButton(c, "failure-close", "close", false).Clicked() {
			s.failure.open = false
		}
	})
}

// failureDetails shows the details of a track that couldn't play: the
// track, where it comes from and when it failed, what was tried, in turn,
// and why each try failed, and Copy, which puts them on the clipboard as
// text for an issue, and which it returns. It shows them wherever it's
// built, as in the dialog of the toast's Details.
func (s *shell) failureDetails(c *ui.Context, f player.Failure) ui.Element {
	k := &s.lk
	pr := s.privacy()
	var copied ui.Element
	ui.Column(c.Key("failure-view-" + strconv.Itoa(f.ID))).Gap(16).MaxWidth(720).Children(func() {
		ui.Column(c).Gap(4).Children(func() {
			ui.Text(c, pr.clean(failureTrack(f.Track))).FontSize(k.bodySize()).FontWeight(k.leadWeight()).Selectable()
			ui.Text(c, sentenceStart(failureFrom(f.Track, pr))+" · "+failureWhen(f.At, c.Now())).
				FontSize(k.smallSize()).TextColor(k.muted).Selectable()
			if r := f.Recording; r.ID != "" {
				ui.Text(c, "Played as "+pr.clean(failureTrack(r))+", from "+failureFrom(r, pr)).
					FontSize(k.smallSize()).TextColor(k.muted).Selectable()
			}
		})
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "What was tried").FontSize(k.smallSize()).FontWeight(k.leadWeight())
			for i, st := range f.Steps {
				ui.Text(c, strconv.Itoa(i+1)+". "+failureStep(st, pr)).FontSize(k.smallSize()).Selectable()
			}
			if f.Stopped {
				ui.Text(c, failureStopped).FontSize(k.smallSize()).TextColor(k.muted)
			}
		})
		copied = s.button(c, "failure-copy-"+strconv.Itoa(f.ID), "copy", false).Label("copy the details")
		if copied.Clicked() {
			copyText(failureReport(f, s.svc.System.Info(), pr))
			s.notes = append(s.notes, notice("Copied the details, without your accounts' names"))
		}
	})
	return copied
}

// privacy is what the details of failures leave out: the names of the
// accounts the user signed in to, and their home folder.
func (s *shell) privacy() privacy {
	home, _ := os.UserHomeDir()
	return privacyOf(s.accounts(), home)
}

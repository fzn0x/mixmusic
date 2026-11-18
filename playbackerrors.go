package main

import (
	"os"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/locales"
)

// The details of the tracks that couldn't play (#351): the toast that
// tells of one has Details, which shows them in a dialog, with Copy, and
// Settings › About lists the last few, until MixMusic closes. The text of
// the details is failurereport.go's.

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
// notice n does, with Details, which Tab reaches. When the playback
// settings change why, as for a track YouTube plays only to signed-in
// viewers whose words tell what to change (#577), it offers Settings ›
// Playback in its place: a toast has one action, and the details stay in
// Settings › About.
func (s *shell) failureToast(c *ui.Context, n player.Notice) { c.AddToast(s.failureToastOf(n)) }

// failureToastOf is the toast of the failure the notice n tells of.
func (s *shell) failureToastOf(n player.Notice) ui.Toast {
	id := n.Failure
	t := ui.Toast{ID: n.Message, Title: n.Message, Type: "error", Timeout: failureToastTime,
		Action: "details", OnAction: func() { s.showFailure(id) }}
	if n.Settings {
		t.Action, t.OnAction = locales.ShellToastSettings(), func() { s.navigate("/settings/playback") }
	}
	return t
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
	s.dialog(c, &s.failure.open, dialogOptions{key: "failure", title: locales.FailureTitle(), width: 560}, func() {
		s.failureDetails(c, f).AutoFocus()
	}, func() {
		if s.dialogButton(c, "failure-close", locales.FailureClose(), false).Clicked() {
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
			ui.Text(c, pr.clean(failureTrackSaid(f.Track))).FontSize(k.bodySize()).FontWeight(k.leadWeight()).Selectable()
			ui.Text(c, locales.FailureFromWhen(sentenceStart(failureFromSaid(f.Track, pr)), failureWhen(f.At, c.Now()))).
				FontSize(k.smallSize()).TextColor(k.muted).Selectable()
			if r := f.Recording; r.ID != "" {
				ui.Text(c, failurePlayedAsSaid(r, pr)).
					FontSize(k.smallSize()).TextColor(k.muted).Selectable()
			}
		})
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, locales.FailureTried()).FontSize(k.smallSize()).FontWeight(k.leadWeight())
			for i, st := range f.Steps {
				ui.Text(c, locales.FailureStepNumbered(i+1, failureStepSaid(st, pr))).FontSize(k.smallSize()).Selectable()
			}
			if f.Stopped {
				ui.Text(c, locales.FailureStopped()).FontSize(k.smallSize()).TextColor(k.muted)
			}
		})
		copied = s.button(c, "failure-copy-"+strconv.Itoa(f.ID), locales.OverlayCopy(), false).Label(locales.FailureCopyLabel())
		if copied.Clicked() {
			copyText(failureReport(f, s.svc.System.Info(), pr))
			s.notes = append(s.notes, notice(locales.FailureCopied()))
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

func init() { addSettings("about", 25, (*shell).playbackErrorSettings) }

// playbackErrorSettings list the tracks that couldn't play since MixMusic
// started, the latest first, each with its details: Settings › About ›
// Recent playback errors.
func (s *shell) playbackErrorSettings(c *ui.Context) {
	failures := s.p.Failures()
	s.group(c, locales.PlaybackErrorsGroup(), func() {
		if len(failures) == 0 {
			s.hint(c, locales.PlaybackErrorsNone())
			return
		}
		pr := s.privacy()
		for _, f := range failures {
			title := pr.clean(failureTrackSaid(f.Track))
			line := failureWhen(f.At, c.Now())
			if n := len(f.Steps); n > 0 {
				line += " · " + pr.clean(failureWhy(f.Steps[n-1]))
			}
			s.row(c, title, line, func() {
				if s.button(c, "failure-details-"+strconv.Itoa(f.ID), locales.PlaybackErrorsDetails(), false).Label(locales.PlaybackErrorsDetailsOf(title)).Clicked() {
					s.showFailure(f.ID)
				}
			}, "playback errors failed couldn't play details copy report")
		}
	})
}

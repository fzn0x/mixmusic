package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

// audioAsAccountOffer offers, once, to play audio as the account, when the
// accounts tell that the YouTube Music account has Premium and the switch
// is off (#471): a dialog says what it brings and what it risks, and turns
// it on only when asked. It waits while another dialog asks something.
// Answered either way, or closed, it never shows again; the switch stays
// in Settings › Playback › Sound.
func (s *shell) audioAsAccountOffer(c *ui.Context) {
	if s.set.AudioAsAccount || s.set.AudioAsAccountOffered || s.offer != nil || s.asking != nil {
		return
	}
	if yt := s.accounts().YouTube; !yt.SignedIn || !yt.Premium {
		return
	}
	k := &s.lk
	answer := func(on bool) {
		s.set.AudioAsAccountOffered = true
		if on {
			s.set.AudioAsAccount = true
		}
		s.save()
	}
	open := true
	s.dialog(c, &open, dialogOptions{key: "audio-as-account-offer", title: locales.AudioAccount2Title(), width: 480, alert: true,
		message: audioAsAccountOfferText()}, func() {
		ui.Text(c, audioAsAccountOfferText()).FontSize(k.bodySize())
		ui.Text(c, locales.AudioAccount2Change()).FontSize(k.smallSize()).TextColor(k.muted)
	}, func() {
		// The offer comes unasked, and takes the focus from where it was:
		// not now has it, so that a Space pressed for the music or typed
		// in a search turns nothing on.
		if s.dialogButton(c, "audio-as-account-not-now", locales.AudioAccount2NotNow(), false).AutoFocus().Clicked() {
			answer(false)
		}
		if s.dialogButton(c, "audio-as-account-on", locales.AudioAccount2TurnOn(), true).Clicked() {
			answer(true)
		}
	})
	if !open && !s.set.AudioAsAccountOffered {
		answer(false)
	}
}

// audioAsAccountOfferText tells what playing audio as the account brings,
// and what it risks.
func audioAsAccountOfferText() string { return locales.AudioAccount2Offer() }

package main

import (
	"github.com/egoist/mygo/ui"
)

// remoteCodeDialog shows the code this computer shows while another pairs
// with it, with the other's name, until the pairing ends: Cancel, or
// Escape, refuses it.
func (s *shell) remoteCodeDialog(c *ui.Context) {
	v := &s.remote
	rc := s.a.Remote()
	code, ok := rc.Code()
	if !ok {
		v.codeOpen, v.codeOf = false, ""
		return
	}
	if v.codeOf != code.Code {
		v.codeOpen, v.codeOf = true, code.Code
	}
	if !v.codeOpen {
		return
	}
	k := &s.lk
	shown := code.Code[:3] + " " + code.Code[3:]
	message := code.Name + " asks to control MixMusic on this computer. To let it, type this code there: " + shown
	s.dialog(c, &v.codeOpen, dialogOptions{key: "remote-code", title: "pair with " + code.Name, width: 460, alert: true, message: message}, func() {
		ui.Text(c, code.Name+" asks to control MixMusic on this computer. To let it, type this code there:").FontSize(k.bodySize())
		ui.Text(c, shown).FontSize(k.size(2.5)).FontWeight(k.titleWeight()).FontFeatures("tnum").Selectable().
			Label("the code: " + shown)
		ui.Text(c, "It works once, for 2 minutes. If you didn't ask for it, cancel.").FontSize(k.smallSize()).TextColor(k.muted)
	}, func() {
		if s.dialogButton(c, "remote-code-cancel", "cancel", false).AutoFocus().Clicked() {
			v.codeOpen = false
		}
	})
	if !v.codeOpen {
		// Cancel, or Escape.
		rc.CancelCode()
	}
}

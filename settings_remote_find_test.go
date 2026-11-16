package main

import (
	"testing"

	"github.com/fzn0x/mixmusic/locales"
)

// TestRemoteFindPhone searches the settings for remote control: a phone
// can't use it, so "phone" doesn't find it, and the words that describe
// it still do.
func TestRemoteFindPhone(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 6000)
	for _, c := range []struct {
		q         string
		want, not []string
	}{
		{"phone", nil, []string{locales.RemoteSwitch()}},
		{"remote control", []string{locales.RemoteSwitch()}, nil},
		{"laptop", []string{locales.RemoteSwitch()}, nil},
		{"pair", []string{locales.RemoteSwitch()}, nil},
		{"network", []string{locales.RemoteSwitch()}, nil},
	} {
		show(s, tt, settingsPath("", c.q))
		showsOnly(t, tt, "searching "+c.q, c.want, c.not)
		show(s, tt, "/")
	}
}

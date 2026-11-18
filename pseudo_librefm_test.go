package main

import (
	"os"
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
)

// TestPseudoLocaleLibrefm shows the Libre.fm group in en-XA in each of its
// states: the form of another server over plain http with the user's own
// API account, and the group connected, refused, and asking what becomes
// of the scrobbles waiting. Every text of it is the catalogs'.
func TestPseudoLocaleLibrefm(t *testing.T) {
	usePseudoLocale(t)
	const host = "192.168.1.20:8080"
	for _, c := range []struct {
		name string
		st   app.LibrefmAccount
	}{
		{"form", app.LibrefmAccount{}},
		{"connected", app.LibrefmAccount{Connected: true, Name: "ann", Server: "http://" + host, Plain: true, Waiting: 3}},
		{"refused", app.LibrefmAccount{Connected: true, Name: "ann", Expired: true, Waiting: 2}},
		{"another user", app.LibrefmAccount{Connected: true, Name: "ann", Waiting: 2, Other: "bo"}},
	} {
		s := newTestShell(t)
		s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{Librefm: c.st}}
		s.librefm.other, s.librefm.own = true, true
		s.librefmDraft(librefmAddressKey).text = "http://" + host
		tt := testerOf(t, s, 1280, 4800) // the whole page, for MIXMUSIC_RENDER
		tt.SetLocale("en-XA")
		show(s, tt, "/settings/accounts")
		if dir := os.Getenv("MIXMUSIC_RENDER"); dir != "" {
			renderTo(t, s, tt, dir, "en-XA-librefm-"+c.name)
		}
		if !tt.HasText("libre.fm") {
			t.Errorf("%s: no Libre.fm group: %q", c.name, tt.Texts())
		}
		checkPseudo(t, "Libre.fm, "+c.name, tt.Texts(), "ann", "bo", host)
	}
}

package main

import (
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
)

// TestLibrefmForm offers to connect below ListenBrainz, to Libre.fm or
// another GNU FM server by its address, with a warning of a plain http
// one, and with the user's own API account when they say so.
func TestLibrefmForm(t *testing.T) {
	s, texts := showAccounts(t, app.AccountsStatus{})
	got := texts()
	for _, want := range []string{"Libre.fm", "Choose Libre.fm, or another server that runs GNU FM.", "MixMusic never sees your password",
		"Server", "Use your own API account", "connect to libre.fm", "MixMusic keeps the connection in your system's credential store"} {
		if !strings.Contains(got, want) {
			t.Errorf("the form lacks %q: %q", want, got)
		}
	}
	if i, j := strings.Index(got, "ListenBrainz"), strings.LastIndex(got, "Libre.fm"); i < 0 || j < i {
		t.Errorf("Libre.fm isn't below ListenBrainz: %q", got)
	}
	if strings.Contains(got, "Address") || strings.Contains(got, "travel to the server unencrypted") {
		t.Errorf("the form of Libre.fm asks for an address: %q", got)
	}
	// Last.fm's form asks for its API account too, by its row's title and
	// its box's label.
	keys, secrets := strings.Count(got, "API key"), strings.Count(got, "Shared secret")

	s.librefm.other = true
	s.librefmDraft(librefmAddressKey).text = " HTTP://192.168.1.20:8080/2.0/"
	got = texts()
	for _, want := range []string{"Address", "travel to the server unencrypted", "connect to 192.168.1.20:8080"} {
		if !strings.Contains(got, want) {
			t.Errorf("the form of another server lacks %q: %q", want, got)
		}
	}
	s.librefm.own = true
	if got = texts(); strings.Count(got, "API key") != keys+2 || strings.Count(got, "Shared secret") != secrets+2 {
		t.Errorf("the form of the user's own API account lacks its boxes: %q", got)
	}
}

// TestLibrefmConnected says who scrobbles where, how many scrobbles wait,
// and offers the switches and disconnect.
func TestLibrefmConnected(t *testing.T) {
	_, texts := showAccounts(t, app.AccountsStatus{Librefm: app.LibrefmAccount{Connected: true, Name: "ann", Waiting: 3}})
	got := texts()
	for _, want := range []string{"Scrobbling as ann", "What you listen to joins your profile on Libre.fm",
		"3 scrobbles wait to be sent, and will go once Libre.fm answers.", "Send likes as loves", "A like loves the song on Libre.fm", "disconnect"} {
		if !strings.Contains(got, want) {
			t.Errorf("the group lacks %q: %q", want, got)
		}
	}
	_, texts = showAccounts(t, app.AccountsStatus{Librefm: app.LibrefmAccount{Connected: true, Name: "ann", Server: "http://192.168.1.20:8080", Plain: true}})
	got = texts()
	for _, want := range []string{"your profile on 192.168.1.20:8080", "travel to the server unencrypted"} {
		if !strings.Contains(got, want) {
			t.Errorf("the group of another server lacks %q: %q", want, got)
		}
	}
}

// TestLibrefmExpired asks to connect again, and says how many scrobbles
// wait for it.
func TestLibrefmExpired(t *testing.T) {
	_, texts := showAccounts(t, app.AccountsStatus{Librefm: app.LibrefmAccount{Connected: true, Name: "ann", Expired: true, Waiting: 2}})
	got := texts()
	for _, want := range []string{"Libre.fm no longer accepts this connection", "Connect again to go on scrobbling. 2 scrobbles wait to be sent.", "connect again"} {
		if !strings.Contains(got, want) {
			t.Errorf("the alert lacks %q: %q", want, got)
		}
	}
	if strings.Contains(got, "will go once") {
		t.Errorf("the waiting line shows beside the alert: %q", got)
	}
}

// TestLibrefmAnotherUser asks what becomes of the scrobbles waiting when
// another user approved.
func TestLibrefmAnotherUser(t *testing.T) {
	_, texts := showAccounts(t, app.AccountsStatus{Librefm: app.LibrefmAccount{Connected: true, Name: "ann", Waiting: 2, Other: "bo"}})
	got := texts()
	for _, want := range []string{"Send the waiting scrobbles to bo?", "2 scrobbles were heard as ann and not sent yet. Send them to bo's profile, or drop them.", "send them", "drop them"} {
		if !strings.Contains(got, want) {
			t.Errorf("the alert lacks %q: %q", want, got)
		}
	}
}

func TestLibrefmNames(t *testing.T) {
	for _, c := range []struct{ server, want string }{
		{"", "Libre.fm"}, {"https://libre.fm", "Libre.fm"}, {"https://gnufm.example.com/fm", "gnufm.example.com"}, {"http://nas:8080", "nas:8080"},
	} {
		if got := librefmName(c.server); got != c.want {
			t.Errorf("librefmName(%q) = %q, want %q", c.server, got, c.want)
		}
	}
	for _, c := range []struct{ addr, want string }{
		{"", "Libre.fm"}, {"libre.fm", "Libre.fm"}, {"GNUFM.example.com/2.0/", "gnufm.example.com"}, {"ftp://x", "ftp://x"},
	} {
		if got := librefmFormName(c.addr); got != c.want {
			t.Errorf("librefmFormName(%q) = %q, want %q", c.addr, got, c.want)
		}
	}
}

// TestLibrefmFound finds the form by what a user of a server of their own
// would type, and the connected group's rows by theirs.
func TestLibrefmFound(t *testing.T) {
	for _, c := range []struct {
		st   app.AccountsStatus
		q    string
		want string
	}{
		{app.AccountsStatus{}, "gnu fm", "Use your own API account"},
		{app.AccountsStatus{}, "libre.fm", "connect to libre.fm"},
		{app.AccountsStatus{Librefm: app.LibrefmAccount{Connected: true, Name: "ann"}}, "libre.fm", "disconnect"},
	} {
		s := newTestShell(t)
		s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: c.st}
		tt := testerOf(t, s, 1280, 3200)
		show(s, tt, settingsPath("", c.q))
		if got := strings.Join(tt.Texts(), " "); !strings.Contains(got, c.want) {
			t.Errorf("searching %q doesn't find %q: %q", c.q, c.want, got)
		}
	}
}

// TestFailureReportLeavesOutLibrefm leaves the Libre.fm user name out of
// the details of a failure, as Last.fm's.
func TestFailureReportLeavesOutLibrefm(t *testing.T) {
	pr := privacyOf(app.AccountsStatus{Librefm: app.LibrefmAccount{Name: "robinfm"}}, "/home/robin")
	if got := pr.clean("scrobbling as robinfm failed"); strings.Contains(got, "robinfm") {
		t.Errorf("the details hold the name: %q", got)
	}
}

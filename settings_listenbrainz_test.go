package main

import (
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
)

// showAccounts shows Settings › Accounts with the accounts' state st.
func showAccounts(t *testing.T, st app.AccountsStatus) (*shell, func() string) {
	t.Helper()
	s := newTestShell(t)
	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: st}
	tt := testerOf(t, s, 1280, 3200)
	show(s, tt, "/settings/accounts")
	return s, func() string {
		tt.Frame()
		return strings.Join(tt.Texts(), " ")
	}
}

// TestListenBrainzForm offers to connect below Last.fm, with the token and
// the server, and warns of a plain http server.
func TestListenBrainzForm(t *testing.T) {
	s, texts := showAccounts(t, app.AccountsStatus{})
	got := texts()
	for _, want := range []string{"ListenBrainz", "your ListenBrainz settings", "User token", "Server", "Maloja its address followed by /apis/listenbrainz", "connect to listenbrainz"} {
		if !strings.Contains(got, want) {
			t.Errorf("the form lacks %q: %q", want, got)
		}
	}
	if i, j := strings.Index(got, "Last.fm"), strings.LastIndex(got, "ListenBrainz"); i < 0 || j < i {
		t.Errorf("ListenBrainz isn't below Last.fm: %q", got)
	}
	if strings.Contains(got, "travels to the server unencrypted") {
		t.Error("a warning without a plain http server")
	}
	s.listenbrainzDraft(listenbrainzServerKey).text = " HTTP://192.168.1.20:42010/apis/listenbrainz"
	if !strings.Contains(texts(), "travels to the server unencrypted") {
		t.Errorf("no warning of plain http: %q", texts())
	}
}

// TestListenBrainzConnected says who scrobbles, how many listens wait,
// and offers the switches and disconnect.
func TestListenBrainzConnected(t *testing.T) {
	_, texts := showAccounts(t, app.AccountsStatus{ListenBrainz: app.ListenBrainzAccount{Connected: true, Name: "ann", Waiting: 3}})
	got := texts()
	for _, want := range []string{"Scrobbling as ann", "your ListenBrainz listens", "3 listens wait to be sent, and will go once ListenBrainz answers.", "Send likes as loves", "disconnect"} {
		if !strings.Contains(got, want) {
			t.Errorf("the group lacks %q: %q", want, got)
		}
	}
	_, texts = showAccounts(t, app.AccountsStatus{ListenBrainz: app.ListenBrainzAccount{Connected: true, Name: "ann", Server: "http://192.168.1.20:42010/apis/listenbrainz", Plain: true}})
	got = texts()
	for _, want := range []string{"your listens on 192.168.1.20:42010", "travels to the server unencrypted"} {
		if !strings.Contains(got, want) {
			t.Errorf("the group of a server of the user's lacks %q: %q", want, got)
		}
	}
}

// TestListenBrainzExpired asks for a new token, and says how many listens
// wait for it.
func TestListenBrainzExpired(t *testing.T) {
	_, texts := showAccounts(t, app.AccountsStatus{ListenBrainz: app.ListenBrainzAccount{Connected: true, Name: "ann", Expired: true, Waiting: 2}})
	got := texts()
	for _, want := range []string{"ListenBrainz no longer accepts this token", "Paste a new user token from your ListenBrainz settings", "2 listens wait to be sent.", "connect again"} {
		if !strings.Contains(got, want) {
			t.Errorf("the alert lacks %q: %q", want, got)
		}
	}
	if strings.Contains(got, "will go once") {
		t.Errorf("the waiting line shows beside the alert: %q", got)
	}
}

func TestListenBrainzTexts(t *testing.T) {
	two := app.ListenBrainzAccount{Connected: true, Name: "ann", Waiting: 2, Other: "bo"}
	mine := app.ListenBrainzAccount{Connected: true, Name: "ann", Server: "https://music.example.com/apis/listenbrainz"}
	for _, c := range []struct {
		name, got, want string
	}{
		{"other user", listenbrainzOtherText(two), "2 listens were heard as ann and not sent yet. Send them to bo's listens, or drop them."},
		{"disconnect, two", listenbrainzDisconnectText(two), "MixMusic forgets your token; your listens stay on ListenBrainz. 2 listens not sent yet will be dropped."},
		{"disconnect, own server", listenbrainzDisconnectText(mine), "MixMusic forgets your token; your listens stay on music.example.com."},
		{"expired, own server", listenbrainzExpiredText(mine), "Paste a new user token to go on scrobbling."},
		{"one waiting", waitingText(1, "listen", "ListenBrainz"), "1 listen waits to be sent, and will go once ListenBrainz answers."},
	} {
		if c.got != c.want {
			t.Errorf("%s: %q, want %q", c.name, c.got, c.want)
		}
	}
	for _, c := range []struct {
		addr  string
		plain bool
	}{{"http://nas:42010", true}, {" Http://nas", true}, {"https://nas", false}, {"nas", false}, {"", false}} {
		if got := plainHTTP(c.addr); got != c.plain {
			t.Errorf("plainHTTP(%q) = %v", c.addr, got)
		}
	}
}

// TestListenBrainzFound finds the form by what a user of a server of their
// own would type, and the connected group's rows by theirs.
func TestListenBrainzFound(t *testing.T) {
	for _, c := range []struct {
		st   app.AccountsStatus
		q    string
		want string
	}{
		{app.AccountsStatus{}, "maloja", "User token"},
		{app.AccountsStatus{}, "listenbrainz", "connect to listenbrainz"},
		{app.AccountsStatus{ListenBrainz: app.ListenBrainzAccount{Connected: true, Name: "ann"}}, "loves", "Send likes as loves"},
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

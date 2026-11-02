package youtube

import "testing"

// TestParseChannelsWithoutPageID reads an accounts list of two Google
// accounts whose brand channels tell their page IDs in different tokens, or
// in none. One that tells none is listed, but can't be used: it would be
// the account itself.
func TestParseChannelsWithoutPageID(t *testing.T) {
	got := parseChannels(fixture(t, "accounts-list"), 0)
	want := []AccountChannel{
		{Name: "Ada Example", Handle: "@adaexample", Photo: "https://yt3.example.com/ada=s88", Email: "ada@example.com", Current: true},
		{Name: "Ada Records", Handle: "@adarecords", PageID: "100000000000000000002", Email: "ada@example.com"},
		// Its data sync ID is the channel's before the account's.
		{Name: "Ada Live", Handle: "@adalive", PageID: "100000000000000000003", Email: "ada@example.com"},
		{Name: "Ada Archive", Handle: "@adaarchive", Email: "ada@example.com", Unswitchable: true},
		// Listed before the account, which its data sync ID tells.
		{Name: "Bob's Band", Handle: "@bobsband", AuthUser: 1, Email: "bob@example.com", Unswitchable: true},
		{Name: "Bob Example", Handle: "@bobexample", AuthUser: 1, Email: "bob@example.com"},
		// Its sign-in link names its page.
		{Name: "Bob Podcasts", Handle: "@bobpodcasts", PageID: "200000000000000000003", AuthUser: 1, Email: "bob@example.com"},
	}
	if len(got) != len(want) {
		t.Fatalf("channels = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("channel %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestParseChannelsWithoutTokens takes, of the items of an account that
// tell nothing of what they are, the first for the account itself, as the
// list puts it first.
func TestParseChannelsWithoutTokens(t *testing.T) {
	item := func(name string) any {
		return map[string]any{"accountItem": map[string]any{"accountName": map[string]any{"simpleText": name}}}
	}
	n := node{map[string]any{"contents": []any{item("Me"), item("My Band"), item("My Choir")}}}
	got := parseChannels(n, 0)
	if len(got) != 3 || got[0].Unswitchable || !got[1].Unswitchable || !got[2].Unswitchable {
		t.Errorf("channels = %+v", got)
	}
}

// TestChannelIs tells the account YouTube Music answers as by its handle,
// which is the channel's own, else by its name.
func TestChannelIs(t *testing.T) {
	band := AccountChannel{Name: "My Band", Handle: "@myband", PageID: "1234"}
	for _, c := range []struct {
		a    Account
		want bool
	}{
		{Account{Name: "My Band", Handle: "@myband"}, true},
		{Account{Name: "My band", Handle: "@MyBand"}, true},
		{Account{Name: "My Band", Handle: "@me"}, false},
		{Account{Name: "Me", Handle: "@me"}, false},
		{Account{Name: " My Band "}, true},
		{Account{Name: "Me"}, false},
	} {
		if got := band.Is(c.a); got != c.want {
			t.Errorf("%+v is %+v: %v", band, c.a, got)
		}
	}
	if (AccountChannel{Name: "Me"}).Is(Account{Name: "Me", Handle: "@me"}) != true {
		t.Error("a channel without a handle isn't told by its name")
	}
}

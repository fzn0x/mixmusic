package stream

import (
	"testing"

	"github.com/fzn0x/mixmusic/locales"
)

// TestAccountWordsInTheCatalog keeps the English of the windows, in
// locales/en.json, the words of the errors, which the logs and the
// failure details' Copy keep.
func TestAccountWordsInTheCatalog(t *testing.T) {
	en := locales.In("en")
	whys := map[string]string{
		"YouTube plays this track only to signed-in viewers":     en.ErrorStreamSignedInOnly(),
		"YouTube plays this track only to signed-in adults":      en.ErrorStreamAdults(),
		"only the members of its channel can play this track":    en.ErrorStreamMembers(),
		"only YouTube Music Premium members can play this track": en.ErrorStreamPremium(),
	}
	words := map[string]func(string) string{
		AccountOff: en.ErrorStreamAccountOff, AccountSignIn: en.ErrorStreamAccountSignIn,
		AccountSignedOut: en.ErrorStreamAccountSignedOut, AccountEnded: en.ErrorStreamAccountEnded,
		AccountNotFirst: en.ErrorStreamAccountNotFirst, AccountSaver: en.ErrorStreamAccountSaver,
		AccountNoTools: en.ErrorStreamAccountNoTools, AccountRefused: en.ErrorStreamAccountRefused,
	}
	if len(words) != len(accountWords) {
		t.Errorf("%d ways have words, the catalog %d", len(accountWords), len(words))
	}
	for plain, said := range whys {
		if plain != said {
			t.Errorf("the catalog says %q, the error %q", said, plain)
		}
		for way, say := range words {
			e := &accountFailure{err: failure{plain, ErrUnavailable}, way: way}
			want := say(said)
			if way == AccountOff && Reason(e) == ReasonSignedIn {
				want = en.ErrorStreamSignedIn()
			}
			if e.Error() != want {
				t.Errorf("%s: the error says %q, the catalog %q", way, e.Error(), want)
			}
		}
	}
}

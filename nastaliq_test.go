package main

import (
	"testing"

	"github.com/egoist/mygo/i18n"

	"github.com/fzn0x/mixmusic/internal/sharecard"
)

// TestCardNastaliq sets a card in Urdu in the system's Nastaliq face,
// before the other fallbacks, and a card in another language, Arabic and
// Persian among them, or on a system without one, in the fallbacks alone.
func TestCardNastaliq(t *testing.T) {
	nastaliq, system := &sharecard.Font{}, []*sharecard.Font{{}, {}}
	old := cardNastaliq
	t.Cleanup(func() { cardNastaliq = old })
	cardNastaliq = func() []*sharecard.Font { return []*sharecard.Font{nastaliq} }
	for _, lang := range []string{"ur", "ur-PK", "UR_in"} {
		got := cardFallback(lang, system)
		if len(got) != 3 || got[0] != nastaliq || got[1] != system[0] || got[2] != system[1] {
			t.Errorf("%s: the fallbacks are %v, want the Nastaliq face, then the system's", lang, got)
		}
	}
	for _, lang := range []string{"ar", "fa", "en", "uz", ""} {
		if got := cardFallback(lang, system); len(got) != 2 {
			t.Errorf("%s: the fallbacks are %v, want the system's alone", lang, got)
		}
	}
	cardNastaliq = func() []*sharecard.Font { return nil }
	if got := cardFallback("ur", system); len(got) != 2 {
		t.Errorf("without a Nastaliq face, the fallbacks are %v, want the system's", got)
	}
}

// TestCardFamilyInUrdu gives a card the Nastaliq face first when the
// window shows Urdu.
func TestCardFamilyInUrdu(t *testing.T) {
	nastaliq := &sharecard.Font{}
	old := cardNastaliq
	t.Cleanup(func() { cardNastaliq = old })
	cardNastaliq = func() []*sharecard.Font { return []*sharecard.Font{nastaliq} }
	was := i18n.Locale()
	t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return was }) })
	s := newTestShell(t)
	i18n.SetLocaleFunc(func() string { return "ur" })
	if fb := s.lk.cardFamily().Fallback; len(fb) == 0 || fb[0] != nastaliq {
		t.Errorf("a card in Urdu falls back first to %v, want the Nastaliq face", fb)
	}
	i18n.SetLocaleFunc(func() string { return "en" })
	if fb := s.lk.cardFamily().Fallback; len(fb) > 0 && fb[0] == nastaliq {
		t.Error("a card in English falls back to the Nastaliq face first")
	}
}

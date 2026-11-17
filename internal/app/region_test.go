package app

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/i18n"

	"github.com/fzn0x/mixmusic/locales"
)

// fakeRegion is a computer's language and regional formats, as mygo.App
// reports them; its settings stay as they are.
type fakeRegion struct {
	shown     string
	region    i18n.Region
	preferred []string
}

func (f *fakeRegion) Locale() string               { return f.shown }
func (f *fakeRegion) Region() i18n.Region          { return f.region }
func (f *fakeRegion) PreferredLanguages() []string { return f.preferred }
func (f *fakeRegion) Languages() []string          { return locales.Catalog.Languages() }

// TestMessagesFollowTheRegion says the messages, once MixMusic follows
// the region, in English words with a British computer's formats, then
// an American's as the region changes, then in the pseudo-language a
// chosen language keeps (#801).
func TestMessagesFollowTheRegion(t *testing.T) {
	fake := &fakeRegion{shown: "en", region: i18n.Region{Locale: "en-GB", FirstWeekday: time.Monday, HourCycle: 24}, preferred: []string{"en-GB"}}
	was, wasFrom := i18n.Locale(), regionFrom
	regionFrom = fake
	lastRegionPick.Store(nil)
	t.Cleanup(func() {
		i18n.SetLocaleFunc(func() string { return was })
		regionFrom = wasFrom
		lastRegionPick.Store(nil)
	})
	followRegion()
	day := time.Date(2026, 11, 17, 15, 4, 0, 0, time.Local)
	say := func() string { return strings.ReplaceAll(locales.UpdatesLastChecked(day), " ", " ") }
	if got, want := say(), " Last checked 17 Nov 2026, 15:04."; got != want {
		t.Errorf("British: %q, want %q", got, want)
	}
	fake.region = i18n.Region{Locale: "en-US", FirstWeekday: time.Sunday, HourCycle: 12}
	if got, want := say(), " Last checked Nov 17, 2026, 3:04 PM."; got != want {
		t.Errorf("American: %q, want %q", got, want)
	}
	fake.shown = i18n.PseudoAccents
	if got := i18n.Locale(); got != i18n.PseudoAccents {
		t.Errorf("the pseudo-language's locale is %q", got)
	}
	if got := say(); !strings.HasPrefix(got, "[") {
		t.Errorf("the pseudo-language says %q", got)
	}
}

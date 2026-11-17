package app

// Dates, times and numbers are written in the computer's regional formats,
// not the language's (#801): MyGo says the messages in the language of the
// catalog it shows, so English, which has one catalog, wrote every date
// the United States' way. The words stay the catalog's; MyGo's own
// controls follow the region already.

import (
	"sync/atomic"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"

	"github.com/fzn0x/mixmusic/locales"
)

// followRegion makes the messages, which MyGo says in i18n.Locale, and the
// dates and numbers the windows write in it, take the computer's regional
// formats, as they change (locales.FormatLocale). It runs on the main
// thread once the app is ready: before, mygo.App.Locale waits for the
// main thread.
func followRegion() { i18n.SetLocaleFunc(regionalLocale) }

// regionFrom is where regionalLocale reads the language the app shows and
// the computer's formats and languages: mygo.App, or a test's fake.
var regionFrom interface {
	Locale() string
	Region() i18n.Region
	PreferredLanguages() []string
	Languages() []string
} = mygo.App

// regionPick is the locale regionalLocale returned for a language and a
// region.
type regionPick struct {
	shown  string
	region i18n.Region
	locale string
}

// lastRegionPick is the last regionPick, for every message asks
// regionalLocale.
var lastRegionPick atomic.Pointer[regionPick]

// regionalLocale returns the language the app shows with the computer's
// regional formats, from any goroutine. It works it out again only when
// the language or the formats changed: a change of the computer's
// languages alone, which leaves both as they were, waits for the next.
func regionalLocale() string {
	shown, region := regionFrom.Locale(), regionFrom.Region()
	if p := lastRegionPick.Load(); p != nil && p.shown == shown && p.region == region {
		return p.locale
	}
	p := &regionPick{shown, region, locales.FormatLocale(shown, region, regionFrom.PreferredLanguages(), regionFrom.Languages())}
	lastRegionPick.Store(p)
	return p.locale
}

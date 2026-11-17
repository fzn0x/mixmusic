package main

import (
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/locales"
)

// TestDatesAndNumbersInTheLanguage writes the days, months, times and
// counts of messages in the way of the language asked for, whatever
// language the message itself falls back to (#775): a Korean day isn't
// "Nov 17일", nor an Icelandic count 1,234.
//
// Icelandic and Indian English have no catalog, so their words are
// English: the language of the dates and counts is theirs alone. Once
// German, Hindi and the rest got catalogs (#564), they no longer could
// show that.
func TestDatesAndNumbersInTheLanguage(t *testing.T) {
	for _, tag := range []string{"is", "en-IN"} {
		if _, lang, _ := locales.Catalog.Message(tag, "recapBusiest"); lang != "en" {
			t.Fatalf("%s's messages come from %s's catalog now: pick a language without one", tag, lang)
		}
	}
	day := time.Date(2026, 11, 17, 15, 4, 0, 0, time.Local)
	march := time.Date(2026, 3, 1, 12, 0, 0, 0, time.Local)
	for _, tc := range []struct{ got, want string }{
		{locales.In("en").FormatDayMonth(day), "Nov 17"},
		{locales.In("ko").FormatDayMonth(day), "11월 17일"},
		{locales.In("en").FormatWeekdayDayMonth(day), "Tue, Nov 17"},
		{locales.In("is").FormatWeekdayDayMonth(day), "þri., 17. nóv."},
		{locales.In("en").RecapBusiest(march, 1234), "You listened most in March: 1,234 minutes."},
		{locales.In("is").RecapBusiest(march, 1234), "You listened most in mars: 1.234 minutes."},
		{locales.In("en").FormatMinutes(1234), "1,234 min"},
		{locales.In("is").FormatHourMinutes(20, 5), "20 hr 5 min"},
		{locales.In("en-IN").StatsFromSpotify(123456), "1,23,456 from Spotify"},
		{locales.In("en").StatsVennLabel(1, 2), "1 play from Spotify, 2 from YouTube Music"},
		{locales.In("en").PlaylistReadLoadingOf(1200, 3400), "Loading tracks · 1,200 of 3,400"},
		{locales.In("en").UpdatesOutSince(day), "Out since Nov 17, 2026."},
	} {
		if got := strings.ReplaceAll(tc.got, " ", " "); got != tc.want {
			t.Errorf("%q, want %q", got, tc.want)
		}
	}
}

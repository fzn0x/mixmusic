package locales

import (
	"testing"
	"time"

	"github.com/egoist/mygo/i18n"
)

// TestFormatLocaleFollowsTheRegion picks the locale of the messages from
// the regional formats each system reports, as mygo.App.Region has them,
// faked here: the computer's settings stay as they are. Whatever the
// region, the words stay those of the catalog shown (#801).
func TestFormatLocaleFollowsTheRegion(t *testing.T) {
	langs := Catalog.Languages()
	for _, tc := range []struct {
		name      string
		shown     string
		region    i18n.Region
		preferred []string
		want      string
	}{
		// Windows: GetLocaleInfoEx's LOCALE_SNAME and the hour of
		// LOCALE_STIMEFORMAT, apart from the display languages.
		{"windows english (united kingdom)", "en", i18n.Region{Locale: "en-GB", FirstWeekday: time.Monday, HourCycle: 24}, []string{"en-GB"}, "en-GB-u-fw-mon-hc-h23"},
		{"windows english (united states), formats of the united kingdom", "en", i18n.Region{Locale: "en-GB", FirstWeekday: time.Monday, HourCycle: 24}, []string{"en-US"}, "en-GB-u-fw-mon-hc-h23"},
		{"windows english (united states)", "en", i18n.Region{Locale: "en-US", FirstWeekday: time.Sunday, HourCycle: 12}, []string{"en-US"}, "en-US-u-fw-sun-hc-h12"},
		{"windows english, formats of germany", "en", i18n.Region{Locale: "de-DE", FirstWeekday: time.Monday, HourCycle: 24}, []string{"en-US"}, "en-DE-u-fw-mon-hc-h23"},
		// macOS: the current locale, whose region en_US@rg=gbzzzz names
		// (MyGo reads en-GB), and the 24-hour time switch.
		{"macos english, region united kingdom", "en", i18n.Region{Locale: "en-GB", FirstWeekday: time.Monday, HourCycle: 24}, []string{"en-US"}, "en-GB-u-fw-mon-hc-h23"},
		{"macos english (united states), 24-hour time", "en", i18n.Region{Locale: "en-US", FirstWeekday: time.Sunday, HourCycle: 24}, []string{"en-US"}, "en-US-u-fw-sun-hc-h23"},
		{"macos traditional chinese, region china", "zh-Hant", i18n.Region{Locale: "zh-CN", FirstWeekday: time.Monday, HourCycle: 24}, []string{"zh-Hant-TW"}, "zh-Hant-CN-u-fw-mon-hc-h23"},
		// Linux: LC_ALL, LC_TIME or LANG, as en_IE.UTF-8.
		{"linux LC_TIME=en_IE.UTF-8", "en", i18n.Region{Locale: "en-IE", FirstWeekday: time.Monday, HourCycle: 24}, []string{"en-US"}, "en-IE-u-fw-mon-hc-h23"},
		{"linux LANG=es_MX.UTF-8", "es", i18n.Region{Locale: "es-MX", FirstWeekday: time.Sunday, HourCycle: 12}, []string{"es-MX"}, "es-MX-u-fw-sun-hc-h12"},
		// No formats told: the first of the computer's languages that is
		// the language shown.
		{"no region, english (australia) preferred", "en", i18n.Region{}, []string{"is-IS", "en-AU"}, "en-AU"},
		{"no region, no english preferred", "en", i18n.Region{}, []string{"is-IS"}, "en"},
		// A language chosen in Settings: the region's formats when it
		// speaks it, else the computer's languages', else its own.
		{"chosen french, formats of canada", "fr", i18n.Region{Locale: "fr-CA", FirstWeekday: time.Sunday, HourCycle: 24}, []string{"de-DE", "fr-CA"}, "fr-CA-u-fw-sun-hc-h23"},
		{"chosen english, formats of germany, british english preferred", "en", i18n.Region{Locale: "de-DE", FirstWeekday: time.Monday, HourCycle: 24}, []string{"de-DE", "en-GB"}, "en-GB"},
		{"chosen french, formats of germany", "fr", i18n.Region{Locale: "de-DE", FirstWeekday: time.Monday, HourCycle: 24}, []string{"de-DE"}, "fr"},
		// A region whose tag would pick another catalog's words.
		{"brazilian portuguese, formats of angola", "pt-BR", i18n.Region{Locale: "pt-AO", FirstWeekday: time.Monday, HourCycle: 24}, []string{"pt-BR"}, "pt-BR-u-fw-mon-hc-h23"},
		{"european portuguese, formats of brazil", "pt-PT", i18n.Region{Locale: "pt-BR", FirstWeekday: time.Sunday, HourCycle: 24}, []string{"pt-PT"}, "pt-PT-u-fw-sun-hc-h23"},
		// The pseudo-languages stay as they are.
		{"pseudo accents", i18n.PseudoAccents, i18n.Region{Locale: "en-GB", FirstWeekday: time.Monday, HourCycle: 24}, []string{"en-GB"}, i18n.PseudoAccents},
		{"pseudo bidi", i18n.PseudoBidi, i18n.Region{Locale: "en-GB", FirstWeekday: time.Monday, HourCycle: 24}, []string{"en-GB"}, i18n.PseudoBidi},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatLocale(tc.shown, tc.region, tc.preferred, langs)
			if got != tc.want {
				t.Errorf("FormatLocale(%q, %+v, %q) = %q, want %q", tc.shown, tc.region, tc.preferred, got, tc.want)
			}
			_, words, _ := Catalog.Message(got, "updatesLastChecked")
			if _, shown, _ := Catalog.Message(tc.shown, "updatesLastChecked"); words != shown {
				t.Errorf("%s says %s's words, not %s's", got, words, shown)
			}
		})
	}
}

// TestFormatLocaleKeepsTheWords says every language in the formats of
// every region CLDR has, as System shows it: its words stay its catalog's.
func TestFormatLocaleKeepsTheWords(t *testing.T) {
	langs := Catalog.Languages()
	for _, shown := range langs {
		_, want, _ := Catalog.Message(shown, "updatesLastChecked")
		for _, region := range i18n.FormatLocales() {
			tag := FormatLocale(shown, i18n.Region{Locale: region, FirstWeekday: time.Monday, HourCycle: 24}, []string{shown}, langs)
			if _, got, _ := Catalog.Message(tag, "updatesLastChecked"); got != want {
				t.Errorf("%s with the formats of %s is %s, which says %s's words, not %s's", shown, region, tag, got, want)
			}
		}
	}
}

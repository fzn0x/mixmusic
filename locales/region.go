package locales

import (
	"cmp"
	"slices"
	"strings"

	"github.com/egoist/mygo/i18n"
)

// FormatLocale returns the locale MixMusic says its messages in (#801):
// the language shown, the catalog the app shows, with the computer's
// regional formats, region, as mygo.App.Region reports them. Its words
// stay shown's, as a message of en-GB is en's, and its dates, times and
// numbers are the region's, its 12- or 24-hour clock too: English read
// with British formats is en-GB, with German ones en-DE. preferred are the
// computer's languages, languages the app's, as mygo.App has them.
//
// The region's formats go with the language System shows, and with a
// language chosen in Settings that the region speaks. A language chosen
// otherwise takes those of the first of the computer's languages that is
// it, else its own. A region whose tag would pick another catalog's words,
// as pt-AO picks pt-PT's where pt-BR shows, keeps the language's formats
// with the region's clock and first day of the week. The pseudo-languages
// stay as they are, for their messages to show them.
func FormatLocale(shown string, region i18n.Region, preferred, languages []string) string {
	if shown == "" || i18n.IsPseudo(shown) || len(languages) == 0 {
		return shown
	}
	words := catalogOf(shown, languages)
	system := cmp.Or(i18n.Match(preferred, languages), languages[0])
	if region.Locale != "" && (shown == system || i18n.Base(region.Locale) == i18n.Base(shown)) {
		if tag := region.FormatLocale(shown); catalogOf(tag, languages) == words {
			return tag
		}
		return i18n.Region{FirstWeekday: region.FirstWeekday, HourCycle: region.HourCycle}.FormatLocale(shown)
	}
	for _, p := range preferred {
		if i18n.Base(p) != i18n.Base(shown) {
			continue
		}
		if tag := (i18n.Region{Locale: p}).FormatLocale(shown); catalogOf(tag, languages) == words {
			return tag
		}
		break
	}
	return shown
}

// catalogOf returns the language of languages whose catalog says the
// messages of tag, as Catalog.Format looks them up: tag's own or its
// nearest parent's, as en for en-GB and pt-PT for pt-AO, else the one
// that suits it best.
func catalogOf(tag string, languages []string) string {
	tag, _, _ = strings.Cut(i18n.Normalize(tag), "-u-")
	for i, t := 0, tag; t != "" && i < 8; i, t = i+1, i18n.Parent(t) {
		if slices.Contains(languages, t) {
			return t
		}
	}
	return i18n.Match([]string{tag}, languages)
}

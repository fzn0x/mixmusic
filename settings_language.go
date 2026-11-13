package main

import (
	"slices"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings("appearance", 5, (*shell).languageSettings) }

// languageSettings choose the language MixMusic shows, of those of its
// catalogs (locales/), each named in itself. MyGo keeps the choice.
func (s *shell) languageSettings(c *ui.Context) {
	s.group(c, locales.LanguageTitle(), func() {
		s.row(c, locales.LanguageTitle(), locales.LanguageDescription(), func() {
			tags := mygo.App.Languages()
			names := make([]string, len(tags))
			for i, t := range tags {
				names[i] = i18n.Autonym(t)
			}
			chosen := ""
			if i := slices.Index(tags, c.Locale()); i >= 0 {
				chosen = names[i]
			}
			if s.selectOf(c.Key("language"), &chosen, names).Width(220).Label(locales.LanguageTitle()).Changed() {
				if i := slices.Index(names, chosen); i >= 0 {
					mygo.App.SetLocale(tags[i])
				}
			}
		}, "language", "english", "chinese", "语言", "中文", "简体中文")
	})
}

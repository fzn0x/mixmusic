package main

import (
	"cmp"
	"slices"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings("appearance", 5, (*shell).languageSettings) }

// languageSettings choose the language MixMusic shows: System, which
// follows the computer's languages, or one of its catalogs' (locales/),
// each named in itself. MyGo keeps the choice.
func (s *shell) languageSettings(c *ui.Context) {
	s.group(c, locales.LanguageTitle(), func() {
		s.row(c, locales.LanguageTitle(), locales.LanguageDescription(), func() {
			tags := append([]string{""}, mygo.App.Languages()...)
			names := []string{locales.LanguageSystem()}
			for _, t := range tags[1:] {
				names = append(names, i18n.Autonym(t))
			}
			// shortcut: MyGo doesn't say whether a language was chosen, so one
			// chosen that the computer's languages would pick shows as System;
			// upgrade if MyGo tells the choice.
			chosen := names[0]
			system := cmp.Or(i18n.Match(mygo.App.PreferredLanguages(), tags[1:]), tags[1])
			if i := slices.Index(tags, c.Locale()); i > 0 && tags[i] != system {
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

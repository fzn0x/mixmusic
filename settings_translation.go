package main

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings("about", 12, (*shell).translationSettings) }

// fixTranslation is where CONTRIBUTING tells how to fix a translation.
const fixTranslation = "https://github.com/fzn0x/mixmusic/blob/main/CONTRIBUTING.md#adding-or-fixing-a-language"

// translationSettings say, in a language a machine translated, that it
// did, and where a speaker of it sends fixes. Other languages show
// nothing.
func (s *shell) translationSettings(c *ui.Context) {
	if !locales.TranslatedByMachine(c.Locale()) {
		return
	}
	s.group(c, locales.AboutTranslation(), func() {
		s.row(c, locales.AboutTranslation(), locales.AboutTranslationMachine(), func() {
			s.linkOf(c, locales.AboutTranslationFix(), fixTranslation)
		}, "translation", "machine translated", "language", "fix")
	})
}

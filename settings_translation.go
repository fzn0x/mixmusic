package main

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings("about", 12, (*shell).translationSettings) }

// fixTranslation is where CONTRIBUTING tells how to fix a translation.
const fixTranslation = "https://github.com/fzn0x/mixmusic/blob/main/CONTRIBUTING.md#adding-or-fixing-a-language"

// translationSettings name, in a language whose file names them, the
// people who translated it; and say, in a language a machine translated,
// that it did, and where a speaker of it sends fixes. Other languages show
// nothing.
func (s *shell) translationSettings(c *ui.Context) {
	tag := c.Locale()
	names, machine := locales.Translators(tag), locales.TranslatedByMachine(tag)
	if len(names) == 0 && !machine {
		return
	}
	s.group(c, locales.AboutTranslation(), func() {
		if len(names) > 0 {
			s.row(c, locales.AboutTranslators(), locales.AboutTranslatorsNames(strings.Join(names, locales.AboutTranslatorsSeparator())), func() {
				// The machine's row has the link.
				if !machine {
					s.linkOf(c, locales.AboutTranslationFix(), fixTranslation)
				}
			}, "translators", "translation", "language")
		}
		if machine {
			s.row(c, locales.AboutTranslation(), locales.AboutTranslationMachine(), func() {
				s.linkOf(c, locales.AboutTranslationFix(), fixTranslation)
			}, "translation", "machine translated", "language", "fix")
		}
	})
}

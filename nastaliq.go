package main

// Urdu in Nastaliq (#564): Urdu is read in Nastaliq, the sloping hand its
// books and papers are set in, and the upright Naskh of Arabic reads as
// another language's. MixMusic's faces have Latin letters alone, so Urdu's
// letters come from the system's fonts. A card lays out its own lines, as
// tall as their face, so a card in Urdu sets them in a Nastaliq face the
// system has. The window keeps the system's choice for now: a Nastaliq
// line is 2.5 em high, about twice Selawik's, which MyGo's rows and fields
// don't make room for yet (#816, egoist/mygo#2016).

import (
	"slices"
	"strings"

	"github.com/fzn0x/mixmusic/internal/sharecard"
)

// cardNastaliq finds the system's Nastaliq face; it is a variable for the
// tests.
var cardNastaliq = sharecard.Nastaliq

// inNastaliq reports whether lang, a BCP 47 tag, is read in Nastaliq:
// Urdu, in any region.
func inNastaliq(lang string) bool {
	base, _, _ := strings.Cut(strings.ToLower(lang), "-")
	base, _, _ = strings.Cut(base, "_")
	return base == "ur"
}

// cardFallback is system, the fonts a card takes the letters its face
// lacks from, for a card shown in lang: after the system's Nastaliq face
// in Urdu, when it has one, so that Urdu's letters are set in it, and a
// Latin title keeps the card's face.
func cardFallback(lang string, system []*sharecard.Font) []*sharecard.Font {
	if !inNastaliq(lang) {
		return system
	}
	face := cardNastaliq()
	if len(face) == 0 {
		return system
	}
	return append(slices.Clone(face), system...)
}

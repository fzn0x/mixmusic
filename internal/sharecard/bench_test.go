package sharecard

import (
	"runtime"
	"testing"
)

// benchTitles are the titles the benchmarks draw: Latin letters, which
// the bundled face has, and scripts that come from the system's fonts.
var benchTitles = []struct{ name, title, artist string }{
	{"latin", "Glass Harbor", "North Pier"},
	{"hindi", "क्षमा करो मेरे दिल", "श्रेया घोषाल"},
	{"mixed", "Party 🎉 星空の下で歌う สวัสดี", "مرحبا بالعالم"},
}

// benchRender draws a square card of each title, with a cover or without.
func benchRender(b *testing.B, fam Family, withCover bool) {
	for _, tc := range benchTitles {
		b.Run(tc.name, func(b *testing.B) {
			card := song(Square, dark, tc.title, tc.artist, withCover)
			b.ReportAllocs()
			for b.Loop() {
				if _, err := card.Render(fam); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkRender draws a square song card with a cover, as Share does,
// once per title. Run it with -benchmem to see what a card costs.
func BenchmarkRender(b *testing.B) { benchRender(b, family(b), true) }

// BenchmarkRenderText draws the cards without a cover, so that the text
// takes more of the time.
func BenchmarkRenderText(b *testing.B) { benchRender(b, family(b), false) }

// BenchmarkSystemFallback reads the system's fallback fonts, as the first
// card of a run does.
func BenchmarkSystemFallback(b *testing.B) {
	files := systemFontFiles(runtime.GOOS)
	if len(files) == 0 {
		b.Skip("the system has no fallback fonts")
	}
	b.ReportAllocs()
	for b.Loop() {
		if len(loadFonts(files)) == 0 {
			b.Fatal("read no fonts")
		}
	}
}

// BenchmarkFirstCard reads the system's fallback fonts afresh and draws a
// card of each title with them, as the first card of a run does.
func BenchmarkFirstCard(b *testing.B) {
	files := systemFontFiles(runtime.GOOS)
	if len(files) == 0 {
		b.Skip("the system has no fallback fonts")
	}
	fam := family(b)
	for _, tc := range benchTitles {
		b.Run(tc.name, func(b *testing.B) {
			card := song(Square, dark, tc.title, tc.artist, false)
			b.ReportAllocs()
			for b.Loop() {
				fam.Fallback = loadFonts(files)
				if _, err := card.Render(fam); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

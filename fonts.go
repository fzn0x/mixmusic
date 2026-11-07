package main

import (
	"embed"
	"log"
	"sync"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
)

// The interface's face is Selawik, as the web interface's was, in
// TrueType, which MyGo reads: the WOFF2 files of the web interface
// unpacked. OFL.txt is their license. Mochi's round face, Quicksand, which
// no system has, comes along in its static weights, as MyGo reads no
// variable font; quicksand-OFL.txt is theirs. Roboto Flex, the Material
// themes' face, comes as static instances of its variable font, at the
// weights of Material's type scale, and Archivo SemiCondensed ExtraBold
// sets the Walnut themes' headings; robotoflex-OFL.txt and archivo-OFL.txt
// are their licenses.
//
//go:embed fonts/*.ttf fonts/*.txt
var fontFiles embed.FS

const (
	// face is the family of the interface's text.
	face = "Selawik"
	// faceSemilight is Selawik Semilight, the nav labels' weight, 350 in
	// the web interface. It says it weighs 300, as Selawik Light does, so
	// it is registered under a family of its own, which MyGo keeps to its
	// file (fonts).
	faceSemilight = "Selawik Semilight"
	// faceRound is Quicksand, Mochi's face.
	faceRound = "Quicksand"
	// faceMaterial is Roboto Flex, the Material themes' face, and
	// faceArchivo Archivo SemiCondensed, the Walnut themes' headings'.
	faceMaterial = "Roboto Flex"
	faceArchivo  = "Archivo SemiCondensed"
)

var fontsOnce sync.Once

// fonts are the faces MixMusic registers, in the order it registers them.
// Selawik Light and Semilight both say they weigh 300, and are both of the
// family Selawik in their names: weight 300 of Selawik is the one that
// comes first, so Light does, for the page titles and pivots. Registered
// from a map, in another order each launch, the titles were Semilight in
// about half of them, heavier and wider (#241). Before MyGo 0.8, Windows
// and Linux looked the nav labels' Selawik Semilight up as the family of
// its file, Selawik, and set them in Light; every system keeps it to its
// file now.
var fonts = []struct{ file, family string }{
	{"fonts/selawkl.ttf", face},
	{"fonts/selawk.ttf", face},
	{"fonts/selawksb.ttf", face},
	{"fonts/selawkb.ttf", face},
	{"fonts/selawksl.ttf", faceSemilight},
	{"fonts/quicksand-light.ttf", faceRound},
	{"fonts/quicksand-regular.ttf", faceRound},
	{"fonts/quicksand-bold.ttf", faceRound},
	{"fonts/robotoflex-regular.ttf", faceMaterial},
	{"fonts/robotoflex-medium.ttf", faceMaterial},
	{"fonts/robotoflex-bold.ttf", faceMaterial},
	{"fonts/archivo-semicondensed-extrabold.ttf", faceArchivo},
}

// registerFonts registers the faces once.
func registerFonts() {
	fontsOnce.Do(func() {
		for _, f := range fonts {
			data, err := fontFiles.ReadFile(f.file)
			if err == nil {
				err = ui.RegisterFont(data, f.family)
			}
			if err != nil {
				log.Printf("mixmusic: the font %s: %v", f.file, err)
			}
		}
	})
}

// fontNotices are the licenses of the faces MixMusic includes, which its
// copies must reproduce, as those of its code. Settings › About shows them
// in every frame, and every section does while the user searches the
// settings: they are read once.
var fontNotices = sync.OnceValue(func() []app.Notice {
	var notices []app.Notice
	for _, f := range []struct{ name, file string }{
		{"Selawik, the interface's face", "fonts/OFL.txt"},
		{"Quicksand, Mochi's face", "fonts/quicksand-OFL.txt"},
		{"Roboto Flex, the Material themes' face", "fonts/robotoflex-OFL.txt"},
		{"Archivo, the Walnut themes' headings' face", "fonts/archivo-OFL.txt"},
	} {
		if text, err := fontFiles.ReadFile(f.file); err == nil {
			notices = append(notices, app.Notice{Name: f.name, Text: string(text)})
		}
	}
	return notices
})

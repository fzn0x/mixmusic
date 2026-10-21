package native

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
// variable font; quicksand-OFL.txt is theirs.
//
//go:embed fonts/*.ttf fonts/*.txt
var fontFiles embed.FS

const (
	// face is the family of the interface's text.
	face = "Selawik"
	// faceSemilight is Selawik Semilight, which says it weighs 300 as
	// Selawik Light does, so it has a family of its own: the nav labels'
	// weight, 350 in the web interface.
	faceSemilight = "Selawik Semilight"
	// faceRound is Quicksand, Mochi's face.
	faceRound = "Quicksand"
)

var fontsOnce sync.Once

// registerFonts registers the faces once.
func registerFonts() {
	fontsOnce.Do(func() {
		for file, family := range map[string]string{
			"fonts/selawkl.ttf":           face,
			"fonts/selawk.ttf":            face,
			"fonts/selawksb.ttf":          face,
			"fonts/selawkb.ttf":           face,
			"fonts/selawksl.ttf":          faceSemilight,
			"fonts/quicksand-light.ttf":   faceRound,
			"fonts/quicksand-regular.ttf": faceRound,
			"fonts/quicksand-bold.ttf":    faceRound,
		} {
			data, err := fontFiles.ReadFile(file)
			if err == nil {
				err = ui.RegisterFont(data, family)
			}
			if err != nil {
				log.Printf("mixmusic: the font %s: %v", file, err)
			}
		}
	})
}

// fontNotices are the licenses of the faces MixMusic includes, which its
// copies must reproduce, as those of its code.
func fontNotices() []app.Notice {
	var notices []app.Notice
	for _, f := range []struct{ name, file string }{
		{"Selawik, the interface's face", "fonts/OFL.txt"},
		{"Quicksand, Mochi's face", "fonts/quicksand-OFL.txt"},
	} {
		if text, err := fontFiles.ReadFile(f.file); err == nil {
			notices = append(notices, app.Notice{Name: f.name, Text: string(text)})
		}
	}
	return notices
}

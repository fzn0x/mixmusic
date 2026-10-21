package native

import (
	"embed"
	"log"
	"sync"

	"github.com/egoist/mygo/ui"
)

// The interface's face is Selawik, as the web interface's was, in
// TrueType, which MyGo reads: the WOFF2 files of the web interface
// unpacked. OFL.txt is their license.
//
//go:embed fonts/*.ttf fonts/OFL.txt
var fontFiles embed.FS

const (
	// face is the family of the interface's text.
	face = "Selawik"
	// faceSemilight is Selawik Semilight, which says it weighs 300 as
	// Selawik Light does, so it has a family of its own: the nav labels'
	// weight, 350 in the web interface.
	faceSemilight = "Selawik Semilight"
)

var fontsOnce sync.Once

// registerFonts registers Selawik once.
func registerFonts() {
	fontsOnce.Do(func() {
		for file, family := range map[string]string{
			"fonts/selawkl.ttf":  face,
			"fonts/selawk.ttf":   face,
			"fonts/selawksb.ttf": face,
			"fonts/selawkb.ttf":  face,
			"fonts/selawksl.ttf": faceSemilight,
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

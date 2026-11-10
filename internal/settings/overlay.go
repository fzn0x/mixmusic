package settings

// DefaultOverlayPort is the port of the now-playing server for overlays
// until the user chooses another.
const DefaultOverlayPort = 8284

// Overlay is Settings › On the desktop › Now playing for overlays (#344):
// a server on this computer that tells what plays to the overlays of
// streams.
type Overlay struct {
	// OverlayServer turns the server on. It is off at first.
	OverlayServer bool `json:"overlayServer"`
	// OverlayPort is the port the server listens on, on 127.0.0.1 only.
	OverlayPort int `json:"overlayPort"`
}

// defaultOverlay is how a new install serves overlays: not at all.
func defaultOverlay() Overlay { return Overlay{OverlayPort: DefaultOverlayPort} }

// clean is o with a port that a program can listen on, 1024 to 65535, or
// the default.
func (o Overlay) clean() Overlay {
	if o.OverlayPort < 1024 || o.OverlayPort > 65535 {
		o.OverlayPort = DefaultOverlayPort
	}
	return o
}

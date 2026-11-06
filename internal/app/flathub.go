package app

// Flathub builds MixMusic from source with the manifest of
// packaging/flathub, whose build.sh leaves flathubMarker in the app: the
// release's own .flatpak, packed from the Linux archive, has none. Flathub
// updates its copies, through the desktop's software center, and they run
// the yt-dlp that comes with them, which Flathub updates too.

// flathubMarker is the file only Flathub's build of MixMusic has.
const flathubMarker = "/app/share/mixmusic/flathub"

// flathubYTDLP is the yt-dlp of Flathub's build, a module of its manifest.
const flathubYTDLP = "/app/bin/yt-dlp"

// bundledYTDLP returns the yt-dlp that comes with this copy of MixMusic,
// which it runs instead of downloading one, or "" when it has none.
func bundledYTDLP() string {
	if installKind() == "flathub" {
		return flathubYTDLP
	}
	return ""
}

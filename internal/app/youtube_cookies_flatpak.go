package app

// Browsers installed from Flathub keep their profiles in folders of their
// own, in ~/.var/app, where yt-dlp doesn't look for those of the browsers
// made from Chromium. On Linux, each is a browser of its own, "Chrome
// (Flatpak)", listed once its folder is there, so that a browser installed
// both ways shows twice, each saying where it's from. MixMusic's own
// Flatpak may read those folders (packaging/flatpak).

import "path/filepath"

// browserFlatpak is a browser's app on Flathub: its ID, and where in its
// folder, ~/.var/app/<ID>, it keeps its profiles, of which the import reads
// the first that's there.
type browserFlatpak struct {
	browser, app string
	dirs         []string
}

// browserFlatpaks are the browsers' apps on Flathub, with the folders their
// manifests give them. A browser made from Chromium keeps its profiles in
// the app's XDG_CONFIG_HOME, as it does in ~/.config outside it. Firefox
// keeps them in ~/.mozilla, which its app persists, or, when that has none,
// in its XDG_CONFIG_HOME, as Firefox 147 and later make new ones.
var browserFlatpaks = []browserFlatpak{
	{"firefox", "org.mozilla.firefox", []string{".mozilla/firefox", "config/mozilla/firefox"}},
	{"chrome", "com.google.Chrome", []string{"config/google-chrome"}},
	{"chromium", "org.chromium.Chromium", []string{"config/chromium"}},
	{"edge", "com.microsoft.Edge", []string{"config/microsoft-edge"}},
	{"brave", "com.brave.Browser", []string{"config/BraveSoftware/Brave-Browser"}},
}

// browsersOn returns the browsers whose cookies MixMusic imports on goos:
// on Linux, each that has an app on Flathub is followed by its Flatpak.
func browsersOn(goos string) []cookieBrowser {
	var list []cookieBrowser
	for _, b := range cookieBrowsers {
		if b.darwin && goos != "darwin" {
			continue
		}
		list = append(list, b)
		for _, f := range browserFlatpaks {
			if goos == "linux" && f.browser == b.id {
				list = append(list, f.of(b))
			}
		}
	}
	return list
}

// of returns b installed as this Flatpak.
func (f browserFlatpak) of(b cookieBrowser) cookieBrowser {
	return cookieBrowser{
		id: b.id + "-flatpak", name: b.name + " (Flatpak)", engine: b.engine, flatpak: true,
		dirs: func(_, home string, _ func(string) string) []string {
			dirs := make([]string, len(f.dirs))
			for i, d := range f.dirs {
				dirs[i] = filepath.Join(home, ".var", "app", f.app, filepath.FromSlash(d))
			}
			return dirs
		},
	}
}

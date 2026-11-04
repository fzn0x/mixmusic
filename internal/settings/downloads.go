package settings

import (
	"path/filepath"
	"strings"
)

// Downloads is where the downloads are kept, and when they download.
type Downloads struct {
	// DownloadsFolder is where the downloads are kept, an absolute path;
	// "" is the default folder of the system.
	DownloadsFolder string `json:"downloadsFolder"`
	// DownloadOnMetered lets downloads go on over a metered connection, as
	// a phone's hotspot: they wait without it.
	DownloadOnMetered bool `json:"downloadOnMetered"`
	// DownloadAdded downloads the tracks added to a playlist, an album, a
	// mix or the liked songs downloaded, as its page shows them: off, the
	// page offers them.
	DownloadAdded bool `json:"downloadAdded"`
}

// defaultDownloads is when a new install downloads.
func defaultDownloads() Downloads {
	var d Downloads
	d.DownloadAdded = true
	return d
}

// clean is d with its folder cleaned, or the default one in place of a
// relative one, which would follow the working directory.
func (d Downloads) clean() Downloads {
	if d.DownloadsFolder = strings.TrimSpace(d.DownloadsFolder); d.DownloadsFolder != "" {
		if filepath.IsAbs(d.DownloadsFolder) {
			d.DownloadsFolder = filepath.Clean(d.DownloadsFolder)
		} else {
			d.DownloadsFolder = ""
		}
	}
	return d
}

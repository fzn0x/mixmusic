package artwork

import (
	"net/url"
	"strings"
)

// stillSizes are the stills YouTube keeps of a video, the largest first:
// a music video's or an upload's cover is one of them, at
// https://i.ytimg.com/vi/<id>/<name>.jpg.
var stillSizes = []struct {
	name          string
	width, height int
}{
	{"maxresdefault", 1280, 720},
	{"hq720", 1280, 720},
	{"sddefault", 640, 480},
	{"hqdefault", 480, 360},
	{"mqdefault", 320, 180},
	{"default", 120, 90},
}

// stillOf splits a YouTube still's address into the video's ID, the path
// it is under, and the still's name, without its extension.
func stillOf(rawURL string) (id, dir, name string, ok bool) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Host != "i.ytimg.com" && u.Host != "img.youtube.com") {
		return "", "", "", false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 3 || (parts[0] != "vi" && parts[0] != "vi_webp") || parts[1] == "" {
		return "", "", "", false
	}
	stem := parts[2]
	if i := strings.LastIndexByte(stem, '.'); i > 0 {
		stem = stem[:i]
	}
	for _, s := range stillSizes {
		if s.name == stem {
			return parts[1], parts[0], stem, true
		}
	}
	return "", "", "", false
}

// Stills returns the addresses of the stills of the same video that are
// larger than the one rawURL names, the largest first, then rawURL itself:
// the first that answers is the cover to keep. A URL that isn't a YouTube
// still gives nil.
//
// YouTube keeps a different set of stills for each video, and a still it
// doesn't keep answers 404, so the caller tries them in order.
func Stills(rawURL string) []string {
	id, dir, name, ok := stillOf(rawURL)
	if !ok {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	ext := u.Path[strings.LastIndexByte(u.Path, '.'):]
	base := u.Scheme + "://" + u.Host + "/" + dir + "/" + id + "/"
	width := 0
	for _, s := range stillSizes {
		if s.name == name {
			width = s.width
		}
	}
	out := make([]string, 0, len(stillSizes))
	for _, s := range stillSizes {
		if s.width > width {
			out = append(out, base+s.name+ext)
		}
	}
	return append(out, rawURL)
}

// stillsKept bounds the videos whose still is remembered in a run: a
// session that plays more forgets the oldest whole map rather than grow.
const stillsKept = 4000

// StillsWidth is the pixels across of the still rawURL names, 0 for an
// address that isn't a still.
func StillsWidth(rawURL string) int {
	_, _, name, ok := stillOf(rawURL)
	if !ok {
		return 0
	}
	for _, s := range stillSizes {
		if s.name == name {
			return s.width
		}
	}
	return 0
}

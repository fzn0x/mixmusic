package native

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// clock writes a duration as 3:28 or 1:02:03.
func clock(d time.Duration) string {
	if d <= 0 {
		return "0:00"
	}
	total := int(d / time.Second)
	h, m, s := total/3600, total/60%60, total%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// longDuration writes a duration as "1 hr 12 min" or "42 min".
func longDuration(d time.Duration) string {
	min := int((d + 30*time.Second) / time.Minute)
	if min < 60 {
		return fmt.Sprintf("%d min", min)
	}
	return fmt.Sprintf("%d hr %d min", min/60, min%60)
}

// plural writes a count with its noun: "1 track", "12 tracks".
func plural(n int, one string, many ...string) string {
	noun := one + "s"
	if len(many) > 0 {
		noun = many[0]
	}
	if n == 1 {
		noun = one
	}
	return thousands(n) + " " + noun
}

// thousands writes n with its thousands apart, as 12,345.
func thousands(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	if neg {
		s = "-" + s
	}
	return s
}

// names joins the names of artists.
func names(refs []media.Ref) string {
	out := make([]string, len(refs))
	for i, r := range refs {
		out[i] = r.Name
	}
	return strings.Join(out, ", ")
}

// artworkURL picks the smallest image at least size pixels wide, else the
// largest.
func artworkURL(images []media.Image, size int) string {
	if len(images) == 0 {
		return ""
	}
	sorted := slices.Clone(images)
	slices.SortStableFunc(sorted, func(a, b media.Image) int { return a.Width - b.Width })
	for _, img := range sorted {
		if img.Width >= size {
			return img.URL
		}
	}
	return sorted[len(sorted)-1].URL
}

// sourceName names a service.
func sourceName(s media.Source) string {
	switch s {
	case media.SourceSpotify:
		return "Spotify"
	case media.SourceYouTube:
		return "YouTube Music"
	}
	return "MixMusic"
}

// trackLine is "Title — Artist", for notices.
func trackLine(t media.Track) string { return t.Title + " — " + names(t.Artists) }

// byteSize writes a size as "512 MB".
func byteSize(n int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if v < 10 && i > 0 {
		return strconv.FormatFloat(v, 'f', 1, 64) + " " + units[i]
	}
	return strconv.FormatFloat(v, 'f', 0, 64) + " " + units[i]
}

// ms is v milliseconds.
func ms(v int) time.Duration { return time.Duration(v) * time.Millisecond }

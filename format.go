package main

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/i18n"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
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

// clockLength writes how long a track or an episode lasts, as clock does,
// but rounded up to the whole second, as YouTube Music lists lengths: a
// stream 213.26 s long is 3:34, as its row in a list says, where clock
// would say 3:33 (#718). Less than a millisecond past a second, as a
// decoder's length may be, doesn't count. Positions keep clock.
func clockLength(d time.Duration) string {
	return clock((d.Truncate(time.Millisecond) + time.Second - time.Millisecond).Truncate(time.Second))
}

// longDuration writes a duration as "1 hr 12 min" or "42 min".
func longDuration(d time.Duration) string {
	min := int((d + 30*time.Second) / time.Minute)
	if min < 60 {
		return locales.FormatMinutes(strconv.Itoa(min))
	}
	return locales.FormatHourMinutes(strconv.Itoa(min/60), strconv.Itoa(min%60))
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

// weekdayName writes the weekday of t in the app's language: "Tue".
func weekdayName(t time.Time) string {
	return i18n.FormatDate(i18n.Locale(), t, i18n.DateOptions{Skeleton: "E"})
}

// monthName writes the month of t in the app's language: "Oct".
func monthName(t time.Time) string {
	return i18n.FormatDate(i18n.Locale(), t, i18n.DateOptions{Skeleton: "MMM"})
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
	case media.SourceFile:
		return locales.FormatThisComputer()
	}
	return "MixMusic"
}

// trackLine is "Title - Artist", for notices.
func trackLine(t media.Track) string { return t.Title + " - " + names(t.Artists) }

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

// compactCount writes a count as YouTube Music does, in three figures at
// most: 148, 2.2K, 757K, 13M, 1.2B. It rounds down, as YouTube Music does:
// 1,299 is 1.2K.
func compactCount(n int64) string {
	v, unit := float64(n), ""
	for _, u := range []string{"K", "M", "B"} {
		if v < 1000 {
			break
		}
		v, unit = v/1000, u
	}
	if unit != "" && v < 10 {
		return strconv.FormatFloat(math.Floor(v*10)/10, 'f', -1, 64) + unit
	}
	return strconv.FormatFloat(math.Floor(v), 'f', 0, 64) + unit
}

// playsLabel says how many times a track played, as "1.2B plays", or how
// many views a video has, as "905M views", or "" where its service says
// not.
func playsLabel(t media.Track) string {
	switch {
	case t.Plays <= 0:
		return ""
	case t.Video:
		return locales.FormatViews(int(t.Plays), compactCount(t.Plays))
	}
	return locales.FormatPlays(int(t.Plays), compactCount(t.Plays))
}

// ms is v milliseconds.
func ms(v int) time.Duration { return time.Duration(v) * time.Millisecond }

// speedLabel writes a speed as 1.25× or 2×.
func speedLabel(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) + "×" }

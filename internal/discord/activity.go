package discord

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
)

// ActivityType is what an activity shows the user doing.
type ActivityType int

const (
	// Playing shows the user "Playing" the application.
	Playing ActivityType = 0
	// Listening shows the user "Listening to" the application, for music.
	Listening ActivityType = 2
	// Watching shows the user "Watching" the application.
	Watching ActivityType = 3
	// Competing shows the user "Competing in" the application.
	Competing ActivityType = 5
)

// StatusDisplay is the part of an activity the member list shows beside
// the user's name.
type StatusDisplay int

const (
	// ShowName shows the application's name, as Discord does by default.
	ShowName StatusDisplay = 0
	// ShowState shows the activity's state: for music, the artists.
	ShowState StatusDisplay = 1
	// ShowDetails shows the activity's details: for music, the title.
	ShowDetails StatusDisplay = 2
)

// Activity is what the user's Discord profile shows them doing: for
// MixMusic, the track playing.
//
// Discord takes texts of 2 to 128 characters. The client pads a text of
// one character with a space, cuts a longer text with an ellipsis, and
// leaves out what is empty.
type Activity struct {
	// Type is the kind of activity, Listening for music.
	Type ActivityType
	// Details is the activity's first line: the track's title.
	Details string
	// State is the activity's second line: the track's artists.
	State string
	// Start and End are when the track started and when it will end, from
	// which Discord shows the time played and the time left. Either may be
	// zero.
	Start, End time.Time
	// LargeImage is the activity's picture: the https URL of the track's
	// artwork, or the key of an image uploaded to the Discord application.
	LargeImage string
	// LargeText is shown when the picture is hovered: the track's album.
	LargeText string
	// SmallImage is a small round picture at the corner of LargeImage, as
	// a badge of the track's service, and SmallText is shown when it is
	// hovered. Discord shows it only on a LargeImage.
	SmallImage, SmallText string
	// Buttons are links shown under the activity, as one that opens the
	// track on its service: Discord shows two at most, to the others who
	// see the user's profile.
	Buttons []Button
	// StatusDisplay is what the member list shows beside the user's name,
	// the application's name when what it names is empty.
	StatusDisplay StatusDisplay
}

// Button is a link under an activity: its label, and the http or https
// URL it opens.
type Button struct {
	Label, URL string
}

// Discord takes texts of minText to maxText characters, and images of up to
// maxImage. A button's label is 1 to maxLabel characters long, its URL up to
// maxURL, and an activity has maxButtons buttons at most. It counts
// characters as JavaScript does, in UTF-16 code units.
const (
	minText    = 2
	maxText    = 128
	maxImage   = 256
	maxLabel   = 32
	maxURL     = 512
	maxButtons = 2
)

// activity is an Activity as Discord's SET_ACTIVITY command takes it.
type activity struct {
	Type          ActivityType  `json:"type"`
	StatusDisplay StatusDisplay `json:"status_display_type,omitempty"`
	Details       string        `json:"details,omitempty"`
	State         string        `json:"state,omitempty"`
	Timestamps    *timestamps   `json:"timestamps,omitempty"`
	Assets        *assets       `json:"assets,omitempty"`
	Buttons       []button      `json:"buttons,omitempty"`
}

// button is a Button as SET_ACTIVITY takes it.
type button struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// timestamps are an activity's start and end, in Unix milliseconds.
type timestamps struct {
	Start int64 `json:"start,omitempty"`
	End   int64 `json:"end,omitempty"`
}

// assets are an activity's pictures.
type assets struct {
	LargeImage string `json:"large_image,omitempty"`
	LargeText  string `json:"large_text,omitempty"`
	SmallImage string `json:"small_image,omitempty"`
	SmallText  string `json:"small_text,omitempty"`
}

// wire returns a as Discord takes it, its texts fitted to Discord's limits.
// It leaves out what is empty, an image or a URL too long for Discord,
// which a cut would break, and the buttons past the second.
func (a Activity) wire() *activity {
	w := &activity{Type: a.Type, Details: fit(a.Details), State: fit(a.State)}
	if (a.StatusDisplay == ShowState && w.State != "") || (a.StatusDisplay == ShowDetails && w.Details != "") {
		w.StatusDisplay = a.StatusDisplay
	}
	if !a.Start.IsZero() || !a.End.IsZero() {
		w.Timestamps = &timestamps{Start: unixMilli(a.Start), End: unixMilli(a.End)}
	}
	as := assets{LargeImage: imageOf(a.LargeImage), LargeText: fit(a.LargeText),
		SmallImage: imageOf(a.SmallImage), SmallText: fit(a.SmallText)}
	if as != (assets{}) {
		w.Assets = &as
	}
	for _, b := range a.Buttons {
		label, url := fitTo(b.Label, maxLabel), strings.TrimSpace(b.URL)
		if len(w.Buttons) == maxButtons || label == "" || !webURL(url) {
			continue
		}
		w.Buttons = append(w.Buttons, button{Label: label, URL: url})
	}
	return w
}

// imageOf is an image as Discord takes it: without surrounding spaces, and
// none when it is longer than Discord takes.
func imageOf(s string) string {
	if s = strings.TrimSpace(s); utf16Len(s) > maxImage {
		return ""
	}
	return s
}

// webURL reports whether a button can open s: an http or https URL that
// Discord takes.
func webURL(s string) bool {
	return (strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://")) && utf16Len(s) <= maxURL
}

// fit returns s without surrounding spaces, padded with a space when it is
// one character long and cut with an ellipsis when it is longer than
// Discord takes. An empty s stays empty.
func fit(s string) string {
	s = fitTo(s, maxText)
	if s != "" && utf16Len(s) < minText {
		return s + " "
	}
	return s
}

// fitTo returns s without surrounding spaces, cut with an ellipsis when it
// is longer than n.
func fitTo(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf16Len(s) <= n {
		return s
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		w := utf16.RuneLen(r)
		if used+w > n-1 {
			break
		}
		b.WriteRune(r)
		used += w
	}
	return strings.TrimRightFunc(b.String(), unicode.IsSpace) + "…"
}

// utf16Len returns the length of s in UTF-16 code units.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// unixMilli returns t in Unix milliseconds, or 0 for the zero time.
func unixMilli(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

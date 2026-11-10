package remote

import (
	"math"
	"strings"
)

// State is what the player of the computer controlled does, as it sends
// it: the track that plays, if any, whether it plays, where it is and how
// long it is, in milliseconds, the volume, and the tracks after it.
type State struct {
	Track *Track `json:"track,omitempty"`
	// Playing is set while it plays, Loading while it loads, and Live
	// while it is a live stream, which has no length and no seeking.
	Playing    bool  `json:"playing,omitempty"`
	Loading    bool  `json:"loading,omitempty"`
	Live       bool  `json:"live,omitempty"`
	PositionMs int64 `json:"positionMs,omitempty"`
	DurationMs int64 `json:"durationMs,omitempty"`
	// Volume is from 0 to 1, without Muted.
	Volume float64 `json:"volume"`
	Muted  bool    `json:"muted,omitempty"`
	// Next are the tracks after the one that plays, MaxNext at most.
	Next []Track `json:"next,omitempty"`
}

// MaxNext bounds the tracks after the current one that a state tells.
const MaxNext = 20

// Track is a track as a state tells it: its title, its artists, its album
// and the address of its cover, where it has one online.
type Track struct {
	Title   string   `json:"title"`
	Artists []string `json:"artists,omitempty"`
	Album   string   `json:"album,omitempty"`
	Artwork string   `json:"artwork,omitempty"`
	// Entry names the track's entry in the queue, for the commands on the
	// queue: an older MixMusic tells none.
	Entry string `json:"entry,omitempty"`
}

// The commands a session sends.
const (
	OpPlay     = "play"
	OpPause    = "pause"
	OpNext     = "next"
	OpPrevious = "previous"
	OpSeek     = "seek"
	OpVolume   = "volume"
)

// The commands on the queue, which a computer whose welcome tells
// FeatureQueue takes: OpPlayEntry plays the entry Entry, OpRemove takes it
// out of the queue, OpMove moves it to the place of the entry To, as
// dragging it there does, and OpClear empties the queue after the track
// that plays. Only the entries after that track move or go.
const (
	OpPlayEntry = "playEntry"
	OpRemove    = "remove"
	OpMove      = "move"
	OpClear     = "clear"
)

// queueOp reports whether op is a command on the queue.
func queueOp(op string) bool {
	switch op {
	case OpPlayEntry, OpRemove, OpMove, OpClear:
		return true
	}
	return false
}

// Command is what the computer controlling asks the player of the one
// controlled: Op, and for OpSeek where to, for OpVolume the volume, from 0
// to 1, and for the commands on the queue the entries, as the tracks of a
// state name them.
type Command struct {
	Op         string  `json:"op"`
	PositionMs int64   `json:"positionMs,omitempty"`
	Volume     float64 `json:"volume,omitempty"`
	Entry      string  `json:"entry,omitempty"`
	To         string  `json:"to,omitempty"`
}

// clean is c with its values in range, or false for a command the
// protocol doesn't have.
func (c Command) clean() (Command, bool) {
	switch c.Op {
	case OpPlay, OpPause, OpNext, OpPrevious, OpClear:
		return Command{Op: c.Op}, true
	case OpPlayEntry, OpRemove:
		e := cleanEntry(c.Entry)
		return Command{Op: c.Op, Entry: e}, e != ""
	case OpMove:
		e, to := cleanEntry(c.Entry), cleanEntry(c.To)
		return Command{Op: c.Op, Entry: e, To: to}, e != "" && to != "" && e != to
	case OpSeek:
		return Command{Op: c.Op, PositionMs: max(c.PositionMs, 0)}, true
	case OpVolume:
		v := c.Volume
		if math.IsNaN(v) {
			return c, false
		}
		return Command{Op: c.Op, Volume: min(max(v, 0), 1)}, true
	}
	return c, false
}

// cleanState is s with its words as cleanText makes them, and no more
// than MaxNext tracks after the current one: what a computer sends is
// shown, so it can't be trusted more than its user.
func cleanState(s State) State {
	if s.Track != nil {
		t := cleanTrack(*s.Track)
		s.Track = &t
	}
	if len(s.Next) > MaxNext {
		s.Next = s.Next[:MaxNext]
	}
	next := make([]Track, len(s.Next))
	for i, t := range s.Next {
		next[i] = cleanTrack(t)
	}
	s.Next = next
	if math.IsNaN(s.Volume) {
		s.Volume = 0
	}
	s.Volume = min(max(s.Volume, 0), 1)
	s.PositionMs, s.DurationMs = max(s.PositionMs, 0), max(s.DurationMs, 0)
	return s
}

func cleanTrack(t Track) Track {
	t.Title, t.Album = cleanText(t.Title, 300), cleanText(t.Album, 300)
	if len(t.Artists) > 10 {
		t.Artists = t.Artists[:10]
	}
	artists := make([]string, 0, len(t.Artists))
	for _, a := range t.Artists {
		if a = cleanText(a, 200); a != "" {
			artists = append(artists, a)
		}
	}
	t.Artists = artists
	if !strings.HasPrefix(t.Artwork, "https://") || len(t.Artwork) > 2048 {
		t.Artwork = ""
	}
	t.Entry = cleanEntry(t.Entry)
	return t
}

// cleanEntry is the name of an entry of the queue, or "" for one that
// isn't one.
func cleanEntry(s string) string { return cleanToken(s, 64) }

// cleanToken is s, or "" when it is longer than n bytes, or holds other
// than the letters, digits and punctuation of ASCII.
func cleanToken(s string, n int) string {
	if len(s) > n {
		return ""
	}
	for i := 0; i < len(s); i++ {
		if s[i] <= ' ' || s[i] >= 0x7f {
			return ""
		}
	}
	return s
}

// cleanText is s without control characters, at most n characters.
func cleanText(s string, n int) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	if r := []rune(s); len(r) > n {
		s = string(r[:n])
	}
	return strings.TrimSpace(s)
}

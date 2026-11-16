package room

import (
	"encoding/json"
	"math"
	"time"

	"github.com/fzn0x/mixmusic/internal/remote"
)

// message is one of the protocol's messages. Its type says which fields it
// holds.
type message struct {
	Type string `json:"type"`
	// hello: what the guest wants, how it proves it may, its name in
	// rooms and the protocol's version.
	Want    string `json:"want,omitempty"`
	By      string `json:"by,omitempty"`
	Name    string `json:"name,omitempty"`
	Version int    `json:"version,omitempty"`
	// hello and welcome: what the side can do beyond the version. An
	// older MixMusic tells none.
	Features []string `json:"features,omitempty"`
	// hello and spake: the side's SPAKE2 message; confirm and welcome:
	// its confirmation.
	Spake []byte `json:"spake,omitempty"`
	MAC   []byte `json:"mac,omitempty"`
	// refused and ended: why.
	Reason string `json:"reason,omitempty"`
	// welcome: what the room shows before the guest joins it.
	Preview *Preview `json:"preview,omitempty"`
	// state, queue and members: what the room plays, the host's queue,
	// and who listens.
	State   *State   `json:"state,omitempty"`
	Queue   *Queue   `json:"queue,omitempty"`
	Members []string `json:"members,omitempty"`
	// rules: what the host lets its guests do (rules.go). add and added
	// tell a song a guest adds, in Track, by its number, in Seq, and what
	// became of it, in Reason (songs.go).
	Rules *Rules `json:"rules,omitempty"`
	// control: what a guest asks the host's player to do (control.go).
	Do string `json:"do,omitempty"`
	// host, hosting, reach and moved: the room moving to a guest
	// (move.go), of the key Key, in hex, whose room listens at Port, at
	// Addr, and keeps Seats and Blocked; OK tells a guest reaches it.
	Key     string `json:"key,omitempty"`
	Addr    string `json:"addr,omitempty"`
	Port    int    `json:"port,omitempty"`
	Seats   []Seat `json:"seats,omitempty"`
	Blocked []Seat `json:"blocked,omitempty"`
	OK      bool   `json:"ok,omitempty"`
	// prepare and ready: a track change, which the host starts once its
	// guests have it ready.
	Seq   int64              `json:"seq,omitempty"`
	Entry string             `json:"entry,omitempty"`
	Track *remote.QueueTrack `json:"track,omitempty"`
	// ping and pong: when the guest sent the ping, by its clock, and when
	// the host answered, by its own.
	Sent   time.Duration `json:"sent,omitempty"`
	HostAt time.Duration `json:"hostAt,omitempty"`
}

// Preview is what a room shows a guest that proved its code, before it
// joins: who hosts, what plays, and how many listen.
type Preview struct {
	Host      string `json:"host"`
	Title     string `json:"title,omitempty"`
	Artists   string `json:"artists,omitempty"`
	Listeners int    `json:"listeners"`
}

func (p Preview) clean() Preview {
	return Preview{Host: CleanName(p.Host), Title: cleanText(p.Title, 300), Artists: cleanText(p.Artists, 300),
		Listeners: min(max(p.Listeners, 0), MaxGuests)}
}

// State is what the room plays: the host's entry and its track, where it
// is, by the host's clock, as a Playhead, and how the queue goes on.
type State struct {
	// Entry names the host's entry that plays, and Track is its track; ""
	// and nil while nothing does.
	Entry string             `json:"entry,omitempty"`
	Track *remote.QueueTrack `json:"track,omitempty"`
	// Seq is that of the track change whose start this is, if it is one.
	Seq      int64         `json:"seq,omitempty"`
	Playing  bool          `json:"playing,omitempty"`
	Starts   bool          `json:"starts,omitempty"`
	Loading  bool          `json:"loading,omitempty"`
	Live     bool          `json:"live,omitempty"`
	Position time.Duration `json:"position,omitempty"`
	At       time.Duration `json:"at,omitempty"`
	Speed    float64       `json:"speed,omitempty"`
	// Volume is the host's, from 0 to 1, which a guest that follows it
	// takes.
	Volume  float64 `json:"volume"`
	Shuffle bool    `json:"shuffle,omitempty"`
	Repeat  string  `json:"repeat,omitempty"`
}

// Playhead is where the state's track is.
func (s State) Playhead() Playhead {
	return Playhead{Entry: s.Entry, Playing: s.Playing && !s.Loading, Starts: s.Starts, Position: s.Position, At: s.At,
		Speed: s.Speed}
}

func (s State) clean() State {
	if s.Track != nil {
		t := s.Track.Clean()
		s.Track = &t
	}
	s.Entry = cleanEntry(s.Entry)
	s.Position = max(s.Position, 0)
	if math.IsNaN(s.Speed) || s.Speed <= 0 {
		s.Speed = 1
	}
	s.Speed = min(max(s.Speed, 0.25), 4)
	if math.IsNaN(s.Volume) {
		s.Volume = 0
	}
	s.Volume = min(max(s.Volume, 0), 1)
	if s.Repeat != "all" && s.Repeat != "one" {
		s.Repeat = ""
	}
	return s
}

// MaxQueue bounds the tracks of the host's queue the guests see.
const MaxQueue = remote.MaxHandOver

// Queue is the host's queue, in the order it plays, and the index of the
// entry that plays in it, with each entry's name.
type Queue struct {
	Tracks  []remote.QueueTrack `json:"tracks"`
	Entries []string            `json:"entries,omitempty"`
	Index   int                 `json:"index"`
}

func (q Queue) clean() Queue {
	if len(q.Tracks) > MaxQueue {
		q.Tracks = q.Tracks[:MaxQueue]
	}
	tracks := make([]remote.QueueTrack, len(q.Tracks))
	entries := make([]string, len(q.Tracks))
	for i, t := range q.Tracks {
		tracks[i] = t.Clean()
		if i < len(q.Entries) {
			entries[i] = cleanEntry(q.Entries[i])
		}
	}
	q.Tracks, q.Entries = tracks, entries
	q.Index = min(max(q.Index, 0), max(len(q.Tracks)-1, 0))
	return q
}

// fitQueue is q with no more than MaxQueue tracks, a tenth of them at most
// before the one that plays, and fewer around it until its message is no
// longer than the protocol takes.
func fitQueue(q Queue) Queue {
	window := func(before, after int) Queue {
		if len(q.Tracks) == 0 {
			return Queue{}
		}
		from := max(q.Index-before, 0)
		to := min(q.Index+1+after, len(q.Tracks))
		out := Queue{Tracks: q.Tracks[from:to], Index: q.Index - from}
		if len(q.Entries) == len(q.Tracks) {
			out.Entries = q.Entries[from:to]
		}
		return out
	}
	before := min(q.Index, MaxQueue/10)
	after := MaxQueue - 1 - before
	for {
		out := window(before, after)
		data, err := json.Marshal(message{Type: "queue", Queue: &out})
		if (err == nil && len(data) <= remote.MaxMessage) || before+after == 0 {
			return out
		}
		before, after = before/2, after/2
	}
}

// cleanEntry is the name of an entry of the queue, or "" for one that
// can't be one.
func cleanEntry(s string) string {
	if len(s) > 64 {
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
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			continue
		}
		if len(out) == n {
			break
		}
		out = append(out, r)
	}
	return string(out)
}

// cleanMembers are the names of the guests a host told, MaxGuests at most.
func cleanMembers(names []string) []string {
	if len(names) > MaxGuests {
		names = names[:MaxGuests]
	}
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = CleanName(n)
	}
	return out
}

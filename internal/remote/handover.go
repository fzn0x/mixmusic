package remote

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

// HandOver is what plays on the computer that controls, which it hands
// over to the one controlled: its queue, in the order it plays, the index
// of the track that plays in it, where that track is, in milliseconds,
// whether it plays, and how the queue goes on: shuffled, and repeated
// ("all" or "one", or "" for neither).
type HandOver struct {
	// ID tells the answer to this hand-over apart from one to another.
	ID         int64        `json:"id"`
	Queue      []QueueTrack `json:"queue"`
	Index      int          `json:"index"`
	PositionMs int64        `json:"positionMs,omitempty"`
	Playing    bool         `json:"playing,omitempty"`
	Shuffle    bool         `json:"shuffle,omitempty"`
	Repeat     string       `json:"repeat,omitempty"`
}

// MaxHandOver bounds the tracks of a hand-over: the one that plays, up to
// MaxHandOver/10 before it, and the rest after it. A long queue is cut, and
// so is one whose message would be longer than the protocol takes.
const MaxHandOver = 1000

// QueueTrack is a track of a queue handed over: where it comes from, its
// ID there, and what shows of it.
type QueueTrack struct {
	// Source is "youtube" or "spotify", whose ID ID is, as "yt:…" or
	// "sp:…"; or "file", a file of the computer that hands it over, which
	// tells no ID, as its path names nothing on the other: that one plays
	// its own file of the same song, if it has one.
	Source string `json:"source"`
	ID     string `json:"id,omitempty"`
	Title  string `json:"title"`
	// Artists and Album name the track's, with their IDs on its service.
	Artists    []Ref   `json:"artists,omitempty"`
	Album      *Ref    `json:"album,omitempty"`
	DurationMs int64   `json:"durationMs,omitempty"`
	Artwork    []Image `json:"artwork,omitempty"`
	Explicit   bool    `json:"explicit,omitempty"`
	// Video and VideoType tell a video of YouTube Music, as its service
	// does.
	Video     bool   `json:"video,omitempty"`
	VideoType string `json:"videoType,omitempty"`
	// Show is the podcast of an episode.
	Show *Ref `json:"show,omitempty"`
}

// Ref names an artist, an album or a podcast, with its ID when it has one.
type Ref struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
}

// Image is a cover at one size, online.
type Image struct {
	URL    string `json:"url"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// Taken is how the computer controlled took a hand-over: how many of its
// tracks it plays, and how many it left out, as files of the other
// computer it doesn't have; CurrentLeft is set when it left out the one
// that played, and plays from the next.
type Taken struct {
	// ID is the hand-over's.
	ID          int64 `json:"id"`
	Kept        int   `json:"kept"`
	Left        int   `json:"left,omitempty"`
	CurrentLeft bool  `json:"currentLeft,omitempty"`
}

// HandOver hands what plays over to the computer controlled, and returns
// how it took it, once it did: the caller then stops it here, unless it
// kept nothing. A computer whose welcome doesn't tell FeatureHandOver
// can't take it: ErrOlder.
func (s *Session) HandOver(ctx context.Context, h HandOver) (Taken, error) {
	if !s.Can(FeatureHandOver) {
		return Taken{}, ErrOlder
	}
	h = fitHandOver(cleanHandOver(h))
	if len(h.Queue) == 0 {
		return Taken{}, errors.New("remote: nothing to hand over")
	}
	s.handing.Lock()
	defer s.handing.Unlock()
	answer := make(chan Taken, 1)
	s.mu.Lock()
	s.handed++
	h.ID = s.handed
	s.taken = answer
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if s.taken == answer {
			s.taken = nil
		}
		s.mu.Unlock()
	}()
	select {
	case <-s.done:
		return Taken{}, s.Err()
	default:
	}
	if err := s.w.send(message{Type: "handover", HandOver: &h}, s.timing.silence()); err != nil {
		return Taken{}, err
	}
	for {
		select {
		case t := <-answer:
			if t.ID != h.ID {
				// The answer to a hand-over that gave up waiting.
				continue
			}
			return t, nil
		case <-s.done:
			return Taken{}, s.Err()
		case <-ctx.Done():
			return Taken{}, ctx.Err()
		}
	}
}

// tookOver hands the answer to a hand-over to the one that waits for it.
func (s *Session) tookOver(t Taken) {
	s.mu.Lock()
	answer := s.taken
	s.mu.Unlock()
	if answer == nil {
		return
	}
	select {
	case answer <- t:
	default:
	}
}

// takeOver runs a hand-over the computer at the other end of c sent, and
// answers how it went.
func (s *Server) takeOver(c *serverConn, h HandOver) error {
	h = cleanHandOver(h)
	taken := Taken{Left: len(h.Queue)}
	if len(h.Queue) > 0 {
		taken = s.Host.TakeOver(c.peer, h)
	}
	taken.ID = h.ID
	return c.w.send(message{Type: "taken", Taken: &taken}, s.Timing.silence())
}

// cleanHandOver is h with its words as cleanText makes them, its covers
// only from https, its index in its queue, and no more than MaxHandOver
// tracks around it: what a computer sends is played, so it can't be
// trusted more than its user.
func cleanHandOver(h HandOver) HandOver {
	h.Index = min(max(h.Index, 0), max(len(h.Queue)-1, 0))
	before := min(h.Index, MaxHandOver/10)
	h = window(h, before, MaxHandOver-1-before)
	queue := make([]QueueTrack, len(h.Queue))
	for i, t := range h.Queue {
		queue[i] = cleanQueueTrack(t)
	}
	h.Queue = queue
	h.PositionMs = max(h.PositionMs, 0)
	if h.Repeat != "all" && h.Repeat != "one" {
		h.Repeat = ""
	}
	return h
}

// window is h with no more than before tracks before the one that plays,
// and after after it.
func window(h HandOver, before, after int) HandOver {
	if len(h.Queue) == 0 {
		h.Index = 0
		return h
	}
	from := max(h.Index-before, 0)
	to := min(h.Index+1+after, len(h.Queue))
	h.Queue = h.Queue[from:to]
	h.Index -= from
	return h
}

// fitHandOver is h with fewer tracks around the one that plays, until its
// message is no longer than the protocol takes.
func fitHandOver(h HandOver) HandOver {
	before, after := h.Index, len(h.Queue)-h.Index-1
	for {
		out := window(h, before, after)
		data, err := json.Marshal(message{Type: "handover", HandOver: &out})
		if (err == nil && len(data) <= maxMessage) || before+after == 0 {
			return out
		}
		before, after = before/2, after/2
	}
}

func cleanQueueTrack(t QueueTrack) QueueTrack {
	out := QueueTrack{Source: cleanID(t.Source), Title: cleanText(t.Title, 300), DurationMs: max(t.DurationMs, 0),
		Explicit: t.Explicit, Video: t.Video, VideoType: cleanID(t.VideoType)}
	if out.Source != "file" {
		out.ID = cleanID(t.ID)
	}
	for _, a := range t.Artists {
		if len(out.Artists) == 10 {
			break
		}
		if r := cleanRef(a); r.Name != "" {
			out.Artists = append(out.Artists, r)
		}
	}
	if t.Album != nil {
		if r := cleanRef(*t.Album); r.Name != "" {
			out.Album = &r
		}
	}
	if t.Show != nil {
		if r := cleanRef(*t.Show); r.Name != "" {
			out.Show = &r
		}
	}
	for _, img := range t.Artwork {
		if len(out.Artwork) == 4 {
			break
		}
		if strings.HasPrefix(img.URL, "https://") && len(img.URL) <= 2048 && !strings.ContainsAny(img.URL, " \t\r\n") {
			out.Artwork = append(out.Artwork, Image{URL: img.URL, Width: min(max(img.Width, 0), 1<<16), Height: min(max(img.Height, 0), 1<<16)})
		}
	}
	return out
}

func cleanRef(r Ref) Ref { return Ref{ID: cleanID(r.ID), Name: cleanText(r.Name, 300)} }

// cleanID is an ID as a computer told it, or "" for one that can't be one.
func cleanID(s string) string { return cleanToken(s, 128) }

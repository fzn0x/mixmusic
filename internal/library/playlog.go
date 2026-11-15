package library

// The play log (#545): every play that counted, kept for the stats with
// no cap, in a file of its own beside library.json, plays.jsonl, a line
// of JSON a play, the oldest first. library.json keeps the last 1,000
// plays for History and Recently played: it is written whole on every
// change, so it is no place for what grows. The log is appended to as
// each play counts. A line a crash tore is skipped as the log is read, and
// the next append starts a line of its own; a play whose append failed is
// written with the next one. The log is read once, as the library opens,
// into memory, where a track is kept once however often it played.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/store"
)

// playLogName is the play log's file, in the library's folder.
const playLogName = "plays.jsonl"

// playLine is a play as a line of the log holds it: when it counted, and
// the track as the stats count it. Covers and the rest of a track stay
// out, which keeps a line near 200 bytes.
type playLine struct {
	// At is when the play counted, to the second, in local time.
	At string `json:"at"`
	// HeardMs is how long the play's audio sounded, in milliseconds: room
	// for #544 to write. 0 is unknown, and the stats count the listed
	// length.
	HeardMs int          `json:"heardMs,omitempty"`
	ID      string       `json:"id"`
	Source  media.Source `json:"source,omitempty"`
	Title   string       `json:"title,omitempty"`
	Artists []media.Ref  `json:"artists,omitempty"`
	Album   *media.Ref   `json:"album,omitempty"`
	// Year is the year of the album, where it is known: no track tells
	// it yet, which #544 brings.
	Year string `json:"year,omitempty"`
	// DurationMs is the track's listed length.
	DurationMs int  `json:"durationMs,omitempty"`
	Video      bool `json:"video,omitempty"`
	// Episode is set for a podcast's episode, which the stats leave out
	// and the Plays sort counts.
	Episode bool `json:"episode,omitempty"`
}

// loggedPlay is a play of the log in memory.
type loggedPlay struct {
	// at is when the play counted, in Unix seconds.
	at      int64
	heardMs int32
	// track is the play's track, of the log's tracks.
	track int32
}

// loggedTrack is a track the log holds, as the stats see it, and the year
// of its album.
type loggedTrack struct {
	media.Track
	year string
}

// playLog is the log of the plays, in memory as in its file. Plays and
// tracks only grow, but for a clear or a retarget, which put new slices in
// their place: a view of them stays as it was.
type playLog struct {
	path string

	mu     sync.Mutex
	plays  []loggedPlay
	tracks []loggedTrack
	// byKey finds a track of tracks by what its lines hold.
	byKey map[string]int32
	// pending holds the lines whose append failed, which go with the next.
	pending []byte
	// torn is set when the file may end partway through a line, which a
	// line break then ends before the next is written.
	torn bool
	// whole is set when the file is to be written whole, rather than
	// appended to: it doesn't hold what memory holds.
	whole bool
}

// openPlayLog reads the play log kept at path; found tells whether the
// file was there.
func openPlayLog(path string) (g *playLog, found bool, err error) {
	g = &playLog{path: path, byKey: map[string]int32{}}
	data, err := store.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return g, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	skipped := 0
	for rest := data; len(rest) > 0; {
		var line []byte
		line, rest, _ = bytes.Cut(rest, []byte{'\n'})
		if line = bytes.TrimSpace(line); len(line) > 0 && !g.read(line) {
			skipped++
		}
	}
	if skipped > 0 {
		log.Printf("mixmusic: %s: skipped %d lines that don't read, as one a crash cut short", path, skipped)
	}
	g.torn = len(data) > 0 && data[len(data)-1] != '\n'
	return g, true, nil
}

// read takes in a line of the file, and reports whether it read. A play
// that is the one before it again, as one written again after its append
// failed partway, counts once.
func (g *playLog) read(line []byte) bool {
	var l playLine
	if err := json.Unmarshal(line, &l); err != nil || l.ID == "" {
		return false
	}
	at, err := time.Parse(time.RFC3339, l.At)
	if err != nil {
		return false
	}
	p := loggedPlay{at: at.Unix(), heardMs: int32(l.HeardMs), track: g.intern(l)}
	if n := len(g.plays); n > 0 && g.plays[n-1] == p {
		return true
	}
	g.plays = append(g.plays, p)
	return true
}

// intern returns the track of a line, which tracks keeps once.
func (g *playLog) intern(l playLine) int32 {
	key := l.key()
	if i, ok := g.byKey[key]; ok {
		return i
	}
	t := loggedTrack{Track: media.Track{ID: l.ID, Source: l.Source, Title: l.Title, Artists: l.Artists, Album: l.Album,
		DurationMs: l.DurationMs, Video: l.Video}, year: l.Year}
	if t.Artists == nil {
		t.Artists = []media.Ref{}
	}
	if l.Episode {
		t.Episode = &media.Episode{}
	}
	i := int32(len(g.tracks))
	g.tracks = append(g.tracks, t)
	g.byKey[key] = i
	return i
}

// key names what a line tells of its track: the lines of a track that
// didn't change share it.
func (l playLine) key() string {
	var b strings.Builder
	field := func(s string) { b.WriteString(s); b.WriteByte(0) }
	field(l.ID)
	field(string(l.Source))
	field(l.Title)
	for _, a := range l.Artists {
		field(a.ID)
		field(a.Name)
	}
	b.WriteByte(1)
	if l.Album != nil {
		field(l.Album.ID)
		field(l.Album.Name)
	}
	field(l.Year)
	field(strconv.Itoa(l.DurationMs))
	field(strconv.FormatBool(l.Video))
	field(strconv.FormatBool(l.Episode))
	return b.String()
}

// lineOf returns the line of a play of t at at.
func lineOf(at time.Time, heardMs int, t loggedTrack) playLine {
	l := playLine{At: at.Format(time.RFC3339), HeardMs: heardMs, ID: t.ID, Source: t.Source, Title: t.Title,
		Artists: t.Artists, Album: t.Album, Year: t.year, DurationMs: t.DurationMs, Video: t.Video, Episode: t.IsEpisode()}
	if l.Album != nil && l.Album.ID == "" && l.Album.Name == "" {
		l.Album = nil
	}
	return l
}

// encode returns a line as the file holds it, its line break too.
func (l playLine) encode() []byte {
	b, err := json.Marshal(l)
	if err != nil {
		return nil // a line of strings, numbers and booleans always marshals
	}
	return append(b, '\n')
}

// add logs a play of t at at: in memory at once, and in the file.
func (g *playLog) add(at time.Time, t media.Track) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	l := lineOf(at, 0, loggedTrack{Track: t})
	g.plays = append(g.plays, loggedPlay{at: at.Unix(), track: g.intern(l)})
	g.pending = append(g.pending, l.encode()...)
	return g.flush()
}

// clear forgets every play. While the file can't be written, the next
// play writes it anew, holding only what played since.
func (g *playLog) clear() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.plays, g.tracks, g.byKey = nil, nil, map[string]int32{}
	g.pending, g.torn, g.whole = nil, false, true
	return g.flush()
}

// flush writes what the file doesn't hold yet: the lines pending, or the
// whole log.
func (g *playLog) flush() error {
	if g.whole {
		if err := g.writeWhole(g.plays, g.tracks); err != nil {
			return err
		}
		g.whole, g.pending, g.torn = false, nil, false
		return nil
	}
	if len(g.pending) == 0 {
		return nil
	}
	var start []byte
	if g.torn {
		start = []byte{'\n'}
	}
	n, err := appendFile(g.path, append(start, g.pending...))
	g.wrote(n-len(start), n >= len(start))
	return err
}

// wrote notes that the first n bytes of the lines pending reached the
// file, after the line break that ended a torn line when ended is set. A
// line cut short stays pending, to be written again after a line break.
func (g *playLog) wrote(n int, ended bool) {
	if !ended {
		return
	}
	n = max(0, min(n, len(g.pending)))
	done := bytes.LastIndexByte(g.pending[:n], '\n') + 1
	g.pending = append([]byte(nil), g.pending[done:]...)
	g.torn = n > done
}

// appendFile appends data to the file at path, and returns how much of it
// was written.
func appendFile(path string, data []byte) (int, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	n, err := f.Write(data)
	if err == nil {
		// What was written stays written: a sync that fails loses it only
		// if the system goes down too, and the reader allows for that.
		f.Sync()
	}
	return n, err
}

// writeWhole writes the file anew, holding plays: the old file or the new
// one, whole, whatever happens meanwhile.
func (g *playLog) writeWhole(plays []loggedPlay, tracks []loggedTrack) error {
	if err := os.MkdirAll(filepath.Dir(g.path), 0o755); err != nil {
		return err
	}
	return store.Replace(g.path, 0o600, func(f *os.File) error {
		w := bufio.NewWriter(f)
		for _, p := range plays {
			w.Write(lineOf(time.Unix(p.at, 0), int(p.heardMs), tracks[p.track]).encode())
		}
		return w.Flush()
	})
}

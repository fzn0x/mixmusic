package playlistio

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
)

// Format is a kind of file a list exports to.
type Format string

// The formats lists export to.
const (
	FormatCSV  Format = "csv"
	FormatM3U8 Format = "m3u8"
	FormatJSON Format = "json"
)

// FormatOf is the format of a file by its extension: CSV unless it's
// .m3u8, .m3u or .json.
func FormatOf(path string) Format {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".m3u8", ".m3u":
		return FormatM3U8
	case ".json":
		return FormatJSON
	}
	return FormatCSV
}

// Row is a track of a list to export, as each format writes it.
type Row struct {
	Title    string
	Artists  []string
	Album    string
	Duration time.Duration
	ISRC     string
	// Service names the track's service, as "YouTube Music".
	Service string
	// MixMusicID is the track's ID in MixMusic, as "yt:…".
	MixMusicID string
	// URL opens the track on its service, or is its file's file:// URL.
	URL string
	// Path is the track's file, of a track on this computer.
	Path string
}

// ExportList is a list to export: its name and its tracks.
type ExportList struct {
	Name string
	Rows []Row
}

// Write writes lists to w in format f.
func Write(w io.Writer, f Format, lists []ExportList) error {
	switch f {
	case FormatM3U8:
		return WriteM3U8(w, lists)
	case FormatJSON:
		return WriteJSON(w, lists)
	}
	return WriteCSV(w, lists)
}

// WriteCSV writes lists as CSV in UTF-8 with a byte order mark and CRLF
// line ends, as spreadsheets read it, in 9 columns: Playlist, Title,
// Artists, Album, Duration, ISRC, Service, MixMusic ID and URL. A cell a
// spreadsheet would take for a formula, which starts with = + - @, a tab
// or a carriage return, starts with an apostrophe.
func WriteCSV(w io.Writer, lists []ExportList) error {
	b := bufio.NewWriter(w)
	b.WriteString("\ufeff")
	c := csv.NewWriter(b)
	c.UseCRLF = true
	if err := c.Write(ownHeader); err != nil {
		return err
	}
	for _, l := range lists {
		for _, r := range l.Rows {
			rec := []string{l.Name, r.Title, strings.Join(r.Artists, ", "), r.Album, clock(r.Duration), r.ISRC, r.Service,
				r.MixMusicID, r.URL}
			for i := range rec {
				rec[i] = escapeCell(rec[i])
			}
			if err := c.Write(rec); err != nil {
				return err
			}
		}
	}
	c.Flush()
	if err := c.Error(); err != nil {
		return err
	}
	return b.Flush()
}

// escapeCell starts with an apostrophe a cell a spreadsheet would take
// for a formula.
func escapeCell(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// unescapeCell takes back the apostrophe escapeCell put.
func unescapeCell(s string) string {
	if len(s) > 1 && s[0] == '\'' && strings.ContainsRune("=+-@\t\r", rune(s[1])) {
		return s[1:]
	}
	return s
}

// clock is a length as "3:45", or "1:02:03"; "" when unknown.
func clock(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	s := int((d + time.Second/2) / time.Second)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// WriteM3U8 writes lists as M3U8: #EXTM3U, then each list's #PLAYLIST and
// its tracks, an #EXTINF line each, then its file's path, or its link.
// MixMusic reads its own back as they are, without a search (ownMark).
func WriteM3U8(w io.Writer, lists []ExportList) error {
	b := bufio.NewWriter(w)
	b.WriteString("#EXTM3U\n" + ownMark + ":1\n")
	for _, l := range lists {
		fmt.Fprintf(b, "#PLAYLIST:%s\n", oneLine(l.Name))
		for _, r := range l.Rows {
			where := r.Path
			if where == "" {
				where = r.URL
			}
			if where == "" {
				continue // nothing that names it
			}
			secs := -1
			if r.Duration > 0 {
				secs = int((r.Duration + time.Second/2) / time.Second)
			}
			name := oneLine(r.Title)
			if len(r.Artists) > 0 {
				name = oneLine(strings.Join(r.Artists, ", ")) + " - " + name
			}
			fmt.Fprintf(b, "#EXTINF:%d,%s\n", secs, name)
			if r.Album != "" {
				fmt.Fprintf(b, "#EXTALB:%s\n", oneLine(r.Album))
			}
			b.WriteString(oneLine(where) + "\n")
		}
	}
	return b.Flush()
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

// ownTrack is a track of MixMusic's own JSON files.
type ownTrack struct {
	Playlist   string   `json:"playlist"`
	Title      string   `json:"title"`
	Artists    []string `json:"artists"`
	Album      string   `json:"album,omitempty"`
	DurationMs int64    `json:"durationMs,omitempty"`
	ISRC       string   `json:"isrc,omitempty"`
	Service    string   `json:"service,omitempty"`
	MixMusicID string   `json:"mixmusicId,omitempty"`
	URL        string   `json:"url,omitempty"`
}

// WriteJSON writes lists as JSON: the 9 fields of the CSV's columns for
// each track, in an object MixMusic knows as its own.
func WriteJSON(w io.Writer, lists []ExportList) error {
	doc := struct {
		App     string     `json:"app"`
		Version int        `json:"version"`
		Tracks  []ownTrack `json:"tracks"`
	}{App: ownApp, Version: 1, Tracks: []ownTrack{}}
	for _, l := range lists {
		for _, r := range l.Rows {
			artists := r.Artists
			if artists == nil {
				artists = []string{}
			}
			doc.Tracks = append(doc.Tracks, ownTrack{Playlist: l.Name, Title: r.Title, Artists: artists, Album: r.Album,
				DurationMs: r.Duration.Milliseconds(), ISRC: r.ISRC, Service: r.Service, MixMusicID: r.MixMusicID, URL: r.URL})
		}
	}
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	e.SetEscapeHTML(false)
	return e.Encode(doc)
}

// EntryRow is the row of an entry, for a list of the songs an import
// didn't find, which reads back as they were.
func EntryRow(e Entry) Row {
	r := Row{Title: e.Title, Artists: e.Artists, Album: e.Album, Duration: e.Duration, ISRC: e.ISRC, Service: e.Service,
		URL: e.URL}
	if r.URL == "" {
		switch {
		case e.VideoID != "":
			r.URL = "https://music.youtube.com/watch?v=" + e.VideoID
		case e.SpotifyID != "":
			r.URL = "https://open.spotify.com/track/" + e.SpotifyID
		case e.Path != "":
			r.URL = e.Path
		}
	}
	return r
}

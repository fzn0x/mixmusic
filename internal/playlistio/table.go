package playlistio

import (
	"encoding/csv"
	"errors"
	"io"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Field is what a column of a table holds.
type Field int

// The fields of a table's columns.
const (
	FieldNone Field = iota
	FieldTitle
	FieldArtists
	FieldAlbum
	FieldDuration
	FieldISRC
	FieldVideoID
	FieldURL
	FieldSpotifyURI
	FieldPlaylist
	FieldService
	FieldMixMusicID
)

// Fields are the fields the user gives a table's columns by hand, in the
// order a menu lists them.
var Fields = []Field{FieldNone, FieldTitle, FieldArtists, FieldAlbum, FieldDuration, FieldISRC, FieldVideoID,
	FieldURL, FieldSpotifyURI, FieldPlaylist}

// fieldNames are the names of columns MixMusic knows, as names reads
// them: in lower case, of letters and digits alone, without a number at
// their end, as Google's "Artist Name 1".
var fieldNames = map[string]Field{
	"title": FieldTitle, "tracktitle": FieldTitle, "trackname": FieldTitle, "track": FieldTitle, "name": FieldTitle,
	"song": FieldTitle, "songname": FieldTitle, "songtitle": FieldTitle,
	"artist": FieldArtists, "artists": FieldArtists, "artistname": FieldArtists, "artistnames": FieldArtists,
	"performer": FieldArtists, "performers": FieldArtists, "creator": FieldArtists, "band": FieldArtists,
	"album": FieldAlbum, "albumname": FieldAlbum, "albumtitle": FieldAlbum, "release": FieldAlbum,
	"duration": FieldDuration, "durationms": FieldDuration, "length": FieldDuration, "lengthms": FieldDuration,
	"time": FieldDuration, "trackduration": FieldDuration, "trackdurationms": FieldDuration, "playtime": FieldDuration,
	"isrc":    FieldISRC,
	"videoid": FieldVideoID, "youtubeid": FieldVideoID, "youtubevideoid": FieldVideoID,
	"url": FieldURL, "link": FieldURL, "trackurl": FieldURL, "songurl": FieldURL, "youtubeurl": FieldURL,
	"spotifyurl": FieldURL, "href": FieldURL,
	"spotifyuri": FieldSpotifyURI, "trackuri": FieldSpotifyURI, "uri": FieldSpotifyURI, "spotifyid": FieldSpotifyURI,
	"spotifytrackuri": FieldSpotifyURI, "spotifytrackid": FieldSpotifyURI, "trackid": FieldSpotifyURI,
	"playlist": FieldPlaylist, "playlistname": FieldPlaylist, "playlisttitle": FieldPlaylist,
	"service": FieldService, "source": FieldService,
	"mixmusicid": FieldMixMusicID,
}

// ownHeader is the header of MixMusic's own CSV files (WriteCSV).
var ownHeader = []string{"Playlist", "Title", "Artists", "Album", "Duration", "ISRC", "Service", "MixMusic ID", "URL"}

// Table is a table of songs: the rows of a CSV or TSV file, and what its
// columns hold.
type Table struct {
	// Name names its list, unless a column names the playlists.
	Name string
	Rows [][]string
	// Header is the row that names the columns, -1 when none names one
	// MixMusic knows.
	Header int
	// Fields are what each column holds, as their names tell.
	Fields []Field
	// millis is set when the name of the duration's column tells it
	// counts milliseconds.
	millis bool
	own    bool
}

// Width is how many columns the table has.
func (t *Table) Width() int {
	n := 0
	for _, r := range t.Rows {
		n = max(n, len(r))
	}
	return n
}

// found reports whether the columns' names tell which holds the songs.
func (t *Table) found() bool { return t.Header >= 0 }

// listsOrSelf is the lists of a table whose columns' names tell what they
// hold, else the table, for the user to tell.
func (t *Table) listsOrSelf() ([]List, []*Table) {
	if !t.found() {
		return nil, []*Table{t}
	}
	return t.Lists(t.Fields, t.Header), nil
}

// readTable reads CSV or TSV text: its cells parted by commas, semicolons
// or tabs, whichever its first line has most of. The header is the first
// of its first 10 rows whose columns have names MixMusic knows, and one of
// IDs or links beats one of titles alone: Google's older lists head
// their songs with the playlist's own details.
func readTable(name, text string) (*Table, error) {
	text = strings.TrimPrefix(text, "\ufeff")
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = delimiter(text)
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	var rows [][]string
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, ErrNotAList
		}
		rows = append(rows, rec)
	}
	if len(rows) == 0 {
		return nil, ErrNoEntries
	}
	t := &Table{Name: name, Rows: rows, Header: -1}
	best := 0
	for i := range min(len(rows), 10) {
		fields, millis := fieldsOf(rows[i])
		score := 0
		for _, f := range fields {
			switch f {
			case FieldVideoID, FieldURL, FieldSpotifyURI, FieldMixMusicID:
				score = max(score, 2)
			case FieldTitle:
				score = max(score, 1)
			}
		}
		if score > best {
			if best == 1 && t.Header >= 0 && i > t.Header+1 {
				// The title of the details above the songs names them.
				if j := slices.Index(t.Fields, FieldTitle); j >= 0 && j < len(rows[t.Header+1]) {
					t.Name = cleanCell(rows[t.Header+1][j])
				}
			}
			best, t.Header, t.Fields, t.millis = score, i, fields, millis
		}
	}
	if t.Header >= 0 {
		t.own = equalFold(rows[t.Header], ownHeader)
	}
	return t, nil
}

func equalFold(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !strings.EqualFold(strings.TrimSpace(a[i]), b[i]) {
			return false
		}
	}
	return true
}

// delimiter is the character that parts the cells of text's first line:
// a comma, a semicolon or a tab, whichever it has most of outside quotes.
func delimiter(text string) rune {
	line, _, _ := strings.Cut(strings.TrimLeft(text, "\r\n"), "\n")
	counts := map[rune]int{}
	quoted := false
	for _, r := range line {
		switch {
		case r == '"':
			quoted = !quoted
		case !quoted && (r == ',' || r == ';' || r == '\t'):
			counts[r]++
		}
	}
	best := ','
	for _, r := range []rune{'\t', ';'} {
		if counts[r] > counts[best] {
			best = r
		}
	}
	return best
}

// fieldsOf is what each column holds, by its name in row, and whether the
// durations count milliseconds. Each field goes to the first column of
// its names, but for the artists, which several columns may name.
func fieldsOf(row []string) ([]Field, bool) {
	fields := make([]Field, len(row))
	millis := false
	seen := map[Field]bool{}
	for i, cell := range row {
		n := columnName(cell)
		f := fieldNames[n]
		if f == FieldNone || (seen[f] && f != FieldArtists) {
			continue
		}
		seen[f] = true
		fields[i] = f
		if f == FieldDuration && strings.HasSuffix(n, "ms") {
			millis = true
		}
	}
	return fields, millis
}

// columnName is a column's name as fieldNames has them.
func columnName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return strings.TrimRightFunc(b.String(), unicode.IsDigit)
}

// Lists are the songs of the table, whose columns hold fields, in the
// rows after header, or all of them when it's -1: a list of the table's
// name, or one of each playlist a column names, in the order they come.
func (t *Table) Lists(fields []Field, header int) []List {
	var lists []List
	index := map[string]int{}
	millis := t.millis && header == t.Header
	for i, row := range t.Rows {
		if i <= header {
			continue
		}
		e, playlist := t.entry(row, fields, millis)
		if e.Title == "" && e.VideoID == "" && e.SpotifyID == "" && e.MixMusicID == "" && e.Path == "" {
			continue
		}
		if playlist == "" {
			playlist = t.Name
		}
		j, ok := index[playlist]
		if !ok {
			j = len(lists)
			index[playlist] = j
			lists = append(lists, List{Name: playlist, Own: t.own})
		}
		lists[j].Entries = append(lists[j].Entries, e)
	}
	return lists
}

// entry is the song of a row, and the playlist it names, if any.
func (t *Table) entry(row []string, fields []Field, millis bool) (e Entry, playlist string) {
	for i, f := range fields {
		if i >= len(row) {
			break
		}
		cell := strings.TrimSpace(row[i])
		if t.own {
			cell = unescapeCell(cell)
		}
		if cell == "" {
			continue
		}
		switch f {
		case FieldTitle:
			e.Title = cell
		case FieldArtists:
			e.Artists = append(e.Artists, splitArtists(cell, t.own)...)
		case FieldAlbum:
			e.Album = cell
		case FieldDuration:
			e.Duration = parseDuration(cell, millis)
		case FieldISRC:
			e.ISRC = isrc(cell)
		case FieldVideoID, FieldSpotifyURI:
			v, s := link(cell, true)
			e.VideoID, e.SpotifyID = or(e.VideoID, v), or(e.SpotifyID, s)
		case FieldURL:
			e.URL = cell
			if strings.HasPrefix(strings.ToLower(cell), "file:") {
				e.Path = cell
				continue
			}
			v, s := link(cell, false)
			e.VideoID, e.SpotifyID = or(e.VideoID, v), or(e.SpotifyID, s)
		case FieldPlaylist:
			playlist = cell
		case FieldService:
			e.Service = cell
		case FieldMixMusicID:
			if ownID(cell) {
				e.MixMusicID = cell
			}
		}
	}
	return e, playlist
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// ownID reports whether id is a track's ID in MixMusic.
func ownID(id string) bool {
	for _, p := range []string{"yt:", "sp:", "file:"} {
		if strings.HasPrefix(id, p) && len(id) > len(p) {
			return true
		}
	}
	return false
}

// splitArtists parts a cell of artists: by semicolons, and of MixMusic's
// own files, which join them with commas, by commas.
func splitArtists(cell string, own bool) []string {
	sep := ";"
	if own {
		sep = ", "
	}
	var out []string
	for _, a := range strings.Split(cell, sep) {
		if a = strings.TrimSpace(a); a != "" {
			out = append(out, a)
		}
	}
	return out
}

var isrcPattern = regexp.MustCompile(`^[A-Z]{2}[A-Z0-9]{3}\d{7}$`)

// isrc is a cell's ISRC, "" for what isn't one.
func isrc(cell string) string {
	s := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(cell), "-", ""))
	if isrcPattern.MatchString(s) {
		return s
	}
	return ""
}

var isoDuration = regexp.MustCompile(`^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+(?:\.\d+)?)S)?$`)

// parseDuration reads a length: "3:45", "1:02:03", "PT3M45S", or a
// number of seconds, or of milliseconds when millis or when it's more than
// three hours of seconds.
func parseDuration(s string, millis bool) time.Duration {
	s = strings.TrimSpace(s)
	if m := isoDuration.FindStringSubmatch(strings.ToUpper(s)); m != nil && s != "" {
		h, _ := strconv.Atoi(m[1])
		mi, _ := strconv.Atoi(m[2])
		sec, _ := strconv.ParseFloat(m[3], 64)
		return time.Duration(h)*time.Hour + time.Duration(mi)*time.Minute + time.Duration(sec*float64(time.Second))
	}
	if strings.Contains(s, ":") {
		var d time.Duration
		for _, part := range strings.Split(s, ":") {
			n, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
			if err != nil || n < 0 {
				return 0
			}
			d = d*60 + time.Duration(n*float64(time.Second))
		}
		return d
	}
	n, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	if err != nil || n <= 0 || math.IsInf(n, 0) || math.IsNaN(n) {
		return 0
	}
	if millis || n > 3*60*60 {
		return time.Duration(n * float64(time.Millisecond))
	}
	return time.Duration(n * float64(time.Second))
}

// cleanCell is a cell as a name: without the spaces around it.
func cleanCell(s string) string { return strings.TrimSpace(s) }

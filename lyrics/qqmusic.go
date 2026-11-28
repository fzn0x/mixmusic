package lyrics

import (
	"context"
	"html"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
)

// QQMusicURL is the base URL of QQ Music's public song search and lyrics API.
const QQMusicURL = "https://c.y.qq.com"

const (
	qqMusicTimeout = 3 * time.Second
	qqMusicPreload = 3
)

var qqMusicHeaders = map[string]string{
	"Referer": "https://y.qq.com/",
}

// QQMusic finds synced lyrics (and embedded translations when present) on
// QQ Music.
type QQMusic struct {
	HTTP      *http.Client
	BaseURL   string
	UserAgent string

	wait holdoff
}

// NewQQMusic returns a client for QQ Music lyrics.
func NewQQMusic(userAgent string) *QQMusic {
	return &QQMusic{
		HTTP:      &http.Client{Timeout: qqMusicTimeout},
		BaseURL:   QQMusicURL,
		UserAgent: userAgent,
	}
}

type qqSearchResp struct {
	Data struct {
		Song struct {
			List []qqSong `json:"list"`
		} `json:"song"`
	} `json:"data"`
}

type qqSong struct {
	SongMID   string `json:"songmid"`
	SongName  string `json:"songname"`
	AlbumName string `json:"albumname"`
	Interval  int    `json:"interval"`
	Singer    []struct {
		Name string `json:"name"`
	} `json:"singer"`
}

type qqLyricResp struct {
	RetCode int    `json:"retcode"`
	Lyric   string `json:"lyric"`
	Trans   string `json:"trans"`
}

func (s qqSong) artistString() string {
	names := make([]string, 0, len(s.Singer))
	for _, sg := range s.Singer {
		if n := strings.TrimSpace(sg.Name); n != "" {
			names = append(names, n)
		}
	}
	return strings.Join(names, ", ")
}

// Asks reports whether t has enough metadata to search QQ Music.
func (q *QQMusic) Asks(t Query) bool {
	return q != nil && q.BaseURL != "" && strings.TrimSpace(t.Title) != ""
}

type qqScored struct {
	song   qqSong
	artist string
	dur    time.Duration
	score  float64
}

func (q *QQMusic) searchSongs(ctx context.Context, t Query) ([]qqScored, error) {
	if !q.Asks(t) {
		return nil, ErrNotFound
	}
	kw := strings.TrimSpace(t.Title)
	if a := strings.TrimSpace(t.Artist); a != "" {
		kw = kw + " " + a
	}
	vals := url.Values{
		"w":      {kw},
		"format": {"json"},
		"p":      {"1"},
		"n":      {"8"},
	}
	u := strings.TrimRight(q.BaseURL, "/") + "/soso/fcgi-bin/client_search_cp?" + vals.Encode()
	var resp qqSearchResp
	if err := fetchJSON(ctx, q.HTTP, &q.wait, "QQ Music", q.UserAgent, u, qqMusicHeaders, &resp); err != nil {
		return nil, err
	}
	if len(resp.Data.Song.List) == 0 {
		return nil, ErrNotFound
	}
	scored := make([]qqScored, 0, len(resp.Data.Song.List))
	seen := map[string]bool{}
	for _, s := range resp.Data.Song.List {
		mid := strings.TrimSpace(s.SongMID)
		if mid == "" || seen[mid] {
			continue
		}
		seen[mid] = true
		artist := s.artistString()
		dur := time.Duration(s.Interval) * time.Second
		ta := titleAffinity(t.Title, s.SongName)
		aa := artistAffinity(t.Artist, artist)
		if ta == 0 && (normMatch(t.Artist) == "" || aa == 0) {
			continue
		}
		sc := matchScore(t.Title, t.Artist, t.Duration, s.SongName, artist, dur)
		scored = append(scored, qqScored{song: s, artist: artist, dur: dur, score: sc})
	}
	if len(scored) == 0 {
		return nil, ErrNotFound
	}
	slices.SortStableFunc(scored, func(a, b qqScored) int {
		if a.score != b.score {
			if a.score > b.score {
				return -1
			}
			return 1
		}
		return 0
	})
	return scored, nil
}

// Get returns the best strictly matching lyrics from QQ Music for t within
// a single 3-second budget across search and lyric fetch.
func (q *QQMusic) Get(ctx context.Context, t Query) (Lyrics, error) {
	ctx, cancel := context.WithTimeout(ctx, qqMusicTimeout)
	defer cancel()
	items, err := q.searchSongs(ctx, t)
	if err != nil {
		return Lyrics{}, err
	}
	tried := 0
	for _, it := range items {
		if tried >= 3 {
			break
		}
		if titleAffinity(t.Title, it.song.SongName) == 0 {
			continue
		}
		if normMatch(t.Artist) != "" && artistAffinity(t.Artist, it.artist) == 0 {
			continue
		}
		if t.Duration > 0 && it.dur > 0 && (it.dur-t.Duration).Abs() > 12*time.Second {
			exact := normMatch(it.song.SongName) == normMatch(t.Title) &&
				(normMatch(t.Artist) == "" || normMatch(it.artist) == normMatch(t.Artist))
			if !exact {
				continue
			}
		}
		tried++
		l, err := q.GetByID(ctx, it.song.SongMID)
		if err == nil {
			return l, nil
		}
		if err != ErrNotFound {
			return Lyrics{}, err
		}
	}
	return Lyrics{}, ErrNotFound
}

// Search returns matching QQ Music lyric candidates, preloading the top
// matches concurrently and leaving the rest for lazy previewing via GetByID.
func (q *QQMusic) Search(ctx context.Context, t Query) ([]Candidate, error) {
	items, err := q.searchSongs(ctx, t)
	if err != nil {
		return nil, err
	}
	preload := min(qqMusicPreload, len(items))
	preLyrics := make([]Lyrics, preload)
	preErrs := make([]error, preload)
	var wg sync.WaitGroup
	for i := range preload {
		wg.Go(func() {
			preLyrics[i], preErrs[i] = q.GetByID(ctx, items[i].song.SongMID)
		})
	}
	wg.Wait()

	out := make([]Candidate, 0, len(items))
	for i, it := range items {
		cand := Candidate{
			ID:       it.song.SongMID,
			Title:    it.song.SongName,
			Artist:   it.artist,
			Album:    it.song.AlbumName,
			Duration: it.dur,
		}
		if i < preload {
			if preErrs[i] != nil {
				continue
			}
			cand.Lyrics = preLyrics[i]
			cand.Loaded = true
			cand.SyncHint = syncHintOf(preLyrics[i])
		}
		out = append(out, cand)
	}
	if len(out) == 0 {
		return nil, ErrNotFound
	}
	return out, nil
}

// GetByID fetches lyrics for a specific QQ Music songmid.
func (q *QQMusic) GetByID(ctx context.Context, songMID string) (Lyrics, error) {
	if q == nil || q.BaseURL == "" || strings.TrimSpace(songMID) == "" {
		return Lyrics{}, ErrNotFound
	}
	vals := url.Values{
		"songmid":  {songMID},
		"format":   {"json"},
		"nobase64": {"1"},
	}
	u := strings.TrimRight(q.BaseURL, "/") + "/lyric/fcgi-bin/fcg_query_lyric_new.fcg?" + vals.Encode()
	var resp qqLyricResp
	if err := fetchJSON(ctx, q.HTTP, &q.wait, "QQ Music", q.UserAgent, u, qqMusicHeaders, &resp); err != nil {
		return Lyrics{}, err
	}
	if resp.RetCode != 0 || strings.TrimSpace(resp.Lyric) == "" {
		return Lyrics{}, ErrNotFound
	}
	rawLyric := html.UnescapeString(resp.Lyric)
	if isQQInstrumentalNotice(rawLyric) {
		return Lyrics{Instrumental: true, Source: "QQ Music"}, nil
	}
	l, ok := FromText(rawLyric, "QQ Music")
	if !ok || len(l.Lines) == 0 {
		return Lyrics{}, ErrNotFound
	}
	if len(l.Lines) <= 2 && isQQInstrumentalNotice(l.Lines[0].Text) {
		return Lyrics{Instrumental: true, Source: "QQ Music"}, nil
	}
	l.Lines = stripQQMusicHeaderLines(l.Lines)
	if len(l.Lines) == 0 {
		return Lyrics{}, ErrNotFound
	}
	if rawTrans := strings.TrimSpace(html.UnescapeString(resp.Trans)); rawTrans != "" {
		if tl, ok := FromText(rawTrans, "QQ Music"); ok && len(tl.Lines) > 0 {
			attachQQMusicTranslation(&l, tl)
		}
	}
	return l, nil
}

func isQQInstrumentalNotice(text string) bool {
	return strings.Contains(text, "此歌曲为没有填词的纯音乐") ||
		strings.Contains(text, "纯音乐，请欣赏") ||
		strings.Contains(text, "没有填词的纯音乐")
}

var qqCreditRoles = []string{
	"词", "曲", "编曲", "制作", "混音", "母带", "合声", "和声",
	"吉他", "贝斯", "鼓", "监制", "录音", "发行", "出品", "策划",
	"op", "sp", "mix", "master", "drum", "percussion", "keyboard",
	"synth", "guitar", "bass", "vocal", "record", "program",
	"engineer", "lyric", "compos", "produc", "written", "arrang",
}

func isQQCreditLine(text string) bool {
	low := strings.ToLower(strings.TrimSpace(text))
	if low == "" {
		return true
	}
	head, _, hasColon := strings.Cut(low, "：")
	if !hasColon {
		head, _, hasColon = strings.Cut(low, ":")
	}
	if !hasColon {
		for _, p := range []string{"lyrics by ", "composed by ", "produced by ", "written by ", "arranged by "} {
			if strings.HasPrefix(low, p) {
				return true
			}
		}
		return false
	}
	head = strings.TrimSpace(head)
	if head == "" || len([]rune(head)) > 24 {
		return false
	}
	for _, r := range qqCreditRoles {
		if strings.Contains(head, r) {
			return true
		}
	}
	return false
}

// stripQQMusicHeaderLines trims leading title/artist banners (in the opening
// two seconds) and composer/production credit lines that QQ Music prepends
// before the first sung line.
func stripQQMusicHeaderLines(lines []Line) []Line {
	start := 0
	for start < len(lines) {
		line := lines[start]
		txt := strings.TrimSpace(line.Text)
		if start == 0 && line.TimeMs <= 2000 && strings.Contains(txt, " - ") {
			start++
			continue
		}
		if isQQCreditLine(txt) {
			start++
			continue
		}
		break
	}
	if start >= len(lines) {
		return lines
	}
	return lines[start:]
}

func attachQQMusicTranslation(l *Lyrics, tl Lyrics) {
	if !l.Synced || !tl.Synced {
		return
	}
	transLines := make([]TranslatedLine, len(l.Lines))
	var sample []Line
	matched := 0
	for i, line := range l.Lines {
		bestIdx := -1
		bestDiff := 151 // ms tolerance
		for j, tr := range tl.Lines {
			txt := strings.TrimSpace(tr.Text)
			if txt == "" || txt == "//" {
				continue
			}
			diff := line.TimeMs - tr.TimeMs
			if diff < 0 {
				diff = -diff
			}
			if diff < bestDiff {
				bestDiff = diff
				bestIdx = j
			}
		}
		if bestIdx >= 0 {
			txt := strings.TrimSpace(tl.Lines[bestIdx].Text)
			transLines[i].Text = txt
			sample = append(sample, Line{Text: txt})
			matched++
		}
	}
	if matched >= 2 {
		l.Translation = &Translation{
			From:  GuessLanguage(l.Lines),
			To:    GuessLanguage(sample),
			Lines: transLines,
		}
	}
}

package romaji

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

// served serves data as the dictionary, counting the requests, which
// cut, when set, cuts off halfway.
type served struct {
	data     []byte
	cut      bool
	status   int
	requests atomic.Int32
}

func (s *served) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.requests.Add(1)
	if s.status != 0 {
		w.WriteHeader(s.status)
		return
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(s.data)))
	if s.cut {
		// Fewer bytes than told: the server closes the connection.
		w.Write(s.data[:len(s.data)/2])
		return
	}
	w.Write(s.data)
}

// testDictionary returns a dictionary kept in a folder of its own, which
// downloads from srv, of the hash and size of data.
func testDictionary(t *testing.T, srv *httptest.Server, data []byte) *Dictionary {
	t.Helper()
	d := NewDictionary(t.TempDir(), "MixMusic/test")
	d.URL = srv.URL + "/ipa.dict"
	sum := sha256.Sum256(data)
	d.SHA256, d.Size = hex.EncodeToString(sum[:]), int64(len(data))
	return d
}

// waitDownload waits for the download a Romanize started.
func waitDownload(t *testing.T, d *Dictionary) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := d.Wait(ctx); err != nil {
		t.Fatal(err)
	}
}

// files lists the files of the dictionary's folder.
func files(t *testing.T, d *Dictionary) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(d.path))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

var weather = []string{"今日はいい天気", "", "Oh yeah"}

// TestDictionaryDownloads downloads the dictionary the first time lines
// are romanized, and keeps it, as it came: meanwhile, the lines' kana are
// in romaji, and their kanji as they are.
func TestDictionaryDownloads(t *testing.T) {
	data := bytes.Repeat([]byte("辞書"), 1000)
	srv := &served{data: data}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	d := testDictionary(t, ts, data)

	got := d.Romanize(weather)
	if got.Readings != Downloading || !slices.Equal(got.Romaji, []string{"今日 haii 天気", "", ""}) {
		t.Fatalf("while downloading: %+v", got)
	}
	waitDownload(t, d)
	kept, err := os.ReadFile(d.path)
	if err != nil || !bytes.Equal(kept, data) {
		t.Fatalf("kept %d bytes (%v)", len(kept), err)
	}
	if names := files(t, d); !slices.Equal(names, []string{dictionaryFile}) {
		t.Errorf("files %q", names)
	}
	// Lines with nothing Japanese need no dictionary.
	if got := d.Romanize([]string{"Oh yeah"}); got.Readings != Read || got.Romaji[0] != "" || srv.requests.Load() != 1 {
		t.Errorf("lines in English: %+v, %d requests", got, srv.requests.Load())
	}
}

// TestDictionaryNotUsed never keeps a download that isn't the dictionary,
// of another hash, cut off, or refused, and tries again later, or when
// asked to.
func TestDictionaryNotUsed(t *testing.T) {
	data := bytes.Repeat([]byte("辞書"), 1000)
	for name, srv := range map[string]*served{
		"another hash": {data: bytes.Repeat([]byte("じしょ"), 1000)[:len(data)]},
		"another size": {data: data[:len(data)-1]},
		"cut off":      {data: data, cut: true},
		"refused":      {status: http.StatusNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			ts := httptest.NewServer(srv)
			defer ts.Close()
			d := testDictionary(t, ts, data)
			if got := d.Romanize(weather); got.Readings != Downloading {
				t.Fatalf("readings %v", got.Readings)
			}
			waitDownload(t, d)
			if names := files(t, d); len(names) != 0 {
				t.Fatalf("kept %q", names)
			}
			// Held off: not downloaded again at once.
			if got := d.Romanize(weather); got.Readings != Unavailable || got.Romaji[0] != "今日 haii 天気" {
				t.Fatalf("after the failure: %+v", got)
			}
			if n := srv.requests.Load(); n != 1 {
				t.Errorf("%d requests", n)
			}
			// Until the user asks.
			d.Retry()
			if got := d.Romanize(weather); got.Readings != Downloading {
				t.Fatalf("again: %+v", got)
			}
			waitDownload(t, d)
			if n := srv.requests.Load(); n != 2 {
				t.Errorf("%d requests after trying again", n)
			}
		})
	}
}

// TestDictionaryHoldsOff holds off downloading again twice as long after
// each failure in a row, up to longestHold.
func TestDictionaryHoldsOff(t *testing.T) {
	srv := &served{status: http.StatusServiceUnavailable}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	d := testDictionary(t, ts, []byte("x"))
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	d.now = func() time.Time { return now }
	var holds []time.Duration
	for range 7 {
		d.Romanize(weather)
		waitDownload(t, d)
		holds = append(holds, d.until.Sub(now))
		now = d.until
	}
	want := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 10 * time.Minute, 10 * time.Minute}
	if !slices.Equal(holds, want) {
		t.Errorf("holds %v", holds)
	}
}

// TestDictionaryRomanizes reads kanji once the dictionary came, from the
// file it was kept in afterwards, and downloads it again when the file
// isn't the dictionary any more.
func TestDictionaryRomanizes(t *testing.T) {
	data, err := os.ReadFile(moduleDictionary(t))
	if err != nil {
		t.Fatal(err)
	}
	srv := &served{data: data}
	ts := httptest.NewServer(srv)
	d := testDictionary(t, ts, data)
	if d.SHA256 != DictionarySHA256 || d.Size != DictionarySize {
		t.Fatalf("the module's dictionary is %s, %d bytes: not the one MixMusic downloads", d.SHA256, d.Size)
	}
	d.Romanize(weather)
	waitDownload(t, d)
	got := d.Romanize(weather)
	if got.Readings != Read || !slices.Equal(got.Romaji, []string{"kyō wa ii tenki", "", ""}) || !d.Loaded() {
		t.Fatalf("with the dictionary: %+v", got)
	}

	// Another run reads it from its file, without downloading it.
	ts.Close()
	again := NewDictionary(filepath.Dir(d.path), "MixMusic/test")
	again.URL = ts.URL
	if got := again.Romanize(weather); got.Readings != Read || got.Romaji[0] != "kyō wa ii tenki" {
		t.Errorf("from its file: %+v", got)
	}
	if n := srv.requests.Load(); n != 1 {
		t.Errorf("%d downloads", n)
	}

	// A file changed since goes, and the dictionary downloads again.
	data[len(data)/2] ^= 1
	if err := os.WriteFile(d.path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	third := NewDictionary(filepath.Dir(d.path), "MixMusic/test")
	third.URL = ts.URL
	if got := third.Romanize(weather); got.Readings != Downloading || got.Romaji[0] != "今日 haii 天気" {
		t.Errorf("a changed file: %+v", got)
	}
	waitDownload(t, third)
	if _, err := os.Stat(d.path); !os.IsNotExist(err) {
		t.Errorf("the changed file stays: %v", err)
	}
}

// TestDictionaryLetsGo lets the dictionary go from memory once unused for
// a while.
func TestDictionaryLetsGo(t *testing.T) {
	data, err := os.ReadFile(moduleDictionary(t))
	if err != nil {
		t.Fatal(err)
	}
	d := NewDictionary(t.TempDir(), "MixMusic/test")
	if err := os.WriteFile(d.path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	d.Idle = 50 * time.Millisecond
	if got := d.Romanize(weather); got.Readings != Read || !d.Loaded() {
		t.Fatalf("%+v", got)
	}
	for start := time.Now(); d.Loaded(); time.Sleep(10 * time.Millisecond) {
		if time.Since(start) > 5*time.Second {
			t.Fatal("still in memory")
		}
	}
	// It loads again when lines need it.
	if got := d.Romanize(weather); got.Readings != Read || got.Romaji[0] != "kyō wa ii tenki" {
		t.Errorf("loaded again: %+v", got)
	}
}

// song is a song's worth of made-up lyrics in Japanese.
var song = []string{
	"夜明け前の街を歩いて", "君の声を探していた", "冷たい風が頬をなでる", "それでも僕は止まらない",
	"", "ねえ、聞こえる？", "遠くで鳴るベルの音", "忘れかけた約束を", "もう一度だけ思い出して",
	"空に描いた地図の上", "二人で歩いた道がある", "涙の数だけ強くなれると", "誰かが言っていたっけ",
	"", "走れ 走れ 明日へ", "光の中へ飛び込もう", "迷っても 転んでも", "この手を離さないで",
	"Oh oh oh", "季節が巡るたびに", "変わらないものを数えて", "小さな花が咲いていた", "あの坂道の向こう側",
	"", "走れ 走れ 明日へ", "光の中へ飛び込もう", "迷っても 転んでも", "この手を離さないで",
	"いつか見た夢の続きを", "今ならきっと描けるはず", "ありがとう", "さよならは言わないよ",
	"", "走れ 走れ 明日へ", "光の中へ飛び込もう", "迷っても 転んでも", "この手を離さないで", "Oh oh oh",
}

// TestDictionaryCost tells how much memory the dictionary takes, and how
// long it takes to load and to read a song: go test -run Cost -v.
func TestDictionaryCost(t *testing.T) {
	data, err := os.ReadFile(moduleDictionary(t))
	if err != nil {
		t.Fatal(err)
	}
	d := NewDictionary(t.TempDir(), "MixMusic/test")
	if err := os.WriteFile(d.path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	start := time.Now()
	d.Romanize([]string{"今日"})
	loaded := time.Since(start)
	runtime.GC()
	runtime.ReadMemStats(&after)
	// A song reads faster than some clocks tick: the time of 20.
	var got Lines
	start = time.Now()
	for range 20 {
		got = d.Romanize(song)
	}
	read := time.Since(start) / 20
	if got.Readings != Read {
		t.Fatalf("readings %v", got.Readings)
	}
	mb := func(a, b uint64) float64 { return float64(int64(a)-int64(b)) / (1 << 20) }
	t.Logf("the dictionary takes %.1f MB of heap (%.1f MB allocated to load it), and loads in %v; a song of %d lines reads in %v",
		mb(after.HeapAlloc, before.HeapAlloc), mb(after.TotalAlloc, before.TotalAlloc), loaded, len(song), read)
	for i, l := range song[:8] {
		t.Logf("%s → %s", l, got.Romaji[i])
	}
	// Let go, the memory goes back to the system.
	d.work.Lock()
	d.Idle = time.Millisecond
	d.release.Reset(time.Millisecond)
	d.work.Unlock()
	for start := time.Now(); d.Loaded(); time.Sleep(10 * time.Millisecond) {
		if time.Since(start) > 5*time.Second {
			t.Fatal("still in memory")
		}
	}
	var gone runtime.MemStats
	runtime.ReadMemStats(&gone)
	t.Logf("let go: %.1f MB of heap more than before it loaded; the process holds %.1f MB of heap from the system, %.1f MB while it was loaded",
		mb(gone.HeapAlloc, before.HeapAlloc), mb(gone.HeapSys-gone.HeapReleased, 0), mb(after.HeapSys-after.HeapReleased, 0))
}

// BenchmarkRomanize reads a song with the dictionary loaded.
func BenchmarkRomanize(b *testing.B) {
	r := testRomanizer(b)
	for b.Loop() {
		for _, l := range song {
			r.Line(l)
		}
	}
}

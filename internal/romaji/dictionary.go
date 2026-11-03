package romaji

import (
	"archive/zip"
	"bytes"
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ikawaha/kagome-dict/dict"
)

const (
	// DictionaryURL is where the IPA dictionary of kagome downloads from:
	// the file of version 1.2.6 of its Go module, on GitHub. Its notice is
	// DictionaryNotice.
	DictionaryURL = "https://raw.githubusercontent.com/ikawaha/kagome-dict/ipa/v1.2.6/ipa/ipa.dict"
	// DictionarySHA256 and DictionarySize are those of that file: a file
	// of another hash or size is never used.
	DictionarySHA256 = "19bacdfe1a33ba50054d77192f062f1696089a48a9d8e20863b716ff4af6adf3"
	DictionarySize   = 11691526
	// dictionaryFile names the dictionary in its folder.
	dictionaryFile = "ipa-1.2.6.dict"
	// downloadTimeout bounds the download of the dictionary.
	downloadTimeout = 10 * time.Minute
	// firstHold is how long the dictionary isn't downloaded again after a
	// download failed, and longestHold the longest: each failure in a row
	// holds it off twice as long as the one before.
	firstHold, longestHold = 30 * time.Second, 10 * time.Minute
	// idleRelease is how long the dictionary stays in memory, which it
	// takes some 90 MB of, after romaji last read it.
	idleRelease = 5 * time.Minute
)

// Readings tells how the kanji of lines in romaji were read.
type Readings int

const (
	// Read is lines whose kanji the dictionary read.
	Read Readings = iota
	// Downloading is lines whose kanji stay as they are while the
	// dictionary downloads.
	Downloading
	// Unavailable is lines whose kanji stay as they are, as the dictionary
	// couldn't be downloaded: it is downloaded again after a while.
	Unavailable
)

// Lines are lines of lyrics in romaji.
type Lines struct {
	// Romaji are the lines in romaji, one for each line, in order: "" for
	// those with nothing Japanese in them.
	Romaji []string
	// Readings tells whether their kanji were read, or stay as they are.
	Readings Readings
}

// Dictionary is the IPA dictionary of kagome, which reads kanji: it is
// downloaded the first time lines are romanized, and kept in a folder,
// where it is checked each time it loads. It loads when lines need it,
// and goes from memory a while after they last did.
type Dictionary struct {
	// URL is where the dictionary downloads from, and SHA256 and Size what
	// the file must be: DictionaryURL, DictionarySHA256 and DictionarySize,
	// but in tests.
	URL    string
	SHA256 string
	Size   int64
	// HTTP downloads the dictionary as UserAgent.
	HTTP      *http.Client
	UserAgent string
	// Idle is how long the dictionary stays in memory unused.
	Idle time.Duration

	path string
	now  func() time.Time

	// work is held while the dictionary loads, reads lines, or goes. r is
	// the dictionary loaded, nil while it isn't, and used when it last
	// read lines; release lets it go, once idle.
	work    sync.Mutex
	r       *Romanizer
	used    time.Time
	release *time.Timer
	// ready is set once the dictionary loaded, or downloaded.
	ready atomic.Bool

	mu sync.Mutex
	// done is closed when the download under way ends, nil while none is.
	done chan struct{}
	// until is when the dictionary may be downloaded again, after hold,
	// once a download failed.
	until time.Time
	hold  time.Duration
}

// NewDictionary returns the dictionary kept in the folder dir, which it
// downloads as userAgent.
func NewDictionary(dir, userAgent string) *Dictionary {
	return &Dictionary{
		URL: DictionaryURL, SHA256: DictionarySHA256, Size: DictionarySize,
		HTTP: &http.Client{}, UserAgent: userAgent, Idle: idleRelease,
		path: filepath.Join(dir, dictionaryFile), now: time.Now,
	}
}

// Romanize writes lines in romaji, as Romanizer.Line does once the
// dictionary is there. Until it is, it starts its download, and writes the
// kana of the lines in romaji and leaves their kanji as they are, as
// KanaLine does. Nothing but the dictionary is downloaded: the lines stay
// on the computer.
func (d *Dictionary) Romanize(lines []string) Lines {
	out := Lines{Romaji: make([]string, len(lines))}
	if !slices.ContainsFunc(lines, Japanese) {
		return out
	}
	d.work.Lock()
	defer d.work.Unlock()
	r, readings := d.romanizer()
	out.Readings = readings
	for i, l := range lines {
		if r != nil {
			out.Romaji[i] = r.Line(l)
		} else {
			out.Romaji[i] = KanaLine(l)
		}
		if out.Romaji[i] == strings.TrimSpace(l) {
			out.Romaji[i] = ""
		}
	}
	if r != nil {
		d.used = d.now()
		if d.release == nil {
			d.release = time.AfterFunc(d.Idle, d.letGo)
		} else {
			d.release.Reset(d.Idle)
		}
	}
	return out
}

// Loaded tells whether the dictionary is in memory.
func (d *Dictionary) Loaded() bool {
	d.work.Lock()
	defer d.work.Unlock()
	return d.r != nil
}

// Ready tells whether the dictionary is there to read kanji with, since it
// loaded or downloaded: lines romanized without it can be again.
func (d *Dictionary) Ready() bool { return d.ready.Load() }

// romanizer returns the dictionary, loading it from its file, or, when
// that isn't there, nil, and starts its download. It runs with work held.
func (d *Dictionary) romanizer() (*Romanizer, Readings) {
	if d.r != nil {
		return d.r, Read
	}
	r, err := d.load()
	if err == nil {
		d.r = r
		d.ready.Store(true)
		return r, Read
	}
	if !errors.Is(err, fs.ErrNotExist) {
		// What isn't the dictionary goes, and the dictionary comes again.
		log.Printf("mixmusic: the dictionary of the readings of kanji didn't load (%v): downloading it again", err)
		os.Remove(d.path)
	}
	return nil, d.download()
}

// load loads the dictionary from its file, once its size and SHA-256 are
// those it must have.
func (d *Dictionary) load() (*Romanizer, error) {
	data, err := os.ReadFile(d.path)
	if err != nil {
		return nil, err
	}
	if err := d.check(int64(len(data)), sha256.Sum256(data)); err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	// dict.LoadDictFile loads a file so: the bytes checked are those read.
	loaded, err := dict.Load(zr, true)
	if err != nil {
		return nil, err
	}
	return NewRomanizer(loaded)
}

// check tells why a file of size bytes whose SHA-256 is sum isn't the
// dictionary, nil when it is.
func (d *Dictionary) check(size int64, sum [sha256.Size]byte) error {
	switch {
	case size != d.Size:
		return fmt.Errorf("%d bytes, not %d", size, d.Size)
	case hex.EncodeToString(sum[:]) != d.SHA256:
		return errors.New("its SHA-256 isn't the dictionary's")
	}
	return nil
}

// download starts the download of the dictionary, unless one is under way
// or a failure holds it off, and tells which.
func (d *Dictionary) download() Readings {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch {
	case d.done != nil:
		return Downloading
	case d.now().Before(d.until):
		return Unavailable
	}
	done := make(chan struct{})
	d.done = done
	go func() {
		defer close(done)
		err := d.fetch()
		d.mu.Lock()
		defer d.mu.Unlock()
		d.done = nil
		if err != nil {
			log.Printf("mixmusic: the dictionary of the readings of kanji didn't download: %v", err)
			d.hold = min(max(d.hold*2, firstHold), longestHold)
			d.until = d.now().Add(d.hold)
			return
		}
		d.hold, d.until = 0, time.Time{}
		d.ready.Store(true)
	}()
	return Downloading
}

// fetch downloads the dictionary into its file, through a file of its own
// that goes unless the dictionary came whole.
func (d *Dictionary) fetch() error {
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", d.UserAgent)
	res, err := d.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode != http.StatusOK:
		return fmt.Errorf("HTTP %d", res.StatusCode)
	case res.ContentLength >= 0 && res.ContentLength != d.Size:
		return fmt.Errorf("%d bytes, not %d", res.ContentLength, d.Size)
	}
	dir := filepath.Dir(d.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	d.clearParts(dir)
	tmp, err := os.CreateTemp(dir, "*.part")
	if err != nil {
		return err
	}
	h := sha256.New()
	// A byte more than the dictionary tells one too many.
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(res.Body, d.Size+1))
	err = cmp.Or(err, tmp.Close())
	if err == nil {
		err = d.check(n, [sha256.Size]byte(h.Sum(nil)))
	}
	if err == nil {
		err = os.Rename(tmp.Name(), d.path)
	}
	if err != nil {
		os.Remove(tmp.Name())
	}
	return err
}

// clearParts deletes the parts of downloads left in dir by a MixMusic
// that quit as they came: those older than a download lasts at most.
func (d *Dictionary) clearParts(dir string) {
	parts, _ := filepath.Glob(filepath.Join(dir, "*.part"))
	for _, p := range parts {
		if info, err := os.Stat(p); err == nil && d.now().Sub(info.ModTime()) > downloadTimeout {
			os.Remove(p)
		}
	}
}

// Wait waits for the download under way, when there is one, to end, or
// ctx to.
func (d *Dictionary) Wait(ctx context.Context) error {
	d.mu.Lock()
	done := d.done
	d.mu.Unlock()
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Retry has the next lines download the dictionary, though a download
// failed not long ago: the user asked to try again.
func (d *Dictionary) Retry() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.until = time.Time{}
}

// letGo lets the dictionary go from memory once it was idle long enough,
// and gives the memory back to the system.
func (d *Dictionary) letGo() {
	d.work.Lock()
	if d.r == nil {
		d.work.Unlock()
		return
	}
	if left := d.Idle - d.now().Sub(d.used); left > 0 {
		d.release.Reset(left)
		d.work.Unlock()
		return
	}
	d.r = nil
	d.work.Unlock()
	// kagome's pool of lattices holds on to the dictionary until the
	// second collection after their last use.
	runtime.GC()
	debug.FreeOSMemory()
}

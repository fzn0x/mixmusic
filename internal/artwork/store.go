package artwork

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/diag"
)

const (
	// fetchTimeout bounds a download.
	fetchTimeout = 20 * time.Second
	// downloads is how many images download at once.
	downloads = 6
	// minFresh and maxFresh bound how long a cover of the services is
	// shown from disk before its host is asked whether it changed; their
	// hosts ask for a day or two hours, and their covers rarely change.
	// Other images keep the time their host asks for, up to maxFresh, and
	// freshFor without one.
	minFresh = 24 * time.Hour
	maxFresh = 30 * 24 * time.Hour
	freshFor = 7 * 24 * time.Hour
	// checkAgain is how long a check of a stale image that failed waits
	// before the next.
	checkAgain = 10 * time.Minute
	// maxRetryAfter bounds the wait a host asks for.
	maxRetryAfter = 30 * time.Minute
	// logEvery is how often a host's failures are logged at most.
	logEvery = time.Minute
)

// pingAfter is how long a connection goes silent before the store asks it
// whether it is still there, and how long it then waits for the answer: a
// connection a network change left without closing it is dropped, rather
// than given cover after cover to time out on (#73).
var pingAfter = 15 * time.Second

// Store downloads artwork and keeps it on disk as it came, so that covers
// shown once show again at once, at any size, without the network. It
// keeps up to a number of bytes, the images shown least recently going
// first. Each image downloads once however many ask for it, and an image
// past the time its host gives shows from disk while it is checked. Only
// images of https addresses are kept.
type Store struct {
	// HTTP downloads the images. Set it before the first download.
	HTTP *http.Client

	dir   string
	limit int64
	slots chan struct{}
	// now and logf are time.Now and log.Printf, which tests replace.
	now  func() time.Time
	logf func(format string, args ...any)
	// wg counts the goroutines, for Close.
	wg      sync.WaitGroup
	counted sync.Once
	// counting is held while the folder is counted.
	counting sync.Mutex

	mu sync.Mutex
	// calls are the downloads under way, and checks the checks of stale
	// images, by address; checked is when one last failed.
	calls   map[string]*call
	checks  map[string]bool
	checked map[string]time.Time
	// bytes and images are what the folder holds, once counted.
	bytes  int64
	images int
	ready  bool
	hosts  map[string]*hostLog
	// diskTold is set once a disk error was logged.
	diskTold bool
}

// call is a download, which its callers wait for.
type call struct {
	done chan struct{}
	data []byte
	err  error
}

// hostLog is what was logged of a host's failures.
type hostLog struct {
	failing bool
	// logged is when the last line was; skipped counts the failures since,
	// from since.
	logged  time.Time
	skipped int
	since   time.Time
}

// NewStore returns a store of artwork in dir, of limit bytes.
func NewStore(dir string, limit int64) *Store {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.HTTP2 = &http.HTTP2Config{SendPingTimeout: pingAfter, PingTimeout: pingAfter}
	return &Store{
		HTTP:    &http.Client{Timeout: fetchTimeout, Transport: t},
		dir:     dir,
		limit:   limit,
		slots:   make(chan struct{}, downloads),
		now:     time.Now,
		logf:    log.Printf,
		calls:   map[string]*call{},
		checks:  map[string]bool{},
		checked: map[string]time.Time{},
		hosts:   map[string]*hostLog{},
	}
}

// Error is why an image didn't load.
type Error struct {
	// Host is the address's host; Status the HTTP status it answered, 0
	// when none came.
	Host   string
	Status int
	// Temporary is set when trying again later may load it: the network
	// failed, or the host was busy. RetryAfter is how long the host asked
	// to wait, if it did.
	Temporary  bool
	RetryAfter time.Duration
	Err        error
}

func (e *Error) Error() string { return "artwork: " + e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// Image returns the image at rawURL, decoded. A file on disk that doesn't
// decode goes, and the image downloads again the next time it's asked
// for.
func (s *Store) Image(ctx context.Context, rawURL string) (image.Image, error) {
	data, err := s.Bytes(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		s.remove(rawURL)
		return nil, &Error{Host: hostOf(rawURL), Err: fmt.Errorf("not an image: %w", err)}
	}
	return img, nil
}

// Bytes returns the image at rawURL as its host sent it.
func (s *Store) Bytes(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, &Error{Err: errors.New("not a web address")}
	}
	s.count()
	if u.Scheme == "https" {
		if f, ok := s.read(rawURL); ok {
			if !s.now().Before(f.Fresh) {
				s.check(rawURL, f)
			}
			return f.data, nil
		}
	}
	return s.download(ctx, rawURL)
}

// Usage returns what the store keeps: its bytes, and its images.
func (s *Store) Usage() (bytes int64, images int) {
	s.count()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bytes, s.images
}

// Clear removes every image the store keeps.
func (s *Store) Clear() error {
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range entries {
		if e.Type().IsRegular() {
			if err := os.Remove(filepath.Join(s.dir, e.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
				errs = append(errs, err)
			}
		}
	}
	s.recount()
	return errors.Join(errs...)
}

// Close waits for the store's downloads, checks and counts to end.
func (s *Store) Close() { s.wg.Wait() }

// file is an image kept on disk: one line of JSON, its header, then the
// image as it came.
type file struct {
	header
	data []byte
}

type header struct {
	URL string `json:"url"`
	// Type is the image's content type, and ETag and Modified what its host
	// tells it changed by.
	Type     string    `json:"type,omitempty"`
	ETag     string    `json:"etag,omitempty"`
	Modified string    `json:"modified,omitempty"`
	Fetched  time.Time `json:"fetched"`
	// Fresh is when its host is to be asked again.
	Fresh time.Time `json:"fresh"`
}

// key names the file of the image at rawURL.
func key(rawURL string) string {
	sum := sha256.Sum256([]byte(rawURL))
	return hex.EncodeToString(sum[:])
}

// read returns the image at rawURL from disk. One read an hour ago or more
// is touched, as the least recently shown go first.
func (s *Store) read(rawURL string) (*file, bool) {
	path := filepath.Join(s.dir, key(rawURL))
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	line, data, ok := bytes.Cut(raw, []byte("\n"))
	var f file
	if !ok || json.Unmarshal(line, &f.header) != nil || f.URL != rawURL {
		return nil, false
	}
	f.data = data
	if info, err := os.Stat(path); err == nil && s.now().Sub(info.ModTime()) > time.Hour {
		now := s.now()
		os.Chtimes(path, now, now)
	}
	return &f, true
}

// download downloads the image at rawURL, once for every caller who asks
// meanwhile. The download is no caller's: a caller that stops waiting
// leaves it to the others.
func (s *Store) download(ctx context.Context, rawURL string) ([]byte, error) {
	s.mu.Lock()
	c, ok := s.calls[rawURL]
	if !ok {
		c = &call{done: make(chan struct{})}
		s.calls[rawURL] = c
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.slots <- struct{}{}
			c.data, c.err = s.fetch(rawURL, nil)
			<-s.slots
			s.mu.Lock()
			delete(s.calls, rawURL)
			s.mu.Unlock()
			close(c.done)
		}()
	}
	s.mu.Unlock()
	select {
	case <-c.done:
		return c.data, c.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// check asks the host of a stale image whether it changed, beside the
// caller, which shows the image from disk meanwhile.
func (s *Store) check(rawURL string, f *file) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.checks[rawURL] || s.calls[rawURL] != nil || s.now().Sub(s.checked[rawURL]) < checkAgain {
		return
	}
	s.checks[rawURL] = true
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.slots <- struct{}{}
		_, err := s.fetch(rawURL, f)
		<-s.slots
		s.mu.Lock()
		delete(s.checks, rawURL)
		if err != nil {
			s.checked[rawURL] = s.now()
		} else {
			delete(s.checked, rawURL)
		}
		s.mu.Unlock()
	}()
}

// fetch downloads the image at rawURL, and keeps it. With stale, the
// image kept, it asks whether it changed, and its failures are no cover's:
// stale shows.
func (s *Store) fetch(rawURL string, stale *file) ([]byte, error) {
	data, h, err := s.get(rawURL, stale)
	host := hostOf(rawURL)
	if err != nil {
		if stale == nil {
			s.failed(host, rawURL, err)
		}
		return nil, err
	}
	s.loaded(host)
	if h != nil && strings.HasPrefix(rawURL, "https:") {
		s.write(rawURL, *h, data)
	}
	return data, nil
}

// get asks the host for the image. It returns the header to keep it with,
// nil when it is not to be kept.
func (s *Store) get(rawURL string, stale *file) ([]byte, *header, error) {
	host := hostOf(rawURL)
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, &Error{Host: host, Err: err}
	}
	if stale != nil {
		// Some hosts answer one and not the other: they get both.
		if stale.ETag != "" {
			req.Header.Set("If-None-Match", stale.ETag)
		}
		if stale.Modified != "" {
			req.Header.Set("If-Modified-Since", stale.Modified)
		}
	}
	res, err := s.HTTP.Do(req)
	if err != nil {
		return nil, nil, &Error{Host: host, Temporary: true, Err: err}
	}
	defer res.Body.Close()
	now := s.now()
	if stale != nil && res.StatusCode == http.StatusNotModified {
		h := stale.header
		h.Fetched, h.Fresh = now, freshUntil(host, res.Header, now)
		return stale.data, &h, nil
	}
	if res.StatusCode != http.StatusOK {
		code := res.StatusCode
		e := &Error{Host: host, Status: code, Err: fmt.Errorf("HTTP %d", code)}
		if code == http.StatusRequestTimeout || code == http.StatusTooEarly || code == http.StatusTooManyRequests || code >= 500 {
			e.Temporary, e.RetryAfter = true, retryAfter(res.Header.Get("Retry-After"), now)
		}
		return nil, nil, e
	}
	// The whole body, before it is decoded: one cut short is the
	// network's failure, not the image's.
	data, err := io.ReadAll(io.LimitReader(res.Body, maxSize+1))
	if err != nil {
		return nil, nil, &Error{Host: host, Temporary: true, Err: err}
	}
	if len(data) > maxSize {
		return nil, nil, &Error{Host: host, Err: fmt.Errorf("larger than %d MB", maxSize>>20)}
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return nil, nil, &Error{Host: host, Err: fmt.Errorf("not an image (%s)", res.Header.Get("Content-Type"))}
	}
	if !keepable(res.Header.Get("Cache-Control")) {
		return data, nil, nil
	}
	return data, &header{
		URL:      rawURL,
		Type:     res.Header.Get("Content-Type"),
		ETag:     res.Header.Get("ETag"),
		Modified: res.Header.Get("Last-Modified"),
		Fetched:  now,
		Fresh:    freshUntil(host, res.Header, now),
	}, nil
}

// keepable reports whether a response's Cache-Control lets it be kept.
func keepable(cc string) bool {
	for d := range strings.SplitSeq(strings.ToLower(cc), ",") {
		switch strings.TrimSpace(d) {
		case "no-store", "no-cache", "private":
			return false
		}
	}
	return true
}

// coverHost reports whether host serves the services' covers, whose
// addresses name one image for good.
func coverHost(host string) bool {
	return host == "i.ytimg.com" || host == "i.scdn.co" || host == "googleusercontent.com" ||
		strings.HasSuffix(host, ".googleusercontent.com")
}

// freshUntil is when an image fetched at now from host is to be checked:
// after the max-age of its Cache-Control, or the time between its Date and
// its Expires.
func freshUntil(host string, h http.Header, now time.Time) time.Time {
	d, ok := time.Duration(0), false
	for dir := range strings.SplitSeq(strings.ToLower(h.Get("Cache-Control")), ",") {
		if v, found := strings.CutPrefix(strings.TrimSpace(dir), "max-age="); found {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
				d, ok = time.Duration(min(n, int64(maxFresh/time.Second)))*time.Second, true
			}
		}
	}
	if !ok {
		date, derr := http.ParseTime(h.Get("Date"))
		expires, eerr := http.ParseTime(h.Get("Expires"))
		if derr == nil && eerr == nil {
			d, ok = max(expires.Sub(date), 0), true
		}
	}
	if !ok {
		d = freshFor
	}
	if coverHost(host) {
		d = max(d, minFresh)
	}
	return now.Add(min(d, maxFresh))
}

// retryAfter reads a Retry-After header, in seconds or as a date.
func retryAfter(v string, now time.Time) time.Duration {
	if v == "" {
		return 0
	}
	var d time.Duration
	if n, err := strconv.Atoi(v); err == nil {
		d = time.Duration(n) * time.Second
	} else if t, err := http.ParseTime(v); err == nil {
		d = t.Sub(now)
	}
	return min(max(d, 0), maxRetryAfter)
}

// write keeps an image, then makes room.
func (s *Store) write(rawURL string, h header, data []byte) {
	line, err := json.Marshal(h)
	if err != nil {
		return
	}
	k := key(rawURL)
	path := filepath.Join(s.dir, k)
	err = os.MkdirAll(s.dir, 0o755)
	var tmp *os.File
	if err == nil {
		tmp, err = os.CreateTemp(s.dir, "."+k+"-*.part")
	}
	if err == nil {
		_, err = tmp.Write(append(append(line, '\n'), data...))
		if cerr := tmp.Close(); err == nil {
			err = cerr
		}
		var old int64 = -1
		if info, serr := os.Stat(path); serr == nil {
			old = info.Size()
		}
		// Windows refuses to replace a file another reader holds: it is
		// written again the next time.
		if err == nil {
			err = os.Rename(tmp.Name(), path)
		}
		if err != nil {
			os.Remove(tmp.Name())
		} else {
			s.added(int64(len(line)+1+len(data)), old)
		}
	}
	if err != nil {
		s.mu.Lock()
		told := s.diskTold
		s.diskTold = true
		s.mu.Unlock()
		if !told {
			s.logf("mixmusic: artwork: keeping covers in %s: %v", s.dir, err)
		}
	}
}

// added counts a file of size written over one of old bytes, -1 for none,
// and makes room past the limit.
func (s *Store) added(size, old int64) {
	s.mu.Lock()
	if !s.ready {
		s.mu.Unlock()
		return // the count will see it
	}
	s.bytes += size - max(old, 0)
	if old < 0 {
		s.images++
	}
	over := s.bytes > s.limit
	s.mu.Unlock()
	if over {
		s.recount()
	}
}

// remove removes the file of the image at rawURL.
func (s *Store) remove(rawURL string) {
	if os.Remove(filepath.Join(s.dir, key(rawURL))) == nil {
		s.recount()
	}
}

// count counts what the folder holds, once, beside the caller.
func (s *Store) count() {
	s.counted.Do(func() {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.recount()
		}()
	})
}

// recount counts what the folder holds, removes what writes left
// unfinished an hour ago or more, and, past the limit, the images shown
// least recently, down to nine tenths of it.
func (s *Store) recount() {
	s.counting.Lock()
	defer s.counting.Unlock()
	entries, _ := os.ReadDir(s.dir)
	type kept struct {
		path string
		size int64
		at   time.Time
	}
	var files []kept
	var total int64
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		if strings.HasPrefix(e.Name(), ".") {
			if strings.HasSuffix(e.Name(), ".part") && s.now().Sub(info.ModTime()) > time.Hour {
				os.Remove(path)
			}
			continue
		}
		files = append(files, kept{path, info.Size(), info.ModTime()})
		total += info.Size()
	}
	images := len(files)
	if total > s.limit {
		slices.SortFunc(files, func(a, b kept) int { return a.at.Compare(b.at) })
		for _, f := range files {
			if total <= s.limit*9/10 {
				break
			}
			// One another reader holds stays, on Windows.
			if os.Remove(f.path) == nil {
				total -= f.size
				images--
			}
		}
	}
	s.mu.Lock()
	s.bytes, s.images, s.ready = total, images, true
	s.mu.Unlock()
}

// failed logs that an image of host didn't load: at most a line a minute
// for each host, with the failures since the last.
func (s *Store) failed(host, rawURL string, err error) {
	s.mu.Lock()
	h := s.hosts[host]
	if h == nil {
		h = &hostLog{}
		s.hosts[host] = h
	}
	h.failing = true
	now := s.now()
	if !h.logged.IsZero() && now.Sub(h.logged) < logEvery {
		if h.skipped == 0 {
			h.since = now
		}
		h.skipped++
		s.mu.Unlock()
		return
	}
	more := ""
	if h.skipped > 0 {
		more = fmt.Sprintf("; %d more didn't since %s", h.skipped, h.since.Format("15:04:05"))
	}
	h.logged, h.skipped = now, 0
	s.mu.Unlock()
	s.logf("mixmusic: artwork: a cover from %s didn't load (%s): %s%s", host, key(rawURL)[:8], describe(err), more)
}

// loaded logs that the images of host load again, after failures.
func (s *Store) loaded(host string) {
	s.mu.Lock()
	h := s.hosts[host]
	again := h != nil && h.failing
	if again {
		delete(s.hosts, host)
	}
	s.mu.Unlock()
	if again {
		s.logf("mixmusic: artwork: covers from %s load again", host)
	}
}

// describe tells what went wrong without the address, which an error of
// Go's client quotes whole, or the computer's own address and port, which
// diag.Redact drops from a socket's error.
func describe(err error) string {
	var ue *url.Error
	var e *Error
	switch {
	case errors.As(err, &ue):
		err = ue.Err
	case errors.As(err, &e):
		err = e.Err
	}
	return diag.Redact(err.Error())
}

// hostOf returns the host of rawURL.
func hostOf(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil {
		return u.Hostname()
	}
	return ""
}

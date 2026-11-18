package autoeq

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fzn0x/mixmusic/internal/settings"
)

const (
	// ResultsURL is the folder of AutoEq's profiles on GitHub, with its
	// index, INDEX.md.
	ResultsURL = "https://raw.githubusercontent.com/jaakkopasanen/AutoEq/master/results/"
	// Recheck is how long the index kept is used before it is checked
	// again, with its ETag.
	Recheck = 7 * 24 * time.Hour
	// maxIndex and maxProfile bound the sizes of the index, some 850 kB in
	// November 2026, and of a profile.
	maxIndex   = 16 << 20
	maxProfile = 64 << 10
	// timeout bounds a request.
	timeout = time.Minute
)

// ErrTooBig is a file far bigger than the index or a profile is.
var ErrTooBig = errors.New("the file is too big")

// StatusError is an answer of the server other than the file.
type StatusError struct {
	Code int
	URL  string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s answered %d %s", e.URL, e.Code, http.StatusText(e.Code))
}

// Client fetches AutoEq's index and its profiles, and keeps them in a
// folder: the index, which it checks again once Recheck went by, and each
// profile picked, which it never fetches again. It sends nothing but the
// requests of those files.
type Client struct {
	// Base is where the results are: ResultsURL, but in tests.
	Base string
	// HTTP fetches them as UserAgent.
	HTTP      *http.Client
	UserAgent string

	dir string
	now func() time.Time

	// mu is held while the index is read or fetched: one at a time.
	mu      sync.Mutex
	entries []Entry
	checked time.Time
}

// New returns a client that keeps what it fetches in dir, and fetches it
// as userAgent.
func New(dir, userAgent string) *Client {
	return &Client{Base: ResultsURL, HTTP: &http.Client{}, UserAgent: userAgent, dir: dir, now: time.Now}
}

// List is AutoEq's index as Index found it.
type List struct {
	Entries []Entry
	// Offline is why the index couldn't be checked again, while the one
	// kept from before plays its part, or nil.
	Offline error
}

// indexMeta is what the client keeps of the index besides it: its ETag,
// and when it was last checked.
type indexMeta struct {
	ETag    string    `json:"etag"`
	Checked time.Time `json:"checked"`
}

func (c *Client) indexFile() string { return filepath.Join(c.dir, "INDEX.md") }
func (c *Client) metaFile() string  { return filepath.Join(c.dir, "index.json") }

// Index returns AutoEq's index: the one kept, until Recheck went by since
// it was checked, then the one on GitHub, which an ETag fetches only once
// it changed. Without the network, it returns the one kept, and why it
// wasn't checked, which the next call checks again; with none kept, the
// error. Its entries are shared: the caller doesn't change them.
func (c *Client) Index(ctx context.Context) (List, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if c.entries != nil && now.Sub(c.checked) < Recheck {
		return List{Entries: c.entries}, nil
	}
	var meta indexMeta
	if data, err := os.ReadFile(c.metaFile()); err == nil {
		_ = json.Unmarshal(data, &meta)
	}
	kept := c.entries
	if kept == nil {
		if data, err := os.ReadFile(c.indexFile()); err == nil {
			kept, _ = ParseIndex(data)
		}
	}
	if kept != nil && now.Sub(meta.Checked) < Recheck && !meta.Checked.After(now) {
		c.entries, c.checked = kept, meta.Checked
		return List{Entries: kept}, nil
	}
	etag := ""
	if kept != nil {
		etag = meta.ETag
	}
	data, tag, err := c.fetch(ctx, c.Base+"INDEX.md", etag, maxIndex)
	var entries []Entry
	if err == nil && data != nil {
		entries, err = ParseIndex(data)
	}
	switch {
	case err != nil && kept != nil:
		// The next call checks again.
		c.entries = kept
		return List{Entries: kept, Offline: err}, nil
	case err != nil:
		return List{}, err
	case data == nil: // the same as the one kept
		entries = kept
	default:
		if err := c.write(c.indexFile(), data); err != nil {
			return List{}, err
		}
		meta.ETag = tag
	}
	meta.Checked = now
	if data, err := json.Marshal(meta); err == nil {
		_ = c.write(c.metaFile(), data)
	}
	c.entries, c.checked = entries, now
	return List{Entries: entries}, nil
}

// profileFile is where the profile of the folder path is kept: under the
// hash of its path, which no name of a model can make a path of its own
// of.
func (c *Client) profileFile(path string) string {
	sum := sha256.Sum256([]byte(path))
	return filepath.Join(c.dir, "profiles", hex.EncodeToString(sum[:12])+".txt")
}

// Kept reports whether the profile e is kept, and plays without the
// network.
func (c *Client) Kept(e Entry) bool {
	_, err := os.Stat(c.profileFile(e.Path))
	return err == nil
}

// ProfileURL is the address of the ParametricEQ.txt of e.
func (c *Client) ProfileURL(e Entry) string {
	parts := strings.Split(e.Path, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	model := e.Path[strings.LastIndex(e.Path, "/")+1:] // as AutoEq names the files of the folder
	return c.Base + strings.Join(parts, "/") + "/" + url.PathEscape(model+" ParametricEQ.txt")
}

// Profile returns the correction of e: the one kept, or the one on
// GitHub, which it keeps once it reads. A profile that doesn't read is
// never kept, and says why.
func (c *Client) Profile(ctx context.Context, e Entry) (settings.Headphones, error) {
	file := c.profileFile(e.Path)
	if data, err := os.ReadFile(file); err == nil {
		if h, err := settings.ParseParametricEQ(data); err == nil {
			return named(h, e), nil
		}
		os.Remove(file) // what isn't the profile goes, and the profile comes again
	}
	data, _, err := c.fetch(ctx, c.ProfileURL(e), "", maxProfile)
	if err != nil {
		return settings.Headphones{}, err
	}
	h, err := settings.ParseParametricEQ(data)
	if err != nil {
		return settings.Headphones{}, fmt.Errorf("reading AutoEq's profile of %s: %w", e.Name, err)
	}
	if err := c.write(file, data); err != nil {
		return settings.Headphones{}, err
	}
	return named(h, e), nil
}

// named is h, the correction of e, named after it.
func named(h settings.Headphones, e Entry) settings.Headphones {
	h.ID, h.Name, h.Source, h.Rig = e.Path, e.Name, e.Source, e.Rig
	return h
}

// fetch gets the file at addr, of up to most bytes: nil when etag is the
// file's still.
func (c *Client) fetch(ctx context.Context, addr, etag string, most int64) (data []byte, tag string, err error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, addr, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotModified && etag != "":
		return nil, etag, nil
	case resp.StatusCode != http.StatusOK:
		return nil, "", &StatusError{Code: resp.StatusCode, URL: addr}
	}
	data, err = io.ReadAll(io.LimitReader(resp.Body, most+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > most {
		return nil, "", fmt.Errorf("%s, past %d bytes: %w", addr, most, ErrTooBig)
	}
	return data, resp.Header.Get("ETag"), nil
}

// write keeps data at path, whole or not at all.
func (c *Client) write(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".autoeq-*")
	if err != nil {
		return err
	}
	_, err = tmp.Write(data)
	err = errors.Join(err, tmp.Close())
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		os.Remove(tmp.Name())
	}
	return err
}

// Package lastfm scrobbles what MixMusic plays to Last.fm.
//
// Client calls the Last.fm API 2.0: it connects the user's account with
// desktop authentication, and sends now-playing updates and scrobbles. It
// is the scrobble.Service of a scrobble.Scrobbler, which follows what the
// player does and decides which plays count as scrobbles. Other servers
// of the same API, as GNU FM's, take its calls at their own addresses.
package lastfm

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/scrobble"
)

const apiURL = "https://ws.audioscrobbler.com/2.0/"

// Error codes of the API that the package tells apart.
const (
	codeInvalidSession    = 9
	codeServiceOffline    = 11
	codeUnauthorizedToken = 14
	codeTemporary         = 16
	codeRateLimited       = 29
)

// ErrSessionExpired is returned when Last.fm no longer accepts a session key,
// such as after the user revoked MixMusic's access. The user must connect
// again. It is scrobble.ErrKeyRefused.
var ErrSessionExpired error = sessionExpired{}

type sessionExpired struct{}

func (sessionExpired) Error() string        { return "lastfm: the session expired, connect to Last.fm again" }
func (sessionExpired) Is(target error) bool { return target == scrobble.ErrKeyRefused }

// ErrNotAuthorized is returned by Session while the user has not approved
// the token in the browser yet.
var ErrNotAuthorized = errors.New("lastfm: the token has not been approved yet")

// APIError is an error the API answered with.
type APIError struct {
	// Code is the Last.fm error code, or 0 when the answer had none, such as
	// an HTTP error of a proxy.
	Code int
	// Message is Last.fm's description of the error.
	Message string
	// Status is the HTTP status of the answer.
	Status int
	// Service names the service in the error's text, as the client's Name
	// does; when empty, "lastfm".
	Service string
}

func (e *APIError) Error() string {
	name := serviceName(e.Service)
	switch {
	case e.Code == codeRateLimited:
		return name + ": too many requests, try again in a moment"
	case e.Code != 0 && e.Message != "":
		return fmt.Sprintf("%s: %s (error %d)", name, e.Message, e.Code)
	case e.Code != 0:
		return fmt.Sprintf("%s: error %d", name, e.Code)
	}
	return fmt.Sprintf("%s: HTTP %d", name, e.Status)
}

// serviceName is name, a service's name in errors, or "lastfm" when it
// is empty.
func serviceName(name string) string {
	if name == "" {
		return "lastfm"
	}
	return name
}

// temporary reports whether the same call may succeed later.
func (e *APIError) temporary() bool {
	switch e.Code {
	case codeServiceOffline, codeTemporary, codeRateLimited:
		return true
	}
	return e.Status >= 500 || e.Status == http.StatusTooManyRequests
}

// Rejected reports whether Last.fm refused the call for what it holds,
// which sending it again won't change: what isn't temporary.
func (e *APIError) Rejected() bool { return !e.temporary() }

// netError is a call to the service of name that got no answer.
type netError struct {
	name string
	err  error
}

func (e *netError) Error() string { return serviceName(e.name) + ": " + e.err.Error() }
func (e *netError) Unwrap() error { return e.err }

// IsTemporary reports whether err may go away when the call is made again
// later: the network failed, Last.fm is offline or failed on its side, or it
// limits the rate of calls.
func IsTemporary(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.temporary()
	}
	var netErr *netError
	return errors.As(err, &netErr)
}

// defaultHTTP sends the requests of clients that have no HTTP client.
var defaultHTTP = &http.Client{Timeout: 20 * time.Second}

// Client calls the Last.fm API with the API account of the app. Its methods
// are safe for concurrent use.
type Client struct {
	// APIKey identifies the API account.
	APIKey string
	// Secret is the shared secret of the API account, which signs calls.
	Secret string
	// HTTP sends the requests. When nil, a client with a 20-second timeout
	// does.
	HTTP *http.Client
	// BaseURL is the API's address, replaced in tests. When empty, it is
	// https://ws.audioscrobbler.com/2.0/.
	BaseURL string
	// UserAgent introduces MixMusic to Last.fm; when empty, as
	// "MixMusic".
	UserAgent string
	// AuthPage is the page where the user approves a token, another
	// server's. When empty, it is https://www.last.fm/api/auth/.
	AuthPage string
	// Name names the service in the errors of the calls, as "librefm";
	// when empty, "lastfm".
	Name string
}

// NewClient returns a client of the API account of apiKey and secret.
func NewClient(apiKey, secret string) *Client {
	return &Client{APIKey: apiKey, Secret: secret, HTTP: &http.Client{Timeout: 20 * time.Second}, BaseURL: apiURL}
}

// call makes a signed call of method with params, and decodes the answer
// into out when out is not nil.
func (c *Client) call(ctx context.Context, method string, params url.Values, out any) error {
	form := url.Values{"method": {method}, "api_key": {c.APIKey}}
	for name, values := range params {
		form[name] = values
	}
	form.Set("api_sig", signature(form, c.Secret))
	form.Set("format", "json")
	base := c.BaseURL
	if base == "" {
		base = apiURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	agent := c.UserAgent
	if agent == "" {
		agent = "MixMusic"
	}
	req.Header.Set("User-Agent", agent)
	hc := c.HTTP
	if hc == nil {
		hc = defaultHTTP
	}
	res, err := hc.Do(req)
	if err != nil {
		return &netError{c.Name, err}
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	res.Body.Close()
	if err != nil {
		return &netError{c.Name, err}
	}
	var answer struct {
		Error   json.RawMessage `json:"error"`
		Message string          `json:"message"`
	}
	if json.Unmarshal(data, &answer) == nil {
		if code, message := errorOf(answer.Error, answer.Message); code != 0 {
			switch code {
			case codeInvalidSession:
				return ErrSessionExpired
			case codeUnauthorizedToken:
				return ErrNotAuthorized
			}
			return &APIError{Code: code, Message: message, Status: res.StatusCode, Service: c.Name}
		}
	}
	if res.StatusCode >= 300 {
		return &APIError{Status: res.StatusCode, Service: c.Name}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("%s: decoding %s: %w", serviceName(c.Name), method, err)
	}
	return nil
}

// errorOf reads the error of an answer, its code and its message: code 0
// when it has none. Last.fm writes the code as a number beside the
// message. GNU FM writes some so, and others, of the calls it failed on
// its side, as an object of the code, a string, and the message:
// {"error": {"code": "16", "#text": "..."}}.
func errorOf(raw json.RawMessage, message string) (int, string) {
	var code int
	if json.Unmarshal(raw, &code) == nil {
		return code, message
	}
	var obj struct {
		Code json.RawMessage `json:"code"`
		Text string          `json:"#text"`
	}
	if json.Unmarshal(raw, &obj) != nil {
		return 0, ""
	}
	code, err := strconv.Atoi(strings.Trim(string(obj.Code), `"`))
	if err != nil {
		return 0, ""
	}
	return code, obj.Text
}

// signature returns the api_sig of params: the MD5 hex of every parameter
// but format, callback and api_sig itself, sorted by name and written as
// name and value, followed by secret.
func signature(params url.Values, secret string) string {
	names := make([]string, 0, len(params))
	for name := range params {
		if name != "format" && name != "callback" && name != "api_sig" {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	h := md5.New()
	for _, name := range names {
		io.WriteString(h, name)
		io.WriteString(h, params.Get(name))
	}
	io.WriteString(h, secret)
	return hex.EncodeToString(h.Sum(nil))
}

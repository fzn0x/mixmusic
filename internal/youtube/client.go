// Package youtube reads YouTube Music's catalog through InnerTube, the API
// the YouTube Music website itself uses: search, the home feed, albums,
// artists, playlists and radios. It needs no account and no API key;
// signed in with the cookies of the user's browser session, it also reads
// and changes their library.
package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultBaseURL = "https://music.youtube.com"
	clientName     = "WEB_REMIX"
	// clientNameID is the numeric ID of clientName, for the headers.
	clientNameID = "67"
	userAgent    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36"
)

// Client reads YouTube Music. The zero Client is not usable: create one
// with New. Its methods are safe for concurrent use.
type Client struct {
	// HTTP makes the requests.
	HTTP *http.Client
	// Language and Region localize results, as "en" and "US".
	Language, Region string
	// BaseURL is the API's origin, replaced in tests.
	BaseURL string

	// OnCookies is called with the session's cookies when YouTube changed
	// some, so that the app keeps the session fresh.
	OnCookies func([]*http.Cookie)
	// OnRejected is called, once, when YouTube stops accepting the
	// session, so that the app asks the user to sign in again.
	OnRejected func()

	mu       sync.Mutex
	visitor  string         // the visitor ID YouTube assigned, sent back to it
	cookies  []*http.Cookie // the signed-in session, if any
	rejected bool           // YouTube answered the session as a visitor
	pageID   string         // the brand channel the session acts as, if any
	// authUser is the Google account the session acts as, by its index
	// among those it is signed in to: a browser signed in to several
	// numbers them from 0.
	authUser int
}

// New returns a client for YouTube Music in English, for the US.
func New() *Client {
	return &Client{
		HTTP:     &http.Client{Timeout: 20 * time.Second},
		Language: "en",
		Region:   "US",
		BaseURL:  defaultBaseURL,
	}
}

// APIError is an error InnerTube answered with.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("youtube music: HTTP %d", e.Status)
	}
	return fmt.Sprintf("youtube music: %s (HTTP %d)", e.Message, e.Status)
}

// clientVersion returns a version of the website's client that YouTube
// accepts: the website's build of a few days ago.
func clientVersion(now time.Time) string { return musicClient.version(now) }

// innerClient is a client of InnerTube: its name, its number, and the
// major version its builds carry.
type innerClient struct{ name, id, major string }

// musicClient is YouTube Music's website, which MixMusic speaks as.
var musicClient = innerClient{name: clientName, id: clientNameID, major: "1"}

// version returns a version of the client that YouTube accepts: its build
// of a few days ago.
func (ic innerClient) version(now time.Time) string {
	return ic.major + "." + now.UTC().AddDate(0, 0, -3).Format("20060102") + ".01.00"
}

// call posts body to an InnerTube endpoint, such as "search" or "browse",
// adding the client context, and returns the decoded response.
func (c *Client) call(ctx context.Context, endpoint string, body map[string]any, query url.Values) (node, error) {
	return c.callAs(ctx, musicClient, endpoint, body, query)
}

// callAs is call, speaking as another client of InnerTube.
func (c *Client) callAs(ctx context.Context, ic innerClient, endpoint string, body map[string]any, query url.Values) (node, error) {
	version := ic.version(time.Now())
	user := map[string]any{}
	if page := c.Channel(); page != "" && c.SignedIn() {
		user["onBehalfOfUser"] = page
	}
	payload := map[string]any{
		"context": map[string]any{
			"client": map[string]any{
				"clientName":    ic.name,
				"clientVersion": version,
				"hl":            c.Language,
				"gl":            c.Region,
			},
			"user": user,
		},
	}
	for k, v := range body {
		payload[k] = v
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return node{}, err
	}
	if query == nil {
		query = url.Values{}
	}
	query.Set("prettyPrint", "false")
	u := strings.TrimRight(c.BaseURL, "/") + "/youtubei/v1/" + endpoint + "?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(data))
	if err != nil {
		return node{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Origin", defaultBaseURL)
	req.Header.Set("Referer", defaultBaseURL+"/")
	req.Header.Set("X-YouTube-Client-Name", ic.id)
	req.Header.Set("X-YouTube-Client-Version", version)
	if v := c.visitorID(); v != "" {
		req.Header.Set("X-Goog-Visitor-Id", v)
	}
	now := time.Now()
	signed := c.signRequest(req, now)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return node{}, fmt.Errorf("youtube music: %w", err)
	}
	defer res.Body.Close()
	c.keepCookies(res.Cookies(), now)
	if signed && (res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden) {
		c.reject()
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return node{}, fmt.Errorf("youtube music: %w", err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		if res.StatusCode != http.StatusOK {
			return node{}, &APIError{Status: res.StatusCode}
		}
		return node{}, fmt.Errorf("youtube music: decoding %s: %w", endpoint, err)
	}
	n := node{v}
	if res.StatusCode != http.StatusOK {
		return node{}, &APIError{Status: res.StatusCode, Message: n.get("error", "message").str()}
	}
	if v := n.get("responseContext", "visitorData").str(); v != "" {
		c.setVisitorID(v)
	}
	if signed && accountEndpoints[endpoint] && loggedOut(n) {
		c.reject()
	}
	return n, nil
}

// accountEndpoints are those whose answer says whether YouTube took the
// session: the account's menu and the pages of its library.
var accountEndpoints = map[string]bool{"browse": true, "account/account_menu": true}

// loggedOut reports whether YouTube answered as to a visitor, signed out.
func loggedOut(n node) bool {
	for _, s := range n.get("responseContext", "serviceTrackingParams").list() {
		for _, p := range s.get("params").list() {
			if p.get("key").str() == "logged_in" {
				return p.get("value").str() == "0"
			}
		}
	}
	return false
}

func (c *Client) visitorID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.visitor
}

func (c *Client) setVisitorID(v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.visitor = v
}

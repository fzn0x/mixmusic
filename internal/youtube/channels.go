package youtube

import (
	"context"
	"net/url"
	"strconv"
)

// AccountChannel is an identity the account can use YouTube Music as: the
// account itself, or a brand channel it manages, each with a library of its
// own.
type AccountChannel struct {
	Name   string `json:"name"`
	Handle string `json:"handle,omitempty"`
	Photo  string `json:"photo,omitempty"`
	// PageID names a brand channel; it is empty for the account itself.
	PageID string `json:"pageId,omitempty"`
	// AuthUser is the index of the Google account the channel belongs to,
	// among those the session is signed in to (SetAuthUser), and Email
	// that account's address, when the list tells it.
	AuthUser int    `json:"authUser,omitempty"`
	Email    string `json:"email,omitempty"`
	// Current is set for the channel the session uses.
	Current bool `json:"current,omitempty"`
}

// webClient is YouTube's website, which lists the channels of an account;
// YouTube Music's client does not.
var webClient = innerClient{name: "WEB", id: "1", major: "2"}

// Channels returns the channels the account can use YouTube Music as.
func (c *Client) Channels(ctx context.Context) ([]AccountChannel, error) {
	if err := c.session(); err != nil {
		return nil, err
	}
	n, err := c.callAs(ctx, webClient, "account/accounts_list", map[string]any{
		"requestType":      "ACCOUNTS_LIST_REQUEST_TYPE_CHANNEL_SWITCHER",
		"callCircumstance": "SWITCHING_USERS_FULL",
	}, nil)
	if err != nil {
		return nil, err
	}
	return parseChannels(n, c.AuthUser()), nil
}

// parseChannels reads the account items of an accounts list, wherever the
// list puts them, leaving out the ones it shows disabled. A session signed
// in to several Google accounts lists the channels of each in a section
// headed by its address; an item's sign-in link names the index of its
// account. An item without one is of the account of the other items of its
// section, else of asked, the account the list was asked as.
func parseChannels(n node, asked int) []AccountChannel {
	var channels []AccountChannel
	var known []bool // whether the item named its account
	seen := map[string]bool{}
	var walk func(v any, email string)
	walk = func(v any, email string) {
		switch v := v.(type) {
		case []any:
			for _, x := range v {
				walk(x, email)
			}
		case map[string]any:
			if e := (node{v}).get("header", "googleAccountHeaderRenderer", "email").text(); e != "" {
				email = e
			}
			for key, x := range v {
				if key != "accountItem" && key != "accountItemRenderer" {
					walk(x, email)
					continue
				}
				item := node{x}
				user, named := channelAuthUser(item)
				ch := AccountChannel{
					Name:     item.get("accountName").text(),
					Handle:   item.get("channelHandle").text(),
					PageID:   channelPageID(item),
					AuthUser: user,
					Email:    email,
					Current:  item.get("isSelected").boolean(),
				}
				if thumbs := item.get("accountPhoto", "thumbnails").list(); len(thumbs) > 0 {
					ch.Photo = thumbs[len(thumbs)-1].get("url").str()
				}
				id := email + "|" + ch.PageID + "|" + ch.Name
				if ch.Name == "" || item.get("isDisabled").boolean() || seen[id] {
					continue
				}
				seen[id] = true
				channels = append(channels, ch)
				known = append(known, named)
			}
		}
	}
	walk(n.v, "")
	sections := map[string]int{}
	for i, ch := range channels {
		if known[i] && ch.Email != "" {
			sections[ch.Email] = ch.AuthUser
		}
	}
	for i := range channels {
		if !known[i] {
			user, ok := sections[channels[i].Email]
			if !ok || channels[i].Email == "" {
				user = asked
			}
			channels[i].AuthUser = user
		}
	}
	return channels
}

// channelPageID returns the page ID of a brand channel's account item, in
// the tokens its switching endpoint carries; the account itself has none.
func channelPageID(item node) string {
	for _, t := range channelTokens(item) {
		if id := t.get("pageIdToken", "pageId").str(); id != "" {
			return id
		}
	}
	return ""
}

// channelAuthUser returns the index of the Google account of an account
// item, from the authuser of the sign-in link its switching endpoint
// carries, and whether the item had one.
func channelAuthUser(item node) (int, bool) {
	for _, t := range channelTokens(item) {
		link := t.get("accountSigninToken", "signinUrl").str()
		if link == "" {
			continue
		}
		u, err := url.Parse(link)
		if err != nil {
			continue
		}
		if i, err := strconv.Atoi(u.Query().Get("authuser")); err == nil && i >= 0 {
			return i, true
		}
	}
	return 0, false
}

func channelTokens(item node) []node {
	return item.get("serviceEndpoint", "selectActiveIdentityEndpoint", "supportedTokens").list()
}

// SetChannel makes the client use YouTube Music as the brand channel of
// pageID, or as the account itself with "".
func (c *Client) SetChannel(pageID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pageID = pageID
}

// Channel returns the page ID of the brand channel the client uses, "" for
// the account itself.
func (c *Client) Channel() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pageID
}

// SetAuthUser makes the client use YouTube Music as the Google account of
// index i among those the session is signed in to: a browser signed in to
// several Google accounts numbers them from 0, in its X-Goog-AuthUser
// header. A brand channel belongs to one of them: SetChannel names it.
func (c *Client) SetAuthUser(i int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.authUser = max(i, 0)
}

// SetAccount makes the client use YouTube Music as the Google account of
// index authUser and, with pageID, as its brand channel, both at once: a
// brand channel goes with its own account alone.
func (c *Client) SetAccount(authUser int, pageID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.authUser, c.pageID = max(authUser, 0), pageID
}

// AuthUser returns the index of the Google account the client uses.
func (c *Client) AuthUser() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.authUser
}

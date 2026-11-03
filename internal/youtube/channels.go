package youtube

import (
	"cmp"
	"context"
	"net/url"
	"strconv"
	"strings"
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
	// Unswitchable is set for a brand channel the list tells no page ID
	// of: without one, MixMusic would use the account itself.
	Unswitchable bool `json:"unswitchable,omitempty"`
}

// Is reports whether a, the account YouTube Music answers as, is ch: by
// their handles when both have one, which are the channels' own, else by
// their names.
func (ch AccountChannel) Is(a Account) bool {
	if ch.Handle != "" && a.Handle != "" {
		return strings.EqualFold(ch.Handle, a.Handle)
	}
	return strings.EqualFold(strings.TrimSpace(ch.Name), strings.TrimSpace(a.Name))
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
//
// Of the items of an account that tell no page ID, one is the account
// itself: the one whose data sync ID says so, else the first, as the list
// puts the account before its brand channels. The others are brand
// channels that MixMusic can't tell apart from it: Unswitchable.
func parseChannels(n node, asked int) []AccountChannel {
	var channels []AccountChannel
	// Of each item, whether it named its account, and whether its tokens
	// tell it is a brand channel, or the account itself.
	type told struct{ account, brand, own bool }
	var tells []told
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
				page, brand, own := channelIdentity(item)
				ch := AccountChannel{
					Name:     item.get("accountName").text(),
					Handle:   item.get("channelHandle").text(),
					PageID:   page,
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
				tells = append(tells, told{named, brand, own})
			}
		}
	}
	walk(n.v, "")
	sections := map[string]int{}
	for i, ch := range channels {
		if tells[i].account && ch.Email != "" {
			sections[ch.Email] = ch.AuthUser
		}
	}
	for i := range channels {
		if !tells[i].account {
			user, ok := sections[channels[i].Email]
			if !ok || channels[i].Email == "" {
				user = asked
			}
			channels[i].AuthUser = user
		}
	}
	accountOf := func(ch AccountChannel) string { return ch.Email + "|" + strconv.Itoa(ch.AuthUser) }
	self := map[string]int{} // the item that is the account itself, of each account
	for i, ch := range channels {
		if ch.PageID != "" || tells[i].brand {
			continue
		}
		if j, ok := self[accountOf(ch)]; !ok || tells[i].own && !tells[j].own {
			self[accountOf(ch)] = i
		}
	}
	for i := range channels {
		if j, ok := self[accountOf(channels[i])]; channels[i].PageID == "" && (!ok || j != i) {
			channels[i].Unswitchable = true
		}
	}
	return channels
}

// channelIdentity returns the page ID of a brand channel's account item,
// from the tokens its switching endpoint carries: its page ID token, else
// the page its sign-in link names, else its data sync ID. That holds the
// IDs of the channel and of its account, "channel||account", for a brand
// channel, and "account||" for the account itself. brand reports an item
// the tokens tell is a brand channel's, own one they tell is the account
// itself.
func channelIdentity(item node) (pageID string, brand, own bool) {
	var linked, synced string
	for _, t := range channelTokens(item) {
		if id := t.get("pageIdToken", "pageId").str(); id != "" && pageID == "" {
			pageID = id
		}
		if link := t.get("accountSigninToken", "signinUrl").str(); link != "" && linked == "" {
			if u, err := url.Parse(link); err == nil {
				linked = u.Query().Get("pageid")
			}
		}
		if id := t.get("datasyncIdToken", "datasyncIdToken").str(); id != "" {
			channel, account, ok := strings.Cut(id, "||")
			switch {
			case !ok:
			case account != "":
				brand, synced = true, channel
			case channel != "":
				own = true
			}
		}
	}
	pageID = cmp.Or(pageID, linked, synced)
	brand = brand || pageID != ""
	return pageID, brand, own && !brand
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

package youtube

import (
	"context"
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
	return parseChannels(n), nil
}

// parseChannels reads the account items of an accounts list, wherever the
// list puts them, leaving out the ones it shows disabled.
func parseChannels(n node) []AccountChannel {
	var channels []AccountChannel
	seen := map[string]bool{}
	var walk func(v any)
	walk = func(v any) {
		switch v := v.(type) {
		case []any:
			for _, x := range v {
				walk(x)
			}
		case map[string]any:
			for key, x := range v {
				if key != "accountItem" && key != "accountItemRenderer" {
					walk(x)
					continue
				}
				item := node{x}
				ch := AccountChannel{
					Name:    item.get("accountName").text(),
					Handle:  item.get("channelHandle").text(),
					PageID:  channelPageID(item),
					Current: item.get("isSelected").boolean(),
				}
				if thumbs := item.get("accountPhoto", "thumbnails").list(); len(thumbs) > 0 {
					ch.Photo = thumbs[len(thumbs)-1].get("url").str()
				}
				if ch.Name == "" || item.get("isDisabled").boolean() || seen[ch.PageID+"|"+ch.Name] {
					continue
				}
				seen[ch.PageID+"|"+ch.Name] = true
				channels = append(channels, ch)
			}
		}
	}
	walk(n.v)
	return channels
}

// channelPageID returns the page ID of a brand channel's account item, in
// the tokens its switching endpoint carries; the account itself has none.
func channelPageID(item node) string {
	for _, t := range item.get("serviceEndpoint", "selectActiveIdentityEndpoint", "supportedTokens").list() {
		if id := t.get("pageIdToken", "pageId").str(); id != "" {
			return id
		}
	}
	return ""
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

// AuthUser returns the index of the Google account the client uses.
func (c *Client) AuthUser() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.authUser
}

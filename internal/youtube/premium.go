package youtube

import (
	"strconv"
	"strings"
)

// Premium: YouTube Music tells, in the responseContext of each answer to a
// signed-in request, among the GFEEDBACK service's tracking parameters,
// whether the account has YouTube Music Premium, which YouTube Premium
// gives too: has_unlimited_entitlement, "True" or "False", "unlimited"
// being its name for Music Premium, and premium_membership,
// "youtube_premium" for YouTube Premium. MixMusic reads it from the answers
// it gets anyway, to offer what Premium brings.

// premiumSeen is what the latest signed-in answer that told of it told of
// the account's Premium, and of which session and account: told is false
// until one did.
type premiumSeen struct {
	of      string
	told    bool
	premium bool
}

// readPremium returns what an answer tells of the account's Premium, and
// whether it told: has_unlimited_entitlement first, else
// premium_membership when it says YouTube Premium.
func readPremium(n node) (premium, told bool) {
	membership := ""
	for _, s := range n.get("responseContext", "serviceTrackingParams").list() {
		for _, p := range s.get("params").list() {
			switch p.get("key").str() {
			case "has_unlimited_entitlement":
				return strings.EqualFold(p.get("value").str(), "true"), true
			case "premium_membership":
				membership = p.get("value").str()
			}
		}
	}
	if membership == "youtube_premium" {
		return true, true
	}
	return false, false
}

// premiumKey names the session and the account the client signs requests
// as, whose Premium their answers tell: "" when signed out. It runs under
// the client's mutex.
func (c *Client) premiumKey() string {
	if signingCookie(c.cookies) == "" {
		return ""
	}
	return signingCookie(c.cookies) + "\x00" + strconv.Itoa(c.authUser)
}

// signedAs is premiumKey of a request just signed, when signed, else "".
func (c *Client) signedAs(signed bool) string {
	if !signed {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.premiumKey()
}

// seePremium keeps what n, the answer to a request signed as of, tells of
// the account's Premium.
func (c *Client) seePremium(of string, n node) {
	premium, told := readPremium(n)
	if of == "" || !told {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.premium = premiumSeen{of: of, told: true, premium: premium}
}

// Premium reports whether YouTube Music told, in its latest answer to a
// signed-in request that told of it, that the account the client uses
// has YouTube Music Premium: false until one did, once the client uses
// another session or account, and once YouTube ended the session.
func (c *Client) Premium() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.premium.told && c.premium.premium && !c.rejected && c.premium.of == c.premiumKey()
}

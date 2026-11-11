package youtube

// Collaboration on a playlist of the user's, as YouTube Music's Share ›
// Invite collaborators does: the playlist's owner turns it on, which makes
// an invite link, and whoever opens the link joins the playlist and adds
// to it, until the owner turns it off. The edits go through
// browse/edit_playlist, as the playlist's other edits do.

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// The actions of browse/edit_playlist that collaboration takes, and the
// one that says who may open a playlist.
const (
	// actionInvite turns collaboration on, and answers with a new invite
	// link, collaborationInviteLink.
	actionInvite = "ACTION_CREATE_COLLABORATION_INVITE_LINK"
	// actionClose turns it off, with closedToContributions.
	actionClose = "ACTION_SET_CLOSED_TO_CONTRIBUTIONS"
	// actionPrivacy sets who may open the playlist, with playlistPrivacy.
	actionPrivacy = "ACTION_SET_PLAYLIST_PRIVACY"
)

// privacyStatuses are YouTube's names of the privacies of a playlist.
var privacyStatuses = map[media.Privacy]string{
	media.Public:   "PUBLIC",
	media.Unlisted: "UNLISTED",
	media.Private:  "PRIVATE",
}

// collaborateTag is the tag of the panel that the avatars of a playlist's
// header open when they're its collaborators', rather than its owner's.
const collaborateTag = "PAplaylist_collaborate"

var (
	// errInviteRefused is an invite YouTube answered without making it.
	errInviteRefused = errors.New("YouTube Music did not turn collaboration on")
	// errInviteDenied is an invite YouTube refused with a status of HTTP:
	// of a private playlist, or another account's.
	errInviteDenied = errors.New("YouTube Music refused to turn collaboration on: a private playlist must be unlisted or public first, and only its owner can invite")
	// errNoInviteLink is an invite YouTube took without giving its link.
	errNoInviteLink = errors.New("YouTube Music turned collaboration on but gave no invite link: share the playlist from YouTube Music")
	// errStopRefused and errStopDenied are a stop of collaboration YouTube
	// answered without making it, and refused.
	errStopRefused = errors.New("YouTube Music did not turn collaboration off")
	errStopDenied  = errors.New("YouTube Music refused to turn collaboration off: it may be off already, or the playlist another account's")
	// errPrivacyRefused is a change of who may open a playlist YouTube
	// answered without making it.
	errPrivacyRefused = errors.New("YouTube Music did not change who can open the playlist")
)

// InviteCollaborators turns YouTube Music's collaboration on for a playlist
// the user made, and returns its invite link: whoever opens it can join
// the playlist and add to it. Each call makes a new link. Collaborators
// can't open a private playlist, which SetPlaylistPrivacy makes unlisted
// first.
func (c *Client) InviteCollaborators(ctx context.Context, playlistID string) (string, error) {
	_, link, err := c.invite(ctx, playlistID, 0)
	return link, err
}

// InviteCollaboratorsOnceUnlisted is InviteCollaborators for a playlist
// that SetPlaylistPrivacy made unlisted a moment ago. YouTube refuses to
// turn collaboration on, with HTTP 400, until the change reaches it, so it
// asks again every unlistedRetry, for up to unlistedWait.
func (c *Client) InviteCollaboratorsOnceUnlisted(ctx context.Context, playlistID string) (string, error) {
	_, link, err := c.invite(ctx, playlistID, unlistedWait)
	return link, err
}

// unlistedWait is how long an invite waits for YouTube to take in that a
// private playlist is unlisted now, and unlistedRetry how often it asks
// meanwhile. YouTube answers "Precondition check failed" for a while
// after the change: 18 s on 2026-11-11. Tests shorten them.
var (
	unlistedWait  = 45 * time.Second
	unlistedRetry = 3 * time.Second
)

// invite is InviteCollaborators, which returns YouTube's answer too. While
// YouTube refuses it with HTTP 400, it asks again for up to wait.
func (c *Client) invite(ctx context.Context, playlistID string, wait time.Duration) (node, string, error) {
	deadline := time.Now().Add(wait)
	for {
		n, err := c.editPlaylistAnswer(ctx, playlistID, map[string]any{"action": actionInvite}, errInviteRefused, errInviteDenied)
		if err == nil {
			link := inviteLink(n)
			if link == "" {
				return n, "", errNoInviteLink
			}
			return n, link, nil
		}
		var api *APIError
		if !errors.As(err, &api) || api.Status != http.StatusBadRequest || time.Now().Add(unlistedRetry).After(deadline) {
			return n, "", badRequest(err, errInviteDenied)
		}
		select {
		case <-ctx.Done():
			return n, "", badRequest(err, errInviteDenied)
		case <-time.After(unlistedRetry):
		}
	}
}

// StopCollaboration turns YouTube Music's collaboration off for a playlist
// the user made, as its own switch does: its invite links stop working.
func (c *Client) StopCollaboration(ctx context.Context, playlistID string) error {
	_, err := c.editPlaylistAnswer(ctx, playlistID, map[string]any{"action": actionClose, "closedToContributions": true},
		errStopRefused, errStopDenied)
	return badRequest(err, errStopDenied)
}

// SetPlaylistPrivacy sets who may open a playlist the user made: Public,
// Unlisted or Private.
func (c *Client) SetPlaylistPrivacy(ctx context.Context, playlistID string, privacy media.Privacy) error {
	status, ok := privacyStatuses[privacy]
	if !ok {
		return errors.New("no such privacy of a playlist: " + string(privacy))
	}
	_, err := c.editPlaylistAnswer(ctx, playlistID, map[string]any{"action": actionPrivacy, "playlistPrivacy": status},
		errPrivacyRefused, errNotYours)
	return err
}

// badRequest is err, the error of an edit, as refused when YouTube answered
// it with 400, which it answers an edit it won't make.
func badRequest(err, refused error) error {
	var api *APIError
	if errors.As(err, &api) && api.Status == http.StatusBadRequest {
		return refused
	}
	return err
}

// inviteLink returns the invite link of YouTube's answer to an invite, or
// "" when it has none on YouTube's sites.
func inviteLink(n node) string {
	link := n.get("collaborationInviteLink").str()
	if link == "" {
		link = n.find("collaborationInviteLink").str()
	}
	u, err := url.Parse(link)
	if err != nil || u.Scheme != "https" || !youtubeHost(u.Hostname()) {
		return ""
	}
	return link
}

// youtubeHost reports whether host is one of YouTube's.
func youtubeHost(host string) bool {
	return host == "youtube.com" || strings.HasSuffix(host, ".youtube.com") || host == "youtu.be"
}

// headerPrivacy returns who may open a playlist of the user's, as the
// tools of its page's header, head, tell, or "" when they don't.
func headerPrivacy(head node) media.Privacy {
	status := head.get("musicEditablePlaylistDetailHeaderRenderer", "editHeader", "musicPlaylistEditHeaderRenderer", "privacy").str()
	for privacy, s := range privacyStatuses {
		if s == status {
			return privacy
		}
	}
	return ""
}

// facepile reads the avatars of a playlist's header h: whether they're its
// collaborators', which open the panel of its collaboration, and the line
// that names them, as "by Ana and 2 others"; or else its owner's, and the
// owner's name.
func facepile(h node) (collaborative bool, names string) {
	stack := h.get("facepile", "avatarStackViewModel")
	if !stack.exists() {
		return false, ""
	}
	rc := stack.get("rendererContext")
	tag := rc.get("commandContext", "onTap", "innertubeCommand", "showEngagementPanelEndpoint", "identifier", "tag").str()
	if tag == collaborateTag {
		return true, rc.get("accessibilityContext", "label").str()
	}
	return false, stack.get("text", "content").str()
}

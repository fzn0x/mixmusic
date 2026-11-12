package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestLiveCollaboration makes a private playlist of one song on a real
// account, makes it unlisted, invites collaborators, and turns that off,
// reading the playlist's page between the steps; then it deletes the
// playlist. It changes the account, so it runs only with
// MIXMUSIC_YT_WRITE=1 too, best with an account kept for tests.
// MIXMUSIC_YT_RECORD names a folder to keep YouTube's answers in, for the
// fixtures of collab_test.go, whose names and IDs are to be taken out
// before they're kept. MIXMUSIC_YT_COOKIES2, the cookies of a second
// account, joins the playlist by its invite link, and reads it and its
// library as a collaborator sees them, which the tests here don't know
// yet:
//
//	MIXMUSIC_LIVE=1 MIXMUSIC_YT_WRITE=1 MIXMUSIC_YT_COOKIES='SAPISID=…; …' MIXMUSIC_YT_COOKIES2='SAPISID=…; …' MIXMUSIC_YT_RECORD=dir go test ./internal/youtube -run LiveCollaboration -v
func TestLiveCollaboration(t *testing.T) {
	header := os.Getenv("MIXMUSIC_YT_COOKIES")
	if os.Getenv("MIXMUSIC_LIVE") == "" || os.Getenv("MIXMUSIC_YT_WRITE") == "" || header == "" {
		t.Skip("set MIXMUSIC_LIVE=1, MIXMUSIC_YT_WRITE=1 and MIXMUSIC_YT_COOKIES to invite collaborators to a playlist of a signed-in account")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c := signedInClient(t, header)
	record := func(name string, n node) {
		t.Helper()
		data, _ := json.MarshalIndent(n.v, "", " ")
		if dir := os.Getenv("MIXMUSIC_YT_RECORD"); dir != "" {
			if err := os.WriteFile(filepath.Join(dir, name+".json"), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	const song = "BH-SnQ8J1VU" // the first song of testdata/playlist.json
	id, err := c.CreatePlaylist(ctx, "MixMusic live test "+time.Now().Format("2006-01-02 15:04:05"), []string{song})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.DeletePlaylist(context.Background(), id); err != nil {
			t.Errorf("deleting %s at the end: %v", id, err)
		}
	})
	// page reads the playlist's page as client reads it once it shows
	// what ok wants, for a few seconds, as YouTube's pages catch up, and
	// keeps it as name.
	page := func(client *Client, name string, ok func(p media.Playlist) bool) media.Playlist {
		t.Helper()
		var last any
		for start := time.Now(); ; time.Sleep(time.Second) {
			n, err := client.browse(ctx, "VL"+id)
			if err == nil {
				p, _, perr := parsePlaylist(n, id)
				last = perr
				if perr == nil && ok(p) {
					record(name, n)
					t.Logf("%s: owner %q, mine %v, editable %v, deletable %v, collaborative %v, privacy %q",
						name, p.Owner, p.Mine, p.Editable, p.Deletable, p.Collaborative, p.Privacy)
					return p
				}
				last = p
			} else {
				last = err
			}
			if time.Since(start) > 20*time.Second {
				t.Fatalf("%s: not after 20 s; the last answer: %+v", name, last)
			}
		}
	}

	page(c, "collab-page-private", func(p media.Playlist) bool { return p.Mine && p.Privacy == media.Private })
	if err := c.SetPlaylistPrivacy(ctx, id, media.Unlisted); err != nil {
		t.Fatal(err)
	}
	page(c, "collab-page-unlisted", func(p media.Playlist) bool { return p.Privacy == media.Unlisted })

	answer, link, err := c.invite(ctx, id, unlistedWait)
	record("collab-invite", answer)
	if err != nil {
		t.Fatalf("inviting: %v; the answer: %v", err, answer.v)
	}
	t.Logf("the invite link: %s", link)
	page(c, "collab-page-invited", func(media.Playlist) bool { return true })

	if header2 := os.Getenv("MIXMUSIC_YT_COOKIES2"); header2 != "" {
		joinAsCollaborator(t, ctx, signedInClient(t, header2), id, link, record, page)
		page(c, "collab-page-joined", func(p media.Playlist) bool { return p.Collaborative })
	}

	closed, err := c.editPlaylistAnswer(ctx, id, map[string]any{"action": actionClose, "closedToContributions": true}, errStopRefused, errStopDenied)
	record("collab-close", closed)
	if err != nil {
		t.Fatalf("turning collaboration off: %v; the answer: %v", err, closed.v)
	}
	page(c, "collab-page-closed", func(media.Playlist) bool { return true })
}

// joinAsCollaborator joins the playlist id with the invite link, as the
// account of c, and reads its page and the account's library, as a
// collaborator sees them; then it takes the playlist out of the library.
func joinAsCollaborator(t *testing.T, ctx context.Context, c *Client, id, link string,
	record func(string, node), page func(*Client, string, func(media.Playlist) bool) media.Playlist) {
	t.Helper()
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	token := u.Query().Get("jct")
	if token == "" {
		t.Fatalf("the invite link %s has no jct", link)
	}
	joined, err := c.call(ctx, "browse/edit_playlist", map[string]any{"playlistId": id,
		"actions": []any{map[string]any{"action": "ACTION_JOIN_COLLABORATION", "joinCollaborationToken": token}}}, nil)
	record("collab-join", joined)
	if err != nil {
		t.Fatalf("joining: %v", err)
	}
	t.Cleanup(func() {
		if err := c.SetSaved(context.Background(), id, false); err != nil {
			t.Logf("taking it out of the collaborator's library: %v", err)
		}
	})
	page(c, "collab-page-collaborator", func(media.Playlist) bool { return true })
	inLibrary(t, ctx, c, id, record)
	editAsCollaborator(t, ctx, c, id, record, page)
}

// inLibrary waits for the playlist id to show in the library of c, a
// collaborator's.
func inLibrary(t *testing.T, ctx context.Context, c *Client, id string, record func(string, node)) {
	t.Helper()
	for start := time.Now(); ; time.Sleep(time.Second) {
		n, err := c.browse(ctx, "FEmusic_liked_playlists")
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range parseShelves(n) {
			for _, it := range s.Items {
				if p := it.Playlist; p != nil && p.ID == media.ID(media.SourceYouTube, id) {
					record("collab-library-collaborator", n)
					t.Logf("in the collaborator's library: %+v", *p)
					return
				}
			}
		}
		if time.Since(start) > 20*time.Second {
			record("collab-library-collaborator", n)
			t.Errorf("the collaborator's library doesn't list it after 20 s")
			return
		}
	}
}

// editAsCollaborator finds out what a collaborator may do, as the account
// of c: whether YouTube Music offers the playlist id to add a song to, and
// whether it adds one, moves it first, and takes out a song of the
// owner's, then its own. It keeps each answer.
func editAsCollaborator(t *testing.T, ctx context.Context, c *Client, id string,
	record func(string, node), page func(*Client, string, func(media.Playlist) bool) media.Playlist) {
	t.Helper()
	const song = "GNCtaWIhdvM" // the second song of testdata/playlist.json
	if n, err := c.call(ctx, "playlist/get_add_to_playlist", map[string]any{"videoIds": []string{song}}, nil); err != nil {
		t.Errorf("the playlists to add to: %v", err)
	} else {
		record("collab-addto-collaborator", n)
		data, _ := json.Marshal(n.v)
		t.Logf("the playlists to add to hold it: %v", strings.Contains(string(data), id))
	}
	for name, of := range map[string]string{"collab-addto-liked": "LM", "collab-addto-itself": id} {
		n, err := c.call(ctx, "playlist/get_add_to_playlist", map[string]any{"playlistId": of}, nil)
		record(name, n)
		data, _ := json.Marshal(n.v)
		t.Logf("%s: holds it %v, err %v", name, strings.Contains(string(data), id), err)
	}
	edit := func(name string, action map[string]any) error {
		t.Helper()
		n, err := c.editPlaylistAnswer(ctx, id, action, errors.New("refused"), errNotYours)
		record(name, n)
		t.Logf("%s: %v", name, err)
		return err
	}
	if err := edit("collab-add-collaborator", map[string]any{"action": "ACTION_ADD_VIDEO", "addedVideoId": song}); err != nil {
		t.Errorf("a collaborator couldn't add a song: %v", err)
		return
	}
	p := page(c, "collab-page-collaborator-added", func(p media.Playlist) bool { return len(p.Tracks) == 2 })
	// The song added goes last. YouTube Music may list it, and the owner's,
	// by another video of theirs.
	for _, tr := range p.Tracks {
		t.Logf("track %s, entry %q", tr.ID, tr.EntryID)
	}
	owners, own := p.Tracks[0], p.Tracks[1]
	_, ownersSong, _ := media.ParseID(owners.ID)
	_, ownSong, _ := media.ParseID(own.ID)
	edit("collab-move-collaborator", map[string]any{"action": "ACTION_MOVE_VIDEO_BEFORE", "setVideoId": own.EntryID, "movedSetVideoIdSuccessor": owners.EntryID})
	edit("collab-move-owners", map[string]any{"action": "ACTION_MOVE_VIDEO_BEFORE", "setVideoId": owners.EntryID})
	edit("collab-remove-owners", map[string]any{"action": "ACTION_REMOVE_VIDEO", "setVideoId": owners.EntryID, "removedVideoId": ownersSong})
	edit("collab-remove-own", map[string]any{"action": "ACTION_REMOVE_VIDEO", "setVideoId": own.EntryID, "removedVideoId": ownSong})
}

// signedInClient is a client signed in with the cookies of header, the
// Cookie header of a signed-in music.youtube.com tab.
func signedInClient(t *testing.T, header string) *Client {
	t.Helper()
	cookies, err := http.ParseCookie(header)
	if err != nil {
		t.Fatal(err)
	}
	c := New()
	c.SetCookies(cookies)
	return c
}

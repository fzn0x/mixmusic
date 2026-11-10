package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
)

// The tracks and playlists of the tests of collaboration, all made up.
var (
	laneTracks = []media.Track{
		testTrack("yt:lane1", "Paper Lanterns", "Ana Testwood", "Small Hours", media.SourceYouTube),
		testTrack("yt:lane2", "Copper Rain", "Bo Fieldfare", "Weathervane", media.SourceYouTube),
	}
	bandTracks = []media.Track{
		testTrack("sp:band1", "Low Tide Waltz", "Cy Marlow", "Harbour Lights", media.SourceSpotify),
		testTrack("sp:band2", "Glass Orchard", "Dee Ashby", "Glass Orchard", media.SourceSpotify),
		testTrack("sp:band3", "Hollow Bells", "Ana Testwood", "Small Hours", media.SourceSpotify),
		testTrack("sp:band4", "Northern Kites", "Bo Fieldfare", "Weathervane", media.SourceSpotify),
	}
	// lanternList is a playlist of the user's on YouTube Music, private.
	lanternList = media.Playlist{ID: "yt:PLlanterns", Source: media.SourceYouTube, Title: "Lantern songs", Owner: "Eve Example",
		Editable: true, Deletable: true, Mine: true, Privacy: media.Private, Tracks: laneTracks, TrackCount: 2,
		URL: "https://music.youtube.com/playlist?list=PLlanterns"}
	// roadList is a playlist of the user's on Spotify, on their profile.
	roadList = media.Playlist{ID: "sp:road", Source: media.SourceSpotify, Title: "Road trip", Owner: "Eve Example",
		Editable: true, Mine: true, Privacy: media.Public, Tracks: bandTracks[:2], TrackCount: 2,
		URL: "https://open.spotify.com/playlist/road"}
	// bandPractice is a playlist of another account's on Spotify that the
	// user collaborates on.
	bandPractice = media.Playlist{ID: "sp:band", Source: media.SourceSpotify, Title: "Band practice", Owner: "Ana Testwood",
		Editable: true, Collaborative: true, Privacy: media.Private, Tracks: bandTracks, TrackCount: 4,
		URL: "https://open.spotify.com/playlist/band"}
	// followedList is a playlist of another account's on Spotify that the
	// user follows.
	followedList = media.Playlist{ID: "sp:followed", Source: media.SourceSpotify, Title: "Morning static", Owner: "Bo Fieldfare",
		Privacy: media.Public, Tracks: bandTracks[2:], TrackCount: 2}
)

// inviteLink is the invite link the fake YouTube Music gives.
const inviteLink = "https://music.youtube.com/playlist?list=PLlanterns&jct=testToken"

// collabAsks records what the window asks the services of collaboration,
// which answer with err, once gate, if set, lets them.
type collabAsks struct {
	mu   sync.Mutex
	asks []string
	err  error
	gate chan struct{}
}

func (a *collabAsks) add(ask string) error {
	a.mu.Lock()
	gate := a.gate
	a.mu.Unlock()
	if gate != nil {
		<-gate
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.asks = append(a.asks, ask)
	return a.err
}

// hold keeps the services from answering until release: no frame then
// shows a toast, and takes it from the notes, before heard reads it.
func (a *collabAsks) hold() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.gate = make(chan struct{})
}

func (a *collabAsks) release() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.gate != nil {
		close(a.gate)
		a.gate = nil
	}
}

func (a *collabAsks) asked() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.asks)
}

// fail has the services refuse from now on with err.
func (a *collabAsks) fail(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.err = err
}

// answerCollab answers for the services what the window asks of them
// for collaborating, and records it.
func answerCollab(t *testing.T) *collabAsks {
	t.Helper()
	a := &collabAsks{}
	invite, stop, collaborative, move, add, share := inviteOnYouTube, stopOnYouTube, collaborativeOnSpotify, moveOnSpotify, addOnSpotify, shareOnYouTube
	t.Cleanup(func() {
		inviteOnYouTube, stopOnYouTube, collaborativeOnSpotify, moveOnSpotify, addOnSpotify, shareOnYouTube = invite, stop, collaborative, move, add, share
	})
	inviteOnYouTube = func(_ *shell, _ context.Context, id string, unlist bool) (string, error) {
		if err := a.add(fmt.Sprintf("invite %s unlist %v", id, unlist)); err != nil {
			return "", err
		}
		return inviteLink, nil
	}
	stopOnYouTube = func(_ *shell, _ context.Context, id string) error { return a.add("stop " + id) }
	collaborativeOnSpotify = func(_ *shell, _ context.Context, id string, on bool) error {
		return a.add(fmt.Sprintf("collaborative %s %v", id, on))
	}
	moveOnSpotify = func(_ *shell, _ context.Context, id string, from, to int) error {
		return a.add(fmt.Sprintf("move %s %d %d", id, from, to))
	}
	addOnSpotify = func(_ *shell, _ context.Context, id string, tracks []media.Track) error {
		var titles []string
		for _, tr := range tracks {
			titles = append(titles, tr.Title)
		}
		return a.add("add " + id + " " + strings.Join(titles, ", "))
	}
	shareOnYouTube = func(_ *shell, _ context.Context, title string, tracks []media.Track) (media.Playlist, error) {
		if err := a.add(fmt.Sprintf("share %s %d", title, len(tracks))); err != nil {
			return media.Playlist{}, err
		}
		return media.Playlist{ID: "yt:PLshared", Source: media.SourceYouTube, Title: title, Editable: true, Deletable: true, Mine: true,
			Privacy: media.Unlisted, URL: "https://music.youtube.com/playlist?list=PLshared"}, nil
	}
	return a
}

// copies records what the window copies.
func copies(t *testing.T) func() []string {
	t.Helper()
	var (
		mu     sync.Mutex
		copied []string
	)
	was := copyText
	copyText = func(text string) {
		mu.Lock()
		defer mu.Unlock()
		copied = append(copied, text)
	}
	t.Cleanup(func() { copyText = was })
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(copied)
	}
}

// signedInTo is the accounts' state of a user signed in to the services
// named.
func signedInTo(youtube, spotify bool) *loaded[app.AccountsStatus] {
	return &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{
		YouTube: app.YouTubeAccount{SignedIn: youtube}, Spotify: app.SpotifyAccount{SignedIn: spotify}}}
}

// collabShell is a window signed in as accounts says, whose Spotify
// library holds playlists, and which shows the page of p.
func collabShell(t *testing.T, accounts *loaded[app.AccountsStatus], p media.Playlist, playlists ...media.Playlist) (*shell, *ui.Tester) {
	t.Helper()
	s := newTestShell(t)
	s.loads["accounts"] = accounts
	s.loads["spotify:playlists"] = &loaded[[]media.Playlist]{done: true, value: playlists}
	s.loads["playlist:"+p.ID] = &loaded[media.Playlist]{done: true, value: p}
	tt := tester(t, s)
	show(s, tt, playlistPath(p.ID))
	until(t, s, tt, "the page of "+p.Title, func() bool { return tt.HasText(p.Tracks[0].Title) })
	return s, tt
}

// pageMenu returns the items of the More menu of the page shown.
func pageMenu(t *testing.T, tt *ui.Tester) []string {
	t.Helper()
	if err := tt.Click("More"); err != nil {
		t.Fatal(err)
	}
	defer tt.CloseMenu()
	return tt.Menu()
}

// choosePage chooses the item of the More menu of the page shown.
func choosePage(t *testing.T, s *shell, tt *ui.Tester, item string) {
	t.Helper()
	if err := tt.Click("More"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(item); err != nil {
		t.Fatalf("%v: the menu is %q", err, tt.Menu())
	}
	s.drain()
	tt.Frame()
}

// shownPlaylist is the playlist the page of id shows.
func shownPlaylist(s *shell, id string) media.Playlist {
	return s.loads["playlist:"+id].(*loaded[media.Playlist]).value
}

// heard waits for a toast that says message, an error's if isError is
// set, without a frame, which would show it and take it from the notes.
func heard(t *testing.T, s *shell, message string, isError bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !told(s, message, isError); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("no toast says %q: %+v", message, s.notes)
		}
		s.drain()
	}
}

// textHolds reports whether a text of the window holds part.
func textHolds(tt *ui.Tester, part string) bool {
	return slices.ContainsFunc(tt.Texts(), func(text string) bool { return strings.Contains(text, part) })
}

// findFold finds the text of the window that is label in whatever case
// the layout writes it.
func findFold(tt *ui.Tester, label string) (ui.Rect, bool) {
	for _, text := range tt.Texts() {
		if strings.EqualFold(text, label) {
			return tt.Find(text)
		}
	}
	return ui.Rect{}, false
}

// TestInviteCollaborators invites collaborators to a playlist of the
// user's on YouTube Music, asking first to make a private one unlisted,
// and copies the invite link; then Stop collaboration turns it off. The
// page tells the playlist is collaborative meanwhile.
func TestInviteCollaborators(t *testing.T) {
	asks := answerCollab(t)
	copied := copies(t)
	s, tt := collabShell(t, signedInTo(true, false), lanternList)
	if items := pageMenu(t, tt); !slices.Contains(items, "Invite collaborators") || slices.Contains(items, "Stop collaboration") {
		t.Fatalf("the menu of a playlist of the user's is %q", items)
	}
	choosePage(t, s, tt, "Invite collaborators")
	if q := s.asking; q == nil || q.ok != "invite" || !strings.Contains(q.message, "unlisted") || !strings.Contains(q.message, lanternList.Title) {
		t.Fatalf("inviting to a private playlist asks %+v", q)
	}
	if len(asks.asked()) != 0 {
		t.Fatalf("asked YouTube before the user agreed: %q", asks.asked())
	}
	asks.hold()
	click(t, s, tt, "invite")
	asks.release()
	const invited = "Copied the invite link of Lantern songs: whoever opens it can add to it"
	heard(t, s, invited, false)
	if got := asks.asked(); !slices.Equal(got, []string{"invite yt:PLlanterns unlist true"}) {
		t.Errorf("YouTube was asked %q", got)
	}
	if got := copied(); !slices.Equal(got, []string{inviteLink}) {
		t.Errorf("copied %q", got)
	}
	if p := shownPlaylist(s, lanternList.ID); !p.Collaborative || p.Privacy != media.Unlisted {
		t.Errorf("the page shows collaborative %v, privacy %q", p.Collaborative, p.Privacy)
	}
	tt.Frame()
	if !textHolds(tt, " · collaborative · ") {
		t.Errorf("the head doesn't say it's collaborative: %q", tt.Texts())
	}

	// Unlisted now, it invites again without asking.
	choosePage(t, s, tt, "Invite collaborators")
	if s.asking != nil {
		t.Fatalf("inviting to an unlisted playlist asks %+v", s.asking)
	}
	until(t, s, tt, "the second invite", func() bool { return len(asks.asked()) == 2 })
	if got := asks.asked()[1]; got != "invite yt:PLlanterns unlist false" {
		t.Errorf("YouTube was asked %q", got)
	}

	asks.hold()
	choosePage(t, s, tt, "Stop collaboration")
	asks.release()
	heard(t, s, "Stopped collaboration on Lantern songs", false)
	if got := asks.asked(); got[len(got)-1] != "stop yt:PLlanterns" {
		t.Errorf("YouTube was asked %q", got)
	}
	if items := pageMenu(t, tt); slices.Contains(items, "Stop collaboration") || shownPlaylist(s, lanternList.ID).Collaborative {
		t.Errorf("stopped, the menu is %q", items)
	}

	// Refused, it says why, and the playlist stays as it was.
	asks.fail(errors.New("YouTube Music refused to turn collaboration on"))
	asks.hold()
	choosePage(t, s, tt, "Invite collaborators")
	asks.release()
	const refused = "Couldn't invite collaborators to Lantern songs: YouTube Music refused to turn collaboration on"
	heard(t, s, refused, true)
	if shownPlaylist(s, lanternList.ID).Collaborative {
		t.Error("refused, the page shows the playlist collaborative")
	}
}

// TestCollaborationItems offers to collaborate only on a playlist the user
// made, while they're signed in to its service.
func TestCollaborationItems(t *testing.T) {
	answerCollab(t)
	collaborating := lanternList
	collaborating.Mine, collaborating.Deletable, collaborating.Collaborative = false, false, true
	for _, tc := range []struct {
		name     string
		accounts *loaded[app.AccountsStatus]
		p        media.Playlist
		has      []string
	}{
		{"the user's, on YouTube Music", signedInTo(true, false), lanternList, []string{"Invite collaborators"}},
		{"the user's, signed out", signedInTo(false, true), lanternList, nil},
		{"another's they collaborate on", signedInTo(true, false), collaborating, nil},
		{"the user's, on Spotify", signedInTo(false, true), roadList, []string{"Make collaborative"}},
		{"another's on Spotify", signedInTo(false, true), bandPractice, nil},
	} {
		_, tt := collabShell(t, tc.accounts, tc.p)
		items := pageMenu(t, tt)
		for _, item := range []string{"Invite collaborators", "Stop collaboration", "Make collaborative"} {
			if slices.Contains(items, item) != slices.Contains(tc.has, item) {
				t.Errorf("%s: the menu is %q", tc.name, items)
			}
		}
	}
}

// TestMakeCollaborative makes a playlist of the user's on Spotify
// collaborative, asking first for one on their profile, which it leaves,
// and copies its link; Stop collaboration makes it theirs alone again.
func TestMakeCollaborative(t *testing.T) {
	asks := answerCollab(t)
	copied := copies(t)
	s, tt := collabShell(t, signedInTo(false, true), roadList, roadList, bandPractice)
	choosePage(t, s, tt, "Make collaborative")
	if q := s.asking; q == nil || q.ok != "make collaborative" || !strings.Contains(q.message, "profile") {
		t.Fatalf("making a public playlist collaborative asks %+v", q)
	}
	asks.hold()
	click(t, s, tt, "make collaborative")
	asks.release()
	const made = "Made Road trip collaborative: copied its link, for those who'll add to it"
	heard(t, s, made, false)
	if got := asks.asked(); !slices.Equal(got, []string{"collaborative sp:road true"}) {
		t.Errorf("Spotify was asked %q", got)
	}
	if got := copied(); !slices.Equal(got, []string{roadList.URL}) {
		t.Errorf("copied %q", got)
	}
	if p := shownPlaylist(s, roadList.ID); !p.Collaborative || p.Privacy != media.Private {
		t.Errorf("the page shows collaborative %v, privacy %q", p.Collaborative, p.Privacy)
	}
	if l := s.loads["spotify:playlists"].(*loaded[[]media.Playlist]); !l.again {
		t.Error("the library doesn't read its playlists again")
	}

	asks.hold()
	choosePage(t, s, tt, "Stop collaboration")
	if s.asking != nil {
		t.Fatalf("stopping asks %+v", s.asking)
	}
	asks.release()
	heard(t, s, "Stopped collaboration on Road trip", false)
	if got := asks.asked(); got[len(got)-1] != "collaborative sp:road false" || shownPlaylist(s, roadList.ID).Collaborative {
		t.Errorf("Spotify was asked %q", got)
	}

	// Off their profile already, it doesn't ask.
	private := roadList
	private.Privacy = media.Private
	s, tt = collabShell(t, signedInTo(false, true), private)
	asks.hold()
	choosePage(t, s, tt, "Make collaborative")
	if s.asking != nil {
		t.Fatalf("a playlist off the profile asks %+v", s.asking)
	}
	asks.release()
	heard(t, s, made, false)
}

// TestCollaborationsShelf lists the Spotify playlists of others the user
// collaborates on in a shelf of their own, with who made them, and the
// others in Playlists.
func TestCollaborationsShelf(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = signedInTo(false, true)
	s.loads["spotify:playlists"] = &loaded[[]media.Playlist]{done: true, value: []media.Playlist{roadList, bandPractice, followedList}}
	s.loads["spotify:albums"] = &loaded[[]media.Album]{done: true}
	s.loads["spotify:artists"] = &loaded[[]media.Artist]{done: true}
	s.loads["spotify:liked"] = &loaded[app.TrackPage]{done: true}
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/spotify")
	shelf, ok := findFold(tt, "Playlists you collaborate on")
	if !ok {
		t.Fatalf("no shelf of collaborations: %q", tt.Texts())
	}
	for title, below := range map[string]bool{bandPractice.Title: true, roadList.Title: false, followedList.Title: false} {
		if r, ok := tt.Find(title); !ok || (r.Y > shelf.Y) != below {
			t.Errorf("%s shows at %v (%v), the shelf of collaborations at %v", title, r, ok, shelf)
		}
	}
	if !tt.HasText("Ana Testwood · 4 tracks") {
		t.Errorf("the tile doesn't say who made it: %q", tt.Texts())
	}

	// Without collaborations, no shelf.
	s.loads["spotify:playlists"] = &loaded[[]media.Playlist]{done: true, value: []media.Playlist{roadList, followedList}}
	tt.Frame()
	if _, ok := findFold(tt, "Playlists you collaborate on"); ok || !tt.HasText(roadList.Title) {
		t.Errorf("without collaborations, the library shows %q", tt.Texts())
	}
}

// TestAddToSpotifyPlaylist adds a Spotify track to a playlist on Spotify
// the user may change, theirs or one they collaborate on, which tells
// whose it is, from the track's menu; not to one they only follow.
func TestAddToSpotifyPlaylist(t *testing.T) {
	asks := answerCollab(t)
	s, tt := collabShell(t, signedInTo(false, true), followedList, roadList, bandPractice, followedList)
	track := followedList.Tracks[0]
	choose := func(item string) error {
		t.Helper()
		if err := tt.RightClick(track.Title); err != nil {
			t.Fatal(err)
		}
		defer tt.CloseMenu()
		return tt.ChooseMenuItem("Add to Spotify playlist", item)
	}
	if err := choose(followedList.Title); err == nil {
		t.Error("offered a playlist the user only follows")
	}
	asks.hold()
	if err := choose("Band practice · Ana Testwood"); err != nil {
		t.Fatal(err)
	}
	asks.release()
	heard(t, s, "Added Hollow Bells to Band practice", false)
	if err := choose(roadList.Title); err != nil {
		t.Fatal(err)
	}
	until(t, s, tt, "the second add", func() bool { return len(asks.asked()) == 2 })
	if got := asks.asked(); !slices.Equal(got, []string{"add sp:band Hollow Bells", "add sp:road Hollow Bells"}) {
		t.Errorf("Spotify was asked %q", got)
	}

	// A track of YouTube Music has no such item; nor signed out.
	s, tt = collabShell(t, signedInTo(true, true), lanternList, roadList)
	if slices.Contains(menuOf(t, tt, laneTracks[0].Title), "Add to Spotify playlist") {
		t.Error("a YouTube Music track offers to go to a Spotify playlist")
	}
	_, tt = collabShell(t, signedInTo(false, false), followedList, roadList)
	if slices.Contains(menuOf(t, tt, track.Title), "Add to Spotify playlist") {
		t.Error("signed out, a Spotify track offers to go to a Spotify playlist")
	}
}

// TestDragInSpotifyPlaylist moves a track of a Spotify playlist the user
// collaborates on there too, by the places Spotify counts; until Spotify
// answers, nothing else moves. A playlist some of whose items the page
// doesn't show doesn't move.
func TestDragInSpotifyPlaylist(t *testing.T) {
	asks := answerCollab(t)
	asks.hold()
	s, tt := collabShell(t, signedInTo(false, true), bandPractice)
	order := func() []string { return titlesShown(t, tt, bandTracks) }
	dragTrack(t, s, tt, bandTracks[3].Title, bandTracks[1].Title)
	want := []string{"Low Tide Waltz", "Northern Kites", "Glass Orchard", "Hollow Bells"}
	if got := order(); !slices.Equal(got, want) {
		t.Errorf("the page shows %q, want %q", got, want)
	}
	// While Spotify answers, a drag moves nothing.
	dragTrack(t, s, tt, bandTracks[0].Title, bandTracks[2].Title)
	if got := order(); !slices.Equal(got, want) {
		t.Errorf("a drag while Spotify answers: the page shows %q", got)
	}
	asks.release()
	until(t, s, tt, "Spotify's answer", func() bool { return !s.keeping[collabKey(bandPractice.ID)] })
	if got := asks.asked(); !slices.Equal(got, []string{"move sp:band 3 1"}) {
		t.Errorf("Spotify was asked %q", got)
	}
	dragTrack(t, s, tt, bandTracks[0].Title, bandTracks[2].Title)
	until(t, s, tt, "the second move", func() bool { return len(asks.asked()) == 2 })
	if got := asks.asked()[1]; got != "move sp:band 0 3" {
		t.Errorf("Spotify was asked %q", got)
	}

	// Refused, it says why, and reads the playlist again.
	asks.fail(errors.New("spotify: Spotify refused to change the playlist"))
	asks.hold()
	dragTrack(t, s, tt, bandTracks[1].Title, bandTracks[2].Title)
	asks.release()
	heard(t, s, "spotify: Spotify refused to change the playlist", true)
	if l := s.loads["playlist:"+bandPractice.ID].(*loaded[media.Playlist]); !l.again {
		t.Error("the page doesn't read the playlist again")
	}

	// One item the page doesn't show, and the places differ.
	asks = answerCollab(t)
	partly := bandPractice
	partly.TrackCount = 5
	s, tt = collabShell(t, signedInTo(false, true), partly)
	dragTrack(t, s, tt, bandTracks[3].Title, bandTracks[1].Title)
	if got := asks.asked(); len(got) != 0 || !slices.Equal(titlesShown(t, tt, bandTracks), collabTitles(bandTracks)) {
		t.Errorf("a playlist with an item not shown moved: asked %q, shows %q", got, titlesShown(t, tt, bandTracks))
	}
}

// collabTitles returns the titles of tracks.
func collabTitles(tracks []media.Track) []string {
	out := make([]string, len(tracks))
	for i, tr := range tracks {
		out[i] = tr.Title
	}
	return out
}

// TestShareMixOnYouTube makes a mix an unlisted playlist on YouTube Music
// from its Share menu, named as the user says, copies its link and opens
// its page; signed out of YouTube Music, Share doesn't offer it.
func TestShareMixOnYouTube(t *testing.T) {
	asks := answerCollab(t)
	copied := copies(t)
	s := newTestShell(t)
	s.loads["accounts"] = signedInTo(true, false)
	mix, err := s.svc.Collection.CreatePlaylist("Night walk", "", laneTracks)
	if err != nil {
		t.Fatal(err)
	}
	// The page of the playlist made, which the window opens.
	s.loads["playlist:yt:PLshared"] = &loaded[media.Playlist]{done: true, value: media.Playlist{ID: "yt:PLshared", Source: media.SourceYouTube,
		Title: "Night walk", Tracks: laneTracks, TrackCount: 2, Editable: true, Deletable: true, Mine: true, Privacy: media.Unlisted}}
	tt := tester(t, s)
	show(s, tt, playlistPath(mix.ID))
	until(t, s, tt, "the mix's page", func() bool { return tt.HasText(laneTracks[1].Title) })
	if err := tt.Click("More"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Share", "As a YouTube Music playlist…"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.naming == nil || s.naming.value != "Night walk" || s.naming.confirm != "share" {
		t.Fatalf("sharing asks %+v", s.naming)
	}
	asks.hold()
	click(t, s, tt, "share")
	asks.release()
	const shared = "Made Night walk on YouTube Music: copied its link, which anyone can open"
	heard(t, s, shared, false)
	if got := asks.asked(); !slices.Equal(got, []string{"share Night walk 2"}) {
		t.Errorf("YouTube was asked %q", got)
	}
	if got := copied(); !slices.Equal(got, []string{"https://music.youtube.com/playlist?list=PLshared"}) {
		t.Errorf("copied %q", got)
	}
	tt.Frame()
	if got := s.router.Path(); got != playlistPath("yt:PLshared") {
		t.Errorf("shared, the window shows %q", got)
	}

	// Refused, it says why.
	asks.fail(errors.New("none of these tracks is on YouTube Music"))
	s.shareMixOnYouTube("Night walk", laneTracks)
	heard(t, s, "Couldn't share Night walk: none of these tracks is on YouTube Music", true)

	// Signed out of YouTube Music, Share has no such item.
	s.loads["accounts"] = signedInTo(false, false)
	show(s, tt, playlistPath(mix.ID))
	if err := tt.Click("More"); err != nil {
		t.Fatal(err)
	}
	defer tt.CloseMenu()
	if err := tt.ChooseMenuItem("Share", "As a YouTube Music playlist…"); err == nil {
		t.Error("signed out, Share offers YouTube Music")
	}
}

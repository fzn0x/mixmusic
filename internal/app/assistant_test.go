package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/mcp"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// fakeCatalog is the catalog of the assistant's tools in tests: what it
// finds, by ID, and the user's playlists and liked songs.
type fakeCatalog struct {
	found     SearchResponse
	albums    map[string]media.Album
	artists   map[string]media.Artist
	playlists map[string]media.Playlist
	mine      []media.Playlist
	mineErrs  []error
	liked     []media.Track
}

func (f *fakeCatalog) Search(context.Context, string) (SearchResponse, error) { return f.found, nil }

func (f *fakeCatalog) Album(_ context.Context, id string) (media.Album, error) {
	if al, ok := f.albums[id]; ok {
		return al, nil
	}
	return media.Album{}, errors.New("no such album")
}

func (f *fakeCatalog) Artist(_ context.Context, id string) (media.Artist, error) {
	if ar, ok := f.artists[id]; ok {
		return ar, nil
	}
	return media.Artist{}, errors.New("no such artist")
}

func (f *fakeCatalog) Playlist(_ context.Context, id string) (media.Playlist, error) {
	if pl, ok := f.playlists[id]; ok {
		return pl, nil
	}
	return media.Playlist{}, errors.New("no such playlist")
}

func (f *fakeCatalog) Playlists(context.Context) ([]media.Playlist, []error) {
	return f.mine, f.mineErrs
}

func (f *fakeCatalog) Liked() []media.Track { return f.liked }

// song is a made-up track of YouTube Music.
func song(id, title, artist string) media.Track {
	return media.Track{ID: "yt:" + id, Source: media.SourceYouTube, Title: title, Artists: []media.Ref{{Name: artist}},
		Album: &media.Ref{Name: "Album of " + id}, DurationMs: 200000}
}

// newAssistantApp is an app with a quiet player, whose radio never asks
// the network, and the catalog cat, with AI assistants on.
func newAssistantApp(t *testing.T, cat assistantCatalog) (*App, *toasts) {
	t.Helper()
	a := newQuietApp(t, nil)
	a.assistants.catalog = cat
	// The tests call faster than an assistant may.
	a.assistantServer().Every = time.Microsecond
	n := &toasts{}
	t.Cleanup(a.GoPlayer().Notices(func(note player.Notice) {
		n.mu.Lock()
		n.told = append(n.told, note.Message)
		n.mu.Unlock()
	}))
	if _, err := (&Preferences{a}).Update(func(s *settings.Settings) {
		s.Autoplay = false
		s.AssistantControl = true
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.stopAssistants)
	return a, n
}

// toasts keeps what the player told the windows.
type toasts struct {
	mu   sync.Mutex
	told []string
}

// wait waits for a toast that holds what.
func (n *toasts) wait(t *testing.T, what string) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
		n.mu.Lock()
		told := slices.Clone(n.told)
		n.mu.Unlock()
		if slices.ContainsFunc(told, func(s string) bool { return strings.Contains(s, what) }) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("no toast tells %q: %q", what, told)
		}
	}
}

// assistant is an AI assistant connected to an app through the command
// it starts.
type assistant struct {
	t     *testing.T
	in    *io.PipeWriter
	lines chan string
	done  chan error
	id    int
}

func connectAssistant(t *testing.T, a *App) *assistant {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	c := &assistant{t: t, in: inW, lines: make(chan string, 100), done: make(chan error, 1)}
	go func() {
		err := mcp.Relay(a.opts.DataDir, inR, outW)
		outW.Close()
		c.done <- err
	}()
	go func() {
		sc := bufio.NewScanner(outR)
		sc.Buffer(nil, 1<<20)
		for sc.Scan() {
			c.lines <- sc.Text()
		}
		close(c.lines)
	}()
	t.Cleanup(func() { inW.Close() })
	var init struct {
		Result struct {
			ServerInfo struct{ Name string } `json:"serverInfo"`
		} `json:"result"`
	}
	c.request("initialize", map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "test", "title": "Test Assistant", "version": "1"}}, &init)
	if init.Result.ServerInfo.Name != "mixmusic" {
		t.Fatalf("initialize: %+v", init)
	}
	return c
}

// request sends a request, and reads its answer into v.
func (c *assistant) request(method string, params any, v any) {
	c.t.Helper()
	c.id++
	data, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": c.id, "method": method, "params": params})
	if _, err := c.in.Write(append(data, '\n')); err != nil {
		c.t.Fatal(err)
	}
	select {
	case line, ok := <-c.lines:
		if !ok {
			c.t.Fatalf("%s: the session ended", method)
		}
		if err := json.Unmarshal([]byte(line), v); err != nil {
			c.t.Fatalf("%s: %q: %v", method, line, err)
		}
	case <-time.After(10 * time.Second):
		c.t.Fatalf("%s: no answer", method)
	}
}

// call calls a tool, and returns its text, and whether it failed.
func (c *assistant) call(tool string, args map[string]any) (string, bool) {
	c.t.Helper()
	var r struct {
		Result struct {
			Content []struct{ Text string } `json:"content"`
			IsError bool                    `json:"isError"`
		} `json:"result"`
		Error *struct{ Message string } `json:"error"`
	}
	c.request("tools/call", map[string]any{"name": tool, "arguments": args}, &r)
	if r.Error != nil || len(r.Result.Content) != 1 {
		c.t.Fatalf("%s: %+v", tool, r)
	}
	return r.Result.Content[0].Text, r.Result.IsError
}

// ok calls a tool that should do what it's asked, and returns its text.
func (c *assistant) ok(tool string, args map[string]any) string {
	c.t.Helper()
	text, failed := c.call(tool, args)
	if failed {
		c.t.Fatalf("%s %v failed: %s", tool, args, text)
	}
	return text
}

// fails calls a tool that should refuse, and returns why.
func (c *assistant) fails(tool string, args map[string]any) string {
	c.t.Helper()
	text, failed := c.call(tool, args)
	if !failed {
		c.t.Fatalf("%s %v did it: %s", tool, args, text)
	}
	return text
}

// TestAssistantSwitch lets assistants in only while the settings do: the
// file that tells the command how to reach MixMusic is there, the status
// tells the command and the assistants connected, and turning it off
// ends every session at once.
func TestAssistantSwitch(t *testing.T) {
	a := newQuietApp(t, nil)
	sys := &System{a}
	reach := filepath.Join(a.opts.DataDir, mcp.FileName)
	if st := sys.AssistantStatus(); st.On {
		t.Fatalf("a new install lets assistants in: %+v", st)
	}
	if _, err := os.Stat(reach); err == nil {
		t.Fatal("off, the file is there")
	}
	if err := mcp.Relay(a.opts.DataDir, strings.NewReader(""), io.Discard); !errors.Is(err, mcp.ErrUnreachable) {
		t.Fatalf("off, the command got %v", err)
	}
	prefs := &Preferences{a}
	if _, err := prefs.Update(func(s *settings.Settings) { s.AssistantControl = true }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.stopAssistants)
	st := sys.AssistantStatus()
	if !st.On || st.Problem != "" || !strings.HasSuffix(st.Command, " --mcp") {
		t.Fatalf("on: %+v", st)
	}
	var config struct {
		Servers map[string]struct {
			Command string
			Args    []string
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(st.Config), &config); err != nil {
		t.Fatalf("%v: %s", err, st.Config)
	}
	if s := config.Servers["mixmusic"]; s.Command == "" || !slices.Equal(s.Args, []string{"--mcp"}) {
		t.Errorf("the configuration %s", st.Config)
	}
	c := connectAssistant(t, a)
	if got := sys.AssistantStatus().Connected; !slices.Equal(got, []string{"Test Assistant"}) {
		t.Errorf("connected: %q", got)
	}
	if _, err := prefs.Update(func(s *settings.Settings) { s.AssistantControl = false }); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-c.done:
		if err != nil {
			t.Errorf("the command ended with %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the session goes on, turned off")
	}
	if _, err := os.Stat(reach); err == nil {
		t.Error("off, the file stays")
	}
	if st := sys.AssistantStatus(); st.On || len(st.Connected) > 0 {
		t.Errorf("off: %+v", st)
	}
}

// TestAssistantStaleFile removes, at start, the file of a run that
// didn't stop its server.
func TestAssistantStaleFile(t *testing.T) {
	dir := t.TempDir()
	data := filepath.Join(dir, "data")
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, mcp.FileName), []byte(`{"port":1,"key":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	newQuietApp(t, func(o *Options) { o.DataDir = data })
	if _, err := os.Stat(filepath.Join(data, mcp.FileName)); err == nil {
		t.Error("the file of the run before stays")
	}
}

// TestAssistantPlayback tells what plays and what comes next, and plays,
// pauses, skips, goes back, seeks and sets the volume, as remote control
// does; each shows a toast.
func TestAssistantPlayback(t *testing.T) {
	a, toasts := newAssistantApp(t, &fakeCatalog{})
	c := connectAssistant(t, a)
	if text := c.fails("play", nil); !strings.Contains(text, "nothing is in the queue") {
		t.Errorf("play with nothing: %s", text)
	}
	p := a.GoPlayer()
	tracks := []media.Track{song("a", "First Light", "Made Up"), song("b", "Second Wind", "Made Up"),
		song("c", "Third Rail", "Other Band"), song("d", "Fourth Wall", "Other Band")}
	p.PlayTracks(tracks, 0)
	waitPlayer(t, p, "a playing", func(s player.State) bool { return s.Playing })
	var now assistantNow
	if err := json.Unmarshal([]byte(c.ok("now_playing", nil)), &now); err != nil {
		t.Fatal(err)
	}
	if !now.Playing || now.Track == nil || now.Track.Title != "First Light" || now.Track.Service != "YouTube Music" ||
		now.DurationSeconds != 200 || now.Repeat != "off" {
		t.Errorf("now playing %+v", now)
	}
	var q assistantQueue
	if err := json.Unmarshal([]byte(c.ok("get_queue", nil)), &q); err != nil {
		t.Fatal(err)
	}
	if len(q.Next) != 3 || q.Next[0].Title != "Second Wind" || q.Next[2].Artists[0] != "Other Band" || q.More != 0 {
		t.Errorf("queue %+v", q)
	}

	c.ok("pause", nil)
	waitPlayer(t, p, "paused", func(s player.State) bool { return !s.Playing })
	toasts.wait(t, "An AI assistant paused “First Light” by Made Up")
	c.ok("play", nil)
	waitPlayer(t, p, "playing again", func(s player.State) bool { return s.Playing })
	toasts.wait(t, "played “First Light”")

	c.ok("next_track", nil)
	waitPlayer(t, p, "b", func(s player.State) bool { return s.Index == 1 })
	toasts.wait(t, "skipped to the next track")
	c.ok("next_track", map[string]any{"count": 2})
	waitPlayer(t, p, "d", func(s player.State) bool { return s.Index == 3 })
	toasts.wait(t, "skipped ahead to “Fourth Wall” by Other Band")
	if text := c.fails("next_track", map[string]any{"count": 5}); !strings.Contains(text, "only 0 tracks come next") {
		t.Errorf("past the end: %s", text)
	}
	c.ok("previous_track", nil)
	waitPlayer(t, p, "back to c", func(s player.State) bool { return s.Index == 2 })
	toasts.wait(t, "went back")

	waitPlayer(t, p, "c playing", func(s player.State) bool { return s.Playing && !s.Loading })
	c.ok("seek", map[string]any{"position_seconds": 30})
	waitPlayer(t, p, "at 0:30", func(s player.State) bool { return s.Position == 30*time.Second })
	toasts.wait(t, "moved “Third Rail” by Other Band to 0:30")
	c.ok("seek", map[string]any{"by_seconds": -10})
	waitPlayer(t, p, "at 0:20", func(s player.State) bool { return s.Position == 20*time.Second })
	c.ok("seek", map[string]any{"position_seconds": 9999})
	waitPlayer(t, p, "at the end", func(s player.State) bool { return s.Position == 200*time.Second || s.Index == 3 })
	for _, args := range []map[string]any{nil, {"position_seconds": 1, "by_seconds": 1}, {"position_seconds": "soon"}} {
		c.fails("seek", args)
	}

	c.ok("set_volume", map[string]any{"percent": 40})
	waitPlayer(t, p, "at 40%", func(s player.State) bool { return math.Abs(s.Volume-0.4) < 1e-9 })
	toasts.wait(t, "set the volume to 40%")
	c.ok("set_volume", map[string]any{"by_percent": -50})
	waitPlayer(t, p, "silent", func(s player.State) bool { return s.Volume == 0 })
	for _, args := range []map[string]any{nil, {"percent": 10, "by_percent": 1}, {"loud": true}} {
		c.fails("set_volume", args)
	}
}

// TestAssistantSearchAndPlay plays, puts next and adds to the queue what
// search found, by its id, and leaves out what's explicit while the
// settings hide it.
func TestAssistantSearchAndPlay(t *testing.T) {
	explicit := song("x", "Rude Words", "Made Up")
	explicit.Explicit = true
	album := media.Album{ID: "yt:MPREb1", Source: media.SourceYouTube, Title: "Made Up Album",
		Artists: []media.Ref{{Name: "Made Up"}}, Year: "2024", TrackCount: 2,
		Tracks: []media.Track{song("al1", "Album One", "Made Up"), song("al2", "Album Two", "Made Up")}}
	cat := &fakeCatalog{
		found: SearchResponse{
			YouTube: media.SearchResults{Tracks: []media.Track{song("s1", "Searched Song", "Made Up"), explicit},
				Albums: []media.Album{album}, Artists: []media.Artist{{ID: "yt:UC1", Source: media.SourceYouTube, Name: "Made Up"}}},
			Spotify: media.SearchResults{Tracks: []media.Track{{ID: "sp:t1", Source: media.SourceSpotify, Title: "Spotify Song",
				Artists: []media.Ref{{Name: "Someone"}}}}},
		},
		albums:  map[string]media.Album{album.ID: album},
		artists: map[string]media.Artist{"yt:UC1": {ID: "yt:UC1", Name: "Made Up", TopTracks: []media.Track{song("top", "Top Song", "Made Up")}}},
	}
	a, toasts := newAssistantApp(t, cat)
	c := connectAssistant(t, a)
	p := a.GoPlayer()
	var found assistantSearch
	if err := json.Unmarshal([]byte(c.ok("search", map[string]any{"query": "made up"})), &found); err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, r := range found.Results {
		ids[r.Title] = r.ID
	}
	if len(found.Results) != 5 || ids["Searched Song"] != "yt:s1" || ids["Spotify Song"] != "sp:t1" ||
		ids["Made Up Album"] != album.ID || ids["Made Up"] != "yt:UC1" {
		t.Fatalf("found %+v", found)
	}
	if found.Results[1].Title != "Spotify Song" {
		t.Errorf("the services' songs don't take turns: %+v", found.Results)
	}

	c.ok("play_result", map[string]any{"id": "yt:s1"})
	waitPlayer(t, p, "the song", func(s player.State) bool { return s.Playing && slices.Equal(queueIDs(s), []string{"yt:s1"}) })
	toasts.wait(t, "An AI assistant played “Searched Song” by Made Up")
	c.ok("play_result", map[string]any{"id": "sp:t1", "when": "last"})
	c.ok("play_result", map[string]any{"id": album.ID, "when": "next"})
	waitPlayer(t, p, "the album next, the song last", func(s player.State) bool {
		return slices.Equal(queueIDs(s), []string{"yt:s1", "yt:al1", "yt:al2", "sp:t1"})
	})
	toasts.wait(t, "put the album “Made Up Album” next")
	toasts.wait(t, "added “Spotify Song” by Someone to the queue")
	c.ok("play_result", map[string]any{"id": "yt:UC1"})
	waitPlayer(t, p, "the artist's top tracks", func(s player.State) bool { return slices.Equal(queueIDs(s), []string{"yt:top"}) })
	toasts.wait(t, "played the top tracks of Made Up")

	if text := c.fails("play_result", map[string]any{"id": "yt:nope"}); !strings.Contains(text, "search") {
		t.Errorf("an unknown id: %s", text)
	}
	c.fails("play_result", map[string]any{"id": "yt:s1", "when": "later"})
	c.fails("play_result", map[string]any{"id": "yt:s1", "when": "next", "shuffle": true})
	c.fails("search", map[string]any{"query": " "})
	c.fails("search", map[string]any{"query": "a", "kind": "podcast"})
	c.fails("search", map[string]any{"query": "a", "limit": 50})

	if _, err := (&Preferences{a}).Update(func(s *settings.Settings) { s.HideExplicit = true }); err != nil {
		t.Fatal(err)
	}
	found = assistantSearch{}
	if err := json.Unmarshal([]byte(c.ok("search", map[string]any{"query": "made up", "kind": "song"})), &found); err != nil {
		t.Fatal(err)
	}
	for _, r := range found.Results {
		if r.Title == "Rude Words" || r.Kind != kindSong {
			t.Errorf("found %+v, with explicit content hidden, of songs alone", r)
		}
	}
}

// TestAssistantPlaylists lists the user's liked songs, mixes and
// playlists, with what a service didn't answer, and plays one by its name
// or its id.
func TestAssistantPlaylists(t *testing.T) {
	mix := media.Playlist{ID: "mx:1", Source: media.SourceLocal, Title: "Evening Mix", Owner: "You", TrackCount: 2,
		Tracks: []media.Track{song("m1", "Mix One", "Made Up"), song("m2", "Mix Two", "Made Up")}}
	other := media.Playlist{ID: "yt:PL2", Source: media.SourceYouTube, Title: "Evening Walk", TrackCount: 1,
		Tracks: []media.Track{song("w1", "Walk One", "Made Up")}}
	cat := &fakeCatalog{
		mine:      []media.Playlist{mix, other},
		mineErrs:  []error{errors.New("Spotify: it didn't answer")},
		playlists: map[string]media.Playlist{mix.ID: mix, other.ID: other},
		liked:     []media.Track{song("l1", "Liked One", "Made Up"), song("l2", "Liked Two", "Made Up")},
	}
	a, toasts := newAssistantApp(t, cat)
	c := connectAssistant(t, a)
	p := a.GoPlayer()
	var list assistantPlaylists
	if err := json.Unmarshal([]byte(c.ok("list_playlists", nil)), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Playlists) != 3 || list.Playlists[0].ID != likedID || list.Playlists[0].Tracks != 2 ||
		list.Playlists[1].Title != "Evening Mix" || list.Playlists[1].Service != "MixMusic" ||
		list.Playlists[2].Service != "YouTube Music" || len(list.Problems) != 1 {
		t.Fatalf("playlists %+v", list)
	}

	c.ok("play_playlist", map[string]any{"name": "my liked songs"})
	waitPlayer(t, p, "the liked songs", func(s player.State) bool {
		return s.Playing && slices.Equal(queueIDs(s), []string{"yt:l1", "yt:l2"})
	})
	toasts.wait(t, "An AI assistant played your liked songs")
	if text := c.fails("play_playlist", map[string]any{"name": "evening"}); !strings.Contains(text, "Evening Mix") ||
		!strings.Contains(text, "yt:PL2") {
		t.Errorf("two that match: %s", text)
	}
	c.ok("play_playlist", map[string]any{"name": "EVENING MIX"})
	waitPlayer(t, p, "the mix", func(s player.State) bool { return slices.Equal(queueIDs(s), []string{"yt:m1", "yt:m2"}) })
	toasts.wait(t, "played “Evening Mix”")
	c.ok("play_playlist", map[string]any{"id": "yt:PL2", "shuffle": true})
	waitPlayer(t, p, "the walk, shuffled", func(s player.State) bool { return s.Shuffle && slices.Equal(queueIDs(s), []string{"yt:w1"}) })
	toasts.wait(t, "shuffled “Evening Walk”")
	c.ok("play_result", map[string]any{"id": "mx:1", "when": "last"})
	waitPlayer(t, p, "the mix added", func(s player.State) bool { return len(s.Queue) == 3 })

	c.fails("play_playlist", nil)
	c.fails("play_playlist", map[string]any{"id": "mx:1", "name": "Evening Mix"})
	c.fails("play_playlist", map[string]any{"name": "Morning"})
	c.fails("play_playlist", map[string]any{"id": "mx:404"})
}

// TestAssistantWithoutPlayer says why the tools can't play before the
// window made the player.
func TestAssistantWithoutPlayer(t *testing.T) {
	dir := t.TempDir()
	a, err := New(Options{DataDir: filepath.Join(dir, "data"), CacheDir: filepath.Join(dir, "cache"), Builtin: builtin.FS, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	a.assistants.catalog = &fakeCatalog{}
	if _, err := (&Preferences{a}).Update(func(s *settings.Settings) { s.AssistantControl = true }); err != nil {
		t.Fatal(err)
	}
	c := connectAssistant(t, a)
	for _, tool := range []string{"now_playing", "pause", "play"} {
		if text := c.fails(tool, nil); !strings.Contains(text, "isn't ready") {
			t.Errorf("%s: %s", tool, text)
		}
	}
}

// TestAssistantTools gives each tool a title, a description, and a
// schema of an object that names its arguments.
func TestAssistantTools(t *testing.T) {
	a := newQuietApp(t, nil)
	tools := a.assistantTools()
	want := []string{"now_playing", "get_queue", "play", "pause", "next_track", "previous_track", "seek", "set_volume",
		"search", "play_result", "list_playlists", "play_playlist"}
	var names []string
	for _, tool := range tools {
		names = append(names, tool.Name)
		var schema struct {
			Type       string                    `json:"type"`
			Properties map[string]map[string]any `json:"properties"`
			Required   []string                  `json:"required"`
			More       *bool                     `json:"additionalProperties"`
		}
		if err := json.Unmarshal(tool.Input, &schema); err != nil {
			t.Errorf("%s: %v", tool.Name, err)
			continue
		}
		if schema.Type != "object" || schema.More == nil || *schema.More {
			t.Errorf("%s: the schema %s isn't of an object that names its arguments", tool.Name, tool.Input)
		}
		for name, prop := range schema.Properties {
			if prop["type"] == nil || prop["description"] == nil {
				t.Errorf("%s: %s has no type or no description", tool.Name, name)
			}
		}
		for _, r := range schema.Required {
			if _, ok := schema.Properties[r]; !ok {
				t.Errorf("%s: requires %s, which it doesn't name", tool.Name, r)
			}
		}
		if tool.Title == "" || len(tool.Description) < 20 || tool.Call == nil {
			t.Errorf("%s: no title, description or call", tool.Name)
		}
	}
	if !slices.Equal(names, want) {
		t.Errorf("tools %q, want %q", names, want)
	}
}

// TestAssistantCommand quotes a program with spaces, and names the data
// folder of a portable copy.
func TestAssistantCommand(t *testing.T) {
	if got := commandLine(`C:\Program Files\MixMusic\MixMusic.exe`, []string{"--mcp"}); got != `"C:\Program Files\MixMusic\MixMusic.exe" --mcp` {
		t.Errorf("command %s", got)
	}
	if got := commandLine("/usr/bin/mixmusic", []string{"--mcp"}); got != "/usr/bin/mixmusic --mcp" {
		t.Errorf("command %s", got)
	}
	cfg := assistantConfig(`C:\MixMusic\MixMusic.exe`, []string{"--mcp"}, map[string]string{"MIXMUSIC_DATA_DIR": `D:\Music`})
	var v struct {
		Servers map[string]struct {
			Command string            `json:"command"`
			Env     map[string]string `json:"env"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(cfg), &v); err != nil {
		t.Fatal(err)
	}
	if s := v.Servers["mixmusic"]; s.Command != `C:\MixMusic\MixMusic.exe` || s.Env["MIXMUSIC_DATA_DIR"] != `D:\Music` {
		t.Errorf("configuration %s", cfg)
	}
	t.Setenv("MIXMUSIC_DATA_DIR", `D:\Portable`)
	if _, args, env := assistantCommand(); env["MIXMUSIC_DATA_DIR"] != `D:\Portable` || !slices.Equal(args[len(args)-1:], []string{"--mcp"}) {
		t.Errorf("a portable copy: %v %v", args, env)
	}
}

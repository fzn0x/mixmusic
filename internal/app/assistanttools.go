package app

// The tools of AI assistants (#477). What plays and the queue are told as
// remote control tells them (remoteStateOf), and play, pause, next,
// previous, seeking and the volume go through remote control's commands
// (remoteHost.Do); what an assistant finds plays as the windows play it
// (assistantfind.go). Each change shows a toast, through the player's
// notices.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/fzn0x/mixmusic/internal/mcp"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/remote"
)

// errNoPlayer is why a tool can't reach the player.
var errNoPlayer = errors.New("MixMusic's player isn't ready yet, as MixMusic is still starting: try again in a moment")

// assistantPlayer is the player, as remote control has it.
func (a *App) assistantPlayer() (*player.Player, error) {
	if p := (remoteHost{a}).player(); p != nil {
		return p, nil
	}
	return nil, errNoPlayer
}

// assistantDid shows the user what an assistant did, as a toast, and logs it.
func assistantDid(p *player.Player, what string) {
	log.Printf("mixmusic: AI assistants: %s", what)
	p.Tell(player.Notice{Message: "An AI assistant " + what})
}

// assistantTrack is a track as the tools tell it.
type assistantTrack struct {
	Title           string   `json:"title"`
	Artists         []string `json:"artists,omitempty"`
	Album           string   `json:"album,omitempty"`
	Service         string   `json:"service"`
	DurationSeconds float64  `json:"durationSeconds,omitempty"`
}

// assistantTrackOf is t as the tools tell it: as remote control does, and
// its service.
func assistantTrackOf(t media.Track) assistantTrack {
	r := remoteTrackOf(t)
	return assistantTrack{Title: r.Title, Artists: r.Artists, Album: r.Album, Service: serviceName(discordSource(t)),
		DurationSeconds: seconds(int64(t.DurationMs))}
}

// serviceName names a service for the assistant.
func serviceName(s media.Source) string {
	switch s {
	case media.SourceYouTube:
		return "YouTube Music"
	case media.SourceSpotify:
		return "Spotify"
	case media.SourceFile:
		return "this computer"
	case media.SourceLocal:
		return "MixMusic"
	}
	return string(s)
}

// seconds is ms in seconds, to a tenth.
func seconds(ms int64) float64 { return math.Round(float64(ms)/100) / 10 }

// minutesOf is ms as minutes and seconds, as 3:07, or hours too.
func minutesOf(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	h, m, s := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// named is a track as a toast names it: its title in quotes, and its
// artists.
func named(t media.Track) string {
	r := remoteTrackOf(t)
	s := "“" + oneLine(r.Title) + "”"
	if len(r.Artists) > 0 {
		s += " by " + oneLine(strings.Join(r.Artists, ", "))
	}
	return s
}

// jsonResult is a result whose text is its data, as JSON.
func jsonResult(v any) (mcp.Result, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return mcp.Result{}, err
	}
	return mcp.Result{Text: string(data), Data: v}, nil
}

// noArgs is the schema of a tool without arguments.
const noArgs = `{"type":"object","properties":{},"additionalProperties":false}`

// assistantTools are what the AI assistants may call, in the order
// tools/list tells them.
func (a *App) assistantTools() []mcp.Tool {
	return []mcp.Tool{
		{Name: "now_playing", Title: "Now playing", ReadOnly: true, Input: json.RawMessage(noArgs),
			Description: "Tells what MixMusic plays: the track, with its artists, album and service, whether it plays or is " +
				"paused, where it is and how long it is in seconds, the volume, shuffle and repeat. The track is null " +
				"when nothing is in the queue.",
			Call: a.toolNowPlaying},
		{Name: "get_queue", Title: "Queue", ReadOnly: true, Input: json.RawMessage(noArgs),
			Description: "Lists the tracks that play after the current one, in the order they play, up to 20, and how " +
				"many more come after those.",
			Call: a.toolQueue},
		{Name: "play", Title: "Play", Idempotent: true, Input: json.RawMessage(noArgs),
			Description: "Plays the current track of the queue, from where it was paused. To play something else, use " +
				"play_result or play_playlist.",
			Call: a.toolPlay},
		{Name: "pause", Title: "Pause", Idempotent: true, Input: json.RawMessage(noArgs),
			Description: "Pauses the music where it is: play goes on from there.",
			Call:        a.toolPause},
		{Name: "next_track", Title: "Next track",
			Input: json.RawMessage(`{"type":"object","properties":{"count":{"type":"integer","minimum":1,"maximum":20,` +
				`"description":"How many tracks to skip: 1, the default, plays the next one; 3 skips the next two and plays the third."}},` +
				`"additionalProperties":false}`),
			Description: "Skips to the next track of the queue, or past several.",
			Call:        a.toolNext},
		{Name: "previous_track", Title: "Previous track", Input: json.RawMessage(noArgs),
			Description: "Goes back to the previous track, as the previous button does: a track that played for more " +
				"than 3 seconds starts again instead.",
			Call: a.toolPrevious},
		{Name: "seek", Title: "Seek", Idempotent: true,
			Input: json.RawMessage(`{"type":"object","properties":{` +
				`"position_seconds":{"type":"number","minimum":0,"description":"Where to move to, in seconds from the track's start."},` +
				`"by_seconds":{"type":"number","description":"How far to move from where the track is, in seconds: negative goes back."}},` +
				`"additionalProperties":false}`),
			Description: "Moves the current track to a place in it: give position_seconds, or by_seconds, not both. A " +
				"live stream can't move.",
			Call: a.toolSeek},
		{Name: "set_volume", Title: "Volume", Idempotent: true,
			Input: json.RawMessage(`{"type":"object","properties":{` +
				`"percent":{"type":"integer","minimum":0,"maximum":100,"description":"The volume, from 0 to 100."},` +
				`"by_percent":{"type":"integer","minimum":-100,"maximum":100,"description":"How much to turn the volume up, or down when negative."}},` +
				`"additionalProperties":false}`),
			Description: "Sets MixMusic's volume: give percent, or by_percent, not both. It unmutes.",
			Call:        a.toolVolume},
		{Name: "search", Title: "Search", ReadOnly: true, OpenWorld: true,
			Input: json.RawMessage(`{"type":"object","properties":{` +
				`"query":{"type":"string","minLength":1,"maxLength":200,"description":"What to look for: a title, an artist, an album, words of a song."},` +
				`"kind":{"type":"string","enum":["any","song","video","album","artist","playlist"],"description":"What to find: any, the default, finds songs, albums, artists and playlists; video finds music videos."},` +
				`"limit":{"type":"integer","minimum":1,"maximum":10,"description":"How many results of each kind, 5 by default."}},` +
				`"required":["query"],"additionalProperties":false}`),
			Description: "Searches YouTube Music, and Spotify when the user signed in to it. Each result has an id, which " +
				"play_result plays, puts next or adds to the queue.",
			Call: a.toolSearch},
		{Name: "play_result", Title: "Play a result", OpenWorld: true,
			Input: json.RawMessage(`{"type":"object","properties":{` +
				`"id":{"type":"string","description":"The id of a result of search, or of a playlist of list_playlists."},` +
				`"when":{"type":"string","enum":["now","next","last"],"description":"now, the default, plays it at once, in place of the queue; next puts it after the current track; last adds it at the end of the queue."},` +
				`"shuffle":{"type":"boolean","description":"Plays an album, an artist's top tracks or a playlist shuffled, when now."}},` +
				`"required":["id"],"additionalProperties":false}`),
			Description: "Plays a song, a video, an album, an artist's top tracks or a playlist that search or " +
				"list_playlists found, or puts it in the queue. A song played now is followed by songs like it, " +
				"when the user's autoplay is on.",
			Call: a.toolPlayResult},
		{Name: "list_playlists", Title: "Your playlists", ReadOnly: true, OpenWorld: true, Input: json.RawMessage(noArgs),
			Description: "Lists the user's own: their liked songs, their mixes made in MixMusic, and their playlists on " +
				"YouTube Music and Spotify, when they signed in to those. Each has an id, which play_playlist and " +
				"play_result take.",
			Call: a.toolPlaylists},
		{Name: "play_playlist", Title: "Play a playlist", OpenWorld: true,
			Input: json.RawMessage(`{"type":"object","properties":{` +
				`"id":{"type":"string","description":"The id list_playlists gives."},` +
				`"name":{"type":"string","description":"The playlist's name, or a part of it, in place of the id: liked songs plays the liked songs."},` +
				`"shuffle":{"type":"boolean","description":"Plays it shuffled."}},` +
				`"additionalProperties":false}`),
			Description: "Plays one of the user's playlists, mixes or their liked songs, by its id or its name, in place " +
				"of the queue.",
			Call: a.toolPlayPlaylist},
	}
}

// assistantNow is what plays, as now_playing tells it.
type assistantNow struct {
	Playing         bool            `json:"playing"`
	Loading         bool            `json:"loading,omitempty"`
	Track           *assistantTrack `json:"track"`
	PositionSeconds float64         `json:"positionSeconds"`
	DurationSeconds float64         `json:"durationSeconds,omitempty"`
	Live            bool            `json:"live,omitempty"`
	VolumePercent   int             `json:"volumePercent"`
	Muted           bool            `json:"muted,omitempty"`
	Shuffle         bool            `json:"shuffle"`
	Repeat          string          `json:"repeat"`
}

func (a *App) toolNowPlaying(context.Context, json.RawMessage) (mcp.Result, error) {
	p, err := a.assistantPlayer()
	if err != nil {
		return mcp.Result{}, err
	}
	st := p.State()
	rs := remoteStateOf(st)
	now := assistantNow{Playing: rs.Playing, Loading: rs.Loading, Live: rs.Live, Muted: rs.Muted,
		PositionSeconds: seconds(rs.PositionMs), DurationSeconds: seconds(rs.DurationMs),
		VolumePercent: int(math.Round(rs.Volume * 100)), Shuffle: st.Shuffle, Repeat: string(st.Repeat)}
	if cur, ok := st.Current(); ok {
		t := assistantTrackOf(cur.Track)
		now.Track = &t
	}
	return jsonResult(now)
}

// assistantQueue is what comes next, as get_queue tells it.
type assistantQueue struct {
	Next []assistantTrack `json:"next"`
	// More counts the tracks after those.
	More    int    `json:"more,omitempty"`
	Shuffle bool   `json:"shuffle"`
	Repeat  string `json:"repeat"`
}

func (a *App) toolQueue(context.Context, json.RawMessage) (mcp.Result, error) {
	p, err := a.assistantPlayer()
	if err != nil {
		return mcp.Result{}, err
	}
	st := p.State()
	q := assistantQueue{Next: []assistantTrack{}, Shuffle: st.Shuffle, Repeat: string(st.Repeat)}
	coming := 0
	for i := st.Index + 1; i < len(st.Queue); i++ {
		if st.Queue[i].Skip == "" {
			coming++
		}
	}
	for _, next := range remoteStateOf(st).Next {
		if i := slices.IndexFunc(st.Queue, func(it player.Item) bool { return it.UID == next.Entry }); i >= 0 {
			q.Next = append(q.Next, assistantTrackOf(st.Queue[i].Track))
		}
	}
	q.More = coming - len(q.Next)
	return jsonResult(q)
}

func (a *App) toolPlay(context.Context, json.RawMessage) (mcp.Result, error) {
	p, err := a.assistantPlayer()
	if err != nil {
		return mcp.Result{}, err
	}
	st := p.State()
	cur, ok := st.Current()
	switch {
	case !ok:
		return mcp.Result{}, errors.New("nothing is in the queue: find something with search and play it with play_result, or play a playlist with play_playlist")
	case remoteStateOf(st).Playing:
		return mcp.Result{Text: "It plays already."}, nil
	}
	remoteHost{a}.Do(remote.Command{Op: remote.OpPlay})
	assistantDid(p, "played "+named(cur.Track))
	return mcp.Result{Text: "Playing " + named(cur.Track) + "."}, nil
}

func (a *App) toolPause(context.Context, json.RawMessage) (mcp.Result, error) {
	p, err := a.assistantPlayer()
	if err != nil {
		return mcp.Result{}, err
	}
	st := p.State()
	cur, ok := st.Current()
	if !ok || !remoteStateOf(st).Playing {
		return mcp.Result{Text: "Nothing plays: it was paused already."}, nil
	}
	remoteHost{a}.Do(remote.Command{Op: remote.OpPause})
	assistantDid(p, "paused "+named(cur.Track))
	return mcp.Result{Text: "Paused."}, nil
}

func (a *App) toolNext(_ context.Context, args json.RawMessage) (mcp.Result, error) {
	in, err := mcp.Args[struct {
		Count *int `json:"count"`
	}](args)
	if err != nil {
		return mcp.Result{}, err
	}
	count := 1
	if in.Count != nil {
		count = *in.Count
	}
	if count < 1 || count > remote.MaxNext {
		return mcp.Result{}, fmt.Errorf("count is from 1 to %d", remote.MaxNext)
	}
	p, err := a.assistantPlayer()
	if err != nil {
		return mcp.Result{}, err
	}
	st := p.State()
	if _, ok := st.Current(); !ok {
		return mcp.Result{}, errors.New("nothing is in the queue")
	}
	if count == 1 {
		remoteHost{a}.Do(remote.Command{Op: remote.OpNext})
		assistantDid(p, "skipped to the next track")
		return mcp.Result{Text: "Skipped to the next track."}, nil
	}
	next := remoteStateOf(st).Next
	if count > len(next) {
		return mcp.Result{}, fmt.Errorf("only %d tracks come next", len(next))
	}
	i := slices.IndexFunc(st.Queue, func(it player.Item) bool { return it.UID == next[count-1].Entry })
	if i < 0 {
		return mcp.Result{}, errors.New("the queue changed: try again")
	}
	remoteHost{a}.Do(remote.Command{Op: remote.OpPlayEntry, Entry: next[count-1].Entry})
	assistantDid(p, "skipped ahead to "+named(st.Queue[i].Track))
	return mcp.Result{Text: "Skipped ahead: " + named(st.Queue[i].Track) + " plays."}, nil
}

func (a *App) toolPrevious(context.Context, json.RawMessage) (mcp.Result, error) {
	p, err := a.assistantPlayer()
	if err != nil {
		return mcp.Result{}, err
	}
	if _, ok := p.State().Current(); !ok {
		return mcp.Result{}, errors.New("nothing is in the queue")
	}
	remoteHost{a}.Do(remote.Command{Op: remote.OpPrevious})
	assistantDid(p, "went back")
	return mcp.Result{Text: "Went back."}, nil
}

func (a *App) toolSeek(_ context.Context, args json.RawMessage) (mcp.Result, error) {
	in, err := mcp.Args[struct {
		Position *float64 `json:"position_seconds"`
		By       *float64 `json:"by_seconds"`
	}](args)
	if err != nil {
		return mcp.Result{}, err
	}
	if (in.Position == nil) == (in.By == nil) {
		return mcp.Result{}, errors.New("give position_seconds or by_seconds, one of them")
	}
	p, err := a.assistantPlayer()
	if err != nil {
		return mcp.Result{}, err
	}
	st := p.State()
	cur, ok := st.Current()
	rs := remoteStateOf(st)
	switch {
	case !ok:
		return mcp.Result{}, errors.New("nothing is in the queue")
	case rs.Live:
		return mcp.Result{}, errors.New("a live stream can't move")
	case rs.Loading:
		return mcp.Result{}, errors.New("the track is loading: try again in a moment")
	}
	var to float64
	if in.Position != nil {
		to = *in.Position * 1000
	} else {
		to = float64(rs.PositionMs) + *in.By*1000
	}
	if math.IsNaN(to) || math.IsInf(to, 0) {
		return mcp.Result{}, errors.New("that isn't a place in the track")
	}
	to = max(to, 0)
	if rs.DurationMs > 0 {
		to = min(to, float64(rs.DurationMs))
	}
	ms := int64(to)
	remoteHost{a}.Do(remote.Command{Op: remote.OpSeek, PositionMs: ms})
	assistantDid(p, "moved "+named(cur.Track)+" to "+minutesOf(ms))
	return mcp.Result{Text: "Moved to " + minutesOf(ms) + "."}, nil
}

func (a *App) toolVolume(_ context.Context, args json.RawMessage) (mcp.Result, error) {
	in, err := mcp.Args[struct {
		Percent *int `json:"percent"`
		By      *int `json:"by_percent"`
	}](args)
	if err != nil {
		return mcp.Result{}, err
	}
	if (in.Percent == nil) == (in.By == nil) {
		return mcp.Result{}, errors.New("give percent or by_percent, one of them")
	}
	p, err := a.assistantPlayer()
	if err != nil {
		return mcp.Result{}, err
	}
	var to int
	if in.Percent != nil {
		to = *in.Percent
	} else {
		to = int(math.Round(p.State().Volume*100)) + *in.By
	}
	to = min(max(to, 0), 100)
	remoteHost{a}.Do(remote.Command{Op: remote.OpVolume, Volume: float64(to) / 100})
	assistantDid(p, fmt.Sprintf("set the volume to %d%%", to))
	return mcp.Result{Text: fmt.Sprintf("The volume is %d%%.", to)}, nil
}

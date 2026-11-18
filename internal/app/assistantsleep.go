package app

// The sleep_timer tool of AI assistants (#594): it starts the sleep timer,
// adds minutes to it or cancels it, as its menus do, and tells what it
// does. The player's toast tells the user an assistant set it.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/fzn0x/mixmusic/internal/mcp"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// sleepTool is the tool of the sleep timer.
func (a *App) sleepTool() mcp.Tool {
	return mcp.Tool{Name: "sleep_timer", Title: "Sleep timer",
		Input: json.RawMessage(fmt.Sprintf(`{"type":"object","properties":{`+
			`"minutes":{"type":"integer","minimum":1,"maximum":%[1]d,"description":"Pauses the music after this many minutes."},`+
			`"until":{"type":"string","enum":["end_of_track","end_of_queue"],"description":"Pauses the music at the end of the track that plays, or at the end of the queue, before the songs autoplay adds."},`+
			`"after_tracks":{"type":"integer","minimum":1,"maximum":%[2]d,"description":"Pauses the music at the end of this many tracks, the one that plays first."},`+
			`"add_minutes":{"type":"integer","minimum":1,"maximum":%[1]d,"description":"Has the timer that runs pause the music this many minutes later."},`+
			`"cancel":{"type":"boolean","description":"Ends the timer that runs: the music plays on."}},`+
			`"additionalProperties":false}`, settings.MaxSleepMinutes, player.MaxSleepTracks)),
		Description: "Sets MixMusic's sleep timer, which fades the music out and pauses it, as the user set the fade: give " +
			"one of minutes, until, after_tracks, add_minutes or cancel, which replaces the timer that runs. With none, " +
			"it tells whether a timer runs, and what it does.",
		Call: a.toolSleepTimer}
}

// assistantSleep is the sleep timer as sleep_timer tells it.
type assistantSleep struct {
	Running bool `json:"running"`
	// Stops says when it pauses the music, in words.
	Stops       string  `json:"stops,omitempty"`
	SecondsLeft float64 `json:"secondsLeft,omitempty"`
}

func (a *App) toolSleepTimer(_ context.Context, args json.RawMessage) (mcp.Result, error) {
	in, err := mcp.Args[struct {
		Minutes *int    `json:"minutes"`
		Until   *string `json:"until"`
		Tracks  *int    `json:"after_tracks"`
		Add     *int    `json:"add_minutes"`
		Cancel  *bool   `json:"cancel"`
	}](args)
	if err != nil {
		return mcp.Result{}, err
	}
	given := 0
	for _, set := range []bool{in.Minutes != nil, in.Until != nil, in.Tracks != nil, in.Add != nil, in.Cancel != nil && *in.Cancel} {
		if set {
			given++
		}
	}
	if given > 1 {
		return mcp.Result{}, errors.New("give one of minutes, until, after_tracks, add_minutes or cancel")
	}
	p, err := a.assistantPlayer()
	if err != nil {
		return mcp.Result{}, err
	}
	st := p.State()
	t := player.SleepTimer{Assistant: true}
	var text string
	switch {
	case given == 0:
		return a.sleepStatus(st)
	case in.Cancel != nil && *in.Cancel:
		if st.Sleep == nil {
			return mcp.Result{Text: "No sleep timer runs."}, nil
		}
		p.CancelSleep(true)
		log.Print("mixmusic: AI assistants: cancelled the sleep timer")
		return mcp.Result{Text: "The sleep timer is off: the music plays on."}, nil
	case in.Add != nil:
		if st.Sleep == nil {
			return mcp.Result{}, errors.New("no sleep timer runs: start one with minutes, until or after_tracks")
		}
		if *in.Add < 1 || *in.Add > settings.MaxSleepMinutes {
			return mcp.Result{}, fmt.Errorf("add_minutes is from 1 to %d", settings.MaxSleepMinutes)
		}
		p.AddSleep(*in.Add, true)
		log.Printf("mixmusic: AI assistants: added %d minutes to the sleep timer", *in.Add)
		return mcp.Result{Text: fmt.Sprintf("The sleep timer pauses the music %d minutes later.", *in.Add)}, nil
	case in.Minutes != nil:
		if *in.Minutes < 1 || *in.Minutes > settings.MaxSleepMinutes {
			return mcp.Result{}, fmt.Errorf("minutes is from 1 to %d", settings.MaxSleepMinutes)
		}
		t.Mode, t.N = player.SleepMinutes, *in.Minutes
		text = fmt.Sprintf("in %d minutes", *in.Minutes)
	case st.Live:
		return mcp.Result{}, errors.New("a live stream never ends: give minutes instead")
	case in.Tracks != nil:
		if *in.Tracks < 1 || *in.Tracks > player.MaxSleepTracks {
			return mcp.Result{}, fmt.Errorf("after_tracks is from 1 to %d", player.MaxSleepTracks)
		}
		t.Mode, t.N = player.SleepTracks, *in.Tracks
		text = fmt.Sprintf("at the end of %d tracks", *in.Tracks)
		if *in.Tracks == 1 {
			text = "at the end of the track that plays"
		}
	case *in.Until == "end_of_track":
		t.Mode, t.N = player.SleepTracks, 1
		text = "at the end of the track that plays"
	case *in.Until == "end_of_queue":
		t.Mode = player.SleepQueue
		text = "at the end of the queue"
	default:
		return mcp.Result{}, errors.New("until is end_of_track or end_of_queue")
	}
	p.StartSleep(t)
	log.Printf("mixmusic: AI assistants: set the sleep timer to pause the music %s", text)
	return mcp.Result{Text: "The sleep timer pauses the music " + text + "."}, nil
}

// sleepStatus tells whether a sleep timer runs, and what it does.
func (a *App) sleepStatus(st player.State) (mcp.Result, error) {
	left, ok := st.SleepLeft(time.Now())
	if !ok {
		return jsonResult(assistantSleep{})
	}
	s := assistantSleep{Running: true, SecondsLeft: seconds(left.Milliseconds())}
	switch sl := st.Sleep; {
	case !sl.Ending():
		s.Stops = "in " + minutesOf(left.Milliseconds())
	case sl.Finishing || sl.Mode == player.SleepTracks && sl.Tracks == 1:
		s.Stops = "at the end of the track that plays"
	case sl.Mode == player.SleepTracks:
		s.Stops = fmt.Sprintf("at the end of %d tracks", sl.Tracks)
	default:
		s.Stops = "at the end of the queue"
	}
	return jsonResult(s)
}

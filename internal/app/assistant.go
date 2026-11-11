package app

// AI assistants (#477): while Settings › Desktop › Let AI assistants
// control MixMusic is on, the AI assistants of this computer that speak
// the Model Context Protocol control MixMusic, through the command they
// start, MixMusic --mcp, which reaches the server internal/mcp runs on
// 127.0.0.1. The tools (assistanttools.go) tell what plays as remote
// control tells it, and have the player do what remote control's
// commands and the windows' buttons do; each change shows a toast.

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/fzn0x/mixmusic/internal/mcp"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// assistantHost keeps the server of AI assistants, made at its first use.
type assistantHost struct {
	once   sync.Once
	server atomic.Pointer[mcp.Server]
	// problem is why the server couldn't start, until it does.
	problem atomic.Pointer[string]
	// forgot is done once the file a server of an earlier run left, when
	// assistants are off, is gone.
	forgot sync.Once
	// found keeps what the tools told of, the results of searches and the
	// playlists, by their IDs, which the tools that play take.
	found     *memo[assistantFound]
	foundOnce sync.Once
	// catalog is where the tools find music: the app's when nil. Tests
	// give theirs.
	catalog assistantCatalog
}

// assistantServer returns the server of AI assistants, not yet listening.
func (a *App) assistantServer() *mcp.Server {
	h := &a.assistants
	h.once.Do(func() {
		h.server.Store(&mcp.Server{Name: "mixmusic", Title: "MixMusic", Version: a.version(),
			Instructions: assistantInstructions, Tools: a.assistantTools(), Dir: a.opts.DataDir,
			Changed: notify})
		// Every session ends as MixMusic quits, and the file goes.
		a.OnFlush(a.stopAssistants)
	})
	return h.server.Load()
}

// assistantInstructions tell the assistant what MixMusic is and how its
// tools go together.
const assistantInstructions = "MixMusic is the music player on this computer: it plays YouTube Music, " +
	"Spotify and the music files of this computer, in one queue. now_playing tells what plays, and get_queue " +
	"what comes next. To play something, search for it, then play a result with play_result, by its id; or " +
	"list the user's own playlists, mixes and liked songs with list_playlists, and play one with play_playlist. " +
	"MixMusic shows the user each thing you do."

// applyAssistants has the server listen as the settings say, or stop,
// which ends every session at once.
func (a *App) applyAssistants(s settings.Settings) {
	h := &a.assistants
	if !s.AssistantControl {
		if srv := h.server.Load(); srv != nil {
			if srv.Running() {
				srv.Stop()
				log.Print("mixmusic: AI assistants: off")
			}
		} else {
			// The file a run that crashed left names a port where no
			// server listens: it goes.
			h.forgot.Do(func() { mcp.Forget(a.opts.DataDir) })
		}
		return
	}
	srv := a.assistantServer()
	if srv.Running() {
		return
	}
	if err := srv.Start(); err != nil {
		log.Printf("mixmusic: AI assistants: %v", err)
		why := err.Error()
		h.problem.Store(&why)
		notify()
		return
	}
	h.problem.Store(nil)
	log.Print("mixmusic: AI assistants: on")
}

// stopAssistants ends every session, as MixMusic quits.
func (a *App) stopAssistants() {
	if srv := a.assistants.server.Load(); srv != nil {
		srv.Stop()
	}
}

// AssistantStatus is what the settings page shows of AI assistants.
type AssistantStatus struct {
	// On is set while assistants may connect, and Problem tells why they
	// can't, while the settings let them.
	On      bool
	Problem string
	// Command is the command an assistant starts, as a line to copy, and
	// Config the JSON of the servers of the assistants that keep them as
	// mcpServers.
	Command string
	Config  string
	// Connected names the assistants connected now.
	Connected []string
}

// AssistantStatus tells what the settings page shows of AI assistants.
func (s *System) AssistantStatus() AssistantStatus {
	h := &s.a.assistants
	var st AssistantStatus
	if p := h.problem.Load(); p != nil {
		st.Problem = *p
	}
	if srv := h.server.Load(); srv != nil && srv.Running() {
		st.On = true
		for _, ss := range srv.Sessions() {
			st.Connected = append(st.Connected, ss.Client)
		}
	}
	cmd, args, env := assistantCommand()
	st.Command = commandLine(cmd, args)
	st.Config = assistantConfig(cmd, args, env)
	return st
}

// assistantCommand is the command an assistant starts to reach this
// MixMusic: its program, its arguments, and the environment it needs. In
// a Flatpak, the sandbox's own; in an AppImage, the AppImage, as the
// program inside it is in a folder that goes as it ends; elsewhere,
// MixMusic's program. A portable copy names its folder.
func assistantCommand() (cmd string, args []string, env map[string]string) {
	id := os.Getenv("FLATPAK_ID")
	_, err := os.Stat("/.flatpak-info")
	switch {
	case id != "" && err == nil:
		cmd, args = "flatpak", []string{"run", id, mcp.Flag}
	case os.Getenv("APPIMAGE") != "":
		cmd, args = os.Getenv("APPIMAGE"), []string{mcp.Flag}
	default:
		exe, err := os.Executable()
		if err != nil {
			exe = "MixMusic"
		}
		cmd, args = exe, []string{mcp.Flag}
	}
	if dir := os.Getenv("MIXMUSIC_DATA_DIR"); dir != "" {
		env = map[string]string{"MIXMUSIC_DATA_DIR": dir}
	}
	return cmd, args, env
}

// commandLine is cmd with args, as a line of a shell: a word with spaces
// in double quotes.
func commandLine(cmd string, args []string) string {
	words := make([]string, 0, 1+len(args))
	for _, w := range append([]string{cmd}, args...) {
		if w == "" || strings.ContainsAny(w, " \t'\"&;|()<>$`") {
			w = `"` + strings.ReplaceAll(w, `"`, `\"`) + `"`
		}
		words = append(words, w)
	}
	return strings.Join(words, " ")
}

// assistantConfig is the JSON of MixMusic as a server of the assistants
// that keep theirs as mcpServers, in a file of JSON.
func assistantConfig(cmd string, args []string, env map[string]string) string {
	type server struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env,omitempty"`
	}
	data, err := json.MarshalIndent(map[string]map[string]server{"mcpServers": {"mixmusic": {cmd, args, env}}}, "", "  ")
	if err != nil {
		return ""
	}
	return string(data)
}

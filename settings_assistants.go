package main

// Settings › Desktop › AI assistants (#477): whether the AI assistants of
// this computer that speak MCP may control MixMusic, how to add MixMusic
// to one, and those connected. It is off at first; internal/app's
// assistant*.go serves them, through internal/mcp.

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

func init() { addSettings("desktop", 20, (*shell).assistantSettings) }

const (
	assistantTitle  = "AI assistants"
	assistantSwitch = "Let AI assistants control MixMusic"
	assistantAbout  = "AI assistants on this computer that speak MCP, the Model Context Protocol, can tell what plays, " +
		"play, pause, skip, search, and play your playlists. Each thing one does shows here. Nothing on the network can reach it."
	assistantWords = "ai assistant assistants mcp model context protocol agent llm chat chatbot claude control command server"
	// assistantHelp is where docs/ai-assistants.md tells how to add
	// MixMusic to an assistant.
	assistantHelp = "https://fzn0x.github.io/mixmusic/docs/ai-assistants/"
)

// assistantSettings turn AI assistants on and off, and, while they're on,
// show the command an assistant starts and its configuration, to copy,
// and the assistants connected.
func (s *shell) assistantSettings(c *ui.Context) {
	k := &s.lk
	s.group(c, assistantTitle, func() {
		s.row(c, assistantSwitch, assistantAbout, func() {
			if s.switchOf(c.Key("assistant-control"), &s.set.AssistantControl).Label(assistantSwitch).Changed() {
				s.save()
			}
		}, assistantWords)
		if !s.set.AssistantControl {
			return
		}
		st := s.svc.System.AssistantStatus()
		if st.Problem != "" {
			s.rowWith(c, "AI assistants can't connect", func() {
				ui.Text(c, st.Problem+".").TextColor(k.danger)
			}, func() {}, assistantWords)
			return
		}
		s.rowWith(c, "Command", func() {
			ui.Text(c, st.Command).TextColor(k.text)
			ui.Text(c, ". An assistant starts it, as a server of MCP that runs a command (stdio), to reach MixMusic while it runs.")
		}, func() {
			if s.button(c, "assistant-command", "copy", false).Label("Copy the command").Clicked() {
				copyText(st.Command)
			}
		}, assistantWords)
		s.rowWith(c, "Configuration", func() {
			ui.Text(c, "For the assistants that keep their servers in a JSON file, as mcpServers. ")
			s.linkOf(c, "How to add MixMusic to an assistant", assistantHelp)
		}, func() {
			if s.button(c, "assistant-config", "copy", false).Label("Copy the configuration").Clicked() {
				copyText(st.Config)
			}
		}, assistantWords)
		s.found(c, "", st.Config, func() {
			ui.Text(c, st.Config).FontSize(k.captionSize()).Font(monospace()).TextColor(k.muted).Padding(0, 0, 12).Selectable()
		})
		connected := "None now. Turning this off ends every session at once."
		if len(st.Connected) > 0 {
			connected = strings.Join(st.Connected, ", ") + ". Turning this off ends their sessions at once."
		}
		s.row(c, "Connected", connected, func() {}, assistantWords)
	})
}

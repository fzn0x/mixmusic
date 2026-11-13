package main

// Settings › Desktop › AI assistants (#477): whether the AI assistants of
// this computer that speak MCP may control MixMusic, how to add MixMusic
// to one, and those connected. It is off at first; internal/app's
// assistant*.go serves them, through internal/mcp.

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

func init() { addSettings("desktop", 20, (*shell).assistantSettings) }

const (
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
	s.group(c, locales.AssistantsTitle(), func() {
		s.row(c, locales.AssistantsSwitch(), locales.AssistantsAbout(), func() {
			if s.switchOf(c.Key("assistant-control"), &s.set.AssistantControl).Label(locales.AssistantsSwitch()).Changed() {
				s.save()
			}
		}, assistantWords)
		if !s.set.AssistantControl {
			return
		}
		st := s.svc.System.AssistantStatus()
		if st.Problem != "" {
			s.rowWith(c, locales.AssistantsProblemTitle(), func() {
				ui.Text(c, locales.AssistantsProblem(st.Problem)).TextColor(k.danger)
			}, func() {}, assistantWords)
			return
		}
		s.rowWith(c, locales.AssistantsCommand(), func() {
			ui.Text(c, st.Command).TextColor(k.text)
			ui.Text(c, locales.AssistantsCommandNote())
		}, func() {
			if s.button(c, "assistant-command", locales.AssistantsCopy(), false).Label(locales.AssistantsCopyCommand()).Clicked() {
				copyText(st.Command)
			}
		}, assistantWords)
		s.rowWith(c, locales.AssistantsConfiguration(), func() {
			ui.Text(c, locales.AssistantsConfigNote())
			s.linkOf(c, locales.AssistantsHowTo(), assistantHelp)
		}, func() {
			if s.button(c, "assistant-config", locales.AssistantsCopy(), false).Label(locales.AssistantsCopyConfig()).Clicked() {
				copyText(st.Config)
			}
		}, assistantWords)
		s.found(c, "", st.Config, func() {
			ui.Text(c, st.Config).FontSize(k.captionSize()).Font(monospace()).TextColor(k.muted).Padding(0, 0, 12).Selectable()
		})
		connected := locales.AssistantsNone()
		if len(st.Connected) > 0 {
			connected = locales.AssistantsConnected(strings.Join(st.Connected, locales.AssistantsNameSeparator()))
		}
		s.row(c, locales.AssistantsConnectedTitle(), connected, func() {}, assistantWords)
	})
}

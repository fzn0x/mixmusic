package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/player"
)

// assistantOf connects an AI assistant to the shell's app through the
// command it starts, MixMusic --mcp, and initializes its session. It
// returns a function that sends a request and reads its answer, and the
// channel of the command's status as it ends.
func assistantOf(t *testing.T, s *shell) (func(method string, params any) string, <-chan int) {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	t.Cleanup(func() { inW.Close() })
	dir := s.svc.System.Info().DataDir
	ended := make(chan int, 1)
	go func() {
		_, status := relayAssistant([]string{"--mcp"}, func() (string, error) { return dir, nil }, inR, outW, io.Discard)
		outW.Close()
		ended <- status
	}()
	lines := make(chan string, 10)
	go func() {
		sc := bufio.NewScanner(outR)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	id := 0
	request := func(method string, params any) string {
		t.Helper()
		id++
		data, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
		if _, err := inW.Write(append(data, '\n')); err != nil {
			t.Fatal(err)
		}
		select {
		case line := <-lines:
			return line
		case <-time.After(10 * time.Second):
			t.Fatalf("%s: no answer", method)
		}
		return ""
	}
	if r := request("initialize", map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{},
		"clientInfo": map[string]any{"name": "test", "title": "Test Assistant", "version": "1"}}); !strings.Contains(r, `"mixmusic"`) {
		t.Fatalf("initialize: %s", r)
	}
	return request, ended
}

// TestAssistantRows shows the switch of AI assistants off at first, with
// nothing else; on, the command and the configuration to copy, and the
// assistants connected; an assistant's pause shows as a toast; and off
// again, the session ends (#477).
func TestAssistantRows(t *testing.T) {
	var s *shell
	s = newTestShellOf(t, func(o *app.Options) {
		// The tools reach the window's player, as remote control's do.
		o.Remote = &app.RemoteOptions{Player: func() *player.Player { return s.p }}
	})
	t.Cleanup(s.p.Notices(func(n player.Notice) { s.update(func() { s.notes = append(s.notes, n) }) }))
	tt := testerOf(t, s, 1280, 4000)
	show(s, tt, "/settings/desktop")
	if !tt.HasText(assistantTitle) || !tt.HasText(assistantSwitch) {
		t.Fatalf("Settings › Desktop doesn't show AI assistants: %q", tt.Texts())
	}
	st := s.svc.System.AssistantStatus()
	if st.On || tt.HasText(st.Command) || tt.HasText("Configuration") {
		t.Fatalf("off, assistants may connect, or the command shows: %+v", st)
	}
	// The switch's label is its row's title, which a click would find
	// first: the keys reach it, as remote control's.
	toggle(t, s, tt, assistantSwitch, assistantAbout)
	if st = s.svc.System.AssistantStatus(); !s.svc.Preferences.Get().AssistantControl || !st.On {
		t.Fatalf("the switch didn't let assistants in: %+v", st)
	}
	for _, text := range []string{st.Command, "Configuration", "How to add MixMusic to an assistant", "None now."} {
		if !tt.HasText(text) {
			t.Errorf("on, the page doesn't show %q: %q", text, tt.Texts())
		}
	}
	if !strings.Contains(strings.Join(tt.Texts(), "\n"), `"mcpServers"`) {
		t.Errorf("the configuration doesn't show: %q", tt.Texts())
	}
	var copied []string
	was := copyText
	copyText = func(text string) { copied = append(copied, text) }
	t.Cleanup(func() { copyText = was })
	click(t, s, tt, "Copy the command")
	click(t, s, tt, "Copy the configuration")
	if len(copied) != 2 || copied[0] != st.Command || copied[1] != st.Config {
		t.Errorf("copied %q", copied)
	}

	request, ended := assistantOf(t, s)
	until(t, s, tt, "the assistant connected", func() bool { return tt.HasText("Test Assistant") })
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	if r := request("tools/call", map[string]any{"name": "pause"}); !strings.Contains(r, `"isError":false`) {
		t.Fatalf("pause: %s", r)
	}
	s.settle(t, func(st player.State) bool { return !st.Playing })
	until(t, s, tt, "the toast", func() bool {
		return tt.HasText("An AI assistant paused “" + testTracks[0].Title + "” by " + testTracks[0].Artists[0].Name)
	})

	show(s, tt, settingsPath("", "mcp"))
	if !tt.HasText(assistantSwitch) {
		t.Errorf("searching mcp doesn't find the switch: %q", tt.Texts())
	}
	show(s, tt, "/settings/desktop")
	toggle(t, s, tt, assistantSwitch, assistantAbout)
	select {
	case status := <-ended:
		if status != 0 {
			t.Errorf("the command ended with %d", status)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("turned off, the session goes on")
	}
	if s.svc.System.AssistantStatus().On || tt.HasText(st.Command) {
		t.Error("turned off, assistants may connect, or the command shows")
	}
}

// TestRelayAssistant runs the command only with --mcp, and tells why it
// can't reach MixMusic.
func TestRelayAssistant(t *testing.T) {
	if relayed, _ := relayAssistant([]string{"mixmusic://open"}, nil, nil, nil, nil); relayed {
		t.Error("a link ran the command")
	}
	var errOut bytes.Buffer
	relayed, status := relayAssistant([]string{"--mcp"}, func() (string, error) { return t.TempDir(), nil },
		strings.NewReader(""), io.Discard, &errOut)
	if !relayed || status != 1 || !strings.Contains(errOut.String(), "Let AI assistants control MixMusic is off") {
		t.Errorf("without MixMusic: %v, %d, %q", relayed, status, errOut.String())
	}
}

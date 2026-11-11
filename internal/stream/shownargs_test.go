package stream

import (
	"strings"
	"testing"
)

func TestShownArgsHidesSecrets(t *testing.T) {
	home := `C:\Users\Someone`
	args := []string{
		"-J", "--no-playlist", "-f", "bestaudio",
		"--cookies", `C:\Temp\mixmusic-session-12345.txt`,
		"--extractor-args", "youtube:po_token=web+MadeUpTokenValue;visitor_data=MadeUpVisitorData;player_client=visionos",
		"--cache-dir", `C:\Users\Someone\AppData\Local\MixMusic\ytdlp-cache`,
		"--js-runtimes", "deno:C:/Users/someone/tools/deno.exe",
		"--proxy", "http://name:madeup-pass@proxy.example:8080",
		"--add-header", "Authorization:Bearer madeup",
		"--", "https://music.youtube.com/watch?v=aaaaaaaaaaa",
	}
	got := strings.Join(shownArgs(args, home), " ")
	for _, secret := range []string{"mixmusic-session-12345", "MadeUpTokenValue", "MadeUpVisitorData", `C:\Users\Someone`,
		"C:/Users/someone", "madeup-pass", "name:", "Bearer"} {
		if strings.Contains(got, secret) {
			t.Errorf("the shown arguments hold %q:\n%s", secret, got)
		}
	}
	for _, kept := range []string{"-J", "--no-playlist", "player_client=visionos", `~\AppData\Local\MixMusic\ytdlp-cache`,
		"deno:~/tools/deno.exe", "proxy.example:8080", "watch?v=aaaaaaaaaaa", "--cookies " + hidden} {
		if !strings.Contains(got, kept) {
			t.Errorf("the shown arguments lack %q:\n%s", kept, got)
		}
	}
}

func TestShownArgsJoinedFormsAndUnix(t *testing.T) {
	got := strings.Join(shownArgs([]string{"--cookies=/home/someone/c.txt", "--cache-dir=/home/someone/.cache/x",
		"--password=madeup", "--other-dir", "/home/someone2/x", "/home/someone"}, "/home/someone/"), " ")
	want := "--cookies=<hidden> --cache-dir=~/.cache/x --password=<hidden> --other-dir /home/someone2/x ~"
	if got != want {
		t.Errorf("shown arguments = %q, want %q", got, want)
	}
}

func TestShownArgsLeavesTheArgumentsAlone(t *testing.T) {
	args := []string{"--cookies", "/tmp/session.txt", "--cache-dir", "/home/someone/c"}
	shownArgs(args, "/home/someone")
	if args[1] != "/tmp/session.txt" || args[3] != "/home/someone/c" {
		t.Errorf("the arguments changed: %v", args)
	}
}

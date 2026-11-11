package stream

import (
	"net/url"
	"os"
	"regexp"
	"strings"
)

// hidden stands where ShownArgs hides a secret.
const hidden = "(hidden)"

// secretFlags are the options of yt-dlp whose next argument is a secret,
// which ShownArgs hides: the file of the session, and what signs in.
var secretFlags = map[string]bool{
	"--cookies": true, "--cookies-from-browser": true, "--username": true, "--password": true,
	"--twofactor": true, "--video-password": true, "--ap-username": true, "--ap-password": true,
	"--add-header": true, "--netrc-cmd": true, "--netrc-location": true,
}

// secretKeys are the names, in the arguments yt-dlp's extractors take as
// "name=value", whose value is a token or a session's: a PO token, the
// visitor data, the data sync ID, and the like.
var secretKeys = regexp.MustCompile(`(?i)((?:[a-z_]*po_token|visitor_data|data_sync_id|[a-z_]*token|[a-z_]*password|authorization|cookie[a-z_]*)=)[^;&\s]*`)

// ShownArgs returns the arguments a run of yt-dlp had, as a bug report can
// show them: without the file of the user's session, the tokens and the
// passwords they hold, and with their home folder as "~". The arguments
// stay as they are, for the stream to keep.
func ShownArgs(args []string) []string {
	home, _ := os.UserHomeDir()
	return shownArgs(args, home)
}

func shownArgs(args []string, home string) []string {
	var homes *regexp.Regexp
	if home = strings.TrimRight(home, `/\`); len(home) > 1 {
		// The home folder with either slash, whatever the case.
		slash := strings.ReplaceAll(regexp.QuoteMeta(strings.ReplaceAll(home, `\`, "/")), "/", `[/\\]`)
		homes = regexp.MustCompile(`(?i)` + slash + `(?:[/\\]|$)`)
	}
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, _, joined := strings.Cut(a, "=")
		switch {
		case secretFlags[a] && i+1 < len(args):
			out = append(out, a, hidden)
			i++
			continue
		case joined && secretFlags[name]:
			out = append(out, name+"="+hidden)
			continue
		case a == "--proxy" && i+1 < len(args):
			out = append(out, a, hideProxyLogin(args[i+1]))
			i++
			continue
		case joined && name == "--proxy":
			out = append(out, name+"="+hideProxyLogin(a[len(name)+1:]))
			continue
		}
		a = secretKeys.ReplaceAllString(a, "${1}"+hidden)
		if homes != nil {
			a = homes.ReplaceAllStringFunc(a, func(m string) string {
				if m[len(m)-1] == '/' || m[len(m)-1] == '\\' {
					return "~" + m[len(m)-1:]
				}
				return "~"
			})
		}
		out = append(out, a)
	}
	return out
}

// hideProxyLogin hides the user name and password of a proxy's address.
func hideProxyLogin(addr string) string {
	u, err := url.Parse(addr)
	if err != nil || u.User == nil {
		return addr
	}
	u.User = url.User(hidden)
	return u.String()
}

package browser

import (
	"bytes"
	"path"
	"strings"
)

// known is a browser MixMusic can sign in with, and where it looks for it.
type known struct {
	id, name string
	engine   Engine
	// windows is where its installer puts its program, under Program
	// Files, Program Files (x86), or the user's own LOCALAPPDATA. The
	// registry tells where it is too.
	windows []string
	// mac is its program, in /Applications or ~/Applications.
	mac []string
	// linux are its commands, looked for on PATH, then the folders where
	// its packages put it.
	linux []string
}

// knownBrowsers are the browsers MixMusic can sign in with, in the order
// it lists them.
var knownBrowsers = []known{
	{"firefox", "Firefox", Firefox,
		[]string{`Mozilla Firefox\firefox.exe`},
		[]string{"Firefox.app/Contents/MacOS/firefox"},
		[]string{"firefox", "firefox-esr"}},
	{"zen", "Zen", Firefox,
		[]string{`Zen Browser\zen.exe`},
		[]string{"Zen.app/Contents/MacOS/zen", "Zen Browser.app/Contents/MacOS/zen"},
		[]string{"zen-browser", "zen"}},
}

// Names are the names of the browsers MixMusic can sign in with, in its
// order.
func Names() []string {
	names := make([]string, len(knownBrowsers))
	for i, k := range knownBrowsers {
		names[i] = k.name
	}
	return names
}

// system is what finding the browsers reads of the computer, which tests
// fake.
type system struct {
	goos   string
	getenv func(string) string
	// program reports whether a program is at path.
	program func(path string) bool
	// head returns the first bytes of a file, which tell a script that
	// starts a snap.
	head func(path string) []byte
	// resolve follows the symbolic links of a path.
	resolve func(path string) string
	// value reads a value of a key of Windows's registry, "" for its
	// default, as "HKLM\SOFTWARE\…"; subkeys lists a key's keys.
	value   func(key, name string) string
	subkeys func(key string) []string
}

// Find returns the browsers installed on this computer that MixMusic can
// sign in with, in its order.
func Find() []Browser { return find(hostSystem()) }

func find(sys system) []Browser {
	var paths map[string]string
	switch sys.goos {
	case "windows":
		paths = findWindows(sys)
	case "darwin":
		paths = findMac(sys)
	default:
		// MixMusic's Flatpak sees none of the system's programs.
		if sys.program("/.flatpak-info") {
			return nil
		}
		paths = findLinux(sys)
	}
	var list []Browser
	for _, k := range knownBrowsers {
		p, ok := paths[k.id]
		if !ok {
			continue
		}
		b := Browser{ID: k.id, Name: k.name, Engine: k.engine, Path: p}
		if sys.goos == "linux" {
			b.Snap = snapOf(sys, p)
		}
		list = append(list, b)
	}
	return list
}

// findWindows finds the browsers where their installers put them, then
// those the registry knows: each browser registers under
// StartMenuInternet, for the default browser's choice, and many under App
// Paths too.
func findWindows(sys system) map[string]string {
	found := map[string]string{}
	add := func(id, p string) {
		if _, ok := found[id]; !ok && id != "" && sys.program(p) {
			found[id] = p
		}
	}
	var roots []string
	for _, v := range []string{"ProgramFiles", "ProgramW6432", "ProgramFiles(x86)", "LOCALAPPDATA"} {
		if r := sys.getenv(v); r != "" {
			roots = append(roots, strings.TrimRight(r, `\`))
		}
	}
	for _, k := range knownBrowsers {
		for _, r := range roots {
			for _, w := range k.windows {
				add(k.id, r+`\`+w)
			}
		}
	}
	for _, hive := range []string{"HKLM", "HKCU"} {
		for _, key := range []string{`SOFTWARE\Clients\StartMenuInternet`, `SOFTWARE\WOW6432Node\Clients\StartMenuInternet`} {
			for _, sub := range sys.subkeys(hive + `\` + key) {
				p := commandProgram(expand(sys, sys.value(hive+`\`+key+`\`+sub+`\shell\open\command`, "")))
				add(windowsBrowser(p), p)
			}
		}
		for _, exe := range []string{"firefox.exe", "zen.exe"} {
			p := commandProgram(expand(sys, sys.value(hive+`\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\`+exe, "")))
			add(windowsBrowser(p), p)
		}
	}
	return found
}

// windowsBrowser tells which browser a program of Windows is, by its name;
// or "".
func windowsBrowser(program string) string {
	p := strings.ToLower(program)
	i := strings.LastIndexByte(p, '\\')
	switch p[i+1:] {
	case "firefox.exe":
		return "firefox"
	case "zen.exe":
		return "zen"
	}
	return ""
}

// commandProgram returns the program of a command line, as the registry
// keeps them: quoted, or not, and followed by arguments, or not.
func commandProgram(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if rest, ok := strings.CutPrefix(cmd, `"`); ok {
		p, _, _ := strings.Cut(rest, `"`)
		return p
	}
	if i := strings.Index(strings.ToLower(cmd), ".exe"); i >= 0 {
		return cmd[:i+4]
	}
	return cmd
}

// expand replaces the %VARIABLES% of a value of the registry.
func expand(sys system, s string) string {
	var b strings.Builder
	for {
		i := strings.IndexByte(s, '%')
		j := -1
		if i >= 0 {
			j = strings.IndexByte(s[i+1:], '%')
		}
		if j < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		if v := sys.getenv(s[i+1 : i+1+j]); v != "" {
			b.WriteString(v)
		} else {
			b.WriteString(s[i : i+2+j])
		}
		s = s[i+2+j:]
	}
}

// findMac finds the browsers in the folders of applications.
func findMac(sys system) map[string]string {
	found := map[string]string{}
	roots := []string{"/Applications"}
	if h := sys.getenv("HOME"); h != "" {
		roots = append(roots, path.Join(h, "Applications"))
	}
	for _, k := range knownBrowsers {
		for _, r := range roots {
			for _, m := range k.mac {
				if p := path.Join(r, m); found[k.id] == "" && sys.program(p) {
					found[k.id] = p
				}
			}
		}
	}
	return found
}

// findLinux finds the browsers' commands on PATH, or in the folders their
// packages put them in.
func findLinux(sys system) map[string]string {
	found := map[string]string{}
	dirs := strings.Split(sys.getenv("PATH"), ":")
	for _, k := range knownBrowsers {
		for _, c := range k.linux {
			if found[k.id] != "" {
				break
			}
			if strings.HasPrefix(c, "/") {
				if sys.program(c) {
					found[k.id] = c
				}
				continue
			}
			for _, d := range dirs {
				if p := path.Join(d, c); d != "" && sys.program(p) {
					found[k.id] = p
					break
				}
			}
		}
	}
	return found
}

// snapOf returns the name of the snap a browser of Linux runs as, or "":
// a command of /snap/bin, or a script that starts one, as Ubuntu's
// firefox is.
func snapOf(sys system, program string) string {
	r := sys.resolve(program)
	switch {
	case r == "/usr/bin/snap" || strings.HasPrefix(program, "/snap/bin/"):
		return path.Base(program)
	case strings.HasPrefix(r, "/snap/bin/"):
		return path.Base(r)
	case strings.HasPrefix(r, "/snap/"):
		name, _, _ := strings.Cut(strings.TrimPrefix(r, "/snap/"), "/")
		return name
	}
	head := sys.head(program)
	if !bytes.HasPrefix(head, []byte("#!")) {
		return ""
	}
	_, rest, ok := bytes.Cut(head, []byte("/snap/bin/"))
	if !ok {
		return ""
	}
	end := bytes.IndexFunc(rest, func(c rune) bool {
		return !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-')
	})
	if end < 0 {
		end = len(rest)
	}
	return string(rest[:end])
}

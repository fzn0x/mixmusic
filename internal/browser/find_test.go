package browser

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// fakeSystem is a computer of goos with the programs files, the symbolic
// links links, the scripts scripts, and the registry's values and keys:
// the tests of each system run on every system.
type fakeSystem struct {
	goos    string
	env     map[string]string
	files   []string
	links   map[string]string
	scripts map[string]string
	// values are the registry's values, by key and name, as
	// `HKLM\SOFTWARE\…\command|` for a key's default.
	values map[string]string
}

func (f fakeSystem) system() system {
	return system{
		goos:    f.goos,
		getenv:  func(k string) string { return f.env[k] },
		program: func(p string) bool { return slices.Contains(f.files, p) },
		head:    func(p string) []byte { return []byte(f.scripts[p]) },
		resolve: func(p string) string {
			if r, ok := f.links[p]; ok {
				return r
			}
			return p
		},
		value: func(key, name string) string { return f.values[key+"|"+name] },
		subkeys: func(key string) []string {
			var keys []string
			for k := range f.values {
				rest, ok := strings.CutPrefix(k, key+`\`)
				if !ok {
					continue
				}
				sub, _, _ := strings.Cut(rest, `\`)
				if sub, _, _ = strings.Cut(sub, "|"); !slices.Contains(keys, sub) {
					keys = append(keys, sub)
				}
			}
			slices.Sort(keys)
			return keys
		},
	}
}

// found writes the browsers found as id=program, with their snaps.
func found(list []Browser) []string {
	var out []string
	for _, b := range list {
		s := b.ID + "=" + b.Path
		if b.Snap != "" {
			s += " (snap " + b.Snap + ")"
		}
		out = append(out, s)
	}
	return out
}

// TestFindWindows finds the browsers where their installers put them, and
// those the registry knows elsewhere: under StartMenuInternet, for the
// default browser's choice, and App Paths. A program the registry names
// that isn't there, and Internet Explorer, aren't browsers to sign in with.
func TestFindWindows(t *testing.T) {
	local := `C:\Users\Ada\AppData\Local`
	sys := fakeSystem{
		goos: "windows",
		env: map[string]string{
			"ProgramFiles": `C:\Program Files`, "ProgramFiles(x86)": `C:\Program Files (x86)`, "ProgramW6432": `C:\Program Files`,
			"LOCALAPPDATA": local,
		},
		files: []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			local + `\imput\Helium\Application\chrome.exe`,
			local + `\Chromium\Application\chrome.exe`,
			local + `\Vivaldi\Application\vivaldi.exe`,
			`D:\Apps\Firefox\firefox.exe`,
		},
		values: map[string]string{
			`HKLM\SOFTWARE\Clients\StartMenuInternet\IEXPLORE.EXE\shell\open\command|`:             `"C:\Program Files\Internet Explorer\iexplore.exe"`,
			`HKLM\SOFTWARE\Clients\StartMenuInternet\Firefox-308046B0AF4A39CB\shell\open\command|`: `"D:\Apps\Firefox\firefox.exe"`,
			`HKCU\SOFTWARE\Clients\StartMenuInternet\Chromium.ABC\shell\open\command|`:             `"` + local + `\Chromium\Application\chrome.exe" -- "%1"`,
			`HKLM\SOFTWARE\Clients\StartMenuInternet\Brave\shell\open\command|`:                    `"C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe"`,
			`HKCU\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\vivaldi.exe|`:                `%LOCALAPPDATA%\Vivaldi\Application\vivaldi.exe`,
		},
	}
	want := []string{
		`chrome=C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`edge=C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`chromium=` + local + `\Chromium\Application\chrome.exe`,
		`vivaldi=` + local + `\Vivaldi\Application\vivaldi.exe`,
		`helium=` + local + `\imput\Helium\Application\chrome.exe`,
		`firefox=D:\Apps\Firefox\firefox.exe`,
	}
	if got := found(find(sys.system())); !slices.Equal(got, want) {
		t.Errorf("found %q, want %q", got, want)
	}

	// Found by the registry alone, through App Paths.
	sys.files = []string{`E:\Zen\zen.exe`}
	sys.values = map[string]string{`HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\zen.exe|`: `E:\Zen\zen.exe`}
	if got := found(find(sys.system())); !slices.Equal(got, []string{`zen=E:\Zen\zen.exe`}) {
		t.Errorf("found %q", got)
	}
}

// TestFindMac finds the browsers in /Applications and ~/Applications.
func TestFindMac(t *testing.T) {
	sys := fakeSystem{
		goos: "darwin",
		env:  map[string]string{"HOME": "/Users/ada"},
		files: []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Users/ada/Applications/Helium.app/Contents/MacOS/Helium",
			"/Applications/Firefox.app/Contents/MacOS/firefox",
			"/Applications/Zen Browser.app/Contents/MacOS/zen",
			"/Applications/Safari.app/Contents/MacOS/Safari",
		},
	}
	want := []string{
		"chrome=/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"brave=/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		"helium=/Users/ada/Applications/Helium.app/Contents/MacOS/Helium",
		"firefox=/Applications/Firefox.app/Contents/MacOS/firefox",
		"zen=/Applications/Zen Browser.app/Contents/MacOS/zen",
	}
	if got := found(find(sys.system())); !slices.Equal(got, want) {
		t.Errorf("found %q, want %q", got, want)
	}
}

// TestFindLinux finds the browsers' commands on PATH, or where their
// packages put them, and tells those that run as a snap: from /snap/bin,
// or a script that starts one, as Ubuntu's firefox is. MixMusic's Flatpak
// sees none.
func TestFindLinux(t *testing.T) {
	sys := fakeSystem{
		goos: "linux",
		env:  map[string]string{"PATH": "/home/ada/.local/bin:/usr/local/bin:/usr/bin:/snap/bin", "HOME": "/home/ada"},
		files: []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/opt/brave.com/brave/brave",
			"/snap/bin/chromium",
			"/usr/bin/firefox",
			"/snap/bin/firefox",
			"/home/ada/.local/bin/zen",
			"/usr/bin/vivaldi-stable",
		},
		links: map[string]string{
			"/snap/bin/chromium":     "/usr/bin/snap",
			"/snap/bin/firefox":      "/usr/bin/snap",
			"/usr/bin/google-chrome": "/opt/google/chrome/google-chrome",
		},
		scripts: map[string]string{
			"/usr/bin/firefox":        "#!/bin/sh\nexec /snap/bin/firefox \"$@\"\n",
			"/usr/bin/vivaldi-stable": "#!/bin/bash\nexec -a \"$0\" /opt/vivaldi/vivaldi-bin \"$@\"\n",
		},
	}
	want := []string{
		"chrome=/usr/bin/google-chrome-stable",
		"brave=/opt/brave.com/brave/brave",
		"chromium=/snap/bin/chromium (snap chromium)",
		"vivaldi=/usr/bin/vivaldi-stable",
		"firefox=/usr/bin/firefox (snap firefox)",
		"zen=/home/ada/.local/bin/zen",
	}
	if got := found(find(sys.system())); !slices.Equal(got, want) {
		t.Errorf("found %q, want %q", got, want)
	}

	sys.files = append(sys.files, "/.flatpak-info")
	if got := find(sys.system()); got != nil {
		t.Errorf("MixMusic's Flatpak found %q", found(got))
	}
}

// TestCommandProgram reads the program of the registry's command lines.
func TestCommandProgram(t *testing.T) {
	for in, want := range map[string]string{
		`"C:\Program Files\Mozilla Firefox\firefox.exe"`:          `C:\Program Files\Mozilla Firefox\firefox.exe`,
		`"C:\A b\chrome.exe" --single-argument %1`:                `C:\A b\chrome.exe`,
		`C:\Program Files\Internet Explorer\iexplore.exe -nohome`: `C:\Program Files\Internet Explorer\iexplore.exe`,
		``: ``,
	} {
		if got := commandProgram(in); got != want {
			t.Errorf("commandProgram(%q) = %q, want %q", in, got, want)
		}
	}
	env := map[string]string{"LOCALAPPDATA": `C:\Users\Ada\AppData\Local`}
	sys := system{getenv: func(k string) string { return env[k] }}
	for in, want := range map[string]string{
		`%LOCALAPPDATA%\Vivaldi\vivaldi.exe`: `C:\Users\Ada\AppData\Local\Vivaldi\vivaldi.exe`,
		`%NOPE%\x.exe`:                       `%NOPE%\x.exe`,
		`100% sure`:                          `100% sure`,
	} {
		if got := expand(sys, in); got != want {
			t.Errorf("expand(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestNames lists every browser MixMusic can sign in with, in its order.
func TestNames(t *testing.T) {
	if got := fmt.Sprint(Names()); got != "[Chrome Edge Brave Chromium Vivaldi Helium Firefox Zen]" {
		t.Errorf("Names() = %s", got)
	}
}

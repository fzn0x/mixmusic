//go:build windows && (amd64 || arm64)

package firewall

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// TestMain runs the test binary as the copy Allow starts, when it's given
// AllowFlag: it changes the rules, of the rule MIXMUSIC_FIREWALL_RULE
// names, and ends.
func TestMain(m *testing.M) {
	if slices.Contains(os.Args[1:], AllowFlag) {
		exe, err := os.Executable()
		if err == nil {
			err = allowHere(exe, os.Getenv("MIXMUSIC_FIREWALL_RULE"))
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// TestLayouts gives the structs Windows reads the sizes C's have on 64-bit
// Windows.
func TestLayouts(t *testing.T) {
	if n := unsafe.Sizeof(shellExecuteInfo{}); n != 112 {
		t.Errorf("SHELLEXECUTEINFOW is %d bytes, want 112", n)
	}
	if n := unsafe.Sizeof(variant{}); n != 24 {
		t.Errorf("VARIANT is %d bytes, want 24", n)
	}
}

// TestCheckLive reads the firewall of the computer the test runs on,
// without changing it: no rule lets the test binary in.
func TestCheckLive(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	s, err := Check(exe)
	if err != nil {
		if strings.Contains(err.Error(), "0x800706d9") {
			t.Skip("the firewall's service doesn't run here:", err)
		}
		t.Fatal(err)
	}
	t.Logf("state: %+v", s)
	if s.Allowed {
		t.Errorf("a rule lets the test binary in: %+v", s)
	}
}

// TestAllowLive lets the test binary in, as the copy Allow starts, run
// twice: a rule that blocks it on private and public networks, as Cancel
// adds, keeps public ones, and one rule of its own lets it in on private
// and domain networks. It changes the firewall of the computer the test
// runs on, as administrator, so it runs only when MIXMUSIC_FIREWALL_WRITE
// is set, and removes its rules after.
func TestAllowLive(t *testing.T) {
	if os.Getenv("MIXMUSIC_FIREWALL_WRITE") == "" {
		t.Skip("MIXMUSIC_FIREWALL_WRITE isn't set: this test changes the computer's firewall, as administrator")
	}
	if !elevated() {
		t.Skip("this test changes the firewall's rules, which only an administrator may: run it in a terminal opened as administrator")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("MixMusic test %d", time.Now().UnixNano())
	block := name + " block"
	t.Cleanup(func() { removeRules(t, name, block) })
	err = withCOM(func() error {
		fw, err := openPolicy()
		if err != nil {
			return err
		}
		defer fw.release()
		return addRule(fw, Rule{Name: block, Program: exe, Inbound: true, Enabled: true, Profiles: ProfilePrivate | ProfilePublic})
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MIXMUSIC_FIREWALL_RULE", name)
	for range 2 {
		// "open" runs the copy as this test runs: an administrator's run
		// starts it as administrator with no question.
		if err := run(exe, "open"); err != nil {
			t.Fatal(err)
		}
	}
	var own []Rule
	var blocking Rule
	for _, r := range named(t, name, block) {
		if r.Name == name {
			own = append(own, r)
		} else {
			blocking = r
		}
	}
	if len(own) != 1 {
		t.Fatalf("%d rules of MixMusic's own, want 1: %+v", len(own), own)
	}
	if r := own[0]; !r.Allow || !r.Inbound || !r.Enabled || r.Profiles != trusted || !SameProgram(r.Program, exe) {
		t.Errorf("MixMusic's rule = %+v, want an inbound rule letting %s in on private and domain networks", r, exe)
	}
	if blocking.Allow || !blocking.Enabled || blocking.Profiles != ProfilePublic {
		t.Errorf("the blocking rule = %+v, want it on, blocking public networks alone", blocking)
	}
}

// named returns the rules of the firewall named one of names.
func named(t *testing.T, names ...string) []Rule {
	t.Helper()
	var out []Rule
	err := withCOM(func() error {
		fw, err := openPolicy()
		if err != nil {
			return err
		}
		defer fw.release()
		rules, err := readRules(fw)
		if err != nil {
			return err
		}
		defer release(rules)
		for _, r := range rules {
			if slices.Contains(names, r.Name) {
				out = append(out, r.Rule)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// removeRules removes the rules of the firewall named one of names.
func removeRules(t *testing.T, names ...string) {
	const slotRulesRemove = 9
	err := withCOM(func() error {
		fw, err := openPolicy()
		if err != nil {
			return err
		}
		defer fw.release()
		var rules *object
		if err := fw.call(slotRules, uintptr(unsafe.Pointer(&rules))); err != nil {
			return err
		}
		defer rules.release()
		for _, name := range names {
			// Remove takes one rule of the name at a time.
			for range 4 {
				rules.putBSTR(slotRulesRemove, name)
			}
		}
		return nil
	})
	if err != nil {
		t.Error(err)
	}
	if left := named(t, names...); len(left) > 0 {
		t.Errorf("rules left after the test: %+v", left)
	}
}

// elevated reports whether the test runs as administrator, its token
// elevated (TokenElevation).
func elevated() bool {
	const tokenElevation = 20
	p, err := syscall.GetCurrentProcess()
	if err != nil {
		return false
	}
	var tok syscall.Token
	if err := syscall.OpenProcessToken(p, syscall.TOKEN_QUERY, &tok); err != nil {
		return false
	}
	defer tok.Close()
	var e, n uint32
	if err := syscall.GetTokenInformation(tok, tokenElevation, (*byte)(unsafe.Pointer(&e)), uint32(unsafe.Sizeof(e)), &n); err != nil {
		return false
	}
	return e != 0
}

// TestHresultWords tells the HRESULTs met most in words, and the others by
// number.
func TestHresultWords(t *testing.T) {
	if err := hresult(0); err != nil {
		t.Errorf("S_OK: %v", err)
	}
	for r, want := range map[uintptr]string{
		0x80070005: "access denied: only an administrator may change the firewall's rules (HRESULT 0x80070005)",
		0x800706d9: "Windows Firewall's service doesn't run (HRESULT 0x800706d9)",
		0x80004005: "HRESULT 0x80004005",
	} {
		if err := hresult(r); err == nil || err.Error() != want {
			t.Errorf("hresult(%#x) = %v, want %q", r, err, want)
		}
	}
}

//go:build windows && (amd64 || arm64)

package firewall

// Windows Defender Firewall through its COM interface, INetFwPolicy2, as
// netfw.h has it. Each object is a pointer to its vtable, which lists the
// methods after those of IUnknown and IDispatch, in order; the slots below
// are their places. Reading needs no administrator; changing the rules
// does, which AllowHere runs as.

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// Supported is set where Check reads the system's firewall.
const Supported = true

type guid struct {
	data1        uint32
	data2, data3 uint16
	data4        [8]byte
}

var (
	clsidNetFwPolicy2 = guid{0xe2b3c97f, 0x6ae1, 0x41ac, [8]byte{0x81, 0x7a, 0xf6, 0xf9, 0x21, 0x66, 0xd7, 0xdd}}
	iidINetFwPolicy2  = guid{0x98325047, 0xc671, 0x4174, [8]byte{0x8d, 0x81, 0xde, 0xfc, 0xd3, 0xf0, 0x31, 0x86}}
	clsidNetFwRule    = guid{0x2c5bc43e, 0x3369, 0x4c33, [8]byte{0xab, 0x0c, 0xbe, 0x94, 0x69, 0x67, 0x7a, 0xf4}}
	iidINetFwRule     = guid{0xaf230d27, 0xbaba, 0x4e42, [8]byte{0xac, 0xed, 0xf5, 0x24, 0xf2, 0x2c, 0xfc, 0xe2}}
	iidIEnumVARIANT   = guid{0x00020404, 0x0000, 0x0000, [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
)

var (
	ole32    = syscall.NewLazyDLL("ole32.dll")
	oleaut32 = syscall.NewLazyDLL("oleaut32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")

	procCoInitializeEx      = ole32.NewProc("CoInitializeEx")
	procCoUninitialize      = ole32.NewProc("CoUninitialize")
	procCoCreateInstance    = ole32.NewProc("CoCreateInstance")
	procSysAllocString      = oleaut32.NewProc("SysAllocString")
	procSysFreeString       = oleaut32.NewProc("SysFreeString")
	procSysStringLen        = oleaut32.NewProc("SysStringLen")
	procVariantClear        = oleaut32.NewProc("VariantClear")
	procShellExecuteExW     = shell32.NewProc("ShellExecuteExW")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
)

const (
	coinitMultithreaded = 0x0
	clsctxAll           = 0x17
	rpcEChangedMode     = 0x80010106
	sFalse              = 1
	vtDispatch          = 9
	variantTrue         = 0xFFFF // VARIANT_BOOL's -1
	ruleDirIn           = 1
	actionAllow         = 1
	modifyStateOK       = 0
)

// The slots of the methods used, after IUnknown's (0 to 2) and IDispatch's
// (3 to 6).
const (
	slotQueryInterface = 0
	slotRelease        = 2

	// INetFwPolicy2.
	slotCurrentProfileTypes    = 7
	slotFirewallEnabled        = 8
	slotBlockAllInbound        = 12
	slotNotificationsOff       = 14
	slotRules                  = 18
	slotLocalPolicyModifyState = 28

	// INetFwRules.
	slotRulesAdd     = 8
	slotRulesNewEnum = 11

	// IEnumVARIANT.
	slotEnumNext = 3

	// INetFwRule.
	slotRuleName           = 7
	slotRulePutName        = 8
	slotRulePutDescription = 10
	slotRuleApplication    = 11
	slotRulePutApplication = 12
	slotRuleDirection      = 27
	slotRulePutDirection   = 28
	slotRuleEnabled        = 33
	slotRulePutEnabled     = 34
	slotRuleProfiles       = 37
	slotRulePutProfiles    = 38
	slotRuleAction         = 41
	slotRulePutAction      = 42
)

// object is a COM object: a pointer to its vtable, whose slots are read
// as far as its methods go.
type object struct{ vtbl *[64]uintptr }

// call calls the method at slot of o, and returns its HRESULT as an error.
// An argument may be a pointer converted to uintptr in call's own argument
// list, as uintptr(unsafe.Pointer(&v)): the directive moves what it points
// to to the heap, which Go never moves, and keeps it for the call, so that
// the method writes where v is.
//
//go:uintptrescapes
func (o *object) call(slot int, args ...uintptr) error {
	r, _, _ := syscall.SyscallN(o.vtbl[slot], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	return hresult(r)
}

func (o *object) release() { o.call(slotRelease) }

// query returns o's interface iid.
func (o *object) query(iid *guid) (*object, error) {
	var q *object
	if err := o.call(slotQueryInterface, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&q))); err != nil {
		return nil, err
	}
	return q, nil
}

// hresult is the error of a failed HRESULT, nil for a success.
func hresult(r uintptr) error {
	if int32(uint32(r)) < 0 {
		return fmt.Errorf("HRESULT %#x", uint32(r))
	}
	return nil
}

// variant is VARIANT on 64-bit Windows: its type, and a union of 16 bytes
// whose first word holds an object.
type variant struct {
	vt        uint16
	_, _, _   uint16
	val, val2 uintptr
}

// withCOM runs f on a thread of COM's multithreaded apartment.
func withCOM(f func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	r, _, _ := procCoInitializeEx.Call(0, coinitMultithreaded)
	switch {
	case uint32(r) == rpcEChangedMode:
		// The thread is in another apartment already: COM works there too.
	case hresult(r) != nil:
		return fmt.Errorf("COM: %w", hresult(r))
	default:
		defer procCoUninitialize.Call()
	}
	return f()
}

// create makes the COM object of class clsid, of its interface iid.
func create(clsid, iid *guid) (*object, error) {
	var o *object
	r, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(clsid)), 0, clsctxAll, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&o)))
	if err := hresult(r); err != nil {
		return nil, err
	}
	return o, nil
}

// getBSTR returns the string the method at slot of o gives, as a BSTR it
// frees.
func (o *object) getBSTR(slot int) (string, error) {
	var b *uint16
	if err := o.call(slot, uintptr(unsafe.Pointer(&b))); err != nil || b == nil {
		return "", err
	}
	defer procSysFreeString.Call(uintptr(unsafe.Pointer(b)))
	n, _, _ := procSysStringLen.Call(uintptr(unsafe.Pointer(b)))
	return syscall.UTF16ToString(unsafe.Slice(b, n)), nil
}

// putBSTR passes s to the method at slot of o as a BSTR.
func (o *object) putBSTR(slot int, s string) error {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		return err
	}
	b, _, _ := procSysAllocString.Call(uintptr(unsafe.Pointer(p)))
	if b == 0 {
		return errors.New("out of memory")
	}
	defer procSysFreeString.Call(b)
	return o.call(slot, b)
}

// getLong returns the 32 bits the method at slot of o gives.
func (o *object) getLong(slot int) (int32, error) {
	var v int32
	err := o.call(slot, uintptr(unsafe.Pointer(&v)))
	return v, err
}

// getBool returns the VARIANT_BOOL the method at slot of o gives.
func (o *object) getBool(slot int) (bool, error) {
	var v int16
	err := o.call(slot, uintptr(unsafe.Pointer(&v)))
	return v != 0, err
}

// getBoolOf returns the VARIANT_BOOL the method at slot of o gives of the
// kind of network kind.
func (o *object) getBoolOf(slot int, kind int32) (bool, error) {
	var v int16
	err := o.call(slot, uintptr(kind), uintptr(unsafe.Pointer(&v)))
	return v != 0, err
}

// rule is a rule of the firewall: what Check reads of it, and the object
// AllowHere changes.
type rule struct {
	Rule
	o *object
}

// readRules returns the rules of the firewall fw, which the caller
// releases.
func readRules(fw *object) ([]rule, error) {
	var rules *object
	if err := fw.call(slotRules, uintptr(unsafe.Pointer(&rules))); err != nil {
		return nil, fmt.Errorf("the rules: %w", err)
	}
	defer rules.release()
	var unk *object
	if err := rules.call(slotRulesNewEnum, uintptr(unsafe.Pointer(&unk))); err != nil {
		return nil, fmt.Errorf("the rules: %w", err)
	}
	enum, err := unk.query(&iidIEnumVARIANT)
	unk.release()
	if err != nil {
		return nil, fmt.Errorf("the rules: %w", err)
	}
	defer enum.release()
	var out []rule
	for {
		var v variant
		var fetched uint32
		r, _, _ := syscall.SyscallN(enum.vtbl[slotEnumNext], uintptr(unsafe.Pointer(enum)), 1, uintptr(unsafe.Pointer(&v)), uintptr(unsafe.Pointer(&fetched)))
		if err := hresult(r); err != nil {
			release(out)
			return nil, fmt.Errorf("the rules: %w", err)
		}
		if r == sFalse || fetched == 0 {
			return out, nil
		}
		// The first word of a VT_DISPATCH's union is its object.
		if disp := *(**object)(unsafe.Pointer(&v.val)); v.vt == vtDispatch && disp != nil {
			if o, err := disp.query(&iidINetFwRule); err == nil {
				out = append(out, rule{Rule: readRule(o), o: o})
			}
		}
		procVariantClear.Call(uintptr(unsafe.Pointer(&v)))
	}
}

// readRule reads what Check needs of the rule o; what it can't read stays
// unset, as for a rule of no program.
func readRule(o *object) Rule {
	var r Rule
	r.Name, _ = o.getBSTR(slotRuleName)
	r.Program, _ = o.getBSTR(slotRuleApplication)
	dir, _ := o.getLong(slotRuleDirection)
	r.Inbound = dir == ruleDirIn
	r.Enabled, _ = o.getBool(slotRuleEnabled)
	action, _ := o.getLong(slotRuleAction)
	r.Allow = action == actionAllow
	r.Profiles, _ = o.getLong(slotRuleProfiles)
	return r
}

// release releases the objects of rules.
func release(rules []rule) {
	for _, r := range rules {
		r.o.release()
	}
}

// readPolicy reads the firewall's policy: the kinds of network the
// computer is on, their settings, whether an organization's policy
// decides, and the rules, which the caller releases.
func readPolicy(fw *object) (policy, []rule, error) {
	var p policy
	var err error
	if p.current, err = fw.getLong(slotCurrentProfileTypes); err != nil {
		return p, nil, fmt.Errorf("the networks: %w", err)
	}
	p.of = map[int32]settings{}
	for _, k := range []int32{ProfileDomain, ProfilePrivate, ProfilePublic} {
		var set settings
		if set.on, err = fw.getBoolOf(slotFirewallEnabled, k); err != nil {
			return p, nil, fmt.Errorf("whether it's on: %w", err)
		}
		set.blockAll, _ = fw.getBoolOf(slotBlockAllInbound, k)
		set.quiet, _ = fw.getBoolOf(slotNotificationsOff, k)
		p.of[k] = set
	}
	if state, err := fw.getLong(slotLocalPolicyModifyState); err == nil {
		p.managed = state != modifyStateOK
	}
	rules, err := readRules(fw)
	if err != nil {
		return p, nil, err
	}
	for _, r := range rules {
		p.rules = append(p.rules, r.Rule)
	}
	return p, rules, nil
}

// openPolicy opens the firewall's policy, which the caller releases.
func openPolicy() (*object, error) {
	fw, err := create(&clsidNetFwPolicy2, &iidINetFwPolicy2)
	if err != nil {
		return nil, fmt.Errorf("Windows Firewall: %w", err)
	}
	return fw, nil
}

// Check reads what the firewall does to connections from the network the
// computer is on now to program, the path of its executable.
func Check(program string) (State, error) {
	var s State
	err := withCOM(func() error {
		fw, err := openPolicy()
		if err != nil {
			return err
		}
		defer fw.release()
		p, rules, err := readPolicy(fw)
		if err != nil {
			return fmt.Errorf("Windows Firewall: %w", err)
		}
		release(rules)
		s = judge(p, program)
		return nil
	})
	return s, err
}

// AllowHere lets program in on private and domain networks: the rules
// that block it there no longer cover them, and a rule named RuleName
// lets it in. It changes the firewall's rules, which only an administrator
// may: MixMusic runs it in the copy Allow starts.
func AllowHere(program string) error { return allowHere(program, RuleName) }

// allowHere is AllowHere, of a rule named name: tests give their own.
func allowHere(program, name string) error {
	return withCOM(func() error {
		fw, err := openPolicy()
		if err != nil {
			return err
		}
		defer fw.release()
		rules, err := readRules(fw)
		if err != nil {
			return fmt.Errorf("Windows Firewall: %w", err)
		}
		defer release(rules)
		read := make([]Rule, len(rules))
		for i, r := range rules {
			read[i] = r.Rule
			// plan knows MixMusic's own rule by RuleName: here, by name.
			read[i].Name = ""
			if r.Name == name {
				read[i].Name = RuleName
			}
		}
		changes, add := plan(read, program)
		for _, c := range changes {
			o, what := rules[c.i].o, rules[c.i].Name
			if err := o.call(slotRulePutProfiles, uintptr(c.profiles)); err != nil {
				return fmt.Errorf("changing the rule %q: %w", what, err)
			}
			on := uintptr(0)
			if c.enabled {
				on = variantTrue
			}
			if err := o.call(slotRulePutEnabled, on); err != nil {
				return fmt.Errorf("changing the rule %q: %w", what, err)
			}
		}
		if add {
			return addRule(fw, Rule{Name: name, Program: program, Inbound: true, Enabled: true, Allow: true, Profiles: trusted})
		}
		return nil
	})
}

// addRule adds rule to the firewall fw: inbound, of any protocol, with
// MixMusic's description.
func addRule(fw *object, rule Rule) error {
	r, err := create(&clsidNetFwRule, &iidINetFwRule)
	if err != nil {
		return fmt.Errorf("a new rule: %w", err)
	}
	defer r.release()
	action, on := uintptr(0), uintptr(0)
	if rule.Allow {
		action = actionAllow
	}
	if rule.Enabled {
		on = variantTrue
	}
	for _, step := range []func() error{
		func() error { return r.putBSTR(slotRulePutName, rule.Name) },
		func() error { return r.putBSTR(slotRulePutDescription, ruleAbout) },
		func() error { return r.putBSTR(slotRulePutApplication, rule.Program) },
		func() error { return r.call(slotRulePutDirection, ruleDirIn) },
		func() error { return r.call(slotRulePutAction, action) },
		func() error { return r.call(slotRulePutProfiles, uintptr(rule.Profiles)) },
		func() error { return r.call(slotRulePutEnabled, on) },
	} {
		if err := step(); err != nil {
			return fmt.Errorf("a new rule: %w", err)
		}
	}
	var rules *object
	if err := fw.call(slotRules, uintptr(unsafe.Pointer(&rules))); err != nil {
		return fmt.Errorf("the rules: %w", err)
	}
	defer rules.release()
	if err := rules.call(slotRulesAdd, uintptr(unsafe.Pointer(r))); err != nil {
		return fmt.Errorf("adding the rule: %w", err)
	}
	return nil
}

// shellExecuteInfo is SHELLEXECUTEINFOW, whose fields Go lays out at the
// offsets of C's on 64-bit Windows: 112 bytes.
type shellExecuteInfo struct {
	size     uint32
	mask     uint32
	hwnd     uintptr
	verb     *uint16
	file     *uint16
	params   *uint16
	dir      *uint16
	show     int32
	instApp  uintptr
	idList   uintptr
	class    *uint16
	keyClass uintptr
	hotKey   uint32
	icon     uintptr
	process  syscall.Handle
}

const (
	seeMaskNoCloseProcess = 0x40
	seeMaskNoAsync        = 0x100
	seeMaskFlagNoUI       = 0x400
	swHide                = 0
	errorCancelled        = 1223
)

// Allow lets program in on private and domain networks with an
// administrator's approval: Windows asks for it, and the copy of program
// started as administrator with AllowFlag changes the rules (AllowHere).
// It waits for the copy, and is ErrDeclined when the approval wasn't
// given.
func Allow(program string) error { return run(program, "runas") }

// run starts program with AllowFlag, by verb ("runas" asks for an
// administrator's approval), and waits for it to end.
func run(program, verb string) error {
	file, err := syscall.UTF16PtrFromString(program)
	if err != nil {
		return err
	}
	v, _ := syscall.UTF16PtrFromString(verb)
	params, _ := syscall.UTF16PtrFromString(AllowFlag)
	// The window in front, MixMusic's whose button was clicked, owns
	// Windows' question, so that it shows in front of it.
	owner, _, _ := procGetForegroundWindow.Call()
	info := shellExecuteInfo{
		mask:   seeMaskNoCloseProcess | seeMaskNoAsync | seeMaskFlagNoUI,
		hwnd:   owner,
		verb:   v,
		file:   file,
		params: params,
		show:   swHide,
	}
	info.size = uint32(unsafe.Sizeof(info))
	ok, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		if errno, isErrno := callErr.(syscall.Errno); isErrno && errno == errorCancelled {
			return ErrDeclined
		}
		return fmt.Errorf("starting MixMusic as administrator: %w", callErr)
	}
	if info.process == 0 {
		return errors.New("starting MixMusic as administrator: no process")
	}
	defer syscall.CloseHandle(info.process)
	if _, err := syscall.WaitForSingleObject(info.process, syscall.INFINITE); err != nil {
		return err
	}
	var code uint32
	if err := syscall.GetExitCodeProcess(info.process, &code); err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("Windows Firewall wasn't changed (code %d)", code)
	}
	return nil
}

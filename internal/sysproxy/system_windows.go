//go:build windows

package sysproxy

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

func init() {
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		t.Proxy = Proxy
	}
}

var (
	kernel32                    = syscall.NewLazyDLL("kernel32.dll")
	procGetSystemDirectoryW     = kernel32.NewProc("GetSystemDirectoryW")
	procGlobalFree              = kernel32.NewProc("GlobalFree")
	procCreateEventW            = kernel32.NewProc("CreateEventW")
	advapi32                    = syscall.NewLazyDLL("advapi32.dll")
	procRegNotifyChangeKeyValue = advapi32.NewProc("RegNotifyChangeKeyValue")

	// winhttp.dll is loaded from System32 by its full path: the syscall
	// package looks for a library it doesn't load itself in MixMusic's
	// folder too.
	winhttp                                   = syscall.NewLazyDLL(filepath.Join(systemDirectory(), "winhttp.dll"))
	procWinHttpGetIEProxyConfigForCurrentUser = winhttp.NewProc("WinHttpGetIEProxyConfigForCurrentUser")
	procWinHttpOpen                           = winhttp.NewProc("WinHttpOpen")
	procWinHttpSetTimeouts                    = winhttp.NewProc("WinHttpSetTimeouts")
	procWinHttpGetProxyForUrl                 = winhttp.NewProc("WinHttpGetProxyForUrl")
)

// systemDirectory is System32's path.
func systemDirectory() string {
	buf := make([]uint16, syscall.MAX_PATH)
	n, _, _ := procGetSystemDirectoryW.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 || int(n) > len(buf) {
		return `C:\Windows\System32`
	}
	return syscall.UTF16ToString(buf[:n])
}

// ieProxyConfig is WINHTTP_CURRENT_USER_IE_PROXY_CONFIG.
type ieProxyConfig struct {
	autoDetect    int32
	autoConfigURL *uint16
	proxy         *uint16
	proxyBypass   *uint16
}

// autoProxyOptions is WINHTTP_AUTOPROXY_OPTIONS.
type autoProxyOptions struct {
	flags                 uint32
	autoDetectFlags       uint32
	autoConfigURL         *uint16
	reserved              uintptr
	reserved2             uint32
	autoLogonIfChallenged int32
}

// proxyInfo is WINHTTP_PROXY_INFO.
type proxyInfo struct {
	accessType  uint32
	proxy       *uint16
	proxyBypass *uint16
}

const (
	winhttpAccessTypeNoProxy   = 1
	winhttpAutoproxyAutoDetect = 0x1
	winhttpAutoproxyConfigURL  = 0x2
	winhttpAutoDetectTypeDHCP  = 0x1
	winhttpAutoDetectTypeDNSA  = 0x2

	errWinhttpLoginFailure           = syscall.Errno(12015)
	errWinhttpBadAutoProxyScript     = syscall.Errno(12166)
	errWinhttpUnableToDownloadScript = syscall.Errno(12167)
	errWinhttpAutodetectionFailed    = syscall.Errno(12180)

	// regNotify is what the watch hears of: values set, keys added or
	// removed, and REG_NOTIFY_THREAD_AGNOSTIC.
	regNotify = 0x1 | 0x4 | 0x10000000
)

// setupFailures are the failures of automatic setup as a whole, rather
// than of one address, in words.
var setupFailures = map[syscall.Errno]string{
	errWinhttpAutodetectionFailed:    "automatic detection found no setup",
	errWinhttpUnableToDownloadScript: "the setup script didn't download",
	errWinhttpBadAutoProxyScript:     "the setup script failed",
}

// install reads the proxy of Windows' settings, then follows it.
func install() {
	s, err := readSettings()
	if err != nil {
		logf("mixmusic: reading the proxy settings: %v", err)
	}
	use(s)
	go watch()
}

// use picks proxies by s from now on, and says so when they changed.
func use(s Settings) {
	if old := current.Load(); old != nil && old.settings == s {
		return
	}
	p := NewPicker(s)
	p.auto = func(target string) (string, string, bool, error) { return askWinHTTP(s, target) }
	current.Store(p)
	logf("mixmusic: the proxy settings: %s", s)
}

// readSettings reads the proxy setup of Windows' settings for the user.
func readSettings() (Settings, error) {
	var c ieProxyConfig
	if ok, _, err := procWinHttpGetIEProxyConfigForCurrentUser.Call(uintptr(unsafe.Pointer(&c))); ok == 0 {
		return Settings{}, err
	}
	return Settings{
		AutoDetect: c.autoDetect != 0,
		Script:     takeString(c.autoConfigURL),
		Server:     takeString(c.proxy),
		Bypass:     takeString(c.proxyBypass),
	}, nil
}

// takeString reads a string WinHTTP gave, and frees it.
func takeString(p *uint16) string {
	if p == nil {
		return ""
	}
	defer procGlobalFree.Call(uintptr(unsafe.Pointer(p)))
	n := 0
	for ptr := unsafe.Pointer(p); *(*uint16)(ptr) != 0; n++ {
		ptr = unsafe.Add(ptr, 2)
	}
	return syscall.UTF16ToString(unsafe.Slice(p, n))
}

// session is the WinHTTP session automatic setup asks with, opened once.
var session = sync.OnceValues(func() (uintptr, error) {
	agent, _ := syscall.UTF16PtrFromString("MixMusic")
	h, _, err := procWinHttpOpen.Call(uintptr(unsafe.Pointer(agent)), winhttpAccessTypeNoProxy, 0, 0, 0)
	if h == 0 {
		return 0, err
	}
	// Bound the download of a setup script, in milliseconds.
	procWinHttpSetTimeouts.Call(h, 5000, 5000, 5000, 5000)
	return h, nil
})

// askWinHTTP asks WinHTTP for the proxy of target, by the automatic setup
// of s: detection, then the setup script, as Windows tries them.
func askWinHTTP(s Settings, target string) (proxy, bypass string, global bool, err error) {
	h, err := session()
	if err != nil {
		return "", "", true, err
	}
	var opts autoProxyOptions
	if s.AutoDetect {
		opts.flags |= winhttpAutoproxyAutoDetect
		opts.autoDetectFlags = winhttpAutoDetectTypeDHCP | winhttpAutoDetectTypeDNSA
	}
	if s.Script != "" {
		opts.flags |= winhttpAutoproxyConfigURL
		if opts.autoConfigURL, err = syscall.UTF16PtrFromString(s.Script); err != nil {
			return "", "", true, err
		}
	}
	u, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return "", "", false, err
	}
	var info proxyInfo
	ok, _, err := procWinHttpGetProxyForUrl.Call(h, uintptr(unsafe.Pointer(u)), uintptr(unsafe.Pointer(&opts)), uintptr(unsafe.Pointer(&info)))
	if ok == 0 && errors.Is(err, errWinhttpLoginFailure) {
		// The script's server asks who the user is: Windows' own sign-in
		// answers.
		opts.autoLogonIfChallenged = 1
		ok, _, err = procWinHttpGetProxyForUrl.Call(h, uintptr(unsafe.Pointer(u)), uintptr(unsafe.Pointer(&opts)), uintptr(unsafe.Pointer(&info)))
	}
	runtime.KeepAlive(opts.autoConfigURL)
	runtime.KeepAlive(u)
	if ok == 0 {
		var errno syscall.Errno
		if errors.As(err, &errno) {
			if why, found := setupFailures[errno]; found {
				return "", "", true, fmt.Errorf("%s (%w)", why, err)
			}
		}
		return "", "", false, err
	}
	proxy, bypass = takeString(info.proxy), takeString(info.proxyBypass)
	if info.accessType == winhttpAccessTypeNoProxy {
		proxy = ""
	}
	return proxy, bypass, false, nil
}

// watch reads the settings again each time they change: proxy and VPN
// apps turn the proxy on and off while MixMusic runs.
func watch() {
	// The change is told to the thread that asked, on Windows before 8:
	// this goroutine keeps its thread, and never ends.
	runtime.LockOSThread()
	path, _ := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Internet Settings`)
	var key syscall.Handle
	if err := syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, path, 0, syscall.KEY_NOTIFY, &key); err != nil {
		logf("mixmusic: following the proxy settings: %v", err)
		return
	}
	event, _, err := procCreateEventW.Call(0, 0, 0, 0)
	if event == 0 {
		logf("mixmusic: following the proxy settings: %v", err)
		return
	}
	for {
		// Listen first, then read: a change while reading is heard.
		if r, _, _ := procRegNotifyChangeKeyValue.Call(uintptr(key), 1, regNotify, event, 1); r != 0 {
			logf("mixmusic: following the proxy settings: %v", syscall.Errno(r))
			return
		}
		if s, err := readSettings(); err == nil {
			use(s)
		}
		syscall.WaitForSingleObject(syscall.Handle(event), syscall.INFINITE)
		// An app writes the settings a value at a time.
		time.Sleep(500 * time.Millisecond)
	}
}

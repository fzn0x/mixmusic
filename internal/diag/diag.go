// Package diag keeps what helps tell what went wrong: a log file of what
// MixMusic reports, without the secrets it may hold, and the report Go
// writes when MixMusic crashes, which the next start offers to send.
package diag

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	// logName is the log file; older ones are logName with .1, .2… before
	// the extension, the newest first.
	logName = "mixmusic.log"
	// maxLogSize is the size at which the log starts a new file.
	maxLogSize = 1 << 20
	// keepLogs is how many older log files are kept.
	keepLogs = 3
	// crashName is the file Go writes a crash to; a start finding it not
	// empty keeps it as crash-<time>.log.
	crashName = "crash.log"
	// keepCrashes is how many crash reports are kept.
	keepCrashes = 5
)

// Log is the log file. It is safe for concurrent use.
type Log struct {
	dir  string
	mu   sync.Mutex
	f    *os.File
	size int64
}

// Start sends the log package's output to the log file in dir too, with a
// first line naming the version, and keeps a crash report there. It
// returns the log, and the crash report of the run before when it crashed.
func Start(dir, version string) (*Log, string, error) {
	l, err := Open(dir)
	if err != nil {
		return nil, "", err
	}
	fmt.Fprintf(l, "MixMusic %s on %s/%s, started %s\n", version, runtime.GOOS, runtime.GOARCH, time.Now().Format(time.RFC3339))
	log.SetOutput(io.MultiWriter(l, os.Stderr))
	crash, err := keepCrash(dir)
	if err != nil {
		log.Printf("mixmusic: keeping crash reports: %v", err)
	}
	return l, crash, nil
}

// Open opens the log file in dir, starting a new one when it is large.
func Open(dir string) (*Log, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	l := &Log{dir: dir}
	if info, err := os.Stat(l.path(0)); err == nil && info.Size() >= maxLogSize {
		l.rotate()
	}
	if err := l.open(); err != nil {
		return nil, err
	}
	return l, nil
}

// Dir is the folder of the logs.
func (l *Log) Dir() string { return l.dir }

func (l *Log) path(n int) string {
	if n == 0 {
		return filepath.Join(l.dir, logName)
	}
	ext := filepath.Ext(logName)
	return filepath.Join(l.dir, fmt.Sprintf("%s.%d%s", strings.TrimSuffix(logName, ext), n, ext))
}

func (l *Log) open() error {
	f, err := os.OpenFile(l.path(0), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	l.f, l.size = f, info.Size()
	return nil
}

// rotate moves the log files one place down, dropping the oldest.
func (l *Log) rotate() {
	os.Remove(l.path(keepLogs))
	for n := keepLogs - 1; n >= 0; n-- {
		os.Rename(l.path(n), l.path(n+1))
	}
}

// Write writes p to the log without the secrets it may hold, starting a new
// file when the log is large.
func (l *Log) Write(p []byte) (int, error) {
	clean := Redact(string(p))
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return len(p), nil
	}
	if l.size+int64(len(clean)) > maxLogSize {
		l.f.Close()
		l.rotate()
		if err := l.open(); err != nil {
			l.f = nil
			return len(p), err
		}
	}
	n, err := l.f.WriteString(clean)
	l.size += int64(n)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close closes the log file.
func (l *Log) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return nil
	}
	err := l.f.Close()
	l.f = nil
	return err
}

// secrets match what a log line must not keep: the cookies and hashes of a
// YouTube session, and the keys and tokens of an API, each with the part
// before its value kept.
var secrets = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b((?:__Secure-[0-9A-Za-z_-]+|SAPISID|APISID|SSID|HSID|SID|SIDCC|LOGIN_INFO)=)[^;\s"']+`),
	regexp.MustCompile(`(?i)\b((?:SAPISID|SAPISID1P|SAPISID3P)HASH )[0-9]+_[0-9a-f]+`),
	regexp.MustCompile(`(?i)\b((?:api_key|api_sig|sk|token|access_token|refresh_token|client_secret|secret|code)=)[^&\s"']+`),
	regexp.MustCompile(`(?i)("(?:access_token|refresh_token|session_key|secret|key)"\s*:\s*")[^"]+`),
	regexp.MustCompile(`(?i)\b(Bearer )[0-9A-Za-z._~+/-]+=*`),
}

// localAddr matches the computer's own address and port, which the error
// of a socket gives before the address it was connected to, as in "read
// tcp 192.168.1.5:50123->142.250.74.118:443".
var localAddr = regexp.MustCompile(`(?:\[[^\]\s]+\]|\b\d{1,3}(?:\.\d{1,3}){3}):\d{1,5}->`)

// Redact returns s with the values of secrets replaced by "…", and the
// computer's own address in socket errors too.
func Redact(s string) string {
	for _, re := range secrets {
		s = re.ReplaceAllString(s, "${1}…")
	}
	return localAddr.ReplaceAllString(s, "…->")
}

// keepCrash keeps the report of a crash of the run before, if it wrote one,
// and has Go write the report of a crash of this run to crash.log. It
// returns the path of the report it kept.
func keepCrash(dir string) (string, error) {
	path := filepath.Join(dir, crashName)
	kept := ""
	if info, err := os.Stat(path); err == nil && info.Size() > 0 {
		kept = filepath.Join(dir, "crash-"+info.ModTime().Format("20060102-150405")+".log")
		if err := os.Rename(path, kept); err != nil {
			kept = ""
		}
	}
	pruneCrashes(dir)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return kept, err
	}
	defer f.Close() // SetCrashOutput keeps a duplicate of the file
	if err := debug.SetCrashOutput(f, debug.CrashOptions{}); err != nil {
		return kept, err
	}
	return kept, nil
}

// pruneCrashes removes all but the newest crash reports.
func pruneCrashes(dir string) {
	reports, _ := filepath.Glob(filepath.Join(dir, "crash-*.log"))
	slices.Sort(reports) // their names sort by time
	for len(reports) > keepCrashes {
		os.Remove(reports[0])
		reports = reports[1:]
	}
}

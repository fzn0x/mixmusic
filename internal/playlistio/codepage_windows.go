package playlistio

import "syscall"

var getACP = syscall.NewLazyDLL("kernel32.dll").NewProc("GetACP")

// systemCodePage is the code page Windows gives the text of programs that
// don't use Unicode, which spreadsheets save CSV files in.
var systemCodePage = func() int {
	if getACP.Find() != nil {
		return 1252
	}
	cp, _, _ := getACP.Call()
	return int(cp)
}

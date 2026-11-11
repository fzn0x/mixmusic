package main

// The copy of MixMusic that Settings › Desktop › Windows Firewall's Allow
// starts as administrator (#496): MixMusic --allow-firewall lets the
// MixMusic it is in on private networks, and ends.

import (
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/fzn0x/mixmusic/internal/firewall"
)

// allowFirewall lets this MixMusic in, when args ask for it. It reports
// whether they did, and the status to exit with, having told errOut why
// it couldn't.
func allowFirewall(args []string, errOut io.Writer) (bool, int) {
	if !slices.Contains(args, firewall.AllowFlag) {
		return false, 0
	}
	exe, err := os.Executable()
	if err == nil {
		err = firewall.AllowHere(exe)
	}
	if err != nil {
		fmt.Fprintln(errOut, "mixmusic:", err)
		return true, 1
	}
	return true, 0
}

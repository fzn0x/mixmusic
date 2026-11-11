package main

// The command an AI assistant starts (#477): MixMusic --mcp passes the
// assistant's messages to the MixMusic that runs, and its answers back,
// through internal/mcp, then ends. It opens no window, and doesn't hand
// over to the MixMusic that runs, as a second start does.

import (
	"fmt"
	"io"
	"slices"

	"github.com/fzn0x/mixmusic/internal/mcp"
)

// relayAssistant runs MixMusic as the command an AI assistant starts,
// when args ask for it, to the MixMusic whose data folder dataDir tells,
// over in and out. It reports whether args asked for it, and the status
// to exit with, having told errOut why it couldn't.
func relayAssistant(args []string, dataDir func() (string, error), in io.Reader, out, errOut io.Writer) (bool, int) {
	if !slices.Contains(args, mcp.Flag) {
		return false, 0
	}
	dir, err := dataDir()
	if err == nil {
		err = mcp.Relay(dir, in, out)
	}
	if err != nil {
		fmt.Fprintln(errOut, "mixmusic:", err)
		return true, 1
	}
	return true, 0
}

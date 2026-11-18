//go:build !windows

package playlistio

// systemCodePage is Windows-1252 off Windows: files saved in a code page
// of Windows come most often in that one.
var systemCodePage = func() int { return 1252 }

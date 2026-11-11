package browser

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// hostSystem is this computer.
func hostSystem() system {
	return system{
		goos:   runtime.GOOS,
		getenv: os.Getenv,
		program: func(p string) bool {
			info, err := os.Stat(p)
			return err == nil && info.Mode().IsRegular() && (runtime.GOOS == "windows" || info.Mode()&0o111 != 0)
		},
		head: func(p string) []byte {
			f, err := os.Open(p)
			if err != nil {
				return nil
			}
			defer f.Close()
			b, _ := io.ReadAll(io.LimitReader(f, 512))
			return b
		},
		resolve: func(p string) string {
			if r, err := filepath.EvalSymlinks(p); err == nil {
				return r
			}
			return p
		},
		value:   registryValue,
		subkeys: registrySubkeys,
	}
}

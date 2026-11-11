//go:build !windows

package browser

// The registry is Windows's alone.
func registryValue(key, name string) string { return "" }
func registrySubkeys(key string) []string   { return nil }

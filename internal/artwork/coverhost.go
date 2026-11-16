package artwork

// CoverHost reports whether host serves the services' covers: a room's
// host takes a guest's covers only from those (#734).
func CoverHost(host string) bool { return coverHost(host) }

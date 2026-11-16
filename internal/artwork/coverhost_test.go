package artwork

import "testing"

// TestCoverHost takes the services' hosts of covers, and no other.
func TestCoverHost(t *testing.T) {
	for host, want := range map[string]bool{"i.ytimg.com": true, "i.scdn.co": true, "lh3.googleusercontent.com": true,
		"192.168.1.1": false, "example.com": false, "i.ytimg.com.example.com": false} {
		if got := CoverHost(host); got != want {
			t.Errorf("CoverHost(%q) = %v", host, got)
		}
	}
}

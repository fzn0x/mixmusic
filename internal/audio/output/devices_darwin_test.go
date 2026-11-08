//go:build darwin && (amd64 || arm64)

package output

import "testing"

// TestCFStrings makes CFStrings of names and UIDs, as a queue takes its
// device's UID, and reads them back, as Core Audio's names are read, in
// any script; it opens no device.
func TestCFStrings(t *testing.T) {
	if err := loadCoreFoundation(); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"BuiltInSpeakerDevice", "Kopfhörer — MixMusic 🎧", "スピーカー", ""} {
		ref := cfString(s)
		if ref == 0 {
			t.Fatalf("no CFString of %q", s)
		}
		got := goString(ref)
		cfRelease(ref)
		if got != s {
			t.Errorf("%q read back %q", s, got)
		}
	}
	if goString(0) != "" {
		t.Error("no CFString read something")
	}
}

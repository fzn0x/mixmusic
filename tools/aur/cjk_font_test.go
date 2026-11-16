package main

import (
	"strings"
	"testing"
)

// The app bundles no Chinese, Japanese or Korean letters, so the package
// suggests a font for them (#558).
func TestOptdependsNameACJKFont(t *testing.T) {
	for name, f := range renderAll(t, testRelease) {
		if !strings.Contains(f, "noto-fonts-cjk: Chinese, Japanese and Korean letters") {
			t.Errorf("%s does not suggest noto-fonts-cjk", name)
		}
	}
}

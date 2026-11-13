package locales

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestChineseTranslatesEveryMessage keeps zh-Hans.json in step with
// en.json: a message added in English alone would show in English.
func TestChineseTranslatesEveryMessage(t *testing.T) {
	read := func(name string) map[string]any {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(name, err)
		}
		return m
	}
	en, zh := read("en.json"), read("zh-Hans.json")
	for k := range en {
		if strings.HasPrefix(k, "@") {
			continue
		}
		if _, ok := zh[k]; !ok {
			t.Errorf("zh-Hans.json lacks %s: %q", k, en[k])
		}
	}
	for k := range zh {
		if _, ok := en[k]; !ok && !strings.HasPrefix(k, "@") {
			t.Errorf("zh-Hans.json has %s, which en.json lacks", k)
		}
	}
}

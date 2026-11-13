package netease

import "testing"

func TestWeapiForm(t *testing.T) {
	form, err := weapi([]byte(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(form.Get("encSecKey")) != 256 || form.Get("params") == "" {
		t.Fatalf("form: %v", form)
	}
}

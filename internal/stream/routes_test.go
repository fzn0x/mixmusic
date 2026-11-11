package stream

import (
	"context"
	"testing"
)

// The stream keeps how it was found, for each route.
func TestStreamKeepsHowItWasFound(t *testing.T) {
	const out = `{"url":"https://rr1.example/audio?expire=4102444800","ext":"webm","acodec":"opus","abr":128,"duration":200}`
	run := func(context.Context, ...string) ([]byte, error) { return []byte(out), nil }
	r := NewResolver(nil, "")
	r.Run = run
	ctx := context.Background()

	quick, err := r.quickYTDLP(ctx, "aaaaaaaaaaa", QualityBest)
	if err != nil || quick.Route != RouteQuick || !hasArg(quick.Args, quickArgs) || hasArg(quick.Args, "--cookies") {
		t.Errorf("quick run: route %q args %v err %v", quick.Route, quick.Args, err)
	}
	full, err := r.ytdlp(ctx, "aaaaaaaaaaa", QualityBest)
	if err != nil || full.Route != RouteFull || hasArg(full.Args, quickArgs) {
		t.Errorf("full run: route %q args %v err %v", full.Route, full.Args, err)
	}
	acct, err := r.ytdlpAsAccount(ctx, "aaaaaaaaaaa", QualityBest, nil)
	if err != nil || acct.Route != RouteAccount || !hasArg(acct.Args, "--cookies") {
		t.Errorf("account run: route %q args %v err %v", acct.Route, acct.Args, err)
	}
}

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

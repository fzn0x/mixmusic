package firewall

import (
	"reflect"
	"testing"
)

const exe = `C:\Users\Someone\AppData\Local\Programs\MixMusic\MixMusic.exe`

// on is the policy of a computer on a private network, the firewall on
// for every kind, asking about the programs it blocks.
func on(rules ...Rule) policy {
	return policy{current: ProfilePrivate, of: map[int32]settings{
		ProfileDomain: {on: true}, ProfilePrivate: {on: true}, ProfilePublic: {on: true}}, rules: rules}
}

// in is an inbound rule of MixMusic, on.
func in(name string, allow bool, profiles int32) Rule {
	return Rule{Name: name, Program: exe, Inbound: true, Enabled: true, Allow: allow, Profiles: profiles}
}

// TestJudge tells what the firewall does to MixMusic: off, allowed,
// blocked as by Cancel, not known yet, on a public network, shielded and
// managed; rules of other programs, of other networks, off or outbound
// don't count.
func TestJudge(t *testing.T) {
	quiet := on()
	quiet.of[ProfilePrivate] = settings{on: true, quiet: true}
	shielded := on(in("MixMusic", true, ProfilePrivate))
	shielded.of[ProfilePrivate] = settings{on: true, blockAll: true}
	public := on(in("MixMusic", true, ProfilePrivate))
	public.current = ProfilePublic
	both := on(in("MixMusic", true, ProfilePrivate))
	both.current = ProfilePrivate | ProfilePublic
	off := on()
	off.of[ProfilePrivate] = settings{}
	managed := on()
	managed.managed = true
	other, outbound, disabled := in("Other", true, ProfilePrivate), in("MixMusic", true, ProfilePrivate), in("MixMusic", true, ProfilePrivate)
	other.Program = `C:\Other\other.exe`
	outbound.Inbound = false
	disabled.Enabled = false
	for _, tc := range []struct {
		name             string
		p                policy
		want             State
		letsIn, canAllow bool
	}{
		{"off", off, State{Asks: true}, true, false},
		{"allowed", on(in("MixMusic", true, ProfilePrivate)), State{On: true, Allowed: true, Asks: true}, true, false},
		{"allowed everywhere", on(in("MixMusic", true, 0x7fffffff)), State{On: true, Allowed: true, Asks: true}, true, false},
		{"cancelled", on(in("MixMusic", false, ProfilePrivate), in("MixMusic", false, ProfilePrivate)), State{On: true, Blocked: true, Asks: true}, false, true},
		{"block wins", on(in("MixMusic", true, ProfilePrivate), in("Block", false, ProfilePrivate)), State{On: true, Allowed: true, Blocked: true, Asks: true}, false, true},
		{"not known", on(), State{On: true, Asks: true}, false, true},
		{"quiet", quiet, State{On: true}, false, true},
		{"public", public, State{On: true, Public: true, Asks: true}, false, false},
		{"private and public", both, State{On: true, Allowed: true, Asks: true}, true, false},
		{"shielded", shielded, State{On: true, Allowed: true, Shielded: true, Asks: true}, false, false},
		{"managed", managed, State{On: true, Managed: true, Asks: true}, false, false},
		{"another network's", on(in("MixMusic", true, ProfilePublic)), State{On: true, Asks: true}, false, true},
		{"another program's", on(other), State{On: true, Asks: true}, false, true},
		{"outbound", on(outbound), State{On: true, Asks: true}, false, true},
		{"off rule", on(disabled), State{On: true, Asks: true}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := judge(tc.p, exe)
			if got != tc.want || got.LetsIn() != tc.letsIn || got.CanAllow() != tc.canAllow {
				t.Errorf("judge = %+v (lets in %v, can allow %v), want %+v (%v, %v)", got, got.LetsIn(), got.CanAllow(), tc.want, tc.letsIn, tc.canAllow)
			}
		})
	}
}

// TestPlan lets MixMusic in: Cancel's rules go off, a rule that blocks it
// on public networks too keeps those, MixMusic's own rule comes back on
// and covers private and domain networks, and a rule is added only when
// MixMusic has none. Other programs' rules and outbound ones stay.
func TestPlan(t *testing.T) {
	other, outbound := in("Other", false, ProfilePrivate), in("MixMusic", false, ProfilePrivate)
	other.Program = `C:\Other\other.exe`
	outbound.Inbound = false
	own := in(RuleName, true, ProfilePublic)
	own.Enabled = false
	for _, tc := range []struct {
		name    string
		rules   []Rule
		changes []change
		add     bool
	}{
		{"none", nil, nil, true},
		{"cancelled", []Rule{in("MixMusic", false, ProfilePrivate), in("MixMusic", false, ProfilePrivate)},
			[]change{{0, ProfilePrivate, false}, {1, ProfilePrivate, false}}, true},
		{"blocked everywhere", []Rule{in("MixMusic", false, 0x7fffffff)}, []change{{0, 0x7fffffff &^ trusted, true}}, true},
		{"blocked on public", []Rule{in("MixMusic", false, ProfilePublic)}, nil, true},
		{"own rule", []Rule{own}, []change{{0, ProfilePublic | trusted, true}}, false},
		{"someone else's allow", []Rule{in("Allowed by hand", true, ProfilePublic)}, nil, true},
		{"others", []Rule{other, outbound}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changes, add := plan(tc.rules, exe)
			if !reflect.DeepEqual(changes, tc.changes) || add != tc.add {
				t.Errorf("plan = %+v, add %v; want %+v, %v", changes, add, tc.changes, tc.add)
			}
		})
	}
}

// TestSameProgram matches a rule's program to MixMusic's whatever its
// case and slashes, and with Windows' variables, as %LOCALAPPDATA%.
func TestSameProgram(t *testing.T) {
	t.Setenv("MIXMUSIC_TEST_APPS", `C:\Users\Someone\AppData\Local`)
	for _, tc := range []struct {
		rule string
		same bool
	}{
		{exe, true},
		{`c:\users\someone\appdata\local\programs\mixmusic\mixmusic.exe`, true},
		{`C:/Users/Someone/AppData/Local/Programs/MixMusic/MixMusic.exe`, true},
		{`%MIXMUSIC_TEST_APPS%\Programs\MixMusic\MixMusic.exe`, true},
		{`%MIXMUSIC_TEST_UNSET%\Programs\MixMusic\MixMusic.exe`, false},
		{`C:\Program Files\MixMusic\MixMusic.exe`, false},
		{"", false},
	} {
		if got := SameProgram(tc.rule, exe); got != tc.same {
			t.Errorf("SameProgram(%q) = %v", tc.rule, got)
		}
	}
	if got := expand(`50%% %MIXMUSIC_TEST_UNSET% %MIXMUSIC_TEST_APPS%`); got != `50%% %MIXMUSIC_TEST_UNSET% C:\Users\Someone\AppData\Local` {
		t.Errorf("expand = %q", got)
	}
}

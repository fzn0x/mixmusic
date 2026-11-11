// Package firewall reads whether Windows Defender Firewall lets other
// computers and cast devices on the network reach a program, and lets them
// in on private networks with an administrator's approval (#496).
//
// Remote control and casting need connections from the network to reach
// MixMusic. Windows asks about them once; Cancel, a public network, "Block
// all incoming connections" or an organization's policy keep them out with
// no word of why. Check tells which, without an administrator, through the
// firewall's COM interface. Allow starts a copy of the program as
// administrator, with AllowFlag, which changes the rules (AllowHere) and
// exits: one approval from Windows, which a user who isn't an
// administrator can get too.
package firewall

import (
	"errors"
	"os"
	"strings"
)

// The kinds of network the firewall keeps rules for, as NET_FW_PROFILE2_*
// numbers them: a domain's, a private one, as at home, and a public one.
const (
	ProfileDomain  = 0x1
	ProfilePrivate = 0x2
	ProfilePublic  = 0x4
)

// trusted are the kinds of network Allow lets a program in on. Public
// networks stay closed: a user who trusts one makes it private.
const trusted = ProfileDomain | ProfilePrivate

// AllowFlag is the argument of the copy of MixMusic that Allow starts as
// administrator, which changes the rules (AllowHere) and exits.
const AllowFlag = "--allow-firewall"

// RuleName is the name of the rule Allow adds.
const RuleName = "MixMusic"

// ruleAbout is the description of the rule Allow adds, as the firewall's
// settings show it.
const ruleAbout = "Lets other computers and cast devices on private networks reach MixMusic, for remote control and casting. MixMusic's Settings › Desktop › Windows Firewall added it."

var (
	// ErrUnsupported is the error of a system whose firewall MixMusic
	// doesn't read: all but Windows.
	ErrUnsupported = errors.New("only Windows' firewall is read")
	// ErrDeclined is Allow's error when the administrator's approval
	// wasn't given.
	ErrDeclined = errors.New("the administrator's approval wasn't given")
)

// State is what the firewall does to connections from the network the
// computer is on now to a program.
type State struct {
	// On is set when the firewall is on for the network.
	On bool
	// Public is set when Windows counts the network as public, and the
	// computer is on no private or domain network.
	Public bool
	// Managed is set when an organization's policy decides the firewall's
	// rules, so that the rules of the computer may not count.
	Managed bool
	// Shielded is set when the firewall blocks every incoming connection
	// on the network, those of the programs it allows too.
	Shielded bool
	// Allowed is set when a rule lets the program in on the network, and
	// Blocked when one keeps it out, which wins.
	Allowed, Blocked bool
	// Asks is set when the firewall asks about a program it doesn't know,
	// the first time it listens.
	Asks bool
}

// LetsIn reports whether connections from the network reach the program.
func (s State) LetsIn() bool { return !s.On || (s.Allowed && !s.Blocked && !s.Shielded) }

// CanAllow reports whether Allow would let the program in: the firewall
// keeps it out of a private or domain network, by rules of the computer's
// alone.
func (s State) CanAllow() bool { return !s.LetsIn() && !s.Public && !s.Managed && !s.Shielded }

// Rule is what Check and AllowHere read of a rule of the firewall.
type Rule struct {
	Name string
	// Program is the program the rule is of, as the rule has it, maybe
	// with variables as %ProgramFiles%; "" for every program.
	Program string
	Inbound bool
	Enabled bool
	// Allow is set for a rule that lets connections in, unset for one that
	// blocks them.
	Allow bool
	// Profiles are the kinds of network the rule is for (Profile*).
	Profiles int32
}

// settings are the firewall's settings for a kind of network.
type settings struct {
	on       bool
	blockAll bool
	// quiet is set when the firewall doesn't ask about a program it
	// blocks.
	quiet bool
}

// policy is what Check reads of the firewall.
type policy struct {
	// current are the kinds of the networks the computer is on now.
	current int32
	// of are the settings of each kind of network.
	of map[int32]settings
	// managed is set when an organization's policy decides the rules.
	managed bool
	rules   []Rule
}

// judge tells what the firewall of policy p does to program on the
// networks the computer is on: the private and domain ones, else the
// public one.
func judge(p policy, program string) State {
	s := State{Managed: p.managed, Asks: true}
	var kinds []int32
	for _, k := range []int32{ProfileDomain, ProfilePrivate} {
		if p.current&k != 0 {
			kinds = append(kinds, k)
		}
	}
	if len(kinds) == 0 && p.current&ProfilePublic != 0 {
		kinds, s.Public = []int32{ProfilePublic}, true
	}
	allowed := true
	for _, k := range kinds {
		set := p.of[k]
		if !set.on {
			continue
		}
		s.On = true
		s.Shielded = s.Shielded || set.blockAll
		s.Asks = s.Asks && !set.quiet
		in := false
		for _, r := range p.rules {
			if !r.Inbound || !r.Enabled || r.Profiles&k == 0 || !SameProgram(r.Program, program) {
				continue
			}
			if r.Allow {
				in = true
			} else {
				s.Blocked = true
			}
		}
		allowed = allowed && in
	}
	s.Allowed = s.On && allowed
	return s
}

// change is what AllowHere changes of a rule, the one at index i of the
// rules read: the kinds of network it's for, and whether it's on.
type change struct {
	i        int
	profiles int32
	enabled  bool
}

// plan returns what AllowHere changes of rules to let program in on
// private and domain networks:
//   - a rule that blocks it there no longer covers them, and goes off when
//     it covered nothing else, as the ones Cancel adds;
//   - a rule it added before (RuleName) is on again, and covers them.
//
// add is set when no such rule is left to change, and one is added.
func plan(rules []Rule, program string) (changes []change, add bool) {
	add = true
	for i, r := range rules {
		if !r.Inbound || !SameProgram(r.Program, program) {
			continue
		}
		switch {
		case !r.Allow && r.Enabled && r.Profiles&trusted != 0:
			if left := r.Profiles &^ trusted; left != 0 {
				changes = append(changes, change{i: i, profiles: left, enabled: true})
			} else {
				changes = append(changes, change{i: i, profiles: r.Profiles, enabled: false})
			}
		case r.Allow && r.Name == RuleName:
			changes = append(changes, change{i: i, profiles: r.Profiles | trusted, enabled: true})
			add = false
		}
	}
	return changes, add
}

// SameProgram reports whether the program of a rule, which may hold
// variables as %ProgramFiles%, is program: Windows' paths, whatever their
// case and slashes.
func SameProgram(rule, program string) bool {
	if rule == "" || program == "" {
		return false
	}
	norm := func(p string) string {
		return strings.TrimRight(strings.ReplaceAll(strings.TrimSpace(expand(p)), "/", `\`), `\`)
	}
	return strings.EqualFold(norm(rule), norm(program))
}

// expand replaces the variables of p written as Windows writes them,
// %NAME%, with their values; a variable not set stays as it is.
func expand(p string) string {
	var b strings.Builder
	for {
		i := strings.IndexByte(p, '%')
		if i < 0 {
			break
		}
		j := strings.IndexByte(p[i+1:], '%')
		if j < 0 {
			break
		}
		name := p[i+1 : i+1+j]
		if v, ok := os.LookupEnv(name); ok && name != "" {
			b.WriteString(p[:i])
			b.WriteString(v)
		} else {
			b.WriteString(p[:i+2+j])
		}
		p = p[i+2+j:]
	}
	b.WriteString(p)
	return b.String()
}
